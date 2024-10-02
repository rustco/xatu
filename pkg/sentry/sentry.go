package sentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	//nolint:gosec // only exposed if pprofAddr config is set
	_ "net/http/pprof"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/spec/altair"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/beevik/ntp"
	"github.com/ethpandaops/xatu/pkg/networks"
	"github.com/ethpandaops/xatu/pkg/observability"
	"github.com/ethpandaops/xatu/pkg/output"
	oxatu "github.com/ethpandaops/xatu/pkg/output/xatu"
	xatuethv1 "github.com/ethpandaops/xatu/pkg/proto/eth/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/ethpandaops/xatu/pkg/sentry/cache"
	"github.com/ethpandaops/xatu/pkg/sentry/ethereum"
	v1 "github.com/ethpandaops/xatu/pkg/sentry/event/beacon/eth/v1"
	v2 "github.com/ethpandaops/xatu/pkg/sentry/event/beacon/eth/v2"
	"github.com/go-co-op/gocron"
	"github.com/google/uuid"
	perrors "github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/sdk/trace"
	"gopkg.in/yaml.v2"
)

type Sentry struct {
	Config *Config

	sinks []output.Sink

	beacon *ethereum.BeaconNode

	clockDrift time.Duration

	log logrus.FieldLogger

	duplicateCache *cache.DuplicateCache

	id uuid.UUID

	metrics *Metrics

	scheduler *gocron.Scheduler

	latestForkChoice   *v1.ForkChoice
	latestForkChoiceMu sync.RWMutex

	preset *Preset

	shutdownFuncs []func(context.Context) error
}

func New(ctx context.Context, log logrus.FieldLogger, config *Config, overrides *Override) (*Sentry, error) {
	log = log.WithField("module", "sentry")

	if config == nil {
		return nil, errors.New("config is required")
	}

	var preset *Preset

	// Merge preset if its set
	if overrides.Preset.Enabled || config.Preset != "" {
		var presetName string

		if overrides.Preset.Enabled {
			presetName = overrides.Preset.Value
		} else {
			presetName = config.Preset
		}

		p, err := GetPreset(presetName)
		if err != nil {
			return nil, fmt.Errorf("failed to get preset: %w", err)
		}

		preset = p

		log.WithField("preset", presetName).Info("Applying preset")

		// Merge the 2 yaml files
		if err := yaml.Unmarshal(preset.Value, config); err != nil {
			return nil, fmt.Errorf("failed to merge config and preset: %w", err)
		}
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	sinks, err := config.CreateSinks(log)
	if err != nil {
		return nil, err
	}

	// If the beacon node override is set, use it
	if overrides.BeaconNodeURL.Enabled {
		log.Info("Overriding beacon node URL")

		config.Ethereum.BeaconNodeAddress = overrides.BeaconNodeURL.Value
	}

	beacon, err := ethereum.NewBeaconNode(ctx, config.Name, &config.Ethereum, log)
	if err != nil {
		return nil, err
	}

	duplicateCache := cache.NewDuplicateCache()
	duplicateCache.Start()

	s := &Sentry{
		Config:             config,
		sinks:              sinks,
		beacon:             beacon,
		clockDrift:         time.Duration(0),
		log:                log,
		duplicateCache:     duplicateCache,
		id:                 uuid.New(),
		metrics:            NewMetrics("xatu_sentry"),
		scheduler:          gocron.NewScheduler(time.Local),
		latestForkChoice:   nil,
		latestForkChoiceMu: sync.RWMutex{},
		shutdownFuncs:      []func(context.Context) error{},
		preset:             preset,
	}

	// If the output authorization override is set, use it
	if overrides.XatuOutputAuth.Enabled {
		log.Info("Overriding output authorization")

		for _, sink := range s.sinks {
			if sink.Type() == string(output.SinkTypeXatu) {
				xatuSink, ok := sink.(*oxatu.Xatu)
				if !ok {
					return nil, errors.New("failed to assert xatu sink")
				}

				xatuSink.SetAuthorization(overrides.XatuOutputAuth.Value)
			}
		}
	}

	return s, nil
}

//nolint:gocyclo // Needs refactoring
func (s *Sentry) Start(ctx context.Context) error {
	if err := s.ServeMetrics(ctx); err != nil {
		return err
	}

	if s.Config.PProfAddr != nil {
		if err := s.ServePProf(ctx); err != nil {
			return err
		}
	}

	s.log.
		WithField("version", xatu.Full()).
		WithField("id", s.id.String()).
		Info("Starting Xatu in sentry mode")

	// Start tracing if enabled
	if s.Config.Tracing.Enabled {
		s.log.Info("Tracing enabled")

		res, err := observability.NewResource(xatu.WithModule(xatu.ModuleName_SENTRY), xatu.Short())
		if err != nil {
			return perrors.Wrap(err, "failed to create tracing resource")
		}

		opts := []trace.TracerProviderOption{
			trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(s.Config.Tracing.Sampling.Rate))),
		}

		tracer, err := observability.NewHTTPTraceProvider(ctx,
			res,
			s.Config.Tracing.AsOTelOpts(),
			opts...,
		)
		if err != nil {
			return perrors.Wrap(err, "failed to create tracing provider")
		}

		shutdown, err := observability.SetupOTelSDK(ctx, tracer)
		if err != nil {
			return perrors.Wrap(err, "failed to setup tracing SDK")
		}

		s.shutdownFuncs = append(s.shutdownFuncs, shutdown)
	}

	if err := s.startBeaconCommitteesWatcher(ctx); err != nil {
		return err
	}

	if err := s.startProposerDutyWatcher(ctx); err != nil {
		return err
	}

	s.beacon.OnReady(ctx, func(ctx context.Context) error {
		s.log.Info("Internal beacon node is ready, subscribing to events")

		if s.beacon.Metadata().Network.Name == networks.NetworkNameUnknown {
			s.log.Fatal("Unable to determine Ethereum network. Provide an override network name via ethereum.overrideNetworkName")
		}

		s.beacon.Node().OnAttestation(ctx, func(ctx context.Context, attestation *phase0.Attestation) error {
			now := time.Now().Add(s.clockDrift)

			meta, err := s.createNewClientMeta(ctx)
			if err != nil {
				return err
			}

			event := v1.NewEventsAttestation(s.log, attestation, now, s.beacon, s.duplicateCache.BeaconETHV1EventsAttestation, meta)

			ignore, err := event.ShouldIgnore(ctx)
			if err != nil {
				return err
			}

			if ignore {
				return nil
			}

			decoratedEvent, err := event.Decorate(ctx)
			if err != nil {
				return err
			}

			return s.handleNewDecoratedEvent(ctx, decoratedEvent)
		})

		s.beacon.Node().OnBlock(ctx, func(ctx context.Context, block *eth2v1.BlockEvent) error {
			now := time.Now().Add(s.clockDrift)

			meta, err := s.createNewClientMeta(ctx)
			if err != nil {
				return err
			}

			event := v1.NewEventsBlock(s.log, block, now, s.beacon, s.duplicateCache.BeaconETHV1EventsBlock, meta)

			ignore, err := event.ShouldIgnore(ctx)
			if err != nil {
				return err
			}

			if ignore {
				return nil
			}

			decoratedEvent, err := event.Decorate(ctx)
			if err != nil {
				return err
			}

			err = s.handleNewDecoratedEvent(ctx, decoratedEvent)
			if err != nil {
				return err
			}

			go func() {
				// some clients require a small delay before being able to fetch the block
				time.Sleep(1 * time.Second)

				blockRoot := xatuethv1.RootAsString(block.Block)

				beaconBlock, err := s.beacon.Node().FetchBlock(ctx, blockRoot)
				if err != nil {
					s.log.WithError(err).Error("Failed to fetch block")
				} else {
					beaconBlockMeta, err := s.createNewClientMeta(ctx)
					if err != nil {
						s.log.WithError(err).Error("Failed to create client meta")

						return
					}

					event := v2.NewBeaconBlock(s.log, blockRoot, beaconBlock, now, s.beacon, s.duplicateCache.BeaconETHV2BeaconBlock, beaconBlockMeta)

					ignore, err := event.ShouldIgnore(ctx)
					if err != nil {
						s.log.
							WithError(err).
							WithField("event", xatu.Event_BEACON_API_ETH_V2_BEACON_BLOCK).
							Error("Failed to check if event should be ignored")

						return
					}

					if ignore {
						return
					}

					decoratedEvent, err := event.Decorate(ctx)
					if err != nil {
						s.log.WithError(err).Error("Failed to decorate event")

						return
					}

					if err := s.handleNewDecoratedEvent(ctx, decoratedEvent); err != nil {
						s.log.WithError(err).Error("Failed to handle new decorated event")

						return
					}
				}
			}()

			return nil
		})

		s.beacon.Node().OnChainReOrg(ctx, func(ctx context.Context, chainReorg *eth2v1.ChainReorgEvent) error {
			now := time.Now().Add(s.clockDrift)

			meta, err := s.createNewClientMeta(ctx)
			if err != nil {
				return err
			}

			event := v1.NewEventsChainReorg(s.log, chainReorg, now, s.beacon, s.duplicateCache.BeaconETHV1EventsChainReorg, meta)

			ignore, err := event.ShouldIgnore(ctx)
			if err != nil {
				return err
			}

			if ignore {
				return nil
			}

			decoratedEvent, err := event.Decorate(ctx)
			if err != nil {
				return err
			}

			return s.handleNewDecoratedEvent(ctx, decoratedEvent)
		})

		s.beacon.Node().OnHead(ctx, func(ctx context.Context, head *eth2v1.HeadEvent) error {
			now := time.Now().Add(s.clockDrift)

			meta, err := s.createNewClientMeta(ctx)
			if err != nil {
				return err
			}

			event := v1.NewEventsHead(s.log, head, now, s.beacon, s.duplicateCache.BeaconETHV1EventsHead, meta)

			ignore, err := event.ShouldIgnore(ctx)
			if err != nil {
				return err
			}

			if ignore {
				return nil
			}

			decoratedEvent, err := event.Decorate(ctx)
			if err != nil {
				return err
			}

			return s.handleNewDecoratedEvent(ctx, decoratedEvent)
		})

		s.beacon.Node().OnVoluntaryExit(ctx, func(ctx context.Context, voluntaryExit *phase0.SignedVoluntaryExit) error {
			now := time.Now().Add(s.clockDrift)

			meta, err := s.createNewClientMeta(ctx)
			if err != nil {
				return err
			}

			event := v1.NewEventsVoluntaryExit(s.log, voluntaryExit, now, s.beacon, s.duplicateCache.BeaconETHV1EventsVoluntaryExit, meta)

			ignore, err := event.ShouldIgnore(ctx)
			if err != nil {
				return err
			}

			if ignore {
				return nil
			}

			decoratedEvent, err := event.Decorate(ctx)
			if err != nil {
				return err
			}

			return s.handleNewDecoratedEvent(ctx, decoratedEvent)
		})

		s.beacon.Node().OnContributionAndProof(ctx, func(ctx context.Context, contributionAndProof *altair.SignedContributionAndProof) error {
			now := time.Now().Add(s.clockDrift)

			meta, err := s.createNewClientMeta(ctx)
			if err != nil {
				return err
			}

			event := v1.NewEventsContributionAndProof(s.log, contributionAndProof, now, s.beacon, s.duplicateCache.BeaconETHV1EventsContributionAndProof, meta)

			ignore, err := event.ShouldIgnore(ctx)
			if err != nil {
				return err
			}

			if ignore {
				return nil
			}

			decoratedEvent, err := event.Decorate(ctx)
			if err != nil {
				return err
			}

			return s.handleNewDecoratedEvent(ctx, decoratedEvent)
		})

		s.beacon.Node().OnFinalizedCheckpoint(ctx, func(ctx context.Context, finalizedCheckpoint *eth2v1.FinalizedCheckpointEvent) error {
			now := time.Now().Add(s.clockDrift)

			meta, err := s.createNewClientMeta(ctx)
			if err != nil {
				return err
			}

			event := v1.NewEventsFinalizedCheckpoint(s.log, finalizedCheckpoint, now, s.beacon, s.duplicateCache.BeaconETHV1EventsFinalizedCheckpoint, meta)

			ignore, err := event.ShouldIgnore(ctx)
			if err != nil {
				return err
			}

			if ignore {
				return nil
			}

			decoratedEvent, err := event.Decorate(ctx)
			if err != nil {
				return err
			}

			return s.handleNewDecoratedEvent(ctx, decoratedEvent)
		})

		s.beacon.Node().OnBlobSidecar(ctx, func(ctx context.Context, blobSidecar *eth2v1.BlobSidecarEvent) error {
			now := time.Now().Add(s.clockDrift)

			meta, err := s.createNewClientMeta(ctx)
			if err != nil {
				return err
			}

			event := v1.NewEventsBlobSidecar(s.log, blobSidecar, now, s.beacon, s.duplicateCache.BeaconEthV1EventsBlobSidecar, meta)

			ignore, err := event.ShouldIgnore(ctx)
			if err != nil {
				return err
			}

			if ignore {
				return nil
			}

			decoratedEvent, err := event.Decorate(ctx)
			if err != nil {
				return err
			}

			return s.handleNewDecoratedEvent(ctx, decoratedEvent)
		})

		if err := s.startForkChoiceSchedule(ctx); err != nil {
			return err
		}

		if err := s.startAttestationDataSchedule(ctx); err != nil {
			return err
		}

		return nil
	})

	if err := s.startCrons(ctx); err != nil {
		s.log.WithError(err).Fatal("Failed to start crons")
	}

	for _, sink := range s.sinks {
		s.log.WithField("type", sink.Type()).WithField("name", sink.Name()).Info("Starting sink")

		if err := sink.Start(ctx); err != nil {
			return err
		}
	}

	if s.Config.Ethereum.OverrideNetworkName != "" {
		s.log.WithField("network", s.Config.Ethereum.OverrideNetworkName).Info("Overriding network name")
	}

	if err := s.beacon.Start(ctx); err != nil {
		return err
	}

	cancel := make(chan os.Signal, 1)
	signal.Notify(cancel, syscall.SIGTERM, syscall.SIGINT)

	sig := <-cancel
	s.log.Printf("Caught signal: %v", sig)

	s.log.Printf("Flushing sinks")

	for _, sink := range s.sinks {
		if err := sink.Stop(ctx); err != nil {
			return err
		}
	}

	for _, f := range s.shutdownFuncs {
		if err := f(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (s *Sentry) ServeMetrics(ctx context.Context) error {
	go func() {
		sm := http.NewServeMux()
		sm.Handle("/metrics", promhttp.Handler())

		server := &http.Server{
			Addr:              s.Config.MetricsAddr,
			ReadHeaderTimeout: 15 * time.Second,
			Handler:           sm,
		}

		s.log.Infof("Serving metrics at %s", s.Config.MetricsAddr)

		if err := server.ListenAndServe(); err != nil {
			s.log.Fatal(err)
		}
	}()

	return nil
}

func (s *Sentry) ServePProf(ctx context.Context) error {
	pprofServer := &http.Server{
		Addr:              *s.Config.PProfAddr,
		ReadHeaderTimeout: 120 * time.Second,
	}

	go func() {
		s.log.Infof("Serving pprof at %s", *s.Config.PProfAddr)

		if err := pprofServer.ListenAndServe(); err != nil {
			s.log.Fatal(err)
		}
	}()

	return nil
}

func (s *Sentry) createNewClientMeta(ctx context.Context) (*xatu.ClientMeta, error) {
	var networkMeta *xatu.ClientMeta_Ethereum_Network

	network := s.beacon.Metadata().Network
	if network != nil {
		networkMeta = &xatu.ClientMeta_Ethereum_Network{
			Name: string(network.Name),
			Id:   network.ID,
		}

		if s.Config.Ethereum.OverrideNetworkName != "" {
			networkMeta.Name = s.Config.Ethereum.OverrideNetworkName
		}
	}

	clientName := s.Config.Name
	if clientName == "" {
		hashed, err := s.beacon.Metadata().NodeIDHash()
		if err != nil {
			return nil, err
		}

		clientName = hashed
	}

	presetName := ""
	if s.preset != nil {
		presetName = s.preset.Name
	}

	return &xatu.ClientMeta{
		Name:           clientName,
		Version:        xatu.Short(),
		Id:             s.id.String(),
		Implementation: xatu.Implementation,
		ModuleName:     xatu.ModuleName_SENTRY,
		Os:             runtime.GOOS,
		ClockDrift:     uint64(s.clockDrift.Milliseconds()),
		Ethereum: &xatu.ClientMeta_Ethereum{
			Network:   networkMeta,
			Execution: &xatu.ClientMeta_Ethereum_Execution{},
			Consensus: &xatu.ClientMeta_Ethereum_Consensus{
				Implementation: s.beacon.Metadata().Client(ctx),
				Version:        s.beacon.Metadata().NodeVersion(ctx),
			},
		},
		Labels:     s.Config.Labels,
		PresetName: presetName,
	}, nil
}

func (s *Sentry) startCrons(ctx context.Context) error {
	if _, err := s.scheduler.Every("5m").Do(func() {
		if err := s.syncClockDrift(ctx); err != nil {
			s.log.WithError(err).Error("Failed to sync clock drift")
		}
	}); err != nil {
		return err
	}

	s.scheduler.StartAsync()

	return nil
}

func (s *Sentry) syncClockDrift(_ context.Context) error {
	response, err := ntp.Query(s.Config.NTPServer)
	if err != nil {
		return err
	}

	err = response.Validate()
	if err != nil {
		return err
	}

	s.clockDrift = response.ClockOffset
	s.log.WithField("drift", s.clockDrift).Info("Updated clock drift")

	return err
}

func (s *Sentry) handleNewDecoratedEvent(ctx context.Context, event *xatu.DecoratedEvent) error {
	if err := s.beacon.Synced(ctx); err != nil {
		return err
	}

	network := event.GetMeta().GetClient().GetEthereum().GetNetwork().GetId()
	networkStr := fmt.Sprintf("%d", network)

	if networkStr == "" || networkStr == "0" {
		networkStr = "unknown"
	}

	eventType := event.GetEvent().GetName().String()
	if eventType == "" {
		eventType = "unknown"
	}

	s.metrics.AddDecoratedEvent(1, eventType, networkStr)

	for _, sink := range s.sinks {
		if err := sink.HandleNewDecoratedEvent(ctx, event); err != nil {
			s.log.
				WithError(err).
				WithField("sink", sink.Type()).
				WithField("event_type", event.GetEvent().GetName()).
				Error("Failed to send event to sink")
		}
	}

	return nil
}
