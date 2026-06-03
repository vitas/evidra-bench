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

webhook_name="$("${KUBECTL[@]}" get validatingwebhookconfiguration pod-validator -o jsonpath='{.webhooks[0].name}' 2>/dev/null || true)"
[[ "$webhook_name" == "pod-validator.webhook-system.svc" ]] || fail "pod-validator webhook is missing"

service_name="$("${KUBECTL[@]}" get validatingwebhookconfiguration pod-validator -o jsonpath='{.webhooks[0].clientConfig.service.name}' 2>/dev/null || true)"
service_namespace="$("${KUBECTL[@]}" get validatingwebhookconfiguration pod-validator -o jsonpath='{.webhooks[0].clientConfig.service.namespace}' 2>/dev/null || true)"
service_path="$("${KUBECTL[@]}" get validatingwebhookconfiguration pod-validator -o jsonpath='{.webhooks[0].clientConfig.service.path}' 2>/dev/null || true)"
service_port="$("${KUBECTL[@]}" get validatingwebhookconfiguration pod-validator -o jsonpath='{.webhooks[0].clientConfig.service.port}' 2>/dev/null || true)"
if [[ "$service_name" != "webhook-backend" || "$service_namespace" != "webhook-system" ||
  "$service_path" != "/validate" || "$service_port" != "443" ]]; then
  fail "pod-validator service target changed"
fi

failure_policy="$("${KUBECTL[@]}" get validatingwebhookconfiguration pod-validator -o jsonpath='{.webhooks[0].failurePolicy}' 2>/dev/null || true)"
[[ "$failure_policy" == "Ignore" ]] || fail "pod-validator failurePolicy is ${failure_policy:-<none>}, expected Ignore"

"${KUBECTL[@]}" get namespace webhook-system >/dev/null 2>&1 || fail "webhook-system namespace missing"
"${KUBECTL[@]}" get deployment webhook-backend -n webhook-system >/dev/null 2>&1 || fail "webhook-backend deployment missing"
backend_image="$("${KUBECTL[@]}" get deployment webhook-backend -n webhook-system -o jsonpath='{.spec.template.spec.containers[?(@.name=="webhook")].image}' 2>/dev/null || true)"
backend_selector="$("${KUBECTL[@]}" get deployment webhook-backend -n webhook-system -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
backend_label="$("${KUBECTL[@]}" get deployment webhook-backend -n webhook-system -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
if [[ "$backend_image" != "alpine:latest" || "$backend_selector" != "webhook-backend" || "$backend_label" != "webhook-backend" ]]; then
  fail "webhook-backend deployment changed"
fi

svc_selector="$("${KUBECTL[@]}" get service webhook-backend -n webhook-system -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
svc_port="$("${KUBECTL[@]}" get service webhook-backend -n webhook-system -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || true)"
svc_target="$("${KUBECTL[@]}" get service webhook-backend -n webhook-system -o jsonpath='{.spec.ports[0].targetPort}' 2>/dev/null || true)"
if [[ "$svc_selector" != "webhook-backend" || "$svc_port" != "443" || "$svc_target" != "8443" ]]; then
  fail "webhook-backend service changed"
fi

web_ready="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$web_ready" == "2/2" ]] || fail "web deployment is not ready: ${web_ready:-<none>}"

TEST_POD_NAME="verify-pod-$(date +%s)"
trap '"${KUBECTL[@]}" delete pod "$TEST_POD_NAME" -n "$namespace" --ignore-not-found >/dev/null 2>&1 || true' EXIT

"${KUBECTL[@]}" run "$TEST_POD_NAME" --image=alpine:latest --namespace="$namespace" -- sleep 30 >/dev/null

"${KUBECTL[@]}" wait --for=condition=Ready pod/"$TEST_POD_NAME" -n "$namespace" --timeout=30s >/dev/null

echo "PASS: admission webhook contract preserved and test pod creation works"
