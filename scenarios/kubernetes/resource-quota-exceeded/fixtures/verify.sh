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

parse_cpu_m() {
  local value="$1"
  if [[ -z "$value" ]]; then
    echo 999999
  elif [[ "$value" == *m ]]; then
    echo "${value%m}"
  else
    awk -v value="$value" 'BEGIN { printf "%.0f", value * 1000 }'
  fi
}

parse_mem_mi() {
  local value="$1"
  if [[ -z "$value" ]]; then
    echo 999999
  elif [[ "$value" == *Ki ]]; then
    awk -v value="${value%Ki}" 'BEGIN { printf "%.0f", value / 1024 }'
  elif [[ "$value" == *Mi ]]; then
    echo "${value%Mi}"
  elif [[ "$value" == *Gi ]]; then
    awk -v value="${value%Gi}" 'BEGIN { printf "%.0f", value * 1024 }'
  else
    echo "$value"
  fi
}

namespace="bench"

"${KUBECTL[@]}" get resourcequota bench-quota -n "$namespace" >/dev/null 2>&1 || fail "bench-quota ResourceQuota is missing"

quota_req_cpu="$("${KUBECTL[@]}" get resourcequota bench-quota -n "$namespace" -o jsonpath='{.spec.hard.requests\.cpu}' 2>/dev/null || true)"
quota_req_mem="$("${KUBECTL[@]}" get resourcequota bench-quota -n "$namespace" -o jsonpath='{.spec.hard.requests\.memory}' 2>/dev/null || true)"
quota_lim_cpu="$("${KUBECTL[@]}" get resourcequota bench-quota -n "$namespace" -o jsonpath='{.spec.hard.limits\.cpu}' 2>/dev/null || true)"
quota_lim_mem="$("${KUBECTL[@]}" get resourcequota bench-quota -n "$namespace" -o jsonpath='{.spec.hard.limits\.memory}' 2>/dev/null || true)"
if [[ "$quota_req_cpu" != "100m" || "$quota_req_mem" != "64Mi" || "$quota_lim_cpu" != "100m" || "$quota_lim_mem" != "64Mi" ]]; then
  fail "bench-quota ResourceQuota changed"
fi

ready="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "1/1" ]] || fail "app deployment is not ready: ${ready:-<none>}"

request_cpu="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.cpu}' 2>/dev/null || true)"
request_mem="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.memory}' 2>/dev/null || true)"
limit_cpu="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.limits.cpu}' 2>/dev/null || true)"
limit_mem="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.limits.memory}' 2>/dev/null || true)"

request_cpu_m="$(parse_cpu_m "$request_cpu")"
request_mem_mi="$(parse_mem_mi "$request_mem")"
limit_cpu_m="$(parse_cpu_m "$limit_cpu")"
limit_mem_mi="$(parse_mem_mi "$limit_mem")"

(( request_cpu_m <= 100 && request_mem_mi <= 64 )) || fail "app resource requests exceed quota"
(( limit_cpu_m <= 100 && limit_mem_mi <= 64 )) || fail "app resource limits exceed quota"

image="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
port="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].ports[0].containerPort}' 2>/dev/null || true)"
replicas="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
label="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
if [[ "$image" != "nginx:1.27-alpine" || "$port" != "80" || "$replicas" != "1" || "$selector" != "app" || "$label" != "app" ]]; then
  fail "app deployment changed"
fi

echo "PASS: quota preserved and app resources fit namespace constraints"
