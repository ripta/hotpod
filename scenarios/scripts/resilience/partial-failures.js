// Partial Failures
//
// Injects 50% error rate on /cpu only while other endpoints remain unaffected.
// Validates that error injection is scoped per-endpoint and does not leak to
// other routes.
//
// Usage:
//   k6 run scenarios/scripts/resilience/partial-failures.js

import { check, sleep } from "k6";
import { cpu, work, latency, setErrorRate, clearErrorRate, resetAll } from "../../lib/hotpod.js";

export const options = {
  scenarios: {
    orchestrator: {
      executor: "shared-iterations",
      vus: 1,
      iterations: 1,
      maxDuration: "4m",
      exec: "orchestrate",
    },
    traffic: {
      executor: "constant-vus",
      vus: 5,
      duration: "3m",
    },
  },
  thresholds: {
    "checks{check:work_ok}": ["rate>0.85"],
    "checks{check:latency_ok}": ["rate>0.85"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

export function orchestrate() {
  console.log("Setting 50% error rate on /cpu only");
  const res = setErrorRate("/cpu", 0.5, "500");
  check(res, { "error rate set on /cpu": (r) => r.status === 200 });

  // Let traffic run for the full duration
  sleep(170);

  console.log("Clearing error rate");
  clearErrorRate();
}

export default function () {
  // Randomly call one of three endpoints
  const pick = Math.random();
  if (pick < 0.33) {
    const res = cpu("500ms", 1, "low");
    check(res, { cpu_response: (r) => r.status === 200 || r.status === 500 });
  } else if (pick < 0.66) {
    const res = work("web");
    check(res, { "work_ok": (r) => r.status === 200 });
  } else {
    const res = latency("100ms");
    check(res, { "latency_ok": (r) => r.status === 200 });
  }
  sleep(0.5);
}

export function teardown() {
  clearErrorRate();
  resetAll();
}
