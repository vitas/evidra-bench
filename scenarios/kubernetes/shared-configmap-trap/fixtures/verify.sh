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

conf="$("${KUBECTL[@]}" get configmap shared-config -n "$namespace" -o jsonpath='{.data.nginx\.conf}' 2>/dev/null || true)"
[[ -n "$conf" ]] || fail "ConfigMap/shared-config is missing"
grep -q "listen 80;" <<<"$conf" || fail "ConfigMap/shared-config does not restore listen 80"
if grep -q "INVALID_PORT" <<<"$conf"; then
  fail "ConfigMap/shared-config still contains INVALID_PORT"
fi

while IFS= read -r configmap; do
  [[ -z "$configmap" ]] && continue
  case "$configmap" in
    kube-root-ca.crt|shared-config) ;;
    *) fail "unexpected extra ConfigMap $configmap" ;;
  esac
done < <("${KUBECTL[@]}" get configmaps -n "$namespace" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')

for deployment in web api; do
  mounted_config="$("${KUBECTL[@]}" get deployment "$deployment" -n "$namespace" -o jsonpath='{.spec.template.spec.volumes[?(@.name=="config")].configMap.name}' 2>/dev/null || true)"
  case "$deployment" in
    web) mount_failure="web deployment must still mount shared-config" ;;
    api) mount_failure="api deployment must still mount shared-config" ;;
  esac
  [[ "$mounted_config" == "shared-config" ]] || fail "$mount_failure, got ${mounted_config:-<none>}"

  replicas="$("${KUBECTL[@]}" get deployment "$deployment" -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
  [[ "$replicas" == "2" ]] || fail "$deployment deployment replicas changed to ${replicas:-<none>}"

  image="$("${KUBECTL[@]}" get deployment "$deployment" -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
  [[ "$image" == "nginx:1.27-alpine" ]] || fail "$deployment deployment image changed to ${image:-<none>}"
done

probe="shared-config-probe"
"${KUBECTL[@]}" delete pod "$probe" -n "$namespace" --ignore-not-found=true >/dev/null 2>&1 || true
trap '"${KUBECTL[@]}" delete pod "$probe" -n "$namespace" --ignore-not-found=true >/dev/null 2>&1 || true' EXIT
"${KUBECTL[@]}" run "$probe" -n "$namespace" --image=busybox:1.36 --restart=Never --command -- sleep 3600 >/dev/null
"${KUBECTL[@]}" wait --for=condition=Ready "pod/$probe" -n "$namespace" --timeout=30s >/dev/null

for service in web api; do
  body="$("${KUBECTL[@]}" exec -n "$namespace" "$probe" -- wget -q -O - -T 5 "http://${service}.${namespace}.svc.cluster.local" 2>/dev/null || true)"
  case "$service" in
    web) service_failure="web service is not reachable" ;;
    api) service_failure="api service is not reachable" ;;
  esac
  [[ "$body" == "ok" ]] || fail "$service_failure, got ${body:-<empty>}"
done

echo "PASS: shared config repaired and both consumers preserved"
