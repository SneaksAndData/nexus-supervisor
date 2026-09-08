package app

import (
	"time"

	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/store/cassandra"
)

type SupervisorConfig struct {
	AstraCqlStore              cassandra.AstraBundleConfig `mapstructure:"astra-cql-store"`
	ScyllaCqlStore             cassandra.ScyllaConfig      `mapstructure:"scylla-cql-store"`
	KeyspacesCqlStore          cassandra.KeyspacesConfig   `mapstructure:"keyspaces-cql-store"`
	CqlStoreType               string                      `mapstructure:"cql-store-type"`
	KubeConfigPath             string                      `mapstructure:"kube-config-path"`
	ResourceNamespace          string                      `mapstructure:"resource-namespace"`
	LogLevel                   string                      `mapstructure:"log-level"`
	FailureRateBaseDelay       time.Duration               `mapstructure:"failure-rate-base-delay,omitempty"`
	FailureRateMaxDelay        time.Duration               `mapstructure:"failure-rate-max-delay,omitempty"`
	RateLimitElementsPerSecond int                         `mapstructure:"rate-limit-elements-per-second,omitempty"`
	RateLimitElementsBurst     int                         `mapstructure:"rate-limit-elements-burst,omitempty"`
	Workers                    int                         `mapstructure:"workers,omitempty"`
}

const (
	CqlStoreAstra     = "cassandra-astra"
	CqlStoreScylla    = "cassandra-scylla"
	CqlStoreKeyspaces = "cassandra-keyspaces"
)
