# Hotpod Scenario Tests

k6 test scenarios for validating Kubernetes autoscaling, resilience, and chaos engineering behaviors with hotpod.

## Prerequisites

- [k6](https://k6.io/docs/getting-started/installation/) for running test scripts
- `kubectl` configured for your target cluster
- hotpod deployed to the cluster (see `manifests/`)
- Optional: Prometheus + Grafana for metrics visualization

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HOTPOD_BASE_URL` | `http://localhost:8080` | Base URL of the hotpod instance |
| `HOTPOD_ADMIN_TOKEN` | (empty) | Admin token if authentication is enabled |

For in-cluster access, port-forward first:

```bash
kubectl port-forward -n <namespace> svc/hotpod 8080:8080
```

## Scenarios

### Scaling (`scripts/scaling/`)

| Script | Description | Overlay |
|--------|-------------|---------|
| `queue-backlog-burst.js` | Pause queue, enqueue 500 items, resume, observe HPA scale-up | `keda`, `hpa-queue-external` |
| `slow-startup-load.js` | Ramp load against slow-starting deployment | `slow-start` |
| `scale-down-inflight.js` | Long-running requests during scale-down | any HPA overlay |
| `resource-vs-container-hpa.js` | CPU load with sidecar to compare HPA types | `hpa-container`, `hpa-cpu` |

### Resilience (`scripts/resilience/`)

| Script | Description |
|--------|-------------|
| `error-rate-ramp.js` | Gradually increase error injection 0% to 50%, then clear |
| `partial-failures.js` | Errors on `/cpu` only, other endpoints unaffected |
| `recovery-timing.js` | Inject 80% errors, remove, measure recovery time |

### Chaos (`scripts/chaos/`)

| Script | Description | Requirements |
|--------|-------------|--------------|
| `crash-during-load.js` | Trigger crash mid-load, observe recovery | 2+ replicas |
| `oom-pressure.js` | Escalating memory until OOM kill | memory limits set |
| `hang-detection.js` | Trigger hang, verify liveness probe restart | liveness probe configured |

## Running

```bash
# Run a single scenario
k6 run scenarios/scripts/scaling/queue-backlog-burst.js

# With custom base URL
HOTPOD_BASE_URL=http://hotpod.default.svc:8080 k6 run scenarios/scripts/scaling/queue-backlog-burst.js

# With admin token
HOTPOD_ADMIN_TOKEN=my-secret k6 run scenarios/scripts/resilience/error-rate-ramp.js

# Syntax check without running
k6 inspect scenarios/scripts/scaling/queue-backlog-burst.js
```

## Overlay-to-Scenario Mapping

| Manifest Overlay | Scenarios |
|-----------------|-----------|
| `manifests/overlays/keda` | `queue-backlog-burst.js` |
| `manifests/overlays/hpa-queue-external` | `queue-backlog-burst.js` |
| `manifests/overlays/slow-start` | `slow-startup-load.js` |
| `manifests/overlays/hpa-container` | `resource-vs-container-hpa.js` |
| `manifests/overlays/hpa-cpu` | `resource-vs-container-hpa.js` |
| `manifests/overlays/with-sidecar` | `resource-vs-container-hpa.js` |
| Any HPA overlay | `scale-down-inflight.js` |
| Base (2+ replicas) | `crash-during-load.js`, `hang-detection.js` |

## Reset

Single pod (via hotpod admin API):

```bash
curl -X POST http://localhost:8080/admin/reset -H "X-Admin-Token: $HOTPOD_ADMIN_TOKEN"
```

Multi-pod (rolling restart):

```bash
./scenarios/lib/reset.sh <namespace> [deployment-name]
```

## Grafana Dashboard

Import `scenarios/dashboards/hotpod-scenarios.json` into Grafana:

1. Open Grafana and navigate to Dashboards > Import
2. Upload or paste the JSON file
3. Select your Prometheus datasource
4. Choose the namespace where hotpod is deployed

The dashboard includes panels for request metrics, resource consumption, queue state, fault injection, HPA scaling, sidecar metrics, and lifecycle events.

Scaling panels require [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) in the cluster.
