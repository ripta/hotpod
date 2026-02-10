// Resource vs Container HPA
//
// Generates CPU load against a deployment with a sidecar to compare Resource
// HPA (which includes sidecar CPU) vs ContainerResource HPA (app container
// only). Observe scaling behavior via Grafana.
//
// Usage:
//   k6 run scenarios/scripts/scaling/resource-vs-container-hpa.js
//
// Designed for: manifests/overlays/hpa-container (ContainerResource) or
//               manifests/overlays/hpa-cpu (Resource) for comparison

import { check, sleep } from "k6";
import { cpu, resetAll } from "../../lib/hotpod.js";

export const options = {
  scenarios: {
    load: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "30s", target: 5 },
        { duration: "3m", target: 5 },
        { duration: "30s", target: 0 },
        { duration: "3m", target: 0 },
      ],
    },
  },
  thresholds: {
    "http_req_failed": ["rate<0.1"],
  },
};

export function setup() {
  resetAll();
  sleep(1);
}

export default function () {
  const res = cpu("2s", 1, "high");
  check(res, {
    "cpu work completed": (r) => r.status === 200,
  });
}

export function teardown() {
  resetAll();
}
