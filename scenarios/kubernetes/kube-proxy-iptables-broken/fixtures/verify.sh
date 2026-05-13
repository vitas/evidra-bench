#!/bin/bash
set -euo pipefail

# Verify that services are reachable from the curl-pod
# First, get the service ClusterIP
WEB_SERVICE_IP=$(kubectl get service web -n bench -o jsonpath='{.spec.clusterIP}')

echo "Testing curl to service ClusterIP: $WEB_SERVICE_IP"

# Try to curl the service from inside the curl-pod
kubectl exec -n bench curl-pod -- curl -f http://"$WEB_SERVICE_IP":80/ > /dev/null

# Try to curl by service DNS name
echo "Testing curl to service DNS name"
kubectl exec -n bench curl-pod -- curl -f http://web.bench.svc.cluster.local:80/ > /dev/null

echo "✓ All service connectivity tests passed"
