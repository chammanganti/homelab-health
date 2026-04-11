package handler

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/getsentry/sentry-go"
)

func Ready(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func ToPanic(w http.ResponseWriter, r *http.Request) {
	span := sentry.StartSpan(r.Context(), "ready.to_panic")
	defer span.Finish()

	magicText := r.PathValue("magic_text")
	if magicText == os.Getenv("SENTRY_PANIC_MAGIC_TEXT") {
		panic("Uh oh!")
	}

	slog.Warn("safe from panic, invalid magic text")
	http.Error(w, "invalid magic text", http.StatusBadRequest)
}
