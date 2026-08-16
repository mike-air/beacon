// Package observability holds logging, and later metrics and tracing, in one
// place. For now it builds the application's structured logger.
//
// Course mapping: Chapter 34 — Structured logging with slog (started early
// because every other package wants a logger from day one).
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
