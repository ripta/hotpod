package main

import (
	"context"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/ripta/hotpod/internal/config"
	"github.com/ripta/hotpod/internal/fault"
	"github.com/ripta/hotpod/internal/handlers"
	"github.com/ripta/hotpod/internal/load"
	"github.com/ripta/hotpod/internal/metrics"
	"github.com/ripta/hotpod/internal/queue"
	"github.com/ripta/hotpod/internal/server"
	"github.com/ripta/hotpod/internal/sidecar"
)

// version is set via ldflags at build time.
var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	initLogger(cfg.LogLevel)

	injector := fault.NewInjector()
	srv := server.New(cfg, injector)

	healthHandlers := handlers.NewHealthHandlers(srv.Lifecycle())
	healthHandlers.Register(srv.Mux())

	metricsHandlers := handlers.NewMetricsHandlers()
	metricsHandlers.Register(srv.Mux())

	infoHandlers := handlers.NewInfoHandlers(version, srv.Lifecycle(), cfg)
	infoHandlers.Register(srv.Mux())

	var runner *sidecar.Runner
	var queueHandlers *handlers.QueueHandlers
	var workQueue *queue.Queue
	var workerPool *queue.WorkerPool

	if cfg.Mode == "sidecar" {
		metrics.SidecarMode.Set(1)
		runner = sidecar.New(cfg.SidecarCPUBaseline, cfg.SidecarCPUJitter, cfg.SidecarMemoryBaseline)
	} else {
		metrics.SidecarMode.Set(0)

		tracker := load.NewTracker(cfg.MaxConcurrentOps)
		latencyHandlers := handlers.NewLatencyHandlers(tracker)
		latencyHandlers.Register(srv.Mux())

		cpuHandlers := handlers.NewCPUHandlers(tracker, cfg)
		cpuHandlers.Register(srv.Mux())

		memoryHandlers := handlers.NewMemoryHandlers(tracker, cfg)
		memoryHandlers.Register(srv.Mux())

		ioHandlers := handlers.NewIOHandlers(tracker, cfg)
		ioHandlers.Register(srv.Mux())

		workHandlers := handlers.NewWorkHandlers(tracker, cfg)
		workHandlers.Register(srv.Mux())

		faultHandlers := handlers.NewFaultHandlers(!cfg.DisableChaos)
		faultHandlers.Register(srv.Mux())

		workQueue = queue.New(cfg.QueueMaxDepth)
		queueHandlers = handlers.NewQueueHandlers(!cfg.DisableQueue, workQueue, cfg.QueueDefaultWorkers)
		queueHandlers.Register(srv.Mux())
		workerPool = queueHandlers.WorkerPool()
	}

	adminHandlers := handlers.NewAdminHandlers(cfg.AdminToken, srv.Lifecycle(), injector, cfg, workQueue, workerPool)
	adminHandlers.Register(srv.Mux())

	if cfg.EnablePprof {
		go startPprof()
	}

	slog.Info("hotpod starting",
		"version", version,
		"mode", cfg.Mode,
		"port", cfg.Port,
		"log_level", cfg.LogLevel,
		"startup_delay", cfg.StartupDelay,
		"startup_jitter", cfg.StartupJitter,
	)

	if runner != nil {
		go runner.Start(context.Background())
	}

	startTime := time.Now()
	if err := srv.Run(context.Background()); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	if runner != nil {
		runner.Stop()
	}
	if queueHandlers != nil {
		queueHandlers.WorkerPool().Stop()
	}
	slog.Info("hotpod shutdown complete", "uptime", time.Since(startTime))
}

func startPprof() {
	slog.Info("pprof server starting", "port", 6060, "bind", "localhost")
	if err := http.ListenAndServe("localhost:6060", nil); err != nil {
		slog.Error("pprof server error", "error", err)
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
