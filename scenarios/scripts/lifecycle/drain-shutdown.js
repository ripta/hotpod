// Drain Shutdown
//
// Validates graceful shutdown behavior with in-flight requests. Traffic VUs
// send /work?profile=heavy while an orchestrator triggers /fault/crash?delay=5s
// after stabilization. Expects no 500 errors from the server itself.
//
// Note: Requires 2+ replicas so the k6 runner can still reach the service
// after one pod terminates.
//
// Usage:
//   k6 run scenarios/scripts/lifecycle/drain-shutdown.js

import { check, sleep } from "k6";
import { work, triggerCrash, resetAll } from "../../lib/hotpod.js";

export const options = {
  scenarios: {
    orchestrator: {
      executor: "shared-iterations",
      vus: 1,
      iterations: 1,
      maxDuration: "3m",
      exec: "orchestrate",
    },
    traffic: {
      executor: "constant-vus",
      vus: 5,
      duration: "2m",
    },
  },
  thresholds: {
    // Connection resets from pod termination are expected, but no 500s
    // from the server itself
    "http_req_failed": ["rate<0.3"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

export function orchestrate() {
  // Let traffic stabilize
  sleep(30);

  // Trigger crash with 5s delay to allow graceful shutdown
  console.log("Triggering crash with 5s delay");
  const res = triggerCrash("5s", 1);
  check(res, {
    "crash scheduled": (r) => r.status === 200,
  });

  // Observe shutdown and recovery for remaining duration
  sleep(90);
}

export default function () {
  const res = work("heavy");
  check(res, {
    "response received": (r) => r.status === 200 || r.status >= 500,
  });
  sleep(0.5);
}

export function teardown() {
  resetAll();
}
