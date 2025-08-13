package services

import (
	"context"
	"github.com/SneaksAndData/nexus-core/pkg/checkpoint/request"
	nexuscore "github.com/SneaksAndData/nexus-core/pkg/generated/clientset/versioned"
	nexusfake "github.com/SneaksAndData/nexus-core/pkg/generated/clientset/versioned/fake"
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

func newFixture(t *testing.T) *fixture {
	_, ctx := ktesting.NewTestContext(t)
	f := &fixture{}

	f.ctx = ctx
	f.cqlStore = request.NewScyllaCqlStore(
		klog.FromContext(ctx), &request.ScyllaCqlStoreConfig{
			Hosts: []string{"127.0.0.1"},
		})
	f.nexusClient = nexusfake.NewClientset()
	f.kubeClient = fake.NewClientset()

	f.supervisor = NewSupervisor(f.kubeClient, f.nexusClient, "nexus", f.cqlStore, klog.FromContext(f.ctx), &noResyncPeriod)

	return f
}
