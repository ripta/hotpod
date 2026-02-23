// I/O Throughput
//
// First test coverage for the /io endpoint. Ramps I/O operations across size
// tiers (1MB, 10MB, 50MB) with write, read, and mixed operations in both
// sync and async modes.
//
// Usage:
//   k6 run scenarios/scripts/capacity/io-throughput.js

import { check, sleep, group } from "k6";
import { io, resetAll } from "../../lib/hotpod.js";

export const options = {
  scenarios: {
    ramp: {
      executor: "ramping-vus",
      startVUs: 1,
      stages: [
        // 1MB tier
        { duration: "30s", target: 2 },
        // 10MB tier
        { duration: "30s", target: 4 },
        // 50MB tier
        { duration: "30s", target: 8 },
        // Cool down
        { duration: "15s", target: 0 },
      ],
    },
  },
  thresholds: {
    "http_req_failed": ["rate==0"],
    "http_req_duration": ["p(95)<30000"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

const TIERS = [
  { size: "1mb", label: "1MB" },
  { size: "10mb", label: "10MB" },
  { size: "50mb", label: "50MB" },
];

const OPERATIONS = ["write", "read", "mixed"];

export default function () {
  const tier = TIERS[__VU % TIERS.length];
  const op = OPERATIONS[__ITER % OPERATIONS.length];
  const sync = __ITER % 2 === 0;

  group(`io-${tier.label}-${op}`, function () {
    const res = io(tier.size, op, sync);
    check(res, {
      [`${tier.label} ${op} succeeded`]: (r) => r.status === 200,
    });
  });

  sleep(0.5);
}

export function teardown() {
  resetAll();
}
