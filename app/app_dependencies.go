package app

import (
	"context"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type ApplicationServices struct {
	cqlStore   *request.CqlStore
	recorder   record.EventRecorder
	kubeClient *kubernetes.Clientset
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

func (appServices *ApplicationServices) CqlStore() *request.CqlStore {
	return appServices.cqlStore
}

func (appServices *ApplicationServices) Start(ctx context.Context) {
	//	appServices.completionActor.Start(ctx)
}
