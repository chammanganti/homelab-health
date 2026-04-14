package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chammanganti/homelab-health/internal/checker"
	"github.com/chammanganti/homelab-health/internal/config"
	"github.com/chammanganti/homelab-health/internal/handler"
	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              os.Getenv("SENTRY_DSN"),
		TracesSampleRate: 1.0,
		EnableTracing:    true,
	}); err != nil {
		slog.Error("sentry initialization failed", "err", err)
	}

	sentryHandler := sentryhttp.New(sentryhttp.Options{})

	checker, err := checker.New(cfg.Targets)
	if err != nil {
		slog.Error("failed to create checker", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go checker.Start(ctx, cfg.Interval)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", sentryHandler.HandleFunc(handler.Health(checker)))
	mux.HandleFunc("GET /health/stream", handler.HealthStream(checker))
	mux.HandleFunc("GET /ready", sentryHandler.HandleFunc(handler.Ready))
	mux.HandleFunc("GET /ready/to_panic/{magic_text}", sentryHandler.HandleFunc(handler.ToPanic))

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}
