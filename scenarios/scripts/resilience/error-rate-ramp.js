// Error Rate Ramp
//
// Gradually increases error injection from 0% to 50% while sending traffic,
// then clears the error rate. Observe how the application and metrics respond
// to increasing failure rates.
//
// Usage:
//   k6 run scenarios/scripts/resilience/error-rate-ramp.js

import { check, sleep } from "k6";
import { work, setErrorRate, clearErrorRate, resetAll } from "../../lib/hotpod.js";

export const options = {
  scenarios: {
    orchestrator: {
      executor: "shared-iterations",
      vus: 1,
      iterations: 1,
      maxDuration: "6m",
      exec: "orchestrate",
    },
    traffic: {
      executor: "constant-vus",
      vus: 5,
      duration: "5m",
    },
  },
  thresholds: {
    checks: ["rate>0.5"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

const ERROR_STEPS = [0.0, 0.1, 0.2, 0.3, 0.4, 0.5];

export function orchestrate() {
  for (const rate of ERROR_STEPS) {
    console.log(`Setting error rate to ${rate * 100}%`);
    const res = setErrorRate(undefined, rate, "500", "60s");
    check(res, { "error rate set": (r) => r.status === 200 });
    sleep(45);
  }

  console.log("Clearing error rate");
  clearErrorRate();
  sleep(30);
}

export default function () {
  const res = work("api");
  check(res, {
    "response received": (r) => r.status === 200 || r.status === 500,
  });
  sleep(0.5);
}

export function teardown() {
  clearErrorRate();
  resetAll();
}
