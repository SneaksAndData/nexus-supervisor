package app

import (
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"
)

type SupervisorConfig struct {
	CqlStore          request.AstraBundleConfig `mapstructure:"cql-store"`
	KubeConfigPath    string                    `mapstructure:"kube-config-path"`
	ResourceNamespace string                    `mapstructure:"resource-namespace"`
	LogLevel          string                    `mapstructure:"log-level"`
}
