package app

import (
	"context"

	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/store"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/store/cassandra"
	"github.com/SneaksAndData/nexus-supervisor/services"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

type ApplicationServices struct {
	cqlStore   store.CheckpointStore
	kubeClient *kubernetes.Clientset
	supervisor *services.Supervisor
}

func (appServices *ApplicationServices) WithAstraCqlStore(ctx context.Context, bundleConfig *cassandra.AstraBundleConfig) *ApplicationServices {
	if appServices.cqlStore == nil {
		logger := klog.FromContext(ctx)
		appServices.cqlStore = cassandra.NewAstraStore(logger, bundleConfig)
	}

	return appServices
}

func (appServices *ApplicationServices) WithScyllaCqlStore(ctx context.Context, config *cassandra.ScyllaConfig) *ApplicationServices {
	if appServices.cqlStore == nil {
		logger := klog.FromContext(ctx)
		appServices.cqlStore = cassandra.NewScyllaStore(logger, config)
	}

	return appServices
}

func (appServices *ApplicationServices) WithKeyspacesCqlStore(ctx context.Context, config *cassandra.KeyspacesConfig) *ApplicationServices {
	if appServices.cqlStore == nil {
		logger := klog.FromContext(ctx)
		appServices.cqlStore = cassandra.NewKeyspacesStore(logger, config)
	}

	return appServices
}

func (appServices *ApplicationServices) WithKubeClient(ctx context.Context, kubeConfigPath string) *ApplicationServices {
	if appServices.kubeClient == nil {
		logger := klog.FromContext(ctx)
		kubeCfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			logger.Error(err, "Error building in-cluster kubeconfig for the application")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}

		appServices.kubeClient, err = kubernetes.NewForConfig(kubeCfg)
		if err != nil {
			logger.Error(err, "Error building in-cluster kubernetes clientset for the application")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}
	}

	return appServices
}

func (appServices *ApplicationServices) WithSupervisor(ctx context.Context, resourceNamespace string) *ApplicationServices {
	if appServices.supervisor == nil {
		logger := klog.FromContext(ctx)
		appServices.supervisor = services.NewSupervisor(appServices.kubeClient, resourceNamespace, appServices.cqlStore, logger, nil, nil)
	}

	return appServices
}

func (appServices *ApplicationServices) CheckpointStore() store.CheckpointStore {
	return appServices.cqlStore
}

func (appServices *ApplicationServices) Start(ctx context.Context, config *SupervisorConfig, logger klog.Logger) {
	logger.V(0).Info("Starting Nexus Supervisor")

	err := appServices.supervisor.Init(ctx, &services.ProcessingConfig{
		FailureRateBaseDelay:       config.FailureRateBaseDelay,
		FailureRateMaxDelay:        config.FailureRateMaxDelay,
		RateLimitElementsPerSecond: config.RateLimitElementsPerSecond,
		RateLimitElementsBurst:     config.RateLimitElementsBurst,
		Workers:                    config.Workers,
	})

	if err != nil {
		logger.Error(err, "Fatal error during startup")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	appServices.supervisor.Start(ctx)
}
