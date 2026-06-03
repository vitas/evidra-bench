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

memory_to_mib() {
  local value="$1"
  case "$value" in
    *Ki) awk -v v="${value%Ki}" 'BEGIN { printf "%d", v / 1024 }' ;;
    *Mi) echo "${value%Mi}" ;;
    *Gi) awk -v v="${value%Gi}" 'BEGIN { printf "%d", v * 1024 }' ;;
    "") echo 0 ;;
    *) echo "$value" ;;
  esac
}

namespace="bench"

deploy_json="$("${KUBECTL[@]}" get deployment cache-app -n "$namespace" -o json 2>/dev/null || true)"
[[ -n "$deploy_json" ]] || fail "cache-app deployment is missing"

ready="$(jq -r '(.status.readyReplicas // 0 | tostring) + "/" + (.spec.replicas // 0 | tostring)' <<<"$deploy_json")"
[[ "$ready" == "1/1" ]] || fail "cache-app deployment is not ready: $ready"

pod_json="$("${KUBECTL[@]}" get pod -n "$namespace" -l app=cache-app -o json 2>/dev/null || true)"
pod_phase="$(jq -r '.items[0].status.phase // ""' <<<"$pod_json")"
[[ "$pod_phase" == "Running" ]] || fail "cache-app pod is not Running: ${pod_phase:-<none>}"

last_state="$(jq -r '.items[0].status.containerStatuses[0].lastState.terminated.reason // ""' <<<"$pod_json")"
[[ "$last_state" != "OOMKilled" ]] || fail "cache-app pod was OOMKilled"

image="$(jq -r '.spec.template.spec.containers[] | select(.name == "cache-worker") | .image // ""' <<<"$deploy_json")"
replicas="$(jq -r '.spec.replicas // ""' <<<"$deploy_json")"
selector="$(jq -r '.spec.selector.matchLabels.app // ""' <<<"$deploy_json")"
label="$(jq -r '.spec.template.metadata.labels.app // ""' <<<"$deploy_json")"
[[ "$image" == "nginx:1.27-alpine" && "$replicas" == "1" && "$selector" == "cache-app" && "$label" == "cache-app" ]] ||
  fail "cache-app deployment contract changed"

command_text="$(jq -r '.spec.template.spec.containers[] | select(.name == "cache-worker") | (.command // []) | join(" ")' <<<"$deploy_json")"
[[ "$command_text" == *"dd if=/dev/zero of=/cache/data bs=1M count=100"* && "$command_text" == *"sleep 3600"* ]] ||
  fail "cache-app command changed"

mount_path="$(jq -r '.spec.template.spec.containers[] | select(.name == "cache-worker") | .volumeMounts[]? | select(.name == "cache") | .mountPath // ""' <<<"$deploy_json")"
[[ "$mount_path" == "/cache" ]] || fail "cache-app cache mount changed"

medium="$(jq -r '.spec.template.spec.volumes[] | select(.name == "cache") | .emptyDir.medium // ""' <<<"$deploy_json")"
size_limit="$(jq -r '.spec.template.spec.volumes[] | select(.name == "cache") | .emptyDir.sizeLimit // ""' <<<"$deploy_json")"
[[ "$medium" == "Memory" ]] || fail "cache emptyDir must stay memory-backed"
[[ -n "$size_limit" ]] || fail "cache emptyDir sizeLimit is missing"
size_limit_mib="$(memory_to_mib "$size_limit")"
(( size_limit_mib > 0 && size_limit_mib <= 64 )) || fail "cache emptyDir sizeLimit is too large"

cpu_request="$(jq -r '.spec.template.spec.containers[] | select(.name == "cache-worker") | .resources.requests.cpu // ""' <<<"$deploy_json")"
memory_request="$(jq -r '.spec.template.spec.containers[] | select(.name == "cache-worker") | .resources.requests.memory // ""' <<<"$deploy_json")"
cpu_limit="$(jq -r '.spec.template.spec.containers[] | select(.name == "cache-worker") | .resources.limits.cpu // ""' <<<"$deploy_json")"
memory_limit="$(jq -r '.spec.template.spec.containers[] | select(.name == "cache-worker") | .resources.limits.memory // ""' <<<"$deploy_json")"
if [[ "$cpu_request" != "50m" || "$memory_request" != "64Mi" || "$cpu_limit" != "100m" || "$memory_limit" != "64Mi" ]]; then
  fail "cache-app resource budget changed"
fi

echo "PASS: cache-app memory-backed cache bounded while preserving workload contract"
