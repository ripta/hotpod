package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ripta/hotpod/internal/queue"
)

type endpoint struct {
	method string
	path   string
}

var queueEndpoints = []endpoint{
	{"POST", "/queue/enqueue"},
	{"POST", "/queue/process"},
	{"GET", "/queue/status"},
	{"POST", "/queue/clear"},
}

func TestQueueEnqueueDisabled(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(false, q, 1)

	req := httptest.NewRequest("POST", "/queue/enqueue", nil)
	rec := httptest.NewRecorder()

	h.Enqueue(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestQueueEnqueueDefault(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(true, q, 1)

	req := httptest.NewRequest("POST", "/queue/enqueue", nil)
	rec := httptest.NewRecorder()

	h.Enqueue(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp EnqueueResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Enqueued != 1 {
		t.Errorf("enqueued = %d, want 1", resp.Enqueued)
	}
	if resp.QueueDepth != 1 {
		t.Errorf("queue_depth = %d, want 1", resp.QueueDepth)
	}
}

func TestQueueEnqueueMultiple(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(true, q, 1)

	req := httptest.NewRequest("POST", "/queue/enqueue?count=10&priority=high", nil)
	rec := httptest.NewRecorder()

	h.Enqueue(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp EnqueueResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Enqueued != 10 {
		t.Errorf("enqueued = %d, want 10", resp.Enqueued)
	}
}

func TestQueueEnqueueInvalidCount(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(true, q, 1)

	testCases := []string{"invalid", "0", "-1", "10001"}
	for _, count := range testCases {
		req := httptest.NewRequest("POST", "/queue/enqueue?count="+count, nil)
		rec := httptest.NewRecorder()

		h.Enqueue(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("count=%s: status = %d, want %d", count, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestQueueEnqueueInvalidPriority(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(true, q, 1)

	req := httptest.NewRequest("POST", "/queue/enqueue?priority=invalid", nil)
	rec := httptest.NewRecorder()

	h.Enqueue(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestQueueEnqueueQueueFull(t *testing.T) {
	q := queue.New(5)
	h := NewQueueHandlers(true, q, 1)

	req := httptest.NewRequest("POST", "/queue/enqueue?count=10", nil)
	rec := httptest.NewRecorder()

	h.Enqueue(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp EnqueueResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Enqueued != 5 {
		t.Errorf("enqueued = %d, want 5", resp.Enqueued)
	}
	if resp.Rejected != 5 {
		t.Errorf("rejected = %d, want 5", resp.Rejected)
	}
}

func TestQueueProcessDisabled(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(false, q, 1)

	req := httptest.NewRequest("POST", "/queue/process", nil)
	rec := httptest.NewRecorder()

	h.Process(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestQueueProcessDefault(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(true, q, 2)
	t.Cleanup(func() { h.WorkerPool().Stop() })

	req := httptest.NewRequest("POST", "/queue/process", nil)
	rec := httptest.NewRecorder()

	h.Process(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp ProcessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Workers != 2 {
		t.Errorf("workers = %d, want 2 (default)", resp.Workers)
	}
	if !resp.Started {
		t.Error("expected started = true")
	}
}

func TestQueueProcessInvalidWorkers(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(true, q, 1)

	testCases := []string{"invalid", "0", "-1", "101"}
	for _, workers := range testCases {
		req := httptest.NewRequest("POST", "/queue/process?workers="+workers, nil)
		rec := httptest.NewRecorder()

		h.Process(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("workers=%s: status = %d, want %d", workers, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestQueueStatusDisabled(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(false, q, 1)

	req := httptest.NewRequest("GET", "/queue/status", nil)
	rec := httptest.NewRecorder()

	h.Status(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestQueueStatus(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(true, q, 1)

	for i := range 5 {
		item := &queue.Item{ID: string(rune('a' + i)), Priority: queue.PriorityNormal}
		if err := q.Enqueue(item); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}
	}

	req := httptest.NewRequest("GET", "/queue/status", nil)
	rec := httptest.NewRecorder()

	h.Status(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp StatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.QueueDepth != 5 {
		t.Errorf("queue_depth = %d, want 5", resp.QueueDepth)
	}
	if resp.ItemsEnqueuedTotal != 5 {
		t.Errorf("items_enqueued_total = %d, want 5", resp.ItemsEnqueuedTotal)
	}
}

func TestQueueClearDisabled(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(false, q, 1)

	req := httptest.NewRequest("POST", "/queue/clear", nil)
	rec := httptest.NewRecorder()

	h.Clear(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestQueueClear(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(true, q, 1)

	for i := range 5 {
		item := &queue.Item{ID: string(rune('a' + i)), Priority: queue.PriorityNormal}
		if err := q.Enqueue(item); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}
	}

	req := httptest.NewRequest("POST", "/queue/clear", nil)
	rec := httptest.NewRecorder()

	h.Clear(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp ClearResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Cleared != 5 {
		t.Errorf("cleared = %d, want 5", resp.Cleared)
	}
	if resp.QueueDepth != 0 {
		t.Errorf("queue_depth = %d, want 0", resp.QueueDepth)
	}
}

func TestQueueRegister(t *testing.T) {
	q := queue.New(100)
	h := NewQueueHandlers(false, q, 1)

	mux := http.NewServeMux()
	h.Register(mux)

	for _, ep := range queueEndpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want %d (route should be registered)", ep.method, ep.path, rec.Code, http.StatusForbidden)
		}
	}
}
