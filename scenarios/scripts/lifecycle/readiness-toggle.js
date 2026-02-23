// Readiness Toggle
//
// Tests the readiness probe lifecycle under active traffic. An orchestrator
// toggles readiness off and back on while traffic VUs send /work requests
// and a monitor VU polls /readyz to observe state transitions.
//
// Usage:
//   k6 run scenarios/scripts/lifecycle/readiness-toggle.js

import { check, sleep } from "k6";
import { setReady, readyz, work, resetAll } from "../../lib/hotpod.js";

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
      vus: 4,
      duration: "2m",
    },
    monitor: {
      executor: "constant-vus",
      vus: 1,
      duration: "2m",
      exec: "monitorReadiness",
    },
  },
  thresholds: {
    "checks": ["rate>0.8"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

export function orchestrate() {
  // Let traffic stabilize
  sleep(20);

  // Verify ready before toggling
  const before = readyz();
  check(before, {
    "initially ready": (r) => r.status === 200,
  });

  // Toggle not-ready
  console.log("Setting readiness to false");
  const offRes = setReady(false);
  check(offRes, {
    "readiness set to false": (r) => r.status === 200,
  });

  // Verify not-ready state
  sleep(2);
  const during = readyz();
  check(during, {
    "readyz returns 503 while not-ready": (r) => r.status === 503,
  });

  // Hold not-ready for 30s
  sleep(30);

  // Toggle back to ready
  console.log("Setting readiness to true");
  const onRes = setReady(true);
  check(onRes, {
    "readiness set to true": (r) => r.status === 200,
  });

  // Verify recovery
  sleep(2);
  const after = readyz();
  check(after, {
    "readyz returns 200 after re-enabling": (r) => r.status === 200,
  });

  sleep(30);
}

export default function () {
  // Traffic VUs: readiness should not affect request handling
  const res = work("light");
  check(res, {
    "work completed during readiness changes": (r) => r.status === 200,
  });
  sleep(0.5);
}

export function monitorReadiness() {
  const res = readyz();
  check(res, {
    "readyz responds": (r) => r.status === 200 || r.status === 503,
  });
  sleep(1);
}

export function teardown() {
  setReady(true);
  resetAll();
}
