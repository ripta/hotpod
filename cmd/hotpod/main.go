package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/ripta/hotpod/internal/config"
	"github.com/ripta/hotpod/internal/handlers"
	"github.com/ripta/hotpod/internal/load"
	"github.com/ripta/hotpod/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	initLogger(cfg.LogLevel)

	srv := server.New(cfg)

	healthHandlers := handlers.NewHealthHandlers(srv.Lifecycle())
	healthHandlers.Register(srv.Mux())

	tracker := load.NewTracker(cfg.MaxConcurrentOps)
	latencyHandlers := handlers.NewLatencyHandlers(tracker)
	latencyHandlers.Register(srv.Mux())

	cpuHandlers := handlers.NewCPUHandlers(tracker, cfg)
	cpuHandlers.Register(srv.Mux())

	memoryHandlers := handlers.NewMemoryHandlers(tracker, cfg)
	memoryHandlers.Register(srv.Mux())

	slog.Info("hotpod starting",
		"port", cfg.Port,
		"log_level", cfg.LogLevel,
		"startup_delay", cfg.StartupDelay,
		"startup_jitter", cfg.StartupJitter,
	)

	if err := srv.Run(context.Background()); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func initLogger(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))
}
