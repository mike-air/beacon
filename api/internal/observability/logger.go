// The application's structured logger, built first because every other package
// wants one from day one. The package overview is in doc.go.
//
// Course mapping: Chapter 34 — structured logging with slog.

package observability

import (
	"log/slog"
	"os"
)

// NewLogger returns a slog.Logger. In production it emits JSON (machines read
// it); in development it emits readable text (humans read it).
func NewLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	if env != "production" {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}
