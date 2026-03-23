#!/bin/bash
# Verify custom scheduler is running and batch pod uses it
set -euo pipefail
KUBECONFIG="${1:-$KUBECONFIG}"

# Check that batch-scheduler is running in kube-system
SCHEDULER_POD=$(kubectl --kubeconfig "$KUBECONFIG" get pod -n kube-system \
  -l app=batch-scheduler -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [[ -z "$SCHEDULER_POD" ]]; then
  echo "FAIL: batch-scheduler pod not found in kube-system namespace"
  exit 1
fi

# Check that batch deployment has schedulerName: batch-scheduler
SCHEDULER_NAME=$(kubectl --kubeconfig "$KUBECONFIG" get deployment batch -n bench \
  -o jsonpath='{.spec.template.spec.schedulerName}' 2>/dev/null || echo "")

if [[ "$SCHEDULER_NAME" != "batch-scheduler" ]]; then
  echo "FAIL: Batch deployment does not have schedulerName: batch-scheduler"
  exit 1
fi

# Check that batch pod is running on a gpu-labeled node
BATCH_POD=$(kubectl --kubeconfig "$KUBECONFIG" get pod -n bench \
  -l app=batch -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [[ -z "$BATCH_POD" ]]; then
  echo "FAIL: Batch pod not found in bench namespace"
  exit 1
fi

# Get the node the batch pod is running on
POD_NODE=$(kubectl --kubeconfig "$KUBECONFIG" get pod "$BATCH_POD" -n bench \
  -o jsonpath='{.spec.nodeName}' 2>/dev/null || echo "")

if [[ -z "$POD_NODE" ]]; then
  echo "FAIL: Batch pod is not scheduled on any node"
  exit 1
fi

# Check if the node has gpu=true label
GPU_LABEL=$(kubectl --kubeconfig "$KUBECONFIG" get node "$POD_NODE" \
  -o jsonpath='{.metadata.labels.gpu}' 2>/dev/null || echo "")

if [[ "$GPU_LABEL" != "true" ]]; then
  echo "FAIL: Batch pod is not running on a gpu=true labeled node (node: $POD_NODE, label: $GPU_LABEL)"
  exit 1
fi

echo "PASS: Custom scheduler batch-scheduler is running, batch pod uses it, and is on gpu node"
exit 0
