package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chammanganti/homelab-health/internal/checker"
)

type mockChecker struct {
	results   map[string]checker.ServiceHealth
	subscribe chan map[string]checker.ServiceHealth
}

func (m *mockChecker) Results() map[string]checker.ServiceHealth {
	return m.results
}

func (m *mockChecker) Subscribe() chan map[string]checker.ServiceHealth {
	return m.subscribe
}

func (m *mockChecker) Unsubscribe(ch chan map[string]checker.ServiceHealth) {
}

func TestHealth(t *testing.T) {
	mock := &mockChecker{
		results: map[string]checker.ServiceHealth{
			"traefik": {Name: "traefik", Ready: true, CheckedAt: time.Now()},
			"argocd":  {Name: "argocd", Ready: false, CheckedAt: time.Now()},
		},
	}

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	Health(mock)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]checker.ServiceHealth
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp["traefik"].Ready {
		t.Error("expected traefik to be ready")
	}
	if resp["argocd"].Ready {
		t.Error("expected argocd to not be ready")
	}
}
