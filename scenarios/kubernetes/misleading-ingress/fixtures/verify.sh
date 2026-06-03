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

ready="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "2/2" ]] || fail "web deployment is not ready: ${ready:-<none>}"

image="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
label="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
port="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].ports[0].containerPort}' 2>/dev/null || true)"
[[ "$image" == "nginx:1.27-alpine" ]] || fail "web deployment image is $image"
[[ "$replicas" == "2" && "$selector" == "web" && "$label" == "web" && "$port" == "80" ]] || fail "web deployment changed"

external_json="$("${KUBECTL[@]}" get service web-external -n "$namespace" -o json 2>/dev/null || true)"
[[ -n "$external_json" ]] || fail "web-external service changed"
external_type="$(jq -r '.spec.type // ""' <<<"$external_json")"
external_selector="$(jq -r '.spec.selector.app // ""' <<<"$external_json")"
external_port="$(jq -r '.spec.ports[0].port // ""' <<<"$external_json")"
external_target="$(jq -r '.spec.ports[0].targetPort // ""' <<<"$external_json")"
external_node_port="$(jq -r '.spec.ports[0].nodePort // ""' <<<"$external_json")"
if [[ "$external_type" != "NodePort" || "$external_selector" != "web" ||
  "$external_port" != "80" || "$external_target" != "80" || "$external_node_port" != "30080" ]]; then
  fail "web-external service changed"
fi

service_selector="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
service_port="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || true)"
service_target="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.ports[0].targetPort}' 2>/dev/null || true)"
[[ "$service_selector" == "web" && "$service_port" == "80" && "$service_target" == "80" ]] || fail "web service changed"

web_endpoint_count="$("${KUBECTL[@]}" get endpoints web -n "$namespace" -o json | jq '[.subsets[]?.addresses[]?] | length')"
external_endpoint_count="$("${KUBECTL[@]}" get endpoints web-external -n "$namespace" -o json | jq '[.subsets[]?.addresses[]?] | length')"
(( web_endpoint_count > 0 )) || fail "web service endpoints are missing"
(( external_endpoint_count > 0 )) || fail "web-external NodePort is not reachable because endpoints are missing"

echo "PASS: external web access restored while preserving service contracts"
