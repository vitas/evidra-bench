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
web_replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
web_selector="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
web_label="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
web_port="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].ports[0].containerPort}' 2>/dev/null || true)"
if [[ "$web_image" != "nginx:1.27-alpine" || "$web_replicas" != "2" ||
  "$web_selector" != "web" || "$web_label" != "web" || "$web_port" != "80" ]]; then
  fail "web deployment changed"
fi

service_selector="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
service_port="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || true)"
service_target="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.ports[0].targetPort}' 2>/dev/null || true)"
[[ "$service_selector" == "web" && "$service_port" == "80" && "$service_target" == "80" ]] || fail "web service changed"

client_json="$("${KUBECTL[@]}" get pod net-client -n "$namespace" -o json 2>/dev/null || true)"
[[ -n "$client_json" ]] || fail "net-client probe pod changed"
client_image="$(jq -r '.spec.containers[] | select(.name == "busybox") | .image // ""' <<<"$client_json")"
client_label="$(jq -r '.metadata.labels.app // ""' <<<"$client_json")"
[[ "$client_image" == "busybox:1.36" && "$client_label" == "net-client" ]] || fail "net-client probe pod changed"

np_json="$("${KUBECTL[@]}" get networkpolicy deny-all-ingress -n "$namespace" -o json 2>/dev/null || true)"
[[ -n "$np_json" ]] || fail "deny-all-ingress NetworkPolicy is missing"
np_selector="$(jq -r '.spec.podSelector.matchLabels.app // ""' <<<"$np_json")"
ingress_count="$(jq -r '(.spec.ingress // []) | length' <<<"$np_json")"
[[ "$np_selector" == "web" ]] || fail "deny-all-ingress NetworkPolicy selector changed"
(( ingress_count > 0 )) || fail "deny-all-ingress still blocks all ingress"

if ! jq -e '(.spec.ingress // [])[] | (.from // [])[]? | .podSelector.matchLabels.app == "net-client"' <<<"$np_json" >/dev/null; then
  fail "deny-all-ingress does not allow net-client ingress"
fi

"${KUBECTL[@]}" exec net-client -n "$namespace" -- wget -q -O - -T 5 http://web.bench.svc.cluster.local >/dev/null 2>&1 ||
  fail "web service not reachable from net-client"

echo "PASS: NetworkPolicy connectivity restored while preserving web service contract"
