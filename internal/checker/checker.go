package checker

import (
	"context"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/chammanganti/homelab-health/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ServiceHealth struct {
	Name      string    `json:"name"`
	Ready     bool      `json:"ready"`
	CheckedAt time.Time `json:"checked_at"`
}

type Checker struct {
	client      kubernetes.Interface
	targets     []config.Target
	results     map[string]ServiceHealth
	mu          sync.RWMutex
	subscribers map[chan map[string]ServiceHealth]struct{}
	subMu       sync.Mutex
}

func New(targets []config.Target) (*Checker, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Checker{
		client:      clientset,
		targets:     targets,
		results:     make(map[string]ServiceHealth),
		subscribers: make(map[chan map[string]ServiceHealth]struct{}),
	}, nil
}

func (c *Checker) Start(ctx context.Context, interval time.Duration) {
	slog.Info("checker starting", "interval", interval)
	c.checkAll()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkAll()
		case <-ctx.Done():
			slog.Info("checker stopped")
			return
		}
	}
}

func (c *Checker) checkAll() {
	for _, target := range c.targets {
		slog.Info("checking service", "service", target.Name, "namespace", target.Namespace)
		c.check(target)
	}
}

func (c *Checker) check(target config.Target) {
	deployment, err := c.client.AppsV1().Deployments(target.Namespace).Get(context.Background(), target.Deployment, metav1.GetOptions{})
	if err != nil {
		slog.Warn("failed to get deployment", "name", target.Name, "err", err)
		c.mu.Lock()
		c.results[target.Name] = ServiceHealth{
			Name:      target.Deployment,
			Ready:     false,
			CheckedAt: time.Now(),
		}
		c.mu.Unlock()
		c.notify()
		return
	}

	health := ServiceHealth{
		Name:      deployment.Name,
		Ready:     deployment.Status.AvailableReplicas == *deployment.Spec.Replicas,
		CheckedAt: time.Now(),
	}

	c.mu.Lock()
	c.results[target.Name] = health
	c.mu.Unlock()

	c.notify()
	slog.Info("checked deployment", "name", health.Name, "ready", health.Ready)
}

func (c *Checker) Results() map[string]ServiceHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return maps.Clone(c.results)
}

func (c *Checker) Subscribe() chan map[string]ServiceHealth {
	ch := make(chan map[string]ServiceHealth, 1)
	c.subMu.Lock()
	c.subscribers[ch] = struct{}{}
	c.subMu.Unlock()
	return ch
}

func (c *Checker) Unsubscribe(ch chan map[string]ServiceHealth) {
	c.subMu.Lock()
	delete(c.subscribers, ch)
	close(ch)
	c.subMu.Unlock()
}

func (c *Checker) notify() {
	snapshot := c.Results()
	c.subMu.Lock()
	defer c.subMu.Unlock()
	for ch := range c.subscribers {
		select {
		case ch <- snapshot:
		default:
		}
	}
}
