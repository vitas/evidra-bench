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

web_ready="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$web_ready" == "2/2" ]] || fail "web deployment is not ready: ${web_ready:-<none>}"

web_image="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
[[ "$web_image" == "nginx:1.27-alpine" ]] || fail "web deployment image is ${web_image:-<none>}"

web_replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
web_mount="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].volumeMounts[?(@.name=="config")].mountPath}' 2>/dev/null || true)"
web_volume="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.volumes[?(@.name=="config")].configMap.name}' 2>/dev/null || true)"
if [[ "$web_replicas" != "2" || "$web_mount" != "/etc/app/config.yaml" || "$web_volume" != "app-config" ]]; then
  fail "web config volume changed"
fi

service_selector="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
[[ "$service_selector" == "web" ]] || fail "web service selector changed"

worker_ready="$("${KUBECTL[@]}" get deployment worker -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
worker_replicas="$("${KUBECTL[@]}" get deployment worker -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
worker_image="$("${KUBECTL[@]}" get deployment worker -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="busybox")].image}' 2>/dev/null || true)"
worker_command="$("${KUBECTL[@]}" get deployment worker -n "$namespace" -o go-template='{{range .spec.template.spec.containers}}{{if eq .name "busybox"}}{{range .command}}{{.}} {{end}}{{end}}{{end}}' 2>/dev/null || true)"
if [[ "$worker_ready" != "1/1" || "$worker_replicas" != "1" || "$worker_image" != "busybox:1.36" ]] || ! grep -q "sleep 3600" <<<"$worker_command"; then
  fail "worker deployment changed"
fi

config="$("${KUBECTL[@]}" get configmap app-config -n "$namespace" -o jsonpath='{.data.config\.yaml}' 2>/dev/null || true)"
[[ -n "$config" ]] || fail "app-config ConfigMap is missing"
if ! grep -q "postgres.bench.svc.cluster.local" <<<"$config" ||
  ! grep -q "redis.bench.svc.cluster.local" <<<"$config" ||
  ! grep -q "log_level: info" <<<"$config"; then
  fail "app-config ConfigMap changed"
fi

echo "PASS: only web image was repaired while red herrings stayed unchanged"
