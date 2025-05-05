package app

import (
	"context"
	"fmt"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"
	"github.com/SneaksAndData/nexus-core/pkg/pipeline"
	"github.com/SneaksAndData/nexus-supervisor/services"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"time"
)

type ApplicationServices struct {
	cqlStore      *request.CqlStore
	recorder      record.EventRecorder
	kubeClient    *kubernetes.Clientset
	runStateCache *services.RunStateCache
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
	if appServices.runStateCache == nil {
		logger := klog.FromContext(ctx)
		appServices.runStateCache = services.NewRunStateCache(appServices.kubeClient, resourceNamespace, appServices.cqlStore, logger)
	}

	return appServices
}

func (appServices *ApplicationServices) CqlStore() *request.CqlStore {
	return appServices.cqlStore
}

func (appServices *ApplicationServices) superviseAction(analysisResult *services.RunStatusAnalysisResult) (types.UID, error) {
	switch analysisResult.Action {
	case services.ToFailStuckInPending:
		return "", nil
	case services.ToFailFatalError:
		return "", nil
	case services.ToSkip:
		return "", nil
	default:
		return "", fmt.Errorf("unknown analysis result action: %v", analysisResult.Action)
	}

	//if output == nil {
	//	return types.UID(""), fmt.Errorf("buffer is nil")
	//}
	//
	//var job = output.Checkpoint.ToV1Job("kubernetes.sneaksanddata.com/service-node-group", output.Checkpoint.AppliedConfiguration.Workgroup, fmt.Sprintf("%s-%s", buildmeta.AppVersion, buildmeta.BuildNumber))
	//var submitted *batchv1.Job
	//var submitErr error
	//
	//// submit to controller cluster if workgroup host is not provided
	//if output.Checkpoint.AppliedConfiguration.WorkgroupHost == "" {
	//	submitted, submitErr = appServices.kubeClient.BatchV1().Jobs(appServices.defaultNamespace).Create(context.TODO(), &job, v1.CreateOptions{})
	//}
	//
	//if shard := appServices.getShardByName(output.Checkpoint.AppliedConfiguration.WorkgroupHost); shard != nil {
	//	submitted, submitErr = shard.SendJob(shard.Namespace, &job)
	//} else {
	//	return "", errors.New(fmt.Sprintf("Shard API server %s not configured", output.Checkpoint.AppliedConfiguration.WorkgroupHost))
	//}
	//
	//if submitErr != nil {
	//	return "", submitErr
	//}
	//
	//return submitted.UID, nil
}

func (appServices *ApplicationServices) Start(ctx context.Context) {
	logger := klog.FromContext(ctx)
	logger.V(0).Info("Starting Nexus Supervisor")
	err := appServices.runStateCache.Init(ctx)

	superviseActor := pipeline.NewDefaultPipelineStageActor[*services.RunStatusAnalysisResult, types.UID](
		"supervisor",
		map[string]string{},
		time.Second*1,
		time.Second*5,
		10,
		100,
		10,
		appServices.superviseAction,
		nil,
	)

	if err != nil {
		logger.Error(err, "Fatal error during startup")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	go superviseActor.Start(ctx)
}
