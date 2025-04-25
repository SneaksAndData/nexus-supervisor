package services

import (
	"context"
	"fmt"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/models"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"

	//batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"time"
)

type RunStateCache struct {
	logger      klog.Logger
	factory     kubeinformers.SharedInformerFactory
	podInformer cache.SharedIndexInformer
	cqlStore    *request.CqlStore
	//jobInformer    cache.SharedIndexInformer
	prefix         string
	selectorLabels map[string]string
}

// NewRunStateCache creates a new cache + resource watcher for pod and job resources
func NewRunStateCache(client *kubernetes.Clientset, resourceNamespace string, cqlStore *request.CqlStore, logger klog.Logger) *RunStateCache {
	factory := kubeinformers.NewSharedInformerFactoryWithOptions(client, time.Second*30, kubeinformers.WithNamespace(resourceNamespace))
	podWatcher := factory.Core().V1().Pods()
	//jobWatcher := factory.Batch().V1().Jobs()

	return &RunStateCache{
		logger:      logger,
		factory:     factory,
		podInformer: podWatcher.Informer(),
		cqlStore:    cqlStore,
		//jobInformer: jobWatcher.Informer(),
		prefix: resourceNamespace,
		selectorLabels: map[string]string{
			"app.kubernetes.io/component": "algorithm-run",
		},
	}
}

// Init starts informers and sync the cache
func (c *RunStateCache) Init(ctx context.Context) error {
	// Set up an event handler for when pod resources change
	_, podErr := c.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onPodAdded,
		UpdateFunc: c.onPodUpdated,
	})

	//// Set up an event handler for when job resources change
	//_, jobErr := c.jobInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc:    c.onJobAdded,
	//	UpdateFunc: c.onJobUpdated,
	//	DeleteFunc: c.onJobDeleted,
	//})

	if podErr != nil {
		return podErr
	}

	c.factory.Start(ctx.Done())

	if ok := cache.WaitForCacheSync(ctx.Done(), c.podInformer.HasSynced); !ok {
		return fmt.Errorf("failed to wait for pod informer caches to sync")
	}

	//if ok := cache.WaitForCacheSync(ctx.Done(), c.jobInformer.HasSynced); !ok {
	//	return fmt.Errorf("failed to wait for job informer caches to sync")
	//}
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
	return false // TODO: implement
}

func isScheduling(status corev1.PodStatus) bool {
	return false // TODO: implement
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
			// handle fatal
		}
	}

	if newPod.Status.Phase == corev1.PodPending {
		if isScheduling(newPod.Status) {
			return
		} else {
			// handle stuck pods
		}
	}

	if newPod.Status.Phase == corev1.PodRunning {
		// update checkpoint status
	}

	if newPod.Status.Phase == corev1.PodSucceeded {
		// update checkpoint status
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

//func (c *MachineLearningAlgorithmCache) cacheKey(algorithmName string) string {
//	return fmt.Sprintf("%s/%s", c.prefix, algorithmName)
//}
