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

traffic_policy="$("${KUBECTL[@]}" get service web-nodeport -n "$namespace" -o jsonpath='{.spec.externalTrafficPolicy}' 2>/dev/null || true)"
[[ "$traffic_policy" == "Local" ]] ||
  fail "web-nodeport service externalTrafficPolicy is ${traffic_policy:-<none>}, expected Local"

service_type="$("${KUBECTL[@]}" get service web-nodeport -n "$namespace" -o jsonpath='{.spec.type}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get service web-nodeport -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
port="$("${KUBECTL[@]}" get service web-nodeport -n "$namespace" -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || true)"
target_port="$("${KUBECTL[@]}" get service web-nodeport -n "$namespace" -o jsonpath='{.spec.ports[0].targetPort}' 2>/dev/null || true)"
node_port="$("${KUBECTL[@]}" get service web-nodeport -n "$namespace" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || true)"
[[ "$service_type" == "NodePort" ]] || fail "web-nodeport service type changed to ${service_type:-<none>}"
[[ "$selector" == "web" ]] || fail "web-nodeport service selector changed to ${selector:-<none>}"
[[ "$port" == "80" && "$target_port" == "80" ]] || fail "web-nodeport service port changed"
[[ "$node_port" == "30080" ]] || fail "web-nodeport nodePort changed to ${node_port:-<none>}"

ready="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "2/2" ]] || fail "Deployment web is not ready: ${ready:-<none>}"

replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
image="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
deploy_selector="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
pod_label="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
probe_path="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.path}' 2>/dev/null || true)"
if [[ "$replicas" != "2" || "$image" != "nginx:1.27-alpine" || "$deploy_selector" != "web" ||
  "$pod_label" != "web" || "$probe_path" != "/" ]]; then
  fail "web deployment changed"
fi

endpoints="$("${KUBECTL[@]}" get endpoints web-nodeport -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}' | wc -c | tr -d ' ')"
[[ "$endpoints" -gt 0 ]] || fail "No endpoints found for web-nodeport service"

echo "PASS: NodePort service has externalTrafficPolicy: Local, deployment ready, and endpoints exist"
