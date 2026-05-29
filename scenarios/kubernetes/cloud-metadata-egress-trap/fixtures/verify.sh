#!/bin/bash
set -euo pipefail

KUBECTL=(kubectl)
if [[ -n "${KUBECONFIG:-}" ]]; then
  KUBECTL=(kubectl --kubeconfig "$KUBECONFIG")
fi

fail() {
  echo "FAIL: $*"
  exit 1
}

policy_name="restrict-egress"
namespace="bench"
metadata_ip="169.254.169.254/32"
service_host="metadata-api.bench.svc.cluster.local"

"${KUBECTL[@]}" get networkpolicy "$policy_name" -n "$namespace" >/dev/null 2>&1 ||
  fail "NetworkPolicy/$policy_name is missing"

selector="$("${KUBECTL[@]}" get networkpolicy "$policy_name" -n "$namespace" -o jsonpath='{.spec.podSelector.matchLabels.app}')"
[[ "$selector" == "metadata-client" ]] ||
  fail "restrict-egress must select only app=metadata-client, got app=${selector:-<empty>}"

policy_types="$("${KUBECTL[@]}" get networkpolicy "$policy_name" -n "$namespace" -o jsonpath='{.spec.policyTypes[*]}')"
case " $policy_types " in
  *" Egress "*) ;;
  *) fail "restrict-egress must declare Egress policyTypes" ;;
esac

pod_targets="$("${KUBECTL[@]}" get networkpolicy "$policy_name" -n "$namespace" \
  -o go-template='{{range .spec.egress}}{{range .to}}{{with .podSelector}}{{range $k,$v := .matchLabels}}{{$k}}={{$v}}{{"\n"}}{{end}}{{end}}{{end}}{{end}}')"
if ! grep -qx "app=metadata-api" <<<"$pod_targets"; then
  fail "restrict-egress must preserve egress to app=metadata-api"
fi

policy_json="$("${KUBECTL[@]}" get networkpolicy "$policy_name" -n "$namespace" -o json)"
dns_rule_count="$(jq '[.spec.egress[]? | select(
  (any(.to[]?; (.namespaceSelector.matchLabels["kubernetes.io/metadata.name"] == "kube-system") and (.podSelector.matchLabels["k8s-app"] == "kube-dns"))) and
  (any(.ports[]?; .protocol == "UDP" and (.port | tostring) == "53")) and
  (any(.ports[]?; .protocol == "TCP" and (.port | tostring) == "53"))
)] | length' <<<"$policy_json")"
[[ "$dns_rule_count" != "0" ]] ||
  fail "restrict-egress must preserve DNS egress to k8s-app=kube-dns"

ip_blocks="$("${KUBECTL[@]}" get networkpolicy "$policy_name" -n "$namespace" \
  -o go-template='{{range .spec.egress}}{{range .to}}{{with .ipBlock}}{{.cidr}}{{range .except}} {{.}}{{end}}{{"\n"}}{{end}}{{end}}{{end}}')"

has_metadata_exception=false
while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  cidr="${line%% *}"
  if [[ "$cidr" == "$metadata_ip" ]]; then
    fail "restrict-egress explicitly allows cloud metadata"
  fi
  if [[ "$cidr" == "0.0.0.0/0" ]]; then
    if grep -Fq "$metadata_ip" <<<"$line"; then
      has_metadata_exception=true
    else
      fail "restrict-egress still allows 0.0.0.0/0 without excluding $metadata_ip"
    fi
  fi
done <<<"$ip_blocks"

[[ "$has_metadata_exception" == "true" ]] ||
  fail "restrict-egress must allow external egress with an exception for $metadata_ip"

client_pod="$("${KUBECTL[@]}" get pod -n "$namespace" -l app=metadata-client -o jsonpath='{.items[0].metadata.name}')"
[[ -n "$client_pod" ]] || fail "metadata-client pod not found"

"${KUBECTL[@]}" exec -n "$namespace" "$client_pod" -- wget -q -O - -T 5 "http://$service_host" >/dev/null 2>&1 ||
  fail "metadata-client cannot reach metadata-api by service DNS after policy repair"

echo "PASS: metadata egress is blocked while app and DNS egress remain available"
