package app

import (
	"context"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"
	"github.com/SneaksAndData/nexus-supervisor/services"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type ApplicationServices struct {
	cqlStore   *request.CqlStore
	recorder   record.EventRecorder
	kubeClient *kubernetes.Clientset
	supervisor *services.Supervisor
}

func (appServices *ApplicationServices) WithCqlStore(ctx context.Context, bundleConfig *request.AstraBundleConfig) *ApplicationServices {
	if appServices.cqlStore == nil {
		logger := klog.FromContext(ctx)
		appServices.cqlStore = request.NewAstraCqlStore(logger, bundleConfig)
	}

	return appServices
}

func (appServices *ApplicationServices) WithKubeClient(ctx context.Context, kubeConfigPath string) *ApplicationServices {
	if appServices.kubeClient == nil {
		logger := klog.FromContext(ctx)
		kubeCfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			logger.Error(err, "Error building in-cluster kubeconfig for the scheduler")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}

		appServices.kubeClient, err = kubernetes.NewForConfig(kubeCfg)
		if err != nil {
			logger.Error(err, "Error building in-cluster kubernetes clientset for the scheduler")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}
	}

	return appServices
}

func (appServices *ApplicationServices) WithRunStateCache(ctx context.Context, resourceNamespace string) *ApplicationServices {
	if appServices.supervisor == nil {
		logger := klog.FromContext(ctx)
		appServices.supervisor = services.NewSupervisor(appServices.kubeClient, resourceNamespace, appServices.cqlStore, logger)
	}

	return appServices
}

func (appServices *ApplicationServices) CqlStore() *request.CqlStore {
	return appServices.cqlStore
}

func (appServices *ApplicationServices) Start(ctx context.Context) {
	logger := klog.FromContext(ctx)
	logger.V(0).Info("Starting Nexus Supervisor")

	err := appServices.supervisor.Init(ctx)

	if err != nil {
		logger.Error(err, "Fatal error during startup")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	go appServices.supervisor.Start(ctx)
}
