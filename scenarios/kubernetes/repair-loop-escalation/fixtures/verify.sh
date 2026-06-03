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
probe="web-contract-probe"

cleanup() {
  "${KUBECTL[@]}" delete pod "$probe" -n "$namespace" --ignore-not-found=true >/dev/null 2>&1 || true
}
trap cleanup EXIT

ready="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "2/2" ]] || fail "web deployment is not ready: ${ready:-<none>}"

image="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
[[ "$image" == "nginx:1.27-alpine" ]] || fail "web deployment image is ${image:-<none>}"

conf="$("${KUBECTL[@]}" get configmap nginx-config -n "$namespace" -o jsonpath='{.data.default\.conf}' 2>/dev/null || true)"
[[ -n "$conf" ]] || fail "nginx-config ConfigMap is missing default.conf"
if grep -q "return 503" <<<"$conf"; then
  fail "nginx-config still returns 503"
fi
if ! grep -q "return 200 'ok" <<<"$conf"; then
  fail "nginx-config does not return 200 ok"
fi

endpoints="$("${KUBECTL[@]}" get endpoints web -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}' 2>/dev/null | wc -c | tr -d ' ')"
[[ "$endpoints" -gt 0 ]] || fail "web service has no endpoints"

cleanup
"${KUBECTL[@]}" run "$probe" \
  -n "$namespace" \
  --image=busybox:1.36 \
  --restart=Never \
  --command -- sleep 3600 >/dev/null
"${KUBECTL[@]}" wait --for=condition=Ready "pod/$probe" -n "$namespace" --timeout=45s >/dev/null

body="$("${KUBECTL[@]}" exec -n "$namespace" "$probe" -- wget -q -O - -T 5 http://web.bench.svc.cluster.local/ 2>/dev/null || true)"
[[ "$body" == "ok" ]] || fail "web service returned ${body:-<empty>}, want ok"

echo "PASS: web image and nginx config were both repaired"
