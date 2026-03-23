#!/bin/bash
set -euo pipefail

# Verify that new pods can be created
# Try to create a test pod and verify it starts

TEST_POD_NAME="verify-pod-$(date +%s)"

# Create a test pod
kubectl run "$TEST_POD_NAME" --image=alpine:latest --namespace=bench -- sleep 30

# Wait for it to be in Running state
kubectl wait --for=condition=Ready pod/"$TEST_POD_NAME" -n bench --timeout=30s

# Clean up
kubectl delete pod "$TEST_POD_NAME" -n bench --ignore-not-found

echo "Successfully created and verified test pod"
