package checker

import (
	"context"
	"testing"
	"time"

	"github.com/chammanganti/homelab-health/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestChecker(targets []config.Target, deployments []*appsv1.Deployment) *Checker {
	client := fake.NewSimpleClientset()
	for _, d := range deployments {
		client.AppsV1().Deployments(d.Namespace).Create(context.Background(), d, metav1.CreateOptions{})
	}
	return &Checker{
		client:  client,
		targets: targets,
		results: make(map[string]ServiceHealth),
	}
}

func deployment(name, namespace string, desired, available int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desired,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          desired,
			AvailableReplicas: available,
		},
	}
}

func TestCheck_Ready(t *testing.T) {
	targets := []config.Target{
		{Name: "traefik", Namespace: "traefik", Deployment: "traefik"},
	}
	checker := newTestChecker(targets, []*appsv1.Deployment{
		deployment("traefik", "traefik", 1, 1),
	})

	checker.check(targets[0])

	results := checker.Results()
	h, ok := results["traefik"]
	if !ok {
		t.Fatal("expected result for 'traefik', got none")
	}
	if !h.Ready {
		t.Errorf("expected ready=true, got ready=false")
	}
}

func TestCheck_NotReady(t *testing.T) {
	targets := []config.Target{
		{Name: "traefik", Namespace: "traefik", Deployment: "traefik"},
	}
	checker := newTestChecker(targets, []*appsv1.Deployment{
		deployment("traefik", "traefik", 1, 0),
	})

	checker.check(targets[0])

	results := checker.Results()
	h, ok := results["traefik"]
	if !ok {
		t.Fatal("expected result for 'traefik', got none")
	}
	if h.Ready {
		t.Errorf("expected ready=false, got ready=true")
	}
}

func TestCheck_DeploymentNotFound(t *testing.T) {
	targets := []config.Target{
		{Name: "traefik", Namespace: "traefik", Deployment: "traefik"},
	}
	checker := newTestChecker(targets, nil)

	checker.check(targets[0])

	results := checker.Results()
	h, ok := results["traefik"]
	if !ok {
		t.Fatal("expected result for 'traefik' even on error, got none")
	}
	if h.Ready {
		t.Errorf("expected ready=false for missing deployment")
	}
}

func TestCheck_CheckAll(t *testing.T) {
	targets := []config.Target{
		{Name: "traefik", Namespace: "traefik", Deployment: "traefik"},
		{Name: "argocd", Namespace: "argocd", Deployment: "argocd-server"},
	}
	checker := newTestChecker(targets, []*appsv1.Deployment{
		deployment("traefik", "traefik", 1, 1),
		deployment("argocd-server", "argocd", 1, 1),
	})

	checker.checkAll()

	results := checker.Results()
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	for _, name := range []string{"traefik", "argocd"} {
		h, ok := results[name]
		if !ok {
			t.Errorf("expected result for %q, got none", name)
		}
		if !h.Ready {
			t.Errorf("expected %q to be ready", name)
		}
	}
}

func TestCheck_ReturnCopy(t *testing.T) {
	checker := newTestChecker(nil, nil)
	checker.results["traefik"] = ServiceHealth{Name: "traefik", Ready: true}

	r1 := checker.Results()
	r1["traefik"] = ServiceHealth{Name: "traefik", Ready: true}

	r2 := checker.Results()
	if !r2["traefik"].Ready {
		t.Error("Results() should return a copy, not a reference to internal state")
	}
}

func TestStart_ChecksOnStartup(t *testing.T) {
	targets := []config.Target{
		{Name: "traefik", Namespace: "traefik", Deployment: "app-1"},
	}
	checker := newTestChecker(targets, []*appsv1.Deployment{
		deployment("traefik", "traefik", 1, 1),
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		checker.Start(ctx, 1*time.Hour)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	results := checker.Results()
	if _, ok := results["traefik"]; !ok {
		t.Error("expected startup check to populate results before first tick")
	}
}

func TestStart_StopsOnContextCancel(t *testing.T) {
	checker := newTestChecker(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		checker.Start(ctx, 1*time.Hour)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("checker did not stop after context cancel")
	}
}
