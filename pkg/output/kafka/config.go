package kafka

import (
	"errors"
	"time"
)

type Config struct {
	Brokers            string              `yaml:"brokers"`
	Topic              string              `yaml:"topic"`
	TLS                bool                `yaml:"tls" default:"false"`
	TLSClientConfig    TLSClientConfig     `yaml:"tlsClientConfig"`
	MaxQueueSize       int                 `yaml:"maxQueueSize" default:"51200"`
	BatchTimeout       time.Duration       `yaml:"batchTimeout" default:"5s"`
	MaxExportBatchSize int                 `yaml:"maxExportBatchSize" default:"512"`
	Workers            int                 `yaml:"workers" default:"5"`
	FlushFrequency     time.Duration       `yaml:"flushFrequency" default:"10s"`
	FlushMessages      int                 `yaml:"flushMessages" default:"500"`
	FlushBytes         int                 `yaml:"flushBytes" default:"1000000"`
	MaxRetries         int                 `yaml:"maxRetries" default:"3"`
	Compression        CompressionStrategy `yaml:"compression" default:"none"`
	RequiredAcks       RequiredAcks        `yaml:"requiredAcks" default:"leader"`
	Partitioning       PartitionStrategy   `yaml:"partitioning" default:"none"`
}

type TLSClientConfig struct {
	CertificatePath string `yaml:"certificatePath"`
	KeyPath         string `yaml:"keyPath"`
	CACertificate   string `yaml:"caCertificate"`
}

func (c *Config) Validate() error {
	if c.Brokers == "" {
		return errors.New("brokers is required")
	}

	if c.Topic == "" {
		return errors.New("topic is required")
	}

	if err := c.TLSClientConfig.Validate(); err != nil {
		return err
	}

	return nil
}

func (c *TLSClientConfig) Validate() error {
	if c.CertificatePath != "" && c.KeyPath == "" {
		return errors.New("client key is required")
	}

	return nil
}
