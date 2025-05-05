package app

import (
	"context"
	"fmt"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	"os"
	"strings"
)

const (
	EnvPrefix = "NEXUS_" // varnames will be NEXUS__MY_ENV_VAR
)

type SupervisorConfig struct {
	CqlStore          request.AstraBundleConfig `mapstructure:"cql-store"`
	KubeConfigPath    string                    `mapstructure:"kube-config-path"`
	ResourceNamespace string                    `mapstructure:"resource-namespace"`
}

func LoadConfig(ctx context.Context) SupervisorConfig {
	logger := klog.FromContext(ctx)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.SetConfigFile(fmt.Sprintf("appconfig.%s.yaml", strings.ToLower(os.Getenv("APPLICATION_ENVIRONMENT"))))
	viper.SetEnvPrefix(EnvPrefix)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		logger.Error(err, "Error loading application config from appconfig.yaml")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	var appConfig SupervisorConfig
	err := viper.Unmarshal(&appConfig)

	if err != nil {
		logger.Error(err, "Error loading application config from appconfig.yaml")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	return appConfig
}
