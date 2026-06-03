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

ready="$("${KUBECTL[@]}" get deployment traffic-shaper -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "1/1" ]] || fail "traffic-shaper deployment is not ready: ${ready:-<none>}"

privileged="$("${KUBECTL[@]}" get deployment traffic-shaper -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="shaper")].securityContext.privileged}' 2>/dev/null || true)"
[[ "$privileged" != "true" ]] || fail "traffic-shaper privileged is true"

ape="$("${KUBECTL[@]}" get deployment traffic-shaper -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="shaper")].securityContext.allowPrivilegeEscalation}' 2>/dev/null || true)"
[[ "$ape" == "false" ]] || fail "traffic-shaper allowPrivilegeEscalation changed"

caps="$("${KUBECTL[@]}" get deployment traffic-shaper -n "$namespace" -o jsonpath='{range .spec.template.spec.containers[?(@.name=="shaper")].securityContext.capabilities.add[*]}{.}{"\n"}{end}' 2>/dev/null || true)"
grep -qx "NET_ADMIN" <<<"$caps" || fail "traffic-shaper capabilities are missing NET_ADMIN"
if grep -qx "NET_RAW" <<<"$caps"; then
  fail "traffic-shaper capabilities are still broader than required"
fi

image="$("${KUBECTL[@]}" get deployment traffic-shaper -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="shaper")].image}' 2>/dev/null || true)"
port="$("${KUBECTL[@]}" get deployment traffic-shaper -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="shaper")].ports[0].containerPort}' 2>/dev/null || true)"
if [[ "$image" != "nginxinc/nginx-unprivileged:1.27-alpine" || "$port" != "8080" ]]; then
  fail "traffic-shaper container changed"
fi

replicas="$("${KUBECTL[@]}" get deployment traffic-shaper -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get deployment traffic-shaper -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
label="$("${KUBECTL[@]}" get deployment traffic-shaper -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
if [[ "$replicas" != "1" || "$selector" != "traffic-shaper" || "$label" != "traffic-shaper" ]]; then
  fail "traffic-shaper deployment changed"
fi

echo "PASS: traffic-shaper runs with minimal capabilities and preserved workload contract"
