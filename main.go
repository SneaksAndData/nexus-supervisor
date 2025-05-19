package main

import (
	"context"
	nexusconf "github.com/SneaksAndData/nexus-core/pkg/configurations"
	"github.com/SneaksAndData/nexus-core/pkg/signals"
	"github.com/SneaksAndData/nexus-core/pkg/telemetry"
	"github.com/SneaksAndData/nexus-supervisor/app"
	"k8s.io/klog/v2"
)

func launchApp(ctx context.Context, config *app.SupervisorConfig) {

	appServices := (&app.ApplicationServices{}).
		WithCqlStore(ctx, &config.CqlStore).
		WithKubeClient(ctx, config.KubeConfigPath)

	go func() {
		appServices.Start(ctx, config)
		// handle exit
		logger := klog.FromContext(ctx)
		reason := ctx.Err()
		if reason.Error() == context.Canceled.Error() {
			logger.V(0).Info("Received SIGTERM, shutting down gracefully")
			klog.FlushAndExit(klog.ExitFlushTimeout, 0)
		}

		logger.V(0).Error(reason, "Fatal error occurred.")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}()

	return
}

func main() {
	ctx := signals.SetupSignalHandler()
	appConfig := nexusconf.LoadConfig[app.SupervisorConfig](ctx)
	appLogger, err := telemetry.ConfigureLogger(ctx, map[string]string{}, appConfig.LogLevel)
	ctx = telemetry.WithStatsd(ctx, "nexus_receiver")
	logger := klog.FromContext(ctx)

	if err != nil {
		logger.Error(err, "One of the logging handlers cannot be configured")
	}

	klog.SetSlogLogger(appLogger)

	launchApp(ctx, &appConfig)
}
