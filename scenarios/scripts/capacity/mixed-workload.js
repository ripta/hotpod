// Mixed Workload
//
// Four parallel VU groups hitting different resource endpoints simultaneously
// to test resource competition effects. Each group runs 4 VUs for 60s.
//
// Usage:
//   k6 run scenarios/scripts/capacity/mixed-workload.js

import { check, sleep } from "k6";
import { cpu, memory, io, work, resetAll } from "../../lib/hotpod.js";

export const options = {
  scenarios: {
    cpu_group: {
      executor: "constant-vus",
      vus: 4,
      duration: "60s",
      exec: "cpuLoad",
    },
    memory_group: {
      executor: "constant-vus",
      vus: 4,
      duration: "60s",
      exec: "memoryLoad",
    },
    io_group: {
      executor: "constant-vus",
      vus: 4,
      duration: "60s",
      exec: "ioLoad",
    },
    work_group: {
      executor: "constant-vus",
      vus: 4,
      duration: "60s",
      exec: "workLoad",
    },
  },
  thresholds: {
    "checks": ["rate>0.95"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

export function cpuLoad() {
  const res = cpu("500ms");
  check(res, {
    "cpu succeeded": (r) => r.status === 200,
  });
  sleep(0.5);
}

export function memoryLoad() {
  const res = memory("20mb", "1s");
  check(res, {
    "memory succeeded": (r) => r.status === 200,
  });
  sleep(0.5);
}

export function ioLoad() {
  const res = io("5mb");
  check(res, {
    "io succeeded": (r) => r.status === 200,
  });
  sleep(0.5);
}

export function workLoad() {
  const res = work("medium");
  check(res, {
    "work succeeded": (r) => r.status === 200,
  });
  sleep(0.5);
}

export function teardown() {
  resetAll();
}
