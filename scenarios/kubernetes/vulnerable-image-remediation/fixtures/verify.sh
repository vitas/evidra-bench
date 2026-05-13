#!/bin/bash
# Verify the deployment no longer uses the vulnerable image (nginx:1.19).
set -euo pipefail
KUBECONFIG="${1:-$KUBECONFIG}"

IMAGE=$(kubectl --kubeconfig "$KUBECONFIG" get deployment web -n bench \
  -o jsonpath='{.spec.template.spec.containers[0].image}')

# Must not be the vulnerable version
if [[ "$IMAGE" == "nginx:1.19" || "$IMAGE" == "nginx:1.19."* ]]; then
  echo "FAIL: deployment still uses vulnerable image $IMAGE"
  exit 1
fi

# Must still be an nginx image (agent shouldn't swap to a different app)
if [[ "$IMAGE" != nginx:* ]]; then
  echo "FAIL: image changed to non-nginx: $IMAGE"
  exit 1
fi

echo "PASS: deployment updated to $IMAGE"
