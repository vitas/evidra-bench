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

pvc_phase="$("${KUBECTL[@]}" get pvc app-data -n bench -o jsonpath='{.status.phase}' 2>/dev/null || true)"
[[ "$pvc_phase" == "Bound" ]] || fail "app-data PVC is missing or not Bound: ${pvc_phase:-<none>}"

available_class="$("${KUBECTL[@]}" get storageclass standard-rwo -o jsonpath='{.metadata.name}' 2>/dev/null || true)"
[[ "$available_class" == "standard-rwo" ]] || fail "standard-rwo StorageClass is missing"

storage_class="$("${KUBECTL[@]}" get pvc app-data -n bench -o jsonpath='{.spec.storageClassName}' 2>/dev/null || true)"
[[ "$storage_class" == "standard-rwo" ]] || fail "app-data PVC storageClassName is ${storage_class:-<none>}, want standard-rwo"

claim="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.spec.volumes[?(@.name=="data")].persistentVolumeClaim.claimName}' 2>/dev/null || true)"
mount="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].volumeMounts[?(@.name=="data")].mountPath}' 2>/dev/null || true)"
[[ "$claim" == "app-data" ]] || fail "app deployment volume claim changed to ${claim:-<none>}"
[[ "$mount" == "/usr/share/nginx/html" ]] || fail "app deployment volume mount changed to ${mount:-<none>}"

replicas="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
image="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
label="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
if [[ "$replicas" != "1" || "$image" != "nginx:1.27-alpine" || "$selector" != "app" || "$label" != "app" ]]; then
  fail "app deployment changed"
fi

echo "PASS: PVC/app-data storage class repaired while Deployment/app still mounts it"
