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
if [[ "$web_image" != "nginx:1.27-alpine" || "$web_replicas" != "2" || "$web_selector" != "web" || "$web_label" != "web" ]]; then
  fail "web deployment changed"
fi

coredns_ready="$("${KUBECTL[@]}" get deployment coredns -n kube-system -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$coredns_ready" == "2/2" || "$coredns_ready" == "1/1" ]] || fail "coredns deployment is not ready: ${coredns_ready:-<none>}"

corefile="$("${KUBECTL[@]}" get configmap coredns -n kube-system -o jsonpath='{.data.Corefile}' 2>/dev/null || true)"
[[ "$corefile" != *"192.0.2.1"* ]] || fail "CoreDNS still forwards to broken resolver"
[[ "$corefile" == *"kubernetes cluster.local"* ]] || fail "CoreDNS kubernetes plugin changed"
[[ "$corefile" == *"forward ."* ]] || fail "CoreDNS forward resolver not restored"

"${KUBECTL[@]}" exec dns-client -n "$namespace" -- nslookup web.bench.svc.cluster.local >/dev/null 2>&1 || fail "dns-client cannot resolve web service"
"${KUBECTL[@]}" exec dns-client -n "$namespace" -- nslookup kubernetes.default.svc.cluster.local >/dev/null 2>&1 || fail "dns-client cannot resolve kubernetes service"
"${KUBECTL[@]}" exec dns-client -n "$namespace" -- nslookup example.com >/dev/null 2>&1 || fail "dns-client cannot resolve external DNS"

echo "PASS: DNS resolution restored with web workload preserved"
