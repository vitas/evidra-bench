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

ready="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "1/1" ]] || fail "app deployment is not ready: ${ready:-<none>}"

ro="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="app")].securityContext.readOnlyRootFilesystem}' 2>/dev/null || true)"
[[ "$ro" == "true" ]] || fail "app container readOnlyRootFilesystem is ${ro:-<none>}, expected true"

image="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="app")].image}' 2>/dev/null || true)"
port="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="app")].ports[0].containerPort}' 2>/dev/null || true)"
replicas="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
label="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
if [[ "$image" != "nginx:1.27-alpine" || "$port" != "80" || "$replicas" != "1" || "$selector" != "app" || "$label" != "app" ]]; then
  fail "app deployment changed"
fi

command_script="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="app")].command[2]}' 2>/dev/null || true)"
if [[ "$command_script" != *"/tmp/healthcheck"* || "$command_script" != *"/var/log/app/access.log"* ||
  "$command_script" != *"nginx -g 'daemon off;'"* ]]; then
  fail "app command changed"
fi

mount_paths="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{range .spec.template.spec.containers[?(@.name=="app")].volumeMounts[*]}{.mountPath}{"\n"}{end}' 2>/dev/null || true)"
grep -qx "/tmp" <<<"$mount_paths" || fail "app writable mounts missing /tmp"
grep -qx "/var/log/app" <<<"$mount_paths" || fail "app writable mounts missing /var/log/app"

svc_selector="$("${KUBECTL[@]}" get service app -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
svc_port="$("${KUBECTL[@]}" get service app -n "$namespace" -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || true)"
svc_target="$("${KUBECTL[@]}" get service app -n "$namespace" -o jsonpath='{.spec.ports[0].targetPort}' 2>/dev/null || true)"
if [[ "$svc_selector" != "app" || "$svc_port" != "80" || "$svc_target" != "80" ]]; then
  fail "app service changed"
fi

echo "PASS: readonly filesystem hardening preserved with scoped writable mounts"
