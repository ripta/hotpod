#!/bin/bash
#
# Reset hotpod deployment by triggering a rolling restart.
#
# Usage:
#   reset.sh <namespace> [deployment-name]
#
# Arguments:
#   namespace       Kubernetes namespace where hotpod is running
#   deployment-name Deployment name (default: hotpod)
#
# Examples:
#   ./reset.sh default
#   ./reset.sh hotpod-test my-hotpod

set -euo pipefail

NAMESPACE="${1:?Usage: reset.sh <namespace> [deployment-name]}"
DEPLOYMENT="${2:-hotpod}"

echo "Restarting deployment/${DEPLOYMENT} in namespace ${NAMESPACE}..."
kubectl rollout restart "deployment/${DEPLOYMENT}" -n "${NAMESPACE}"

echo "Waiting for rollout to complete..."
kubectl rollout status "deployment/${DEPLOYMENT}" -n "${NAMESPACE}" --timeout=120s

echo "Rollout complete. Current pods:"
kubectl get pods -n "${NAMESPACE}" -l "app=${DEPLOYMENT}" -o wide
