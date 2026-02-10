// Queue Backlog Burst
//
// Pauses the queue, enqueues 500 items, resumes processing, and observes
// HPA scale-up driven by queue depth. Use with a KEDA or external-metrics HPA
// that scales on hotpod_queue_depth.
//
// Usage:
//   k6 run scenarios/scripts/scaling/queue-backlog-burst.js
//
// Designed for: manifests/overlays/keda or manifests/overlays/hpa-queue-external

import { check, sleep } from "k6";
import {
  enqueueItems,
  startProcessing,
  pauseQueue,
  resumeQueue,
  queueStatus,
  waitForQueueDrain,
  resetAll,
} from "../../lib/hotpod.js";

export const options = {
  scenarios: {
    orchestrator: {
      executor: "shared-iterations",
      vus: 1,
      iterations: 1,
      maxDuration: "10m",
    },
    monitor: {
      executor: "constant-vus",
      vus: 1,
      duration: "5m",
      exec: "monitorQueue",
    },
  },
  thresholds: {
    checks: ["rate>0.9"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

export default function () {
  // Pause the queue so items accumulate
  const pauseRes = pauseQueue();
  check(pauseRes, { "queue paused": (r) => r.status === 200 });
  sleep(2);

  // Enqueue 500 items in batches of 100
  for (let i = 0; i < 5; i++) {
    const res = enqueueItems(100, "200ms", "normal");
    check(res, { "batch enqueued": (r) => r.status === 200 });
  }

  // Verify depth
  const status = queueStatus();
  check(status, {
    "queue has items": (r) => {
      const body = JSON.parse(r.body);
      return body.queue_depth >= 400;
    },
  });

  // Resume processing with 4 workers
  const resumeRes = resumeQueue();
  check(resumeRes, { "queue resumed": (r) => r.status === 200 });

  const processRes = startProcessing(4, "10ms", "1MB");
  check(processRes, { "processing started": (r) => r.status === 200 });

  // Wait for queue to drain
  const drained = waitForQueueDrain(300, 5);
  check(null, { "queue drained": () => drained });
}

export function monitorQueue() {
  const res = queueStatus();
  if (res.status === 200) {
    const body = JSON.parse(res.body);
    console.log(
      `queue_depth=${body.queue_depth} active_workers=${body.active_workers} processed=${body.items_processed_total}`
    );
  }
  sleep(5);
}

export function teardown() {
  resetAll();
}
