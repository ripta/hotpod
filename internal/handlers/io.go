package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ripta/hotpod/internal/config"
	"github.com/ripta/hotpod/internal/load"
)

const (
	ioOpWrite = "write"
	ioOpRead  = "read"
	ioOpMixed = "mixed"

	ioBlockSize = 64 * 1024 // 64KB blocks for I/O operations
)

// IOHandlers provides the /io endpoint handler.
type IOHandlers struct {
	tracker *load.Tracker
	maxSize int64
	ioPath  string
}

// NewIOHandlers creates handlers for I/O load endpoints.
func NewIOHandlers(tracker *load.Tracker, cfg *config.Config) *IOHandlers {
	return &IOHandlers{
		tracker: tracker,
		maxSize: cfg.MaxIOSize,
		ioPath:  cfg.IOPath(),
	}
}

// Register adds I/O load routes to the mux.
func (h *IOHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /io", h.IO)
}

// IOResponse is the JSON response for /io.
type IOResponse struct {
	// RequestedSize is the size parameter value in bytes
	RequestedSize int64 `json:"requested_size"`
	// RequestedSizeHuman is the human-readable size
	RequestedSizeHuman string `json:"requested_size_human"`
	// Operation is the I/O operation type
	Operation string `json:"operation"`
	// Sync indicates if fsync was used
	Sync bool `json:"sync"`
	// ActualDuration is how long the operation took
	ActualDuration string `json:"actual_duration"`
	// BytesWritten is the number of bytes written
	BytesWritten int64 `json:"bytes_written,omitempty"`
	// BytesRead is the number of bytes read
	BytesRead int64 `json:"bytes_read,omitempty"`
	// Cancelled indicates if the operation was cancelled
	Cancelled bool `json:"cancelled,omitempty"`
	// LimitApplied indicates if the size was capped by the safety limit
	LimitApplied bool `json:"limit_applied,omitempty"`
}

func (h *IOHandlers) IO(w http.ResponseWriter, r *http.Request) {
	size, err := parseSize(r, "size", 10<<20)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}
	if size < 0 {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "size must be non-negative")
		return
	}

	operation := r.URL.Query().Get("operation")
	if operation == "" {
		operation = ioOpWrite
	}
	if operation != ioOpWrite && operation != ioOpRead && operation != ioOpMixed {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "operation must be write, read, or mixed")
		return
	}

	syncParam := r.URL.Query().Get("sync")
	doSync := false
	if syncParam != "" {
		doSync, err = strconv.ParseBool(syncParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "sync must be true or false")
			return
		}
	}

	limitApplied := false
	if h.maxSize > 0 && size > h.maxSize {
		size = h.maxSize
		limitApplied = true
	}

	release, err := h.tracker.Acquire(load.OpTypeIO)
	if err != nil {
		writeError(w, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "concurrent operation limit exceeded")
		return
	}
	defer release()

	start := time.Now()
	bytesWritten, bytesRead, cancelled := h.performIO(r.Context(), size, operation, doSync)
	elapsed := time.Since(start)

	resp := IOResponse{
		RequestedSize:      size,
		RequestedSizeHuman: formatSize(size),
		Operation:          operation,
		Sync:               doSync,
		ActualDuration:     elapsed.String(),
		BytesWritten:       bytesWritten,
		BytesRead:          bytesRead,
		Cancelled:          cancelled,
		LimitApplied:       limitApplied,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode io response", "error", err)
	}
}

func (h *IOHandlers) performIO(ctx context.Context, size int64, operation string, doSync bool) (bytesWritten, bytesRead int64, cancelled bool) {
	if err := os.MkdirAll(h.ioPath, 0750); err != nil {
		slog.Error("failed to create I/O directory", "path", h.ioPath, "error", err)
		return 0, 0, false
	}

	filename := filepath.Join(h.ioPath, fmt.Sprintf("hotpod-%d-%d.tmp", time.Now().UnixNano(), rand.Uint64()))
	defer func() {
		if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to remove temp file", "file", filename, "error", err)
		}
	}()

	switch operation {
	case ioOpWrite:
		bytesWritten, cancelled = h.writeFile(ctx, filename, size, doSync)
	case ioOpRead:
		bytesWritten, cancelled = h.writeFile(ctx, filename, size, false)
		if !cancelled {
			bytesRead, cancelled = h.readFile(ctx, filename, size)
		}
	case ioOpMixed:
		bytesWritten, bytesRead, cancelled = h.mixedIO(ctx, filename, size, doSync)
	}

	return bytesWritten, bytesRead, cancelled
}

func (h *IOHandlers) writeFile(ctx context.Context, filename string, size int64, doSync bool) (bytesWritten int64, cancelled bool) {
	f, err := os.Create(filename)
	if err != nil {
		slog.Error("failed to create file", "file", filename, "error", err)
		return 0, false
	}
	defer f.Close()

	data := make([]byte, ioBlockSize)
	fillMemory(data, patternRandom)

	remaining := size
	for remaining > 0 {
		select {
		case <-ctx.Done():
			return bytesWritten, true
		default:
		}

		toWrite := min(int64(len(data)), remaining)

		n, err := f.Write(data[:toWrite])
		if err != nil {
			slog.Error("failed to write to file", "file", filename, "error", err)
			return bytesWritten, false
		}
		bytesWritten += int64(n)
		remaining -= int64(n)
	}

	if doSync {
		if err := f.Sync(); err != nil {
			slog.Error("failed to sync file", "file", filename, "error", err)
		}
	}

	return bytesWritten, false
}

func (h *IOHandlers) readFile(ctx context.Context, filename string, size int64) (bytesRead int64, cancelled bool) {
	f, err := os.Open(filename)
	if err != nil {
		slog.Error("failed to open file for reading", "file", filename, "error", err)
		return 0, false
	}
	defer f.Close()

	data := make([]byte, ioBlockSize)

	remaining := size
	for remaining > 0 {
		select {
		case <-ctx.Done():
			return bytesRead, true
		default:
		}

		toRead := min(int64(len(data)), remaining)

		n, err := f.Read(data[:toRead])
		if err != nil {
			if n > 0 {
				bytesRead += int64(n)
			}
			break
		}

		bytesRead += int64(n)
		remaining -= int64(n)
	}

	return bytesRead, false
}

func (h *IOHandlers) mixedIO(ctx context.Context, filename string, size int64, doSync bool) (bytesWritten, bytesRead int64, cancelled bool) {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		slog.Error("failed to create file for mixed I/O", "file", filename, "error", err)
		return 0, 0, false
	}
	defer f.Close()

	writeData := make([]byte, ioBlockSize)
	fillMemory(writeData, patternRandom)
	readBuf := make([]byte, ioBlockSize)

	remaining := size
	writePhase := true

	for remaining > 0 {
		select {
		case <-ctx.Done():
			return bytesWritten, bytesRead, true
		default:
		}

		blockSize := min(int64(ioBlockSize), remaining)

		if writePhase {
			n, err := f.Write(writeData[:blockSize])
			if err != nil {
				slog.Error("failed to write in mixed mode", "file", filename, "error", err)
				return bytesWritten, bytesRead, false
			}
			bytesWritten += int64(n)
			remaining -= int64(n)

			if doSync {
				if err := f.Sync(); err != nil {
					slog.Error("failed to sync in mixed mode", "file", filename, "error", err)
				}
			}
		} else {
			// Seek back to read what we just wrote
			if _, err := f.Seek(-blockSize, 1); err != nil {
				slog.Error("failed to seek for read in mixed mode", "file", filename, "error", err)
				return bytesWritten, bytesRead, false
			}

			n, err := f.Read(readBuf[:blockSize])
			if err != nil {
				slog.Error("failed to read in mixed mode", "file", filename, "error", err)
				return bytesWritten, bytesRead, false
			}
			bytesRead += int64(n)

			// Seek forward to continue writing
			if _, err := f.Seek(0, 2); err != nil {
				slog.Error("failed to seek to end in mixed mode", "file", filename, "error", err)
				return bytesWritten, bytesRead, false
			}
		}

		writePhase = !writePhase
	}

	return bytesWritten, bytesRead, false
}
