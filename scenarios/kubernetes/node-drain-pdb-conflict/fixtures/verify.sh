#!/bin/bash
set -euo pipefail

# Verify that:
# 1. One node is cordoned (SchedulingDisabled)
# 2. All pods are running
# 3. The web deployment has enough replicas available

# Check that at least one node is cordoned
CORDONED_NODES=$(kubectl get nodes -o json | jq -r '.items[] | select(.spec.unschedulable == true) | .metadata.name' | wc -l)
if [ "$CORDONED_NODES" -lt 1 ]; then
  echo "ERROR: No cordoned nodes found"
  exit 1
fi
echo "✓ Found $CORDONED_NODES cordoned node(s)"

# Check that all pods in bench namespace are running
RUNNING_PODS=$(kubectl get pods -n bench -o json | jq -r '.items[] | select(.status.phase == "Running") | .metadata.name' | wc -l)
ALL_PODS=$(kubectl get pods -n bench -o json | jq '.items | length')
if [ "$RUNNING_PODS" -ne "$ALL_PODS" ]; then
  echo "ERROR: Not all pods are running ($RUNNING_PODS/$ALL_PODS)"
  exit 1
fi
echo "✓ All $ALL_PODS pods in bench namespace are running"

# Check that web deployment has at least 1 ready replica
WEB_READY=$(kubectl get deployment web -n bench -o json | jq '.status.readyReplicas // 0')
if [ "$WEB_READY" -lt 1 ]; then
  echo "ERROR: Web deployment has no ready replicas"
  exit 1
fi
echo "✓ Web deployment has $WEB_READY ready replica(s)"

echo "All verifications passed"
