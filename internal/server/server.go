package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/ripta/hotpod/internal/config"
)

// Server is the main HTTP server for hotpod.
type Server struct {
	cfg        *config.Config
	lifecycle  *Lifecycle
	httpServer *http.Server
	mux        *http.ServeMux
}

// New creates a new Server with the given configuration.
func New(cfg *config.Config) *Server {
	lc := NewLifecycle(
		cfg.StartupDelay,
		cfg.StartupJitter,
		cfg.ShutdownDelay,
		cfg.ShutdownTimeout,
		cfg.DrainImmediately,
	)

	mux := http.NewServeMux()

	s := &Server{
		cfg:       cfg,
		lifecycle: lc,
		mux:       mux,
	}

	return s
}

// Lifecycle returns the server's lifecycle manager.
func (s *Server) Lifecycle() *Lifecycle {
	return s.lifecycle
}

// Mux returns the server's ServeMux for registering routes.
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// Run starts the server and blocks until shutdown signal is received.
func (s *Server) Run(ctx context.Context) error {
	var handler http.Handler = s.mux
	handler = Chain(handler,
		DrainCheck(s.lifecycle),
		RequestTracking(s.lifecycle),
		Metrics,
		Recovery,
		Logging,
	)

	if s.cfg.RequestTimeout > 0 {
		handler = http.TimeoutHandler(handler, s.cfg.RequestTimeout, `{"error":"request timeout exceeded","code":"OPERATION_TIMEOUT"}`)
	}

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Port),
		Handler: handler,
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", s.cfg.Port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout+s.cfg.ShutdownDelay+5*time.Second)
	defer cancel()

	go func() {
		if err := s.lifecycle.Shutdown(shutdownCtx); err != nil {
			slog.Warn("lifecycle shutdown interrupted", "error", err)
		}
	}()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	return nil
}
