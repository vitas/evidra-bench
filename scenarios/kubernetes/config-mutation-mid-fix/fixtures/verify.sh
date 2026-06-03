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

mode="$("${KUBECTL[@]}" get configmap web-config -n "$namespace" -o jsonpath='{.data.mode}' 2>/dev/null || true)"
[[ "$mode" == "good" ]] || fail "web-config mode is ${mode:-<none>}"

ready="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "2/2" ]] || fail "web deployment is not ready: ${ready:-<none>}"

replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
image="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="web")].image}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
pod_label="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
command="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o go-template='{{range .spec.template.spec.containers}}{{if eq .name "web"}}{{range .command}}{{.}} {{end}}{{end}}{{end}}' 2>/dev/null || true)"
if [[ "$replicas" != "2" || "$image" != "busybox:1.36" || "$selector" != "web" || "$pod_label" != "web" ]] ||
  ! grep -q "httpd -f -p 8080 -h /www" <<<"$command"; then
  fail "web deployment changed"
fi

readiness_cmd="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o go-template='{{range .spec.template.spec.containers}}{{if eq .name "web"}}{{range .readinessProbe.exec.command}}{{.}} {{end}}{{end}}{{end}}' 2>/dev/null || true)"
if ! grep -q 'cat /config/mode' <<<"$readiness_cmd" || ! grep -q '= good' <<<"$readiness_cmd"; then
  fail "web readinessProbe changed"
fi

config_mount="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="web")].volumeMounts[?(@.name=="config")].mountPath}' 2>/dev/null || true)"
config_volume="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.volumes[?(@.name=="config")].configMap.name}' 2>/dev/null || true)"
web_root="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="web")].volumeMounts[?(@.name=="web-root")].mountPath}' 2>/dev/null || true)"
if [[ "$config_mount" != "/config" || "$config_volume" != "web-config" || "$web_root" != "/www" ]]; then
  fail "web config mount changed"
fi

service_selector="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
service_port="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.ports[0].targetPort}' 2>/dev/null || true)"
[[ "$service_selector" == "web" && "$service_port" == "8080" ]] || fail "web service changed"

echo "PASS: config was restored after drift while deployment stayed intact"
