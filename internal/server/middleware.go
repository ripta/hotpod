package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/ripta/hotpod/internal/metrics"
)

// responseWriter wraps http.ResponseWriter to capture status code.
//
// TODO(ripta): No support for http.Hijacker, http.Flusher, or http.CloseNotifier
type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.statusCode = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

// Logging returns middleware that logs requests.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration", time.Since(start),
			"remote", r.RemoteAddr,
		)
	})
}

// Recovery returns middleware that recovers from panics.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				http.Error(w, `{"error":"internal server error","code":"INTERNAL_ERROR"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RequestTracking returns middleware that tracks in-flight requests.
func RequestTracking(lc *Lifecycle) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			done := lc.TrackRequest()
			defer done()
			next.ServeHTTP(w, r)
		})
	}
}

// DrainCheck returns middleware that rejects requests when draining.
func DrainCheck(lc *Lifecycle) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if lc.ShouldRejectRequest() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				if _, err := w.Write([]byte(`{"error":"server is shutting down","code":"OPERATION_TIMEOUT"}`)); err != nil {
					slog.Warn("failed to write drain response", "error", err)
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Metrics returns middleware that records Prometheus metrics.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics.InFlightRequests.Inc()
		defer metrics.InFlightRequests.Dec()

		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		endpoint := normalizeEndpoint(r.URL.Path)
		status := strconv.Itoa(rw.statusCode)

		metrics.RequestsTotal.WithLabelValues(endpoint, status).Inc()
		metrics.RequestDuration.WithLabelValues(endpoint).Observe(duration)
	})
}

// normalizeEndpoint maps request paths to known routes to prevent unbounded
// cardinality in Prometheus metrics. Unknown paths are grouped as "unknown".
func normalizeEndpoint(path string) string {
	switch {
	case path == "/healthz":
		return "/healthz"
	case path == "/readyz":
		return "/readyz"
	case path == "/startupz":
		return "/startupz"
	case path == "/metrics":
		return "/metrics"
	case path == "/info":
		return "/info"
	case path == "/cpu":
		return "/cpu"
	case path == "/memory":
		return "/memory"
	case path == "/io":
		return "/io"
	case path == "/work":
		return "/work"
	case path == "/latency":
		return "/latency"
	case strings.HasPrefix(path, "/queue/"):
		return "/queue/*"
	case strings.HasPrefix(path, "/fault/"):
		return "/fault/*"
	case strings.HasPrefix(path, "/admin/"):
		return "/admin/*"
	default:
		return "unknown"
	}
}

// Chain applies middlewares in order (first middleware wraps outermost).
func Chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
