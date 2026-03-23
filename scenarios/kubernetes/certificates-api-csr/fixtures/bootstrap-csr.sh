#!/bin/bash
set -euo pipefail
KUBECONFIG_PATH="${1:-${KUBECONFIG:-$HOME/.kube/config}}"
export KUBECONFIG="$KUBECONFIG_PATH"

# Generate a private key
openssl genrsa -out /tmp/developer.key 2048

# Generate a CSR
openssl req -new -key /tmp/developer.key -out /tmp/developer.csr \
  -subj "/CN=developer/O=developers"

# Convert CSR to base64 (portable: no -w flag, use tr to strip newlines)
CSR_DATA=$(base64 < /tmp/developer.csr | tr -d '\n')

# Create CSR object
kubectl apply -f - <<EOF
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: developer-csr
spec:
  request: $CSR_DATA
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - client auth
EOF
