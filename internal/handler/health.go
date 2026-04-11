package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/chammanganti/homelab-health/internal/checker"
	"github.com/getsentry/sentry-go"
)

type HealthCheck interface {
	Results() map[string]checker.ServiceHealth
}

func Health(hc HealthCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		span := sentry.StartSpan(r.Context(), "handler.health")
		defer span.Finish()

		checkSpan := sentry.StartSpan(r.Context(), "checker.results")
		results := hc.Results()
		checkSpan.Finish()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", os.Getenv("CORS_ORIGIN"))

		if err := json.NewEncoder(w).Encode(results); err != nil {
			slog.Error("failed to encode health results", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		slog.Info("health requested", "remote", r.RemoteAddr, "results", len(results))
	}
}
