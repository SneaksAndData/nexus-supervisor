package services

import (
	"context"
	"fmt"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/models"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"
	"testing"
	"time"
)

var noResyncPeriod = time.Second * 0

type fixture struct {
	supervisor *Supervisor
	ctx        context.Context
	finish     context.CancelFunc
	kubeClient kubernetes.Interface
	cqlStore   *request.CqlStore
}

func newFixture(t *testing.T, k8sObjects []runtime.Object) *fixture {
	_, ctx := ktesting.NewTestContext(t)
	f := &fixture{}

	f.ctx, f.finish = context.WithCancel(ctx)
	f.cqlStore = request.NewScyllaCqlStore(
		klog.FromContext(ctx), &request.ScyllaCqlStoreConfig{
			Hosts: []string{"127.0.0.1"},
		})
	f.kubeClient = fake.NewClientset(k8sObjects...)

	f.supervisor = NewSupervisor(f.kubeClient, "nexus", f.cqlStore, klog.FromContext(f.ctx), &noResyncPeriod)

	return f
}

func getFailedCreateObjects(recordId string) []runtime.Object {
	event := &corev1.Event{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Event",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-failed-create-event",
			Namespace: "nexus",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Job",
			Name:      recordId,
			Namespace: "nexus",
		},
		Reason:  "FailedCreate",
		Message: "",
	}

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      recordId,
			Namespace: "nexus",
			Labels: map[string]string{
				models.NexusComponentLabel: models.JobLabelAlgorithmRun,
				models.JobTemplateNameKey:  "test-algorithm",
			},
		},
		Spec: batchv1.JobSpec{},
	}

	return []runtime.Object{event, job}
}

func validateFailedCreateObjects(f *fixture, recordId string, t *testing.T) {
	result, err := f.supervisor.cqlStore.ReadCheckpoint("test-algorithm", recordId)

	if err != nil {
		t.Errorf("cannot read a checkpoint %v", err)
		t.FailNow()
	}

	if result == nil {
		t.Errorf("result should not be nil")
		t.FailNow()
	}

	if result.LifecycleStage != models.LifecycleStageSchedulingFailed {
		t.Errorf("lifecycle stage should be %s, but is %s", models.LifecycleStageSchedulingFailed, result.LifecycleStage)
		t.FailNow()
	}
}

func getDeadlinedJobObjects(deadlinedId string, backoffId string) []runtime.Object {
	events := []*corev1.Event{
		{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Event",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deadlineexceeded",
				Namespace: "nexus",
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Job",
				Name:      deadlinedId,
				Namespace: "nexus",
			},
			Reason:  "DeadlineExceeded",
			Message: "",
		},
		{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Event",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backoffexceeded",
				Namespace: "nexus",
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Job",
				Name:      backoffId,
				Namespace: "nexus",
			},
			Reason:  "BackoffLimitExceeded",
			Message: "",
		},
	}

	jobs := []*batchv1.Job{
		{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "batch/v1",
				Kind:       "Job",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      deadlinedId,
				Namespace: "nexus",
				Labels: map[string]string{
					models.NexusComponentLabel: models.JobLabelAlgorithmRun,
					models.JobTemplateNameKey:  "test-algorithm",
				},
			},
			Spec: batchv1.JobSpec{},
		},
		{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "batch/v1",
				Kind:       "Job",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      backoffId,
				Namespace: "nexus",
				Labels: map[string]string{
					models.NexusComponentLabel: models.JobLabelAlgorithmRun,
					models.JobTemplateNameKey:  "test-algorithm",
				},
			},
			Spec: batchv1.JobSpec{},
		},
	}

	k8sObjects := []runtime.Object{}

	for _, job := range jobs {
		k8sObjects = append(k8sObjects, job)
	}
	for _, event := range events {
		k8sObjects = append(k8sObjects, event)
	}

	return k8sObjects
}

func validateDeadlinedJobObjects(f *fixture, deadlinedId string, backoffId string, t *testing.T) {
	result1, err1 := f.supervisor.cqlStore.ReadCheckpoint("test-algorithm", deadlinedId)
	result2, err2 := f.supervisor.cqlStore.ReadCheckpoint("test-algorithm", backoffId)

	if err1 != nil {
		t.Errorf("cannot read a checkpoint %v", err1)
		t.FailNow()
	}

	if err2 != nil {
		t.Errorf("cannot read a checkpoint %v", err2)
		t.FailNow()
	}

	if result1 == nil {
		t.Errorf("result for %s should not be nil", deadlinedId)
		t.FailNow()
	}

	if result1.LifecycleStage != models.LifecycleStageDeadlineExceeded {
		t.Errorf("lifecycle stage for %s should be %s, but is %s", result1.Id, models.LifecycleStageDeadlineExceeded, result1.LifecycleStage)
		t.FailNow()
	}

	if result2.LifecycleStage != models.LifecycleStageDeadlineExceeded {
		t.Errorf("lifecycle stage for %s should be %s, but is %s", result2.Id, models.LifecycleStageDeadlineExceeded, result2.LifecycleStage)
		t.FailNow()
	}
}

func getPodStartedObjects(recordId string) []runtime.Object {
	event := &corev1.Event{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Event",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-started",
			Namespace: "nexus",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      fmt.Sprintf("%s-acdey", recordId),
			Namespace: "nexus",
		},
		Reason:  "Started",
		Message: "",
	}

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-acdey", recordId),
			Namespace: "nexus",
			Labels: map[string]string{
				models.NexusComponentLabel:     models.JobLabelAlgorithmRun,
				models.JobTemplateNameKey:      "test-algorithm",
				"batch.kubernetes.io/job-name": recordId,
			},
		},
		Spec: corev1.PodSpec{},
	}

	return []runtime.Object{event, pod}
}

func validatePodStartedObjects(f *fixture, recordId string, t *testing.T) {
	result, err := f.supervisor.cqlStore.ReadCheckpoint("test-algorithm", recordId)

	if err != nil {
		t.Errorf("cannot read a checkpoint %v", err)
		t.FailNow()
	}

	if result == nil {
		t.Errorf("result should not be nil")
		t.FailNow()
	}

	if result.LifecycleStage != models.LifecycleStageRunning {
		t.Errorf("lifecycle stage should be %s, but is %s", models.LifecycleStageRunning, result.LifecycleStage)
		t.FailNow()
	}
}

func getPodOutOfMemoryObjects(recordId string) []runtime.Object {
	event := &corev1.Event{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Event",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-out-of-memory",
			Namespace: "nexus",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Job",
			Name:      recordId,
			Namespace: "nexus",
		},
		Reason:  "PodFailurePolicy",
		Message: "",
	}

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      recordId,
			Namespace: "nexus",
			Labels: map[string]string{
				models.NexusComponentLabel: models.JobLabelAlgorithmRun,
				models.JobTemplateNameKey:  "test-algorithm",
			},
		},
		Spec: batchv1.JobSpec{},
	}

	return []runtime.Object{event, job}
}

func validatePodOutOfMemoryObjects(f *fixture, recordId string, t *testing.T) {
	result, err := f.supervisor.cqlStore.ReadCheckpoint("test-algorithm", recordId)

	if err != nil {
		t.Errorf("cannot read a checkpoint %v", err)
		t.FailNow()
	}

	if result == nil {
		t.Errorf("result should not be nil")
		t.FailNow()
	}

	if result.LifecycleStage != models.LifecycleStageFailed {
		t.Errorf("lifecycle stage should be %s, but is %s", models.LifecycleStageFailed, result.LifecycleStage)
		t.FailNow()
	}
}

func TestSupervisor(t *testing.T) {
	k8sObjects := []runtime.Object{}
	k8sObjects = append(k8sObjects, getFailedCreateObjects("f47ac10b-58cc-4372-a567-0e02b2c3d479")...)
	k8sObjects = append(k8sObjects, getDeadlinedJobObjects("2c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2a", "3c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2b")...)
	k8sObjects = append(k8sObjects, getPodStartedObjects("4c7b6e8d-cc3c-fb5b-a3f6-5d7b9e2c7f2b")...)
	k8sObjects = append(k8sObjects, getPodOutOfMemoryObjects("1d7b6e8d-cc3c-fb5b-a3f6-5d7b9e2c7f2b")...)

	f := newFixture(t, k8sObjects)
	err := f.supervisor.Init(f.ctx, &ProcessingConfig{
		FailureRateBaseDelay:       time.Second,
		FailureRateMaxDelay:        time.Second * 2,
		RateLimitElementsPerSecond: 100,
		RateLimitElementsBurst:     100,
		Workers:                    1,
	})

	if err != nil {
		t.Errorf("supervisor init failed %v", err)
	}

	go f.supervisor.Start(f.ctx)

	time.Sleep(time.Second * 10)

	validateFailedCreateObjects(f, "f47ac10b-58cc-4372-a567-0e02b2c3d479", t)
	validateDeadlinedJobObjects(f, "2c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2a", "3c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2b", t)
	validatePodStartedObjects(f, "4c7b6e8d-cc3c-fb5b-a3f6-5d7b9e2c7f2b", t)
	validatePodOutOfMemoryObjects(f, "1d7b6e8d-cc3c-fb5b-a3f6-5d7b9e2c7f2b", t)

	f.finish()
}
