// Crash During Load
//
// Generates steady traffic then triggers a process crash mid-test. Observes
// how Kubernetes handles pod restart and whether other replicas absorb the
// traffic. Must run against a deployment with 2+ replicas.
//
// Usage:
//   k6 run scenarios/scripts/chaos/crash-during-load.js

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
    load: {
      executor: "constant-vus",
      vus: 5,
      duration: "2m",
    },
  },
  thresholds: {
    // Some failures expected during crash, but most should succeed
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

  console.log("Triggering crash");
  const res = triggerCrash("0s", 1);
  check(res, {
    "crash scheduled": (r) => r.status === 200,
  });

  // Observe recovery for remaining duration
  sleep(90);
}

export default function () {
  const res = work("api");
  check(res, {
    "response received": (r) => r.status === 200 || r.status >= 500,
  });
  sleep(0.5);
}

export function teardown() {
  resetAll();
}
