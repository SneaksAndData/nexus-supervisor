package main

import (
	"context"
	"flag"
	"github.com/SneaksAndData/nexus-core/pkg/signals"
	"github.com/SneaksAndData/nexus-core/pkg/telemetry"
	"github.com/SneaksAndData/nexus-supervisor/app"
	"k8s.io/klog/v2"
)

var (
	logLevel string
)

func init() {
	flag.StringVar(&logLevel, "log-level", "INFO", "Log level for the application.")
}

func launchApp(ctx context.Context) {
	appConfig := app.LoadConfig(ctx)
	appServices := (&app.ApplicationServices{}).
		WithCqlStore(ctx, &appConfig.CqlStore).
		WithKubeClient(ctx, appConfig.KubeConfigPath)

	go func() {
		appServices.Start(ctx)
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
	appLogger, err := telemetry.ConfigureLogger(ctx, map[string]string{}, logLevel)
	ctx = telemetry.WithStatsd(ctx, "nexus_receiver")
	logger := klog.FromContext(ctx)

	if err != nil {
		logger.Error(err, "One of the logging handlers cannot be configured")
	}

	klog.SetSlogLogger(appLogger)

	launchApp(ctx)
}
