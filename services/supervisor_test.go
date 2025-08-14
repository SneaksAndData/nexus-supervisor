package services

import (
	"context"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/models"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"
	nexuscore "github.com/SneaksAndData/nexus-core/pkg/generated/clientset/versioned"
	nexusfake "github.com/SneaksAndData/nexus-core/pkg/generated/clientset/versioned/fake"
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
	supervisor  *Supervisor
	ctx         context.Context
	kubeClient  kubernetes.Interface
	nexusClient nexuscore.Interface
	cqlStore    *request.CqlStore
}

func newFixture(t *testing.T, k8sObjects []runtime.Object, nexusObjects []runtime.Object) *fixture {
	_, ctx := ktesting.NewTestContext(t)
	f := &fixture{}

	f.ctx = ctx
	f.cqlStore = request.NewScyllaCqlStore(
		klog.FromContext(ctx), &request.ScyllaCqlStoreConfig{
			Hosts: []string{"127.0.0.1"},
		})
	f.nexusClient = nexusfake.NewClientset(nexusObjects...)
	f.kubeClient = fake.NewClientset(k8sObjects...)

	f.supervisor = NewSupervisor(f.kubeClient, f.nexusClient, "nexus", f.cqlStore, klog.FromContext(f.ctx), &noResyncPeriod)

	return f
}

func TestSupervisor_JobFailedCreate(t *testing.T) {
	event := &corev1.Event{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Event",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "nexus",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Job",
			Name:      "f47ac10b-58cc-4372-a567-0e02b2c3d479",
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
			Name:      "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			Namespace: "nexus",
			Labels: map[string]string{
				models.NexusComponentLabel: models.JobLabelAlgorithmRun,
				models.JobTemplateNameKey:  "test-algorithm",
			},
		},
		Spec: batchv1.JobSpec{},
	}

	k8sObjects := []runtime.Object{event, job}

	f := newFixture(t, k8sObjects, []runtime.Object{})
	err := f.supervisor.Init(f.ctx, &ProcessingConfig{
		FailureRateBaseDelay:       time.Second,
		FailureRateMaxDelay:        time.Second * 2,
		RateLimitElementsPerSecond: 10,
		RateLimitElementsBurst:     10,
		Workers:                    2,
	})

	if err != nil {
		t.Errorf("supervisor init failed %v", err)
	}

	go f.supervisor.Start(f.ctx)

	time.Sleep(time.Second * 3)

	result, err := f.supervisor.cqlStore.ReadCheckpoint("test-algorithm", "f47ac10b-58cc-4372-a567-0e02b2c3d479")

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

func TestSupervisor_JobDeadlined(t *testing.T) {
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
				Name:      "2c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2a",
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
				Name:      "3c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2b",
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
				Name:      "2c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2a",
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
				Name:      "3c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2b",
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

	f := newFixture(t, k8sObjects, []runtime.Object{})
	err := f.supervisor.Init(f.ctx, &ProcessingConfig{
		FailureRateBaseDelay:       time.Second,
		FailureRateMaxDelay:        time.Second * 2,
		RateLimitElementsPerSecond: 10,
		RateLimitElementsBurst:     10,
		Workers:                    2,
	})

	if err != nil {
		t.Errorf("supervisor init failed %v", err)
	}

	go f.supervisor.Start(f.ctx)

	time.Sleep(time.Second * 6)

	result1, err1 := f.supervisor.cqlStore.ReadCheckpoint("test-algorithm", "2c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2a")
	result2, err2 := f.supervisor.cqlStore.ReadCheckpoint("test-algorithm", "3c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2b")

	if err1 != nil {
		t.Errorf("cannot read a checkpoint %v", err1)
		t.FailNow()
	}

	if err2 != nil {
		t.Errorf("cannot read a checkpoint %v", err2)
		t.FailNow()
	}

	if result1 == nil {
		t.Errorf("result for 2c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2a should not be nil")
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
