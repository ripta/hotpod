// OOM Pressure
//
// Escalates memory allocation in steps (100MB, 200MB, 400MB) then triggers
// OOM simulation. Observe how Kubernetes handles OOMKilled pods and whether
// memory limits are enforced.
//
// Usage:
//   k6 run scenarios/scripts/chaos/oom-pressure.js

import { check, sleep } from "k6";
import { memory, triggerOOM, resetAll } from "../../lib/hotpod.js";

export const options = {
  scenarios: {
    escalate: {
      executor: "per-vu-iterations",
      vus: 1,
      iterations: 1,
      maxDuration: "5m",
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

export default function () {
  const steps = ["100MB", "200MB", "400MB"];

  for (const size of steps) {
    console.log(`Allocating ${size} for 15s`);
    const res = memory(size, "15s", "random");
    check(res, {
      [`${size} allocated`]: (r) => r.status === 200,
    });
    sleep(5);
  }

  console.log("Triggering OOM simulation at 200MB/s");
  const oomRes = triggerOOM("200MB");
  check(oomRes, {
    "oom started": (r) => r.status === 200,
  });

  // Wait for OOM kill or timeout
  sleep(60);
}

export function teardown() {
  resetAll();
}
