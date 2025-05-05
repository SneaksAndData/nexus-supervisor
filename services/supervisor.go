package services

import (
	"context"
	"fmt"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/models"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"
	"github.com/SneaksAndData/nexus-core/pkg/pipeline"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"strings"
	"time"
)

type RunStateCache struct {
	logger               klog.Logger
	factory              kubeinformers.SharedInformerFactory
	podInformer          cache.SharedIndexInformer
	cqlStore             *request.CqlStore
	prefix               string
	selectorLabels       map[string]string
	elementReceiverActor *pipeline.DefaultPipelineStageActor[*RunStatusAnalysisResult, types.UID]
}

type DecisionAction = string

const (
	ToFailStuckInPending = DecisionAction("ToFailStuckInPending")
	ToSkip               = DecisionAction("ToSkip")
	ToFailFatalError     = DecisionAction("ToFailFatalError")
	ToRunning            = DecisionAction("ToRunning")
)

type RunStatusAnalysisResult struct {
	Action       DecisionAction
	RunPodStatus *corev1.PodStatus
	RunPodUID    types.UID
	RunId        string
}

func newRunStatusAnalysisResult(pod *corev1.Pod, desiredAction DecisionAction) *RunStatusAnalysisResult {
	return &RunStatusAnalysisResult{
		Action:       desiredAction,
		RunPodStatus: pod.Status.DeepCopy(),
		RunPodUID:    pod.UID,
		RunId:        pod.Spec.Containers[0].Name,
	}
}

// NewRunStateCache creates a new cache + resource watcher for pod and job resources
func NewRunStateCache(client *kubernetes.Clientset, resourceNamespace string, cqlStore *request.CqlStore, logger klog.Logger) *RunStateCache {
	factory := kubeinformers.NewSharedInformerFactoryWithOptions(client, time.Second*30, kubeinformers.WithNamespace(resourceNamespace))
	podWatcher := factory.Core().V1().Pods()

	return &RunStateCache{
		logger:      logger,
		factory:     factory,
		podInformer: podWatcher.Informer(),
		cqlStore:    cqlStore,
		prefix:      resourceNamespace,
		selectorLabels: map[string]string{
			"app.kubernetes.io/component": "algorithm-run",
		},
		elementReceiverActor: nil,
	}
}

// Init starts informers and sync the cache
func (c *RunStateCache) Init(ctx context.Context) error {
	c.elementReceiverActor = pipeline.NewDefaultPipelineStageActor[*RunStatusAnalysisResult, types.UID](
		"supervisor",
		map[string]string{},
		time.Second*1,
		time.Second*5,
		10,
		100,
		10,
		c.superviseAction,
		nil,
	)
	// Set up an event handler for when pod resources change
	_, podErr := c.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onPodAdded,
		UpdateFunc: c.onPodUpdated,
	})

	if podErr != nil {
		return podErr
	}

	c.factory.Start(ctx.Done())

	if ok := cache.WaitForCacheSync(ctx.Done(), c.podInformer.HasSynced); !ok {
		return fmt.Errorf("failed to wait for pod informer caches to sync")
	}

	c.logger.Info("Resource informers synced")

	return nil
}

func isSupervisedObject(objMeta metav1.ObjectMeta, labels map[string]string) bool {
	// only use objects with matching labels
	objectLabels := objMeta.Labels
	for labelKey, labelValue := range labels {
		if objectLabelValue, exists := objectLabels[labelKey]; exists {
			if objectLabelValue != labelValue {
				return false
			}
		} else {
			return false
		}
	}

	return true
}

func isTransientFailure(status corev1.PodStatus) bool {
	// TODO: check algorithm fatal/transient codes here
	exitCode := status.ContainerStatuses[0].State.Terminated.ExitCode
	if exitCode == 137 || exitCode == 255 { // OOM, abnormal termination
		return false
	}

	return true
}

func isScheduling(status corev1.PodStatus) bool {
	for _, condition := range status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse && len(status.Conditions) == 1 {
			return true
		}
		if status.Reason == corev1.PodReasonUnschedulable || status.Reason == corev1.PodReasonSchedulerError {
			return false
		}
		if strings.Contains(status.Message, "ErrImagePull") || strings.Contains(status.Message, "ImagePullBackOff") {
			return false
		}
	}

	return false
}

func (c *RunStateCache) onPodAdded(obj interface{}) {
	objectRef, err := cache.ObjectToName(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	// only handle objects with matching labels
	if !isSupervisedObject(obj.(corev1.Pod).ObjectMeta, c.selectorLabels) {
		return
	}

	if algorithm, annotated := obj.(*corev1.Pod).ObjectMeta.Annotations["science.sneaksanddata.com/algorithm-template-name"]; annotated {
		c.logger.V(3).Info("Algorithm run instance detected", "algorithm", algorithm)
	} else {
		c.logger.V(2).Info("Algorithm run instance does not have science.sneaksanddata.com/algorithm-template-name annotations", "runId", objectRef.Name)
	}
}

func (c *RunStateCache) onPodUpdated(_, new interface{}) {
	_, newErr := cache.ObjectToName(new)

	if newErr != nil {
		utilruntime.HandleError(newErr)
		return
	}

	newPod := new.(*corev1.Pod)
	// only handle objects with matching labels
	if !isSupervisedObject(newPod.ObjectMeta, c.selectorLabels) {
		return
	}

	if newPod.Status.Phase == corev1.PodFailed {
		c.logger.V(2).Info("Algorithm run attempt failed", "requestId", newPod.Spec.Containers[0].Name, "reason", newPod.Status.Reason, "message", newPod.Status.Message)
		if isTransientFailure(newPod.Status) {
			return
		} else {
			c.elementReceiverActor.Receive(newRunStatusAnalysisResult(newPod, ToFailFatalError))
		}
	}

	if newPod.Status.Phase == corev1.PodPending {
		if isScheduling(newPod.Status) {
			return
		} else {
			c.elementReceiverActor.Receive(newRunStatusAnalysisResult(newPod, ToFailStuckInPending))
		}
	}

	if newPod.Status.Phase == corev1.PodRunning {
		c.elementReceiverActor.Receive(newRunStatusAnalysisResult(newPod, ToRunning))
	}

	if newPod.Status.Phase == corev1.PodSucceeded {
		c.elementReceiverActor.Receive(newRunStatusAnalysisResult(newPod, ToSkip))
	}
}

func toLifecycleStage(podState corev1.PodPhase) models.LifecycleStage {
	switch podState {
	case corev1.PodPending:
		return models.LifecyclestageBuffered
	case corev1.PodRunning:
		return models.LifecyclestageRunning
	case corev1.PodSucceeded:
		return models.LifecyclestageCompleted
	case corev1.PodFailed:
		return models.LifecyclestageFailed
	default:
		return models.LifecyclestageBuffered
	}
}

// TODO: implement actions
func (c *RunStateCache) superviseAction(analysisResult *RunStatusAnalysisResult) (types.UID, error) {
	switch analysisResult.Action {
	case ToFailStuckInPending:

		return "", nil
	case ToFailFatalError:
		return "", nil
	case ToSkip:
		return "", nil
	default:
		return "", fmt.Errorf("unknown analysis result action: %v", analysisResult.Action)
	}
}

func (c *RunStateCache) Start(ctx context.Context) {
	c.elementReceiverActor.Start(ctx)
}
