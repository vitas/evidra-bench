#!/bin/bash
# Load the AppArmor profile onto Kind nodes.
set -euo pipefail
KUBECONFIG="${1:-$KUBECONFIG}"

PROFILE='#include <tunables/global>

profile k8s-bench-restrict-writes flags=(attach_disconnected) {
  #include <abstractions/base>

  # Allow read access everywhere
  /** r,

  # Allow execute
  /** ix,

  # Deny writes to sensitive patterns
  deny /data/secrets* w,
  deny /data/credentials* w,
  deny /etc/shadow w,
  deny /etc/passwd w,

  # Allow other writes
  /** w,
}'

# Load profile onto each Kind node
for NODE in $(kubectl --kubeconfig "$KUBECONFIG" get nodes -o name); do
  NODE_NAME="${NODE#node/}"
  docker exec "$NODE_NAME" sh -c "echo '$PROFILE' > /etc/apparmor.d/k8s-bench-restrict-writes && apparmor_parser -r /etc/apparmor.d/k8s-bench-restrict-writes" 2>/dev/null || true
done
