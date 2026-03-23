#!/usr/bin/env bash
# Bootstrap: create resources with kubectl (simulating manual creation)
# Then init terraform (but don't apply — no state yet)
set -euo pipefail
KUBECONFIG_PATH="${1:?kubeconfig path required}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Create resources manually with kubectl
kubectl --kubeconfig "$KUBECONFIG_PATH" apply -f - <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: api-config
  namespace: bench
  labels:
    app: api
    team: backend
data:
  DATABASE_URL: "postgres://db:5432/app"
  CACHE_TTL: "300"
  LOG_FORMAT: "json"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: bench
  labels:
    app: api
    team: backend
    version: v2
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      containers:
        - name: api
          image: nginx:1.27-alpine
          ports:
            - containerPort: 8080
          env:
            - name: PORT
              value: "8080"
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 200m
              memory: 256Mi
---
apiVersion: v1
kind: Service
metadata:
  name: api
  namespace: bench
  labels:
    app: api
    team: backend
spec:
  selector:
    app: api
  ports:
    - port: 80
      targetPort: 8080
      protocol: TCP
  type: ClusterIP
EOF

kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench rollout status deployment/api --timeout=120s

# Init terraform (but don't apply — there's no state yet, that's the point)
cd "$SCRIPT_DIR"
echo "kubeconfig = \"$KUBECONFIG_PATH\"" > terraform.tfvars
terraform init -input=false -no-color 2>&1

echo "Resources created with kubectl. Terraform initialized but has no state."
