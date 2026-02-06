package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Namespace is the Prometheus metrics namespace for all hotpod metrics.
const Namespace = "hotpod"

// Request metrics track HTTP request handling.
var (
	// RequestsTotal counts total HTTP requests by endpoint and status code.
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "requests_total",
			Help:      "Total number of HTTP requests by endpoint and status code.",
		},
		[]string{"endpoint", "status"},
	)

	// RequestDuration tracks request duration in seconds by endpoint.
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds by endpoint.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	// InFlightRequests tracks currently processing requests.
	InFlightRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "in_flight_requests",
			Help:      "Number of HTTP requests currently being processed.",
		},
	)
)

// Resource consumption metrics track load generation operations.
var (
	// CPUSecondsTotal counts total CPU time consumed by /cpu endpoint.
	CPUSecondsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "cpu_seconds_total",
			Help:      "Total CPU time consumed by load generation in seconds.",
		},
	)

	// MemoryAllocatedBytes tracks currently allocated memory for load generation.
	MemoryAllocatedBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "memory_allocated_bytes",
			Help:      "Bytes currently allocated for memory load generation.",
		},
	)

	// IOBytesTotal counts total bytes transferred by I/O operations.
	IOBytesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "io_bytes_total",
			Help:      "Total bytes transferred by I/O operations.",
		},
		[]string{"operation"},
	)
)

// Active operations metrics track concurrent load operations.
var (
	// ActiveCPUOperations tracks concurrent CPU load operations.
	ActiveCPUOperations = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "active_cpu_operations",
			Help:      "Number of concurrent CPU load operations.",
		},
	)

	// ActiveMemoryAllocations tracks concurrent memory allocations.
	ActiveMemoryAllocations = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "active_memory_allocations",
			Help:      "Number of concurrent memory allocations.",
		},
	)

	// ActiveIOOperations tracks concurrent I/O operations.
	ActiveIOOperations = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "active_io_operations",
			Help:      "Number of concurrent I/O operations.",
		},
	)
)

// Lifecycle metrics track server startup and shutdown state.
var (
	// StartupComplete indicates whether the server has completed startup (0 or 1).
	StartupComplete = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "startup_complete",
			Help:      "Whether the server has completed startup (0 or 1).",
		},
	)

	// StartupDurationSeconds records the startup duration.
	StartupDurationSeconds = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "startup_duration_seconds",
			Help:      "Time taken for server to become ready in seconds.",
		},
	)

	// ShutdownInProgress indicates whether shutdown is in progress (0 or 1).
	ShutdownInProgress = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "shutdown_in_progress",
			Help:      "Whether the server is shutting down (0 or 1).",
		},
	)

	// ShutdownStartedTimestamp records when shutdown started as Unix timestamp.
	ShutdownStartedTimestamp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "shutdown_started_timestamp_seconds",
			Help:      "Unix timestamp when shutdown started (0 if not shutting down).",
		},
	)
)

// Fault injection metrics track chaos engineering operations.
var (
	// FaultErrorsInjectedTotal counts errors injected by endpoint and status.
	FaultErrorsInjectedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "fault_errors_injected_total",
			Help:      "Total number of errors injected by fault injection.",
		},
		[]string{"endpoint", "status"},
	)

	// FaultErrorRate tracks the configured error rate by endpoint.
	FaultErrorRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "fault_error_rate",
			Help:      "Configured error injection rate by endpoint.",
		},
		[]string{"endpoint"},
	)
)

// Sidecar metrics track resource consumption in sidecar mode.
var (
	// SidecarCPUBurnSecondsTotal counts total CPU time burned by sidecar mode.
	SidecarCPUBurnSecondsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "sidecar_cpu_burn_seconds_total",
			Help:      "Total CPU time burned by sidecar mode in seconds.",
		},
	)

	// SidecarMemoryHeldBytes tracks memory held by sidecar mode.
	SidecarMemoryHeldBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "sidecar_memory_held_bytes",
			Help:      "Bytes currently held by sidecar memory allocation.",
		},
	)

	// SidecarMode indicates whether sidecar mode is active (0 or 1).
	SidecarMode = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "sidecar_mode",
			Help:      "Whether the server is running in sidecar mode (0 or 1).",
		},
	)
)

// Queue metrics track work queue state for KEDA/external HPA scaling.
var (
	// QueueDepth tracks the total number of items in the queue.
	QueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "queue_depth",
			Help:      "Total number of items in the queue.",
		},
	)

	// QueueDepthByPriority tracks queue depth by priority level.
	QueueDepthByPriority = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "queue_depth_by_priority",
			Help:      "Number of items in the queue by priority.",
		},
		[]string{"priority"},
	)

	// QueueItemsEnqueuedTotal tracks total items ever enqueued.
	QueueItemsEnqueuedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "queue_items_enqueued_total",
			Help:      "Total number of items enqueued.",
		},
	)

	// QueueItemsProcessedTotal counts items successfully processed.
	QueueItemsProcessedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "queue_items_processed_total",
			Help:      "Total number of items processed successfully.",
		},
	)

	// QueueItemsFailedTotal counts items that failed processing.
	QueueItemsFailedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "queue_items_failed_total",
			Help:      "Total number of items that failed processing.",
		},
	)

	// QueueActiveWorkers tracks the number of workers currently processing items.
	QueueActiveWorkers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "queue_active_workers",
			Help:      "Number of workers currently processing items.",
		},
	)

	// QueueProcessingSeconds tracks item processing duration.
	QueueProcessingSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "queue_processing_seconds",
			Help:      "Time spent processing queue items.",
			Buckets:   prometheus.DefBuckets,
		},
	)

	// QueueOldestItemAgeSeconds tracks the age of the oldest item in the queue.
	QueueOldestItemAgeSeconds = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "queue_oldest_item_age_seconds",
			Help:      "Age of the oldest item in the queue in seconds.",
		},
	)
)
