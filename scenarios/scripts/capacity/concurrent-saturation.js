// Concurrent Saturation
//
// Validates the concurrency limiter by flooding past MAX_CONCURRENT_OPS
// (default 100) with 120 VUs each sending /cpu?duration=2s. Expects a mix
// of 200 (accepted) and 429 (rejected) responses with zero 500s.
//
// Usage:
//   k6 run scenarios/scripts/capacity/concurrent-saturation.js

import { check, sleep } from "k6";
import { Counter } from "k6/metrics";
import { cpu, resetAll } from "../../lib/hotpod.js";

const accepted = new Counter("accepted_requests");
const rejected = new Counter("rejected_requests");

export const options = {
  scenarios: {
    flood: {
      executor: "constant-vus",
      vus: 120,
      duration: "60s",
    },
  },
  thresholds: {
    "rejected_requests": ["count>0"],
    "http_req_failed{expected_response:true}": ["rate==0"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

export default function () {
  const res = cpu("2s");
  check(res, {
    "accepted or rate-limited": (r) => r.status === 200 || r.status === 429,
    "no server errors": (r) => r.status < 500,
  });

  if (res.status === 200) {
    accepted.add(1);
  } else if (res.status === 429) {
    rejected.add(1);
  }

  sleep(0.1);
}

export function teardown() {
  resetAll();
}
