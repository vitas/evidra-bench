#!/usr/bin/env bash
set -euo pipefail

KUBECTL=(kubectl)
if [[ -n "${KUBECONFIG:-}" ]]; then
  KUBECTL=(kubectl --kubeconfig "$KUBECONFIG")
fi

fail() {
  echo "FAIL: $*"
  exit 1
}

namespace="bench"

ready="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "2/2" ]] || fail "web deployment is not ready: ${ready:-<none>}"

replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
[[ "$replicas" == "2" ]] || fail "web deployment was scaled down"

image="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
[[ "$image" == "nginx:1.27-alpine" ]] || fail "web deployment image is ${image:-<none>}"

secret_ref="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].envFrom[?(@.secretRef.name=="web-db-credentials")].secretRef.name}' 2>/dev/null || true)"
[[ "$secret_ref" == "web-db-credentials" ]] || fail "web deployment secret reference changed"

"${KUBECTL[@]}" get secret web-db-credentials -n "$namespace" >/dev/null 2>&1 ||
  fail "web-db-credentials Secret is missing"

secret_keys="$("${KUBECTL[@]}" get secret web-db-credentials -n "$namespace" -o go-template='{{len .data}}' 2>/dev/null || true)"
[[ "$secret_keys" -gt 0 ]] || fail "web-db-credentials Secret has no data"

selector="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
pod_label="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
[[ "$selector" == "web" && "$pod_label" == "web" ]] || fail "web deployment selector changed"

service_selector="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
[[ "$service_selector" == "web" ]] || fail "web service selector changed"

echo "PASS: web image and database secret were both repaired"
