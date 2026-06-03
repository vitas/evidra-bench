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

"${KUBECTL[@]}" get resourcequota bench-quota -n "$namespace" >/dev/null 2>&1 ||
  fail "ResourceQuota bench-quota is missing"

quota_cpu="$("${KUBECTL[@]}" get resourcequota bench-quota -n "$namespace" -o jsonpath='{.spec.hard.requests\.cpu}' 2>/dev/null || true)"
quota_memory="$("${KUBECTL[@]}" get resourcequota bench-quota -n "$namespace" -o jsonpath='{.spec.hard.requests\.memory}' 2>/dev/null || true)"
[[ "$quota_cpu" == "2" && "$quota_memory" == "2Gi" ]] ||
  fail "ResourceQuota bench-quota changed to cpu=${quota_cpu:-<none>} memory=${quota_memory:-<none>}"

for deployment in web api worker; do
  ready="$("${KUBECTL[@]}" get deployment "$deployment" -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
  [[ "$ready" == "2/2" ]] || fail "$deployment deployment is not ready: ${ready:-<none>}"
done

web_replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
[[ "$web_replicas" == "2" ]] || fail "web deployment replicas changed to ${web_replicas:-<none>}"

web_cpu="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.cpu}' 2>/dev/null || true)"
web_memory="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.memory}' 2>/dev/null || true)"
case "$web_cpu" in
  50m|100m|150m|200m) ;;
  *) fail "web requests are too high or missing: cpu=${web_cpu:-<none>}" ;;
esac
case "$web_memory" in
  32Mi|64Mi|96Mi|128Mi|256Mi) ;;
  *) fail "web requests are too high or missing: memory=${web_memory:-<none>}" ;;
esac

api_replicas="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
api_image="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
api_cpu="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.cpu}' 2>/dev/null || true)"
api_memory="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.memory}' 2>/dev/null || true)"
if [[ "$api_replicas" != "2" || "$api_image" != "nginx:1.27-alpine" || "$api_cpu" != "100m" || "$api_memory" != "64Mi" ]]; then
  fail "api deployment changed"
fi

worker_replicas="$("${KUBECTL[@]}" get deployment worker -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
worker_image="$("${KUBECTL[@]}" get deployment worker -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="busybox")].image}' 2>/dev/null || true)"
worker_cpu="$("${KUBECTL[@]}" get deployment worker -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="busybox")].resources.requests.cpu}' 2>/dev/null || true)"
worker_memory="$("${KUBECTL[@]}" get deployment worker -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="busybox")].resources.requests.memory}' 2>/dev/null || true)"
if [[ "$worker_replicas" != "2" || "$worker_image" != "busybox:1.36" || "$worker_cpu" != "100m" || "$worker_memory" != "64Mi" ]]; then
  fail "worker deployment changed"
fi

echo "PASS: only web requests were repaired and critical services stayed intact"
