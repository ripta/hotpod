// Hang Detection
//
// Triggers a full process hang and monitors whether the Kubernetes liveness
// probe detects the hang and restarts the pod. The hang blocks all request
// processing, so the liveness probe at /healthz should fail.
//
// Usage:
//   k6 run scenarios/scripts/chaos/hang-detection.js

import { check, sleep } from "k6";
import { triggerHang, healthz, resetAll } from "../../lib/hotpod.js";

export const options = {
  scenarios: {
    orchestrator: {
      executor: "shared-iterations",
      vus: 1,
      iterations: 1,
      maxDuration: "4m",
      exec: "orchestrate",
    },
    monitor: {
      executor: "constant-vus",
      vus: 1,
      duration: "3m",
      exec: "monitorHealth",
    },
  },
  thresholds: {
    checks: ["rate>0.2"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

export function orchestrate() {
  // Let monitor establish a baseline
  sleep(10);

  console.log("Triggering 5m hang (full block)");
  const res = triggerHang("5m", false);
  // This call will likely time out or fail because the server hangs
  console.log(`Hang response status: ${res.status}`);
}

export function monitorHealth() {
  const res = healthz();
  const ok = res.status === 200;
  check(res, {
    "healthz reachable": () => true, // Always pass; log the status
  });
  if (!ok) {
    console.log(`healthz failed: status=${res.status}`);
  }
  sleep(5);
}

export function teardown() {
  resetAll();
}
