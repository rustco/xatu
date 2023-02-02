package coordinator

import (
	"errors"
)

type Config struct {
	Address      string            `yaml:"address"`
	Headers      map[string]string `yaml:"headers"`
	TLS          bool              `yaml:"tls" default:"false"`
	NetworkIDs   []uint64          `yaml:"networkIDs"`
	ForkIDHashes []string          `yaml:"forkIDHashes"`
	MaxPeers     uint32            `yaml:"maxPeers" default:"100"`
}

func (c *Config) Validate() error {
	if c.Address == "" {
		return errors.New("address is required")
	}

	if len(c.NetworkIDs) == 0 {
		return errors.New("networkIDs is required")
	}

	return nil
}
