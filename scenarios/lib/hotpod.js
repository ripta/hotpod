import http from "k6/http";
import { check, sleep } from "k6";

const BASE_URL = __ENV.HOTPOD_BASE_URL || "http://localhost:8080";
const ADMIN_TOKEN = __ENV.HOTPOD_ADMIN_TOKEN || "";

function adminHeaders() {
  const h = { "Content-Type": "application/json" };
  if (ADMIN_TOKEN) {
    h["X-Admin-Token"] = ADMIN_TOKEN;
  }
  return h;
}

// ---------------------------------------------------------------------------
// Load Generation
// ---------------------------------------------------------------------------

export function cpu(duration, cores, intensity) {
  const params = [];
  if (duration !== undefined) params.push(`duration=${duration}`);
  if (cores !== undefined) params.push(`cores=${cores}`);
  if (intensity !== undefined) params.push(`intensity=${intensity}`);
  const qs = params.length > 0 ? `?${params.join("&")}` : "";
  return http.get(`${BASE_URL}/cpu${qs}`);
}

export function memory(size, duration, pattern) {
  const params = [];
  if (size !== undefined) params.push(`size=${size}`);
  if (duration !== undefined) params.push(`duration=${duration}`);
  if (pattern !== undefined) params.push(`pattern=${pattern}`);
  const qs = params.length > 0 ? `?${params.join("&")}` : "";
  return http.get(`${BASE_URL}/memory${qs}`);
}

export function io(size, operation, sync) {
  const params = [];
  if (size !== undefined) params.push(`size=${size}`);
  if (operation !== undefined) params.push(`operation=${operation}`);
  if (sync !== undefined) params.push(`sync=${sync}`);
  const qs = params.length > 0 ? `?${params.join("&")}` : "";
  return http.get(`${BASE_URL}/io${qs}`);
}

export function work(profile, variance) {
  const params = [];
  if (profile !== undefined) params.push(`profile=${profile}`);
  if (variance !== undefined) params.push(`variance=${variance}`);
  const qs = params.length > 0 ? `?${params.join("&")}` : "";
  return http.get(`${BASE_URL}/work${qs}`);
}

export function latency(duration, jitter, status) {
  const params = [];
  if (duration !== undefined) params.push(`duration=${duration}`);
  if (jitter !== undefined) params.push(`jitter=${jitter}`);
  if (status !== undefined) params.push(`status=${status}`);
  const qs = params.length > 0 ? `?${params.join("&")}` : "";
  return http.get(`${BASE_URL}/latency${qs}`);
}

// ---------------------------------------------------------------------------
// Queue
// ---------------------------------------------------------------------------

export function enqueueItems(count, processingTime, priority) {
  const params = [];
  if (count !== undefined) params.push(`count=${count}`);
  if (processingTime !== undefined) params.push(`processing_time=${processingTime}`);
  if (priority !== undefined) params.push(`priority=${priority}`);
  const qs = params.length > 0 ? `?${params.join("&")}` : "";
  return http.post(`${BASE_URL}/queue/enqueue${qs}`, null, {
    headers: adminHeaders(),
  });
}

export function startProcessing(workers, cpuPerItem, memoryPerItem) {
  const params = [];
  if (workers !== undefined) params.push(`workers=${workers}`);
  if (cpuPerItem !== undefined) params.push(`cpu_per_item=${cpuPerItem}`);
  if (memoryPerItem !== undefined) params.push(`memory_per_item=${memoryPerItem}`);
  const qs = params.length > 0 ? `?${params.join("&")}` : "";
  return http.post(`${BASE_URL}/queue/process${qs}`, null, {
    headers: adminHeaders(),
  });
}

export function queueStatus() {
  return http.get(`${BASE_URL}/queue/status`);
}

export function clearQueue() {
  return http.post(`${BASE_URL}/queue/clear`, null, {
    headers: adminHeaders(),
  });
}

export function pauseQueue() {
  return http.post(`${BASE_URL}/admin/queue/pause`, null, {
    headers: adminHeaders(),
  });
}

export function resumeQueue() {
  return http.post(`${BASE_URL}/admin/queue/resume`, null, {
    headers: adminHeaders(),
  });
}

export function waitForQueueDrain(timeoutSeconds, pollInterval) {
  const timeout = timeoutSeconds || 120;
  const interval = pollInterval || 2;
  const deadline = Date.now() + timeout * 1000;

  while (Date.now() < deadline) {
    const res = queueStatus();
    if (res.status === 200) {
      const body = JSON.parse(res.body);
      if (body.queue_depth === 0) {
        return true;
      }
    }
    sleep(interval);
  }
  return false;
}

// ---------------------------------------------------------------------------
// Chaos / Fault Injection
// ---------------------------------------------------------------------------

export function triggerCrash(delay, exitCode) {
  const params = [];
  if (delay !== undefined) params.push(`delay=${delay}`);
  if (exitCode !== undefined) params.push(`exit_code=${exitCode}`);
  const qs = params.length > 0 ? `?${params.join("&")}` : "";
  return http.post(`${BASE_URL}/fault/crash${qs}`, null, {
    headers: adminHeaders(),
  });
}

export function triggerHang(duration, partial) {
  const params = [];
  if (duration !== undefined) params.push(`duration=${duration}`);
  if (partial !== undefined) params.push(`partial=${partial}`);
  const qs = params.length > 0 ? `?${params.join("&")}` : "";
  return http.post(`${BASE_URL}/fault/hang${qs}`, null, {
    headers: adminHeaders(),
    timeout: "600s",
  });
}

export function triggerOOM(rate) {
  const params = [];
  if (rate !== undefined) params.push(`rate=${rate}`);
  const qs = params.length > 0 ? `?${params.join("&")}` : "";
  return http.post(`${BASE_URL}/fault/oom${qs}`, null, {
    headers: adminHeaders(),
  });
}

export function faultError(rate, status) {
  const params = [];
  if (rate !== undefined) params.push(`rate=${rate}`);
  if (status !== undefined) params.push(`status=${status}`);
  const qs = params.length > 0 ? `?${params.join("&")}` : "";
  return http.get(`${BASE_URL}/fault/error${qs}`);
}

// ---------------------------------------------------------------------------
// Admin
// ---------------------------------------------------------------------------

export function setErrorRate(endpoint, rate, codes, duration) {
  const params = [];
  if (endpoint !== undefined) params.push(`endpoint=${endpoint}`);
  if (rate !== undefined) params.push(`rate=${rate}`);
  if (codes !== undefined) params.push(`codes=${codes}`);
  if (duration !== undefined) params.push(`duration=${duration}`);
  const qs = params.length > 0 ? `?${params.join("&")}` : "";
  return http.post(`${BASE_URL}/admin/error-rate${qs}`, null, {
    headers: adminHeaders(),
  });
}

export function clearErrorRate() {
  return http.post(`${BASE_URL}/admin/error-rate?rate=0`, null, {
    headers: adminHeaders(),
  });
}

export function resetAll() {
  return http.post(`${BASE_URL}/admin/reset`, null, {
    headers: adminHeaders(),
  });
}

export function setReady(state) {
  const qs = state !== undefined ? `?state=${state}` : "";
  return http.post(`${BASE_URL}/admin/ready${qs}`, null, {
    headers: adminHeaders(),
  });
}

export function getConfig() {
  return http.get(`${BASE_URL}/admin/config`, {
    headers: adminHeaders(),
  });
}

export function forceGC() {
  return http.post(`${BASE_URL}/admin/gc`, null, {
    headers: adminHeaders(),
  });
}

// ---------------------------------------------------------------------------
// Health / Info
// ---------------------------------------------------------------------------

export function healthz() {
  return http.get(`${BASE_URL}/healthz`);
}

export function readyz() {
  return http.get(`${BASE_URL}/readyz`);
}

export function info() {
  return http.get(`${BASE_URL}/info`);
}
