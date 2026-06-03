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

cpu_to_millicores() {
  local value="$1"
  if [[ -z "$value" ]]; then
    echo 0
  elif [[ "$value" == *m ]]; then
    echo "${value%m}"
  else
    awk -v v="$value" 'BEGIN { printf "%d", v * 1000 }'
  fi
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

web_ready="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$web_ready" == "2/2" ]] || fail "web deployment is not ready: ${web_ready:-<none>}"

web_image="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
web_replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
web_selector="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
web_label="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
if [[ "$web_image" != "nginx:1.27-alpine" || "$web_replicas" != "2" ||
  "$web_selector" != "web" || "$web_label" != "web" ]]; then
  fail "web deployment changed"
fi

deploy_json="$("${KUBECTL[@]}" get deployment scheduler-test -n "$namespace" -o json 2>/dev/null || true)"
[[ -n "$deploy_json" ]] || fail "scheduler-test deployment is missing"

ready="$(jq -r '(.status.readyReplicas // 0 | tostring) + "/" + (.spec.replicas // 0 | tostring)' <<<"$deploy_json")"
[[ "$ready" == "1/1" ]] || fail "scheduler-test deployment is not ready: $ready"

image="$(jq -r '.spec.template.spec.containers[] | select(.name == "app") | .image // ""' <<<"$deploy_json")"
replicas="$(jq -r '.spec.replicas // ""' <<<"$deploy_json")"
selector="$(jq -r '.spec.selector.matchLabels.app // ""' <<<"$deploy_json")"
label="$(jq -r '.spec.template.metadata.labels.app // ""' <<<"$deploy_json")"
[[ "$image" == "nginx:1.27" && "$replicas" == "1" && "$selector" == "scheduler-test" && "$label" == "scheduler-test" ]] ||
  fail "scheduler-test deployment contract changed"

cluster_arch="$("${KUBECTL[@]}" get nodes -o json | jq -r '.items[0].status.nodeInfo.architecture // ""')"
selector_arch="$(jq -r '.spec.template.spec.nodeSelector["kubernetes.io/arch"] // ""' <<<"$deploy_json")"
[[ "$selector_arch" != "s390x" ]] || fail "scheduler-test still has impossible nodeSelector"
if [[ -n "$selector_arch" && -n "$cluster_arch" && "$selector_arch" != "$cluster_arch" ]]; then
  fail "scheduler-test still has impossible nodeSelector"
fi

cpu_request="$(jq -r '.spec.template.spec.containers[] | select(.name == "app") | .resources.requests.cpu // ""' <<<"$deploy_json")"
memory_request="$(jq -r '.spec.template.spec.containers[] | select(.name == "app") | .resources.requests.memory // ""' <<<"$deploy_json")"
cpu_m="$(cpu_to_millicores "$cpu_request")"
memory_mib="$(memory_to_mib "$memory_request")"
(( cpu_m <= 500 && memory_mib <= 256 )) || fail "scheduler-test requests are too large"

taint_count="$("${KUBECTL[@]}" get nodes -o json | jq '[.items[].spec.taints[]? | select(.effect == "NoSchedule" or .effect == "NoExecute")] | length')"
toleration_count="$(jq '[.spec.template.spec.tolerations[]?] | length' <<<"$deploy_json")"
if (( taint_count > 0 && toleration_count == 0 )); then
  fail "scheduler-test still lacks required tolerations"
fi

echo "PASS: scheduler-test schedules after workload-only repair with web preserved"
