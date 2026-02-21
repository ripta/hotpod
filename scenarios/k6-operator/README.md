# k6 Operator TestRun Manifests

Kubernetes manifests for running hotpod k6 scenarios via the [k6 operator](https://github.com/grafana/k6-operator).

## Prerequisites

- k6 operator installed in the cluster
- hotpod deployed (see `manifests/`)

## Usage

Generate ConfigMaps from the scenario scripts:

```bash
# From the repository root:
make k6-configmaps

# Or directly:
./scenarios/k6-operator/build-configmaps.sh > scenarios/k6-operator/configmaps.yaml
```

Apply everything:

```bash
kubectl apply -k scenarios/k6-operator/
```

Run a single scenario:

```bash
# Extract one TestRun from the file
kubectl apply -f scenarios/k6-operator/testruns.yaml -l metadata.name=hotpod-scaling-queue-backlog-burst

# Or apply the full file and delete the ones you don't need
kubectl get testrun
```

## Watching Test Execution

```bash
# Watch TestRun status
kubectl get testrun -w

# View logs for a specific run
kubectl logs -l k6_cr=hotpod-scaling-queue-backlog-burst
```

## Customization

### Namespace

To deploy into a different namespace, add a `namespace` field to `kustomization.yaml`:

```yaml
namespace: k6-tests
```

### Base URL

The default base URL is `http://hotpod.default.svc:80`. To change it, edit the `HOTPOD_BASE_URL` value in `testruns.yaml` or create a Kustomize patch.

### Admin Token

TestRuns reference an optional Secret `hotpod-admin-token` (key: `token`). Create it if your deployment requires authentication:

```bash
kubectl create secret generic hotpod-admin-token --from-literal=token=my-secret
```

## How It Works

The `build-configmaps.sh` script iterates over all scenario scripts in `scenarios/scripts/`, and for each one creates a ConfigMap containing:

- `test.js` — the script with its import path rewritten from `../../lib/hotpod.js` to `./hotpod.js`
- `hotpod.js` — the shared helper library

This flattens the directory structure so ConfigMaps (which don't support subdirectories) work without modifying the source scripts.
