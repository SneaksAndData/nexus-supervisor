package main

import (
	"errors"
	nexusconf "github.com/SneaksAndData/nexus-core/pkg/configurations"
	"github.com/SneaksAndData/nexus-core/pkg/signals"
	"github.com/SneaksAndData/nexus-core/pkg/telemetry"
	"github.com/SneaksAndData/nexus-supervisor/app"
	"k8s.io/klog/v2"
)

func main() {
	ctx := signals.SetupSignalHandler()
	appConfig := nexusconf.LoadConfig[app.SupervisorConfig](ctx)
	appLogger, err := telemetry.ConfigureLogger(ctx, map[string]string{}, appConfig.LogLevel)

	ctx = telemetry.WithStatsd(ctx, "nexus_receiver")

	klog.SetSlogLogger(appLogger)
	logger := klog.FromContext(ctx)

	if err != nil {
		logger.Error(err, "One of the logging handlers cannot be configured")
	}

	appServices := &app.ApplicationServices{}

	switch appConfig.CqlStoreType {
	case app.CqlStoreAstra:
		appServices = appServices.WithAstraCqlStore(ctx, &appConfig.AstraCqlStore)
	case app.CqlStoreScylla:
		appServices = appServices.WithScyllaCqlStore(ctx, &appConfig.ScyllaCqlStore)
	default:
		klog.FromContext(ctx).Error(errors.New("unknown store type "+appConfig.CqlStoreType), "failed to initialize a CqlStore")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	appServices = appServices.
		WithKubeClient(ctx, appConfig.KubeConfigPath).
		WithSupervisor(ctx, appConfig.ResourceNamespace)

	appServices.Start(ctx, &appConfig, logger)
}
