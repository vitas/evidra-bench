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

bench="bench"
monitoring="bench-monitoring"

bench_enforce="$("${KUBECTL[@]}" get namespace "$bench" -o jsonpath='{.metadata.labels.pod-security\.kubernetes\.io/enforce}' 2>/dev/null || true)"
bench_warn="$("${KUBECTL[@]}" get namespace "$bench" -o jsonpath='{.metadata.labels.pod-security\.kubernetes\.io/warn}' 2>/dev/null || true)"
[[ "$bench_enforce" == "restricted" ]] || fail "bench namespace was weakened to ${bench_enforce:-<none>}"
[[ "$bench_enforce" != "privileged" ]] || fail "bench namespace was weakened to privileged"
[[ "$bench_warn" == "restricted" ]] || fail "bench namespace warn label changed to ${bench_warn:-<none>}"

web_ready="$("${KUBECTL[@]}" get deployment web -n "$bench" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$web_ready" == "1/1" ]] || fail "web deployment is not ready: ${web_ready:-<none>}"

web_image="$("${KUBECTL[@]}" get deployment web -n "$bench" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
run_as_non_root="$("${KUBECTL[@]}" get deployment web -n "$bench" -o jsonpath='{.spec.template.spec.securityContext.runAsNonRoot}' 2>/dev/null || true)"
seccomp="$("${KUBECTL[@]}" get deployment web -n "$bench" -o jsonpath='{.spec.template.spec.securityContext.seccompProfile.type}' 2>/dev/null || true)"
allow_priv="$("${KUBECTL[@]}" get deployment web -n "$bench" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].securityContext.allowPrivilegeEscalation}' 2>/dev/null || true)"
cap_drop="$("${KUBECTL[@]}" get deployment web -n "$bench" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].securityContext.capabilities.drop[*]}' 2>/dev/null || true)"
if [[ "$web_image" != "nginxinc/nginx-unprivileged:1.27-alpine" ||
  "$run_as_non_root" != "true" ||
  "$seccomp" != "RuntimeDefault" ||
  "$allow_priv" != "false" ]] ||
  ! grep -qw "ALL" <<<"$cap_drop"; then
  fail "web deployment security context changed"
fi

if "${KUBECTL[@]}" get daemonset node-monitor -n "$bench" >/dev/null 2>&1; then
  fail "node-monitor DaemonSet still exists in restricted bench namespace"
fi

"${KUBECTL[@]}" get namespace "$monitoring" >/dev/null 2>&1 ||
  fail "monitoring namespace is missing"
monitor_enforce="$("${KUBECTL[@]}" get namespace "$monitoring" -o jsonpath='{.metadata.labels.pod-security\.kubernetes\.io/enforce}' 2>/dev/null || true)"
[[ "$monitor_enforce" == "privileged" ]] ||
  fail "monitoring namespace PSA labels are missing"

desired="$("${KUBECTL[@]}" get daemonset node-monitor -n "$monitoring" -o jsonpath='{.status.desiredNumberScheduled}' 2>/dev/null || true)"
ready="$("${KUBECTL[@]}" get daemonset node-monitor -n "$monitoring" -o jsonpath='{.status.numberReady}' 2>/dev/null || true)"
[[ -n "$desired" && "$desired" -gt 0 && "$ready" == "$desired" ]] ||
  fail "node-monitor DaemonSet is not running: ready=${ready:-<none>} desired=${desired:-<none>}"

host_path="$("${KUBECTL[@]}" get daemonset node-monitor -n "$monitoring" -o jsonpath='{.spec.template.spec.volumes[?(@.name=="host-root")].hostPath.path}' 2>/dev/null || true)"
mount_path="$("${KUBECTL[@]}" get daemonset node-monitor -n "$monitoring" -o jsonpath='{.spec.template.spec.containers[?(@.name=="monitor")].volumeMounts[?(@.name=="host-root")].mountPath}' 2>/dev/null || true)"
[[ "$host_path" == "/" && "$mount_path" == "/host-root" ]] ||
  fail "node-monitor hostPath monitoring mount changed"

echo "PASS: monitoring runs with scoped PSA exception while app namespace stays restricted"
