package services

import (
	"context"
	"fmt"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/models"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"
	"github.com/SneaksAndData/nexus-core/pkg/pipeline"
	"github.com/SneaksAndData/nexus-core/pkg/resolvers"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"time"
)

type Supervisor struct {
	logger               klog.Logger
	factory              kubeinformers.SharedInformerFactory
	eventInformer        cache.SharedIndexInformer
	podInformer          cache.SharedIndexInformer
	jobInformer          cache.SharedIndexInformer
	informers            map[string]cache.SharedIndexInformer
	kubeClient           kubernetes.Interface
	resourceNamespace    string
	cqlStore             *request.CqlStore
	elementReceiverActor *pipeline.DefaultPipelineStageActor[*RunStatusAnalysisResult, types.UID]
}

type ProcessingConfig struct {
	FailureRateBaseDelay       time.Duration
	FailureRateMaxDelay        time.Duration
	RateLimitElementsPerSecond int
	RateLimitElementsBurst     int
	Workers                    int
}

type DecisionAction = string

const (
	ToFailStuckInPending = DecisionAction("ToFailStuckInPending")
	ToSkip               = DecisionAction("ToSkip")
	ToFailFatalError     = DecisionAction("ToFailFatalError")
	ToRunning            = DecisionAction("ToRunning")
)

type RunStatusAnalysisResult struct {
	Action           DecisionAction
	RunStatusMessage string
	RunStatusTrace   string
	ObjectUID        types.UID
	ObjectKind       string
	RequestId        string
	Algorithm        string
}

// NewSupervisor creates a new cache + resource watcher for pod and job resources
func NewSupervisor(client *kubernetes.Clientset, resourceNamespace string, cqlStore *request.CqlStore, logger klog.Logger) *Supervisor {
	factory := kubeinformers.NewSharedInformerFactoryWithOptions(client, time.Second*30, kubeinformers.WithNamespace(resourceNamespace))
	eventInformer := factory.Core().V1().Events().Informer()
	podInformer := factory.Core().V1().Pods().Informer()
	jobInformer := factory.Batch().V1().Jobs().Informer()

	return &Supervisor{
		logger:               logger,
		factory:              factory,
		kubeClient:           client,
		resourceNamespace:    resourceNamespace,
		eventInformer:        eventInformer,
		podInformer:          podInformer,
		jobInformer:          jobInformer,
		cqlStore:             cqlStore,
		elementReceiverActor: nil,
	}
}

// Init starts informers and sync the cache
func (c *Supervisor) Init(ctx context.Context, config *ProcessingConfig) error {
	c.elementReceiverActor = pipeline.NewDefaultPipelineStageActor[*RunStatusAnalysisResult, types.UID](
		"supervisor",
		map[string]string{},
		config.FailureRateBaseDelay,
		config.FailureRateMaxDelay,
		config.RateLimitElementsPerSecond,
		config.RateLimitElementsBurst,
		config.Workers,
		c.superviseAction,
		nil,
	)

	c.informers = map[string]cache.SharedIndexInformer{
		"Job": c.jobInformer,
		"Pod": c.podInformer,
	}

	_, eventErr := c.eventInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.onEvent,
	})

	if eventErr != nil {
		return eventErr
	}

	c.factory.Start(ctx.Done())

	if ok := cache.WaitForCacheSync(ctx.Done(), c.eventInformer.HasSynced, c.podInformer.HasSynced, c.jobInformer.HasSynced); !ok {
		return fmt.Errorf("failed to wait for pod informer caches to sync")
	}

	c.logger.Info("Resource informers synced")

	return nil
}

func (c *Supervisor) onEvent(obj interface{}) {
	_, err := cache.ObjectToName(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	event := obj.(*corev1.Event)

	supervised, err := resolvers.IsNexusRunEvent(event, c.resourceNamespace, c.informers)

	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	// only handle events from Nexus runs
	if !supervised {
		return
	}

	if event.InvolvedObject.Kind == "Job" && event.Reason == "FailedCreate" {
		job, cacheErr := resolvers.GetCachedObject[batchv1.Job](event.InvolvedObject.Name, c.resourceNamespace, c.jobInformer)
		if job == nil {
			c.logger.V(0).Info("Algorithm job not found - stale event", "requestId", event.InvolvedObject.Name, "reason", event.Reason, "message", event.Message)
			return
		}

		if cacheErr != nil {
			utilruntime.HandleError(cacheErr)
			return
		}

		c.logger.V(0).Info("Algorithm run attempt failed", "requestId", event.InvolvedObject.Name, "reason", event.Reason, "message", event.Message)

		c.elementReceiverActor.Receive(&RunStatusAnalysisResult{
			Action:           ToFailStuckInPending,
			RunStatusMessage: "Unable to launch a new submission for the algorithm - please review configuration and try again.",
			RunStatusTrace:   event.Message,
			ObjectUID:        event.InvolvedObject.UID,
			ObjectKind:       event.InvolvedObject.Kind,
			RequestId:        event.InvolvedObject.Name,
			Algorithm:        job.GetLabels()[models.JobTemplateNameKey],
		})
	}

	if event.InvolvedObject.Kind == "Pod" {
		pod, cacheErr := resolvers.GetCachedObject[corev1.Pod](event.InvolvedObject.Name, c.resourceNamespace, c.podInformer)

		if cacheErr != nil {
			utilruntime.HandleError(cacheErr)
			return
		}

		if pod == nil {
			c.logger.V(0).Info("Algorithm pod not found - stale event", "requestId", event.InvolvedObject.Name, "reason", event.Reason, "message", event.Message)
			return
		}

		// TODO: move reasons to a configuration?

		switch event.Reason {
		case "Started":
			c.elementReceiverActor.Receive(&RunStatusAnalysisResult{
				Action:           ToRunning,
				RunStatusMessage: event.Reason,
				RunStatusTrace:   event.Message,
				ObjectUID:        event.InvolvedObject.UID,
				ObjectKind:       event.InvolvedObject.Kind,
				RequestId:        pod.Labels["batch.kubernetes.io/job-name"],
				Algorithm:        pod.GetLabels()[models.JobTemplateNameKey],
			})
		case "FailedScheduling", "Nominated", "Scheduled":
			c.elementReceiverActor.Receive(&RunStatusAnalysisResult{
				Action:           ToSkip,
				RunStatusMessage: event.Reason,
				RunStatusTrace:   event.Message,
				ObjectUID:        event.InvolvedObject.UID,
				ObjectKind:       event.InvolvedObject.Kind,
				RequestId:        pod.Labels["batch.kubernetes.io/job-name"],
				Algorithm:        pod.GetLabels()[models.JobTemplateNameKey],
			})
		default:
			c.elementReceiverActor.Receive(&RunStatusAnalysisResult{
				Action:           ToSkip,
				RunStatusMessage: event.Reason,
				RunStatusTrace:   event.Message,
				ObjectUID:        event.InvolvedObject.UID,
				ObjectKind:       event.InvolvedObject.Kind,
				RequestId:        pod.Labels["batch.kubernetes.io/job-name"],
				Algorithm:        pod.GetLabels()[models.JobTemplateNameKey],
			})
			//c.elementReceiverActor.Receive(&RunStatusAnalysisResult{
			//	Action:           ToFailFatalError,
			//	RunStatusMessage: event.Reason,
			//	RunStatusTrace:   event.Message,
			//	ObjectUID:        event.InvolvedObject.UID,
			//	ObjectKind:       event.InvolvedObject.Kind,
			//	RequestId:        pod.Labels["batch.kubernetes.io/job-name"],
			//	Algorithm:        pod.GetLabels()[models.JobTemplateNameKey],
			//})
		}
	}
}

func (c *Supervisor) superviseAction(analysisResult *RunStatusAnalysisResult) (types.UID, error) {
	propagationPolicy := metav1.DeletePropagationBackground

	checkpoint, err := c.cqlStore.ReadCheckpoint(analysisResult.Algorithm, analysisResult.RequestId)
	if err != nil {
		c.logger.V(0).Error(err, "no checkpoint exists for the provided request, job will be deleted without metadata saved", "requestId", analysisResult.RequestId, "algorithm", analysisResult.Algorithm)

		_ = c.kubeClient.BatchV1().Jobs(c.resourceNamespace).Delete(context.TODO(), analysisResult.RequestId, metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		})

		return analysisResult.ObjectUID, err
	}

	checkpointClone := checkpoint.DeepCopy()

	switch analysisResult.Action {
	case ToFailStuckInPending:
		// this decision implies:
		// remove the k8s job
		// update run status to failed, setting reason to scheduling failure

		err := c.kubeClient.BatchV1().Jobs(c.resourceNamespace).Delete(context.TODO(), analysisResult.RequestId, metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			c.logger.V(0).Error(err, "failed to delete an algorithm submission", "requestId", analysisResult.RequestId, "algorithm", analysisResult.Algorithm)
			return analysisResult.ObjectUID, err
		}

		checkpointClone.LifecycleStage = models.LifecycleStageSchedulingFailed
		checkpointClone.AlgorithmFailureCause = fmt.Sprintf("Algorithm submission was buffered, but failed to schedule on the target cluster: %s", analysisResult.RunStatusMessage)
		checkpointClone.AlgorithmFailureDetails = analysisResult.RunStatusTrace

		err = c.cqlStore.UpsertCheckpoint(checkpointClone)

		if err != nil {
			c.logger.V(0).Error(err, "failed to update algorithm submission status", "requestId", analysisResult.RequestId, "algorithm", analysisResult.Algorithm)
			return analysisResult.ObjectUID, err
		}

		return analysisResult.ObjectUID, nil

	case ToFailFatalError:
		// edge case that is invoked when a non-recoverable error occurs, but is not marked by the algorithm as fatal
		// this mainly applies to 137 (out-of-memory) and 255 (unknown fatal error) cases
		err := c.kubeClient.BatchV1().Jobs(c.resourceNamespace).Delete(context.TODO(), analysisResult.RequestId, metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			c.logger.V(0).Error(err, "failed to delete an algorithm submission", "requestId", analysisResult.RequestId, "algorithm", analysisResult.Algorithm)
			return analysisResult.ObjectUID, err
		}

		// check if status has been updated

		checkpointClone.LifecycleStage = models.LifecycleStageFailed
		checkpointClone.AlgorithmFailureCause = fmt.Sprintf("Algorithm encountered a fatal error during execution: %s", analysisResult.RunStatusMessage)
		checkpointClone.AlgorithmFailureDetails = analysisResult.RunStatusTrace

		err = c.cqlStore.UpsertCheckpoint(checkpointClone)

		if err != nil {
			c.logger.V(0).Error(err, "failed to update algorithm submission status", "requestId", analysisResult.RequestId, "algorithm", analysisResult.Algorithm)
			return analysisResult.ObjectUID, err
		}

		return analysisResult.ObjectUID, nil
	case ToRunning:
		checkpointClone.LifecycleStage = models.LifecycleStageRunning
		// transition from buffered to running
		err := c.cqlStore.UpsertCheckpoint(checkpointClone)
		if err != nil {
			c.logger.V(0).Error(err, "failed to update algorithm submission status", "requestId", analysisResult.RequestId, "algorithm", analysisResult.Algorithm)
			return analysisResult.ObjectUID, err
		}

		return analysisResult.ObjectUID, nil
	case ToSkip:
		// nothing to do since the run has completed or has not started yet
		c.logger.V(0).Info("no-op event, ignoring", "requestId", analysisResult.RequestId, "algorithm", analysisResult.Algorithm)
		return analysisResult.ObjectUID, nil
	default:
		return analysisResult.ObjectUID, fmt.Errorf("unknown analysis result action: %v", analysisResult.Action)
	}
}

func (c *Supervisor) Start(ctx context.Context) {
	c.elementReceiverActor.Start(ctx)
}
