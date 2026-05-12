#!/usr/bin/env bash
set -euo pipefail

KUBECONFIG_PATH="${KUBECONFIG:?KUBECONFIG is required}"

selector_app="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get service web-svc -o jsonpath='{.spec.selector.app}')"
selector_tier="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get service web-svc -o jsonpath='{.spec.selector.tier}')"

if [ "$selector_app" != "web" ] || [ "$selector_tier" != "frontend" ]; then
  echo "FAIL: web-svc selector is app=$selector_app tier=$selector_tier, want app=web tier=frontend"
  exit 1
fi

endpoint_count="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get endpoints web-svc -o jsonpath='{range .subsets[*].addresses[*]}x{end}' | wc -c | tr -d ' ')"
if [ "$endpoint_count" -lt 1 ]; then
  echo "FAIL: web-svc has no ready endpoints"
  exit 1
fi

replicas="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment web -o jsonpath='{.spec.replicas}')"
if [ "$replicas" != "2" ]; then
  echo "FAIL: web deployment replicas changed to $replicas"
  exit 1
fi

image="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment web -o jsonpath='{.spec.template.spec.containers[?(@.name=="web")].image}')"
if [ "$image" != "nginx:1.27-alpine" ]; then
  echo "FAIL: web deployment image changed to $image"
  exit 1
fi

pod_tier="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment web -o jsonpath='{.spec.template.metadata.labels.tier}')"
if [ "$pod_tier" != "frontend" ]; then
  echo "FAIL: web pod label tier changed to $pod_tier"
  exit 1
fi

echo "PASS: service selector restored and workload left intact"
