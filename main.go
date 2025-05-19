package main

import (
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

	appServices := (&app.ApplicationServices{}).
		WithCqlStore(ctx, &appConfig.CqlStore).
		WithKubeClient(ctx, appConfig.KubeConfigPath).
		WithSupervisor(ctx, appConfig.ResourceNamespace)

	appServices.Start(ctx, &appConfig, logger)
}
