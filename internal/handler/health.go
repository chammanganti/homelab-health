package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/chammanganti/homelab-health/internal/checker"
	"github.com/getsentry/sentry-go"
)

type HealthCheck interface {
	Results() map[string]checker.ServiceHealth
	Subscribe() chan map[string]checker.ServiceHealth
	Unsubscribe(ch chan map[string]checker.ServiceHealth)
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

func HealthStream(hc HealthCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", os.Getenv("CORS_ORIGIN"))

		ch := hc.Subscribe()
		defer hc.Unsubscribe(ch)

		if snapshot := hc.Results(); len(snapshot) > 0 {
			data, _ := json.Marshal(snapshot)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		keepalive := time.NewTicker(30 * time.Second)
		defer keepalive.Stop()

		for {
			select {
			case snapshot, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(snapshot)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-keepalive.C:
				fmt.Fprintf(w, ": keepalive\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
