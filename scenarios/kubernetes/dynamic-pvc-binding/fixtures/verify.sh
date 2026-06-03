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

provisioner="$("${KUBECTL[@]}" get storageclass standard-rwo -o jsonpath='{.provisioner}' 2>/dev/null || true)"
binding="$("${KUBECTL[@]}" get storageclass standard-rwo -o jsonpath='{.volumeBindingMode}' 2>/dev/null || true)"
reclaim="$("${KUBECTL[@]}" get storageclass standard-rwo -o jsonpath='{.reclaimPolicy}' 2>/dev/null || true)"
if [[ "$provisioner" != "rancher.io/local-path" || "$binding" != "WaitForFirstConsumer" || "$reclaim" != "Delete" ]]; then
  fail "standard-rwo StorageClass changed"
fi

pvc_json="$("${KUBECTL[@]}" get pvc app-data -n "$namespace" -o json 2>/dev/null || true)"
[[ -n "$pvc_json" ]] || fail "app-data PVC is missing"
access_modes="$(jq -r '
  if ((.status.accessModes // []) | length) > 0 then
    .status.accessModes
  elif ((.spec.accessModes // []) | length) > 0 then
    .spec.accessModes
  else
    []
  end | join(" ")
' <<<"$pvc_json" | xargs || true)"
[[ "$access_modes" == "ReadWriteOnce" ]] || fail "app-data PVC accessModes are ${access_modes:-<none>}, expected ReadWriteOnce"

pvc_class="$("${KUBECTL[@]}" get pvc app-data -n "$namespace" -o jsonpath='{.spec.storageClassName}' 2>/dev/null || true)"
pvc_size="$("${KUBECTL[@]}" get pvc app-data -n "$namespace" -o jsonpath='{.spec.resources.requests.storage}' 2>/dev/null || true)"
pvc_phase="$("${KUBECTL[@]}" get pvc app-data -n "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
[[ "$pvc_class" == "standard-rwo" ]] || fail "app-data PVC storageClassName changed"
[[ "$pvc_size" == "1Gi" ]] || fail "app-data PVC size changed"
[[ "$pvc_phase" == "Bound" ]] || fail "app-data PVC is not Bound"

ready="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "1/1" ]] || fail "app deployment is not ready: ${ready:-<none>}"

image="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="app")].image}' 2>/dev/null || true)"
port="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="app")].ports[0].containerPort}' 2>/dev/null || true)"
replicas="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
label="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
claim="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.volumes[?(@.name=="data")].persistentVolumeClaim.claimName}' 2>/dev/null || true)"
mount="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="app")].volumeMounts[?(@.name=="data")].mountPath}' 2>/dev/null || true)"
if [[ "$image" != "nginx:1.27-alpine" || "$port" != "80" || "$replicas" != "1" ||
  "$selector" != "app" || "$label" != "app" || "$claim" != "app-data" || "$mount" != "/data" ]]; then
  fail "app deployment storage contract changed"
fi

echo "PASS: PVC binding repaired while preserving deployment storage contract"
