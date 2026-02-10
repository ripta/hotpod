// Scale-Down In-Flight Requests
//
// Sends long-running requests (10s latency) then ramps down. Validates that
// in-flight requests complete despite pod termination during scale-down.
// Observe SIGTERM handling and graceful shutdown via Grafana.
//
// Usage:
//   k6 run scenarios/scripts/scaling/scale-down-inflight.js

import { check, sleep } from "k6";
import { latency, resetAll } from "../../lib/hotpod.js";

export const options = {
  scenarios: {
    load: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "15s", target: 10 },
        { duration: "2m", target: 10 },
        { duration: "10s", target: 0 },
      ],
    },
  },
  thresholds: {
    "http_req_failed": ["rate<0.05"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

export default function () {
  const res = latency("10s");
  check(res, {
    "request completed": (r) => r.status === 200,
    "not cancelled": (r) => {
      if (r.status !== 200) return true;
      const body = JSON.parse(r.body);
      return !body.cancelled;
    },
  });
}

export function teardown() {
  resetAll();
}
