// Slow Startup Load
//
// Ramps load against a deployment configured with startup delays. Validates
// that readiness gates prevent traffic from reaching pods before startup
// completes.
//
// Usage:
//   k6 run scenarios/scripts/scaling/slow-startup-load.js
//
// Designed for: manifests/overlays/slow-start

import { check, sleep } from "k6";
import { work, readyz, resetAll } from "../../lib/hotpod.js";

export const options = {
  scenarios: {
    load: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "30s", target: 20 },
        { duration: "3m", target: 20 },
        { duration: "30s", target: 0 },
      ],
    },
  },
  thresholds: {
    "http_req_duration{endpoint:work}": ["p(95)<2000"],
    "http_req_failed": ["rate<0.1"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

export default function () {
  const res = work("api", "0.2");
  check(res, {
    "work succeeded": (r) => r.status === 200,
    "not service unavailable": (r) => r.status !== 503,
  });
}

export function teardown() {
  resetAll();
}
