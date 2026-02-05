package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/ripta/hotpod/internal/queue"
)

// QueueHandlers provides queue endpoint handlers.
type QueueHandlers struct {
	enabled        bool
	queue          *queue.Queue
	workerPool     *queue.WorkerPool
	defaultWorkers int
}

// NewQueueHandlers creates handlers for queue endpoints.
func NewQueueHandlers(enabled bool, q *queue.Queue, defaultWorkers int) *QueueHandlers {
	return &QueueHandlers{
		enabled:        enabled,
		queue:          q,
		workerPool:     queue.NewWorkerPool(q),
		defaultWorkers: defaultWorkers,
	}
}

// Register adds queue routes to the mux.
func (h *QueueHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /queue/enqueue", h.Enqueue)
	mux.HandleFunc("POST /queue/process", h.Process)
	mux.HandleFunc("GET /queue/status", h.Status)
	mux.HandleFunc("POST /queue/clear", h.Clear)
}

// Queue returns the underlying queue for admin operations.
func (h *QueueHandlers) Queue() *queue.Queue {
	return h.queue
}

// WorkerPool returns the worker pool for admin operations.
func (h *QueueHandlers) WorkerPool() *queue.WorkerPool {
	return h.workerPool
}

// EnqueueResponse is the JSON response for /queue/enqueue.
type EnqueueResponse struct {
	Enqueued             int    `json:"enqueued"`
	QueueDepth           int    `json:"queue_depth"`
	EstimatedProcessTime string `json:"estimated_process_time"`
	Rejected             int    `json:"rejected,omitempty"`
	RejectionReason      string `json:"rejection_reason,omitempty"`
}

func (h *QueueHandlers) Enqueue(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		writeError(w, http.StatusForbidden, "QUEUE_DISABLED", "queue endpoints are disabled")
		return
	}

	countStr := r.URL.Query().Get("count")
	count := 1
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "count must be an integer")
			return
		}
		if count < 1 {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "count must be at least 1")
			return
		}
		if count > 10000 {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "count must not exceed 10000")
			return
		}
	}

	processingTime, err := parseDuration(r, "processing_time", 100*time.Millisecond)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}

	priority := r.URL.Query().Get("priority")
	if priority == "" {
		priority = queue.PriorityNormal
	}
	if priority != queue.PriorityHigh && priority != queue.PriorityNormal && priority != queue.PriorityLow {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "priority must be high, normal, or low")
		return
	}

	enqueued := 0
	rejected := 0
	now := time.Now()

	for i := range count {
		item := &queue.Item{
			ID:             fmt.Sprintf("%d-%d", now.UnixNano(), i),
			Priority:       priority,
			ProcessingTime: processingTime,
			EnqueuedAt:     now,
		}

		if err := h.queue.Enqueue(item); err != nil {
			rejected++
		} else {
			enqueued++
		}
	}

	depth := h.queue.Depth()
	estimatedTime := time.Duration(depth) * processingTime

	resp := EnqueueResponse{
		Enqueued:             enqueued,
		QueueDepth:           depth,
		EstimatedProcessTime: estimatedTime.String(),
	}

	if rejected > 0 {
		resp.Rejected = rejected
		resp.RejectionReason = "queue full"
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode enqueue response", "error", err)
	}
}

// ProcessResponse is the JSON response for /queue/process.
type ProcessResponse struct {
	Workers       int    `json:"workers"`
	CPUPerItem    string `json:"cpu_per_item"`
	MemoryPerItem string `json:"memory_per_item"`
	Started       bool   `json:"started"`
}

func (h *QueueHandlers) Process(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		writeError(w, http.StatusForbidden, "QUEUE_DISABLED", "queue endpoints are disabled")
		return
	}

	workersStr := r.URL.Query().Get("workers")
	workers := h.defaultWorkers
	if workersStr != "" {
		var err error
		workers, err = strconv.Atoi(workersStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "workers must be an integer")
			return
		}
		if workers < 1 {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "workers must be at least 1")
			return
		}
		if workers > 100 {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "workers must not exceed 100")
			return
		}
	}

	cpuPerItem, err := parseDuration(r, "cpu_per_item", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}

	memoryPerItem, err := parseSize(r, "memory_per_item", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}

	// XXX: use background context since workers run independently
	h.workerPool.Start(context.Background(), workers, cpuPerItem, memoryPerItem)

	resp := ProcessResponse{
		Workers:       workers,
		CPUPerItem:    cpuPerItem.String(),
		MemoryPerItem: formatSize(memoryPerItem),
		Started:       true,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode process response", "error", err)
	}
}

// StatusResponse is the JSON response for /queue/status.
type StatusResponse struct {
	QueueDepth          int    `json:"queue_depth"`
	HighPriorityDepth   int    `json:"high_priority_depth"`
	NormalPriorityDepth int    `json:"normal_priority_depth"`
	LowPriorityDepth    int    `json:"low_priority_depth"`
	ItemsEnqueuedTotal  int64  `json:"items_enqueued_total"`
	ItemsProcessedTotal int64  `json:"items_processed_total"`
	ItemsFailedTotal    int64  `json:"items_failed_total"`
	ActiveWorkers       int    `json:"active_workers"`
	OldestItemAge       string `json:"oldest_item_age"`
	Paused              bool   `json:"paused"`
}

func (h *QueueHandlers) Status(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		writeError(w, http.StatusForbidden, "QUEUE_DISABLED", "queue endpoints are disabled")
		return
	}

	stats := h.queue.Stats()

	resp := StatusResponse{
		QueueDepth:          stats.Depth,
		HighPriorityDepth:   stats.HighDepth,
		NormalPriorityDepth: stats.NormalDepth,
		LowPriorityDepth:    stats.LowDepth,
		ItemsEnqueuedTotal:  stats.EnqueuedTotal,
		ItemsProcessedTotal: stats.ProcessedTotal,
		ItemsFailedTotal:    stats.FailedTotal,
		ActiveWorkers:       h.workerPool.ActiveWorkers(),
		OldestItemAge:       stats.OldestItemAge.Round(time.Millisecond).String(),
		Paused:              stats.Paused,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode status response", "error", err)
	}
}

// ClearResponse is the JSON response for /queue/clear.
type ClearResponse struct {
	Cleared    int `json:"cleared"`
	QueueDepth int `json:"queue_depth"`
}

func (h *QueueHandlers) Clear(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		writeError(w, http.StatusForbidden, "QUEUE_DISABLED", "queue endpoints are disabled")
		return
	}

	cleared := h.queue.Clear()

	resp := ClearResponse{
		Cleared:    cleared,
		QueueDepth: h.queue.Depth(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode clear response", "error", err)
	}
}
