// Recovery Timing
//
// Injects an 80% error rate, waits 60 seconds, then clears it. Measures how
// quickly the application returns to a healthy state after error injection is
// removed.
//
// Usage:
//   k6 run scenarios/scripts/resilience/recovery-timing.js

import { check, sleep } from "k6";
import { work, setErrorRate, clearErrorRate, resetAll } from "../../lib/hotpod.js";

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
      vus: 3,
      duration: "3m",
    },
  },
  thresholds: {
    checks: ["rate>0.3"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

export function orchestrate() {
  // Let baseline traffic flow for 15s
  sleep(15);

  console.log("Injecting 80% error rate");
  const res = setErrorRate(undefined, 0.8, "500,502,503");
  check(res, { "error rate set": (r) => r.status === 200 });

  sleep(60);

  console.log("Clearing error rate - measuring recovery");
  clearErrorRate();

  // Observe recovery for remaining duration
  sleep(90);
}

export default function () {
  const res = work("web");
  check(res, {
    "response ok": (r) => r.status === 200,
  });
  sleep(1);
}

export function teardown() {
  clearErrorRate();
  resetAll();
}
