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

"${KUBECTL[@]}" get namespace "$namespace" >/dev/null 2>&1 ||
  fail "bench namespace is missing"

deleting="$("${KUBECTL[@]}" get namespace "$namespace" -o jsonpath='{.metadata.deletionTimestamp}' 2>/dev/null || true)"
[[ -z "$deleting" ]] || fail "bench namespace is being deleted"

if "${KUBECTL[@]}" get job data-migration-old -n "$namespace" >/dev/null 2>&1; then
  fail "cleanup job data-migration-old still exists"
fi
if "${KUBECTL[@]}" get job test-runner-stale -n "$namespace" >/dev/null 2>&1; then
  fail "cleanup job test-runner-stale still exists"
fi

cleanup_jobs="$("${KUBECTL[@]}" get jobs -n "$namespace" -l cleanup=true --no-headers 2>/dev/null | wc -l | tr -d ' ')"
[[ "$cleanup_jobs" == "0" ]] || fail "cleanup=true jobs still exist: $cleanup_jobs"

check_deployment() {
  local deployment="$1"
  local ready replicas image env_label selector
  ready="$("${KUBECTL[@]}" get deployment "$deployment" -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
  replicas="$("${KUBECTL[@]}" get deployment "$deployment" -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
  image="$("${KUBECTL[@]}" get deployment "$deployment" -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
  env_label="$("${KUBECTL[@]}" get deployment "$deployment" -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.environment}' 2>/dev/null || true)"
  selector="$("${KUBECTL[@]}" get deployment "$deployment" -n "$namespace" -o jsonpath="{.spec.selector.matchLabels.app}" 2>/dev/null || true)"

  [[ "$ready" == "2/2" ]] || fail "production $deployment deployment is not ready: ${ready:-<none>}"
  if [[ "$replicas" != "2" || "$image" != "nginx:1.27-alpine" || "$env_label" != "production" || "$selector" != "$deployment" ]]; then
    case "$deployment" in
      web) fail "production web deployment changed" ;;
      api) fail "production api deployment changed" ;;
      *) fail "production $deployment deployment changed" ;;
    esac
  fi
}

check_service() {
  local service="$1"
  local selector endpoints
  selector="$("${KUBECTL[@]}" get service "$service" -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
  [[ "$selector" == "$service" ]] || fail "production $service service selector changed"

  endpoints="$("${KUBECTL[@]}" get endpoints "$service" -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}' 2>/dev/null | wc -c | tr -d ' ')"
  [[ "$endpoints" -gt 0 ]] || fail "production $service service has no endpoints"
}

check_deployment web
check_deployment api
check_service web
check_service api

echo "PASS: only cleanup jobs were removed and production namespace stayed intact"
