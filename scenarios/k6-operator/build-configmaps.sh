#!/usr/bin/env bash
#
# Generates ConfigMap YAML for each k6 scenario script.
#
# Each ConfigMap bundles:
#   - test.js  — the scenario script with import path rewritten to ./hotpod.js
#   - hotpod.js — the shared helper library
#
# Output is a multi-document YAML stream written to stdout.
#
# Usage:
#   ./build-configmaps.sh > configmaps.yaml

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPTS_DIR="${REPO_ROOT}/scenarios/scripts"
LIB_FILE="${REPO_ROOT}/scenarios/lib/hotpod.js"

if [[ ! -f "${LIB_FILE}" ]]; then
  echo "error: ${LIB_FILE} not found" >&2
  exit 1
fi

first=true

for script in "${SCRIPTS_DIR}"/{scaling,resilience,chaos}/*.js; do
  [[ -f "${script}" ]] || continue

  # Derive category and name from the path:
  #   scripts/scaling/queue-backlog-burst.js -> scaling, queue-backlog-burst
  rel="${script#"${SCRIPTS_DIR}"/}"
  category="${rel%%/*}"
  name="$(basename "${rel}" .js)"
  cm_name="k6-${category}-${name}"

  # Rewrite import path from ../../lib/hotpod.js to ./hotpod.js
  rewritten="$(sed 's|../../lib/hotpod\.js|./hotpod.js|g' "${script}")"

  if [[ "${first}" == true ]]; then
    first=false
  else
    echo "---"
  fi

  cat <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${cm_name}
  labels:
    app.kubernetes.io/component: k6-test
data:
  test.js: |
$(echo "${rewritten}" | sed 's/^/    /')
  hotpod.js: |
$(sed 's/^/    /' "${LIB_FILE}")
EOF
done
