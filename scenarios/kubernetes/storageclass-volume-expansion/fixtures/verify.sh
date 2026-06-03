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

allow_expansion="$("${KUBECTL[@]}" get storageclass bench-storage -o jsonpath='{.allowVolumeExpansion}' 2>/dev/null || true)"
[[ "$allow_expansion" == "true" ]] ||
  fail "StorageClass bench-storage does not have allowVolumeExpansion: true"

provisioner="$("${KUBECTL[@]}" get storageclass bench-storage -o jsonpath='{.provisioner}' 2>/dev/null || true)"
binding_mode="$("${KUBECTL[@]}" get storageclass bench-storage -o jsonpath='{.volumeBindingMode}' 2>/dev/null || true)"
reclaim_policy="$("${KUBECTL[@]}" get storageclass bench-storage -o jsonpath='{.reclaimPolicy}' 2>/dev/null || true)"
if [[ "$provisioner" != "rancher.io/local-path" || "$binding_mode" != "WaitForFirstConsumer" || "$reclaim_policy" != "Delete" ]]; then
  fail "StorageClass bench-storage changed"
fi

pvc_size="$("${KUBECTL[@]}" get pvc app-data -n "$namespace" -o jsonpath='{.spec.resources.requests.storage}' 2>/dev/null || true)"
[[ "$pvc_size" == "5Gi" ]] || fail "PVC app-data size is ${pvc_size:-<none>}, expected 5Gi"

storage_class="$("${KUBECTL[@]}" get pvc app-data -n "$namespace" -o jsonpath='{.spec.storageClassName}' 2>/dev/null || true)"
[[ "$storage_class" == "bench-storage" ]] || fail "PVC app-data storageClassName changed to ${storage_class:-<none>}"

phase="$("${KUBECTL[@]}" get pvc app-data -n "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
[[ "$phase" == "Bound" ]] || fail "PVC app-data is not Bound: ${phase:-<none>}"

ready="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "1/1" ]] || fail "Deployment web is not ready: ${ready:-<none>}"

replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
image="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
label="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
claim="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.volumes[?(@.name=="app-data")].persistentVolumeClaim.claimName}' 2>/dev/null || true)"
mount="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].volumeMounts[?(@.name=="app-data")].mountPath}' 2>/dev/null || true)"
probe_path="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.path}' 2>/dev/null || true)"
probe_port="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.port}' 2>/dev/null || true)"
if [[ "$replicas" != "1" || "$image" != "nginx:1.27-alpine" || "$selector" != "web" || "$label" != "web" ||
  "$claim" != "app-data" || "$mount" != "/var/log/nginx" || "$probe_path" != "/" || "$probe_port" != "80" ]]; then
  fail "web deployment changed"
fi

marker="$("${KUBECTL[@]}" get configmap storage-status -n "$namespace" -o jsonpath='{.data.status}' 2>/dev/null || true)"
[[ "$marker" == "disk_almost_full" ]] || fail "storage status marker changed"

echo "PASS: StorageClass allows expansion, PVC resized to 5Gi, and deployment is ready"
