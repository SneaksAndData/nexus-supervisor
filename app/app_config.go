package app

import (
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"
	"time"
)

type SupervisorConfig struct {
	CqlStore                   request.AstraBundleConfig `mapstructure:"cql-store"`
	KubeConfigPath             string                    `mapstructure:"kube-config-path"`
	ResourceNamespace          string                    `mapstructure:"resource-namespace"`
	LogLevel                   string                    `mapstructure:"log-level"`
	FailureRateBaseDelay       time.Duration             `mapstructure:"failure-rate-base-delay,omitempty"`
	FailureRateMaxDelay        time.Duration             `mapstructure:"failure-rate-max-delay,omitempty"`
	RateLimitElementsPerSecond int                       `mapstructure:"rate-limit-elements-per-second,omitempty"`
	RateLimitElementsBurst     int                       `mapstructure:"rate-limit-elements-burst,omitempty"`
	Workers                    int                       `mapstructure:"workers,omitempty"`
}
