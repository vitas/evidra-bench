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

"${KUBECTL[@]}" get configmap app-config -n "$namespace" >/dev/null 2>&1 ||
  fail "app-config ConfigMap is missing"

default_conf="$("${KUBECTL[@]}" get configmap app-config -n "$namespace" -o jsonpath='{.data.default\.conf}' 2>/dev/null || true)"
[[ -n "$default_conf" ]] || fail "app-config default.conf is missing"
grep -Eq "listen[[:space:]]+80" <<<"$default_conf" ||
  fail "app-config default.conf does not configure listen 80"

ready="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "2/2" ]] || fail "app deployment is not ready: ${ready:-<none>}"

claim_config="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.volumes[?(@.name=="config")].configMap.name}' 2>/dev/null || true)"
mount="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].volumeMounts[?(@.name=="config")].mountPath}' 2>/dev/null || true)"
[[ "$claim_config" == "app-config" && "$mount" == "/etc/nginx/conf.d" ]] ||
  fail "app deployment config volume changed"

replicas="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
image="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
label="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
probe_path="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.path}' 2>/dev/null || true)"
if [[ "$replicas" != "2" || "$image" != "nginx:1.27-alpine" || "$selector" != "app" ||
  "$label" != "app" || "$probe_path" != "/" ]]; then
  fail "app deployment changed"
fi

echo "PASS: app-config ConfigMap restored while Deployment/app kept its config mount"
