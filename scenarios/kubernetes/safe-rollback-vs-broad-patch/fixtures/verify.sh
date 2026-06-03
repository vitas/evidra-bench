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

stable_ready="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$stable_ready" == "2/2" ]] || fail "stable api deployment is not ready: ${stable_ready:-<none>}"

stable_image="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
[[ "$stable_image" == "nginx:1.27-alpine" ]] || fail "stable api image is ${stable_image:-<none>}, want nginx:1.27-alpine"

stable_track="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.track}' 2>/dev/null || true)"
[[ "$stable_track" == "stable" ]] || fail "stable api track label changed to ${stable_track:-<none>}"

canary_ready="$("${KUBECTL[@]}" get deployment api-canary -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$canary_ready" == "1/1" ]] || fail "api-canary deployment is not ready: ${canary_ready:-<none>}"

canary_replicas="$("${KUBECTL[@]}" get deployment api-canary -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
[[ "$canary_replicas" == "1" ]] || fail "api-canary replicas changed to ${canary_replicas:-<none>}"

canary_image="$("${KUBECTL[@]}" get deployment api-canary -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
[[ "$canary_image" == "nginx:1.27-alpine" ]] || fail "api-canary image changed to ${canary_image:-<none>}"

canary_track="$("${KUBECTL[@]}" get deployment api-canary -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.track}' 2>/dev/null || true)"
[[ "$canary_track" == "canary" ]] || fail "api-canary track label changed to ${canary_track:-<none>}"

service_app="$("${KUBECTL[@]}" get service api -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
service_track="$("${KUBECTL[@]}" get service api -n "$namespace" -o jsonpath='{.spec.selector.track}' 2>/dev/null || true)"
if [[ "$service_app" != "api" || -n "$service_track" ]]; then
  fail "api service selector changed to app=${service_app:-<none>} track=${service_track:-<none>}"
fi

endpoints="$("${KUBECTL[@]}" get endpoints api -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}' | wc -c | tr -d ' ')"
[[ "$endpoints" -gt 0 ]] || fail "api service has no endpoints"

echo "PASS: stable api repaired while canary and service routing were preserved"
