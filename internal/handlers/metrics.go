package handlers

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandlers provides the /metrics endpoint handler.
type MetricsHandlers struct{}

// NewMetricsHandlers creates handlers for the Prometheus metrics endpoint.
func NewMetricsHandlers() *MetricsHandlers {
	return &MetricsHandlers{}
}

// Register adds metrics routes to the mux.
func (h *MetricsHandlers) Register(mux *http.ServeMux) {
	mux.Handle("GET /metrics", promhttp.Handler())
}
