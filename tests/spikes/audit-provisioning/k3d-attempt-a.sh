#!/usr/bin/env bash
# Audit provisioning spike, k3d/k3s: k3s embeds the API server (goroutine,
# not a static pod), so audit flags only need the policy file to exist
# inside the server container: create cluster with args, docker cp the
# policy (DooD-reachable), docker restart, probe from a container attached
# to the cluster network (emulates the runner's real perspective; host
# loopback of a port mapping would NOT be reachable from a DooD runner on
# Docker Desktop).
set -euo pipefail

CLUSTER=evidra-audit-spike-k3d
NET="k3d-$CLUSTER"
SRV="k3d-$CLUSTER-server-0"
NONCE="k3dspike-$(date +%s)-$$"
fail() { echo "SPIKE-FAIL(k3d): $*" >&2; exit 1; }
pass() { echo "SPIKE-OK(k3d): $*"; }
[[ -z "${EVIDRA_SPIKE_KEEP:-}" ]] && trap 'k3d cluster delete "$CLUSTER" >/dev/null 2>&1 || true' EXIT
command -v k3d >/dev/null || fail "k3d missing"

k3d cluster delete "$CLUSTER" >/dev/null 2>&1 || true
docker volume rm evidra-audit-spike-k3d-cfg >/dev/null 2>&1 || true
[[ -z "${EVIDRA_SPIKE_KEEP:-}" ]] && trap 'k3d cluster delete "$CLUSTER" >/dev/null 2>&1 || true; docker volume rm evidra-audit-spike-k3d-cfg >/dev/null 2>&1 || true' EXIT

# 1. Policy staged into a NAMED docker volume (no daemon-host path needed:
#    volumes live on the daemon and k3d mounts them fine from a DooD runner).
docker volume create evidra-audit-spike-k3d-cfg >/dev/null
docker run --rm -i -v evidra-audit-spike-k3d-cfg:/data alpine:3.22 \
  sh -c 'cat > /data/policy.yaml' <<'YAML'
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: Metadata
    resources: [{group: "", resources: ["configmaps"]}]
  - level: Request
    verbs: ["create", "update", "patch", "delete", "deletecollection"]
  - level: None
YAML

k3d cluster create "$CLUSTER" \
  --volume "evidra-audit-spike-k3d-cfg:/etc/kubernetes/audit@server:0" \
  --k3s-arg "--kube-apiserver-arg=audit-policy-file=/etc/kubernetes/audit/policy.yaml@server:0" \
  --k3s-arg "--kube-apiserver-arg=audit-log-path=/var/log/kubernetes/audit.log@server:0" \
  --k3s-arg "--kube-apiserver-arg=audit-log-maxage=1@server:0" \
  --k3s-arg "--kube-apiserver-arg=audit-log-maxsize=16@server:0" \
  --k3s-arg "--kube-apiserver-arg=audit-log-maxbackup=1@server:0" \
  --wait --timeout 180s >/dev/null
docker exec "$SRV" mkdir -p /var/log/kubernetes
pass "k3s created with audit args, policy via named volume"
KCTL() { docker exec -i -e KUBECONFIG=/etc/rancher/k3s/k3s.yaml "$SRV" kubectl "$@"; }

# 3. Marker = harness CERT identity resource GETs with nonce names (matches
#    the kind script; production uses one collector path for both providers.
#    k3s admin kubeconfig uses client certs -> instant, no bearer cold window.
admin_marker() { # $1=label -> succeeds when GET returns NotFound (audited)
  local out
  out=$(KCTL get configmap "evidra-window-$NONCE-$1" -n default 2>&1) || true
  printf '%s' "$out" | grep -q "NotFound"
}
admin_marker start || fail "cert start marker GET did not 404"
admin_marker ungranted || fail "cert control marker GET did not 404"
pass "cert markers audited instantly via harness identity"

# SA bearer measurement (production readiness-gate data; k3s expected ~0s).
KCTL create serviceaccount evidra-marker >/dev/null
KCTL create role evidra-window-get --resource=configmaps --verb=get -n default >/dev/null
KCTL create rolebinding evidra-marker-window --role=evidra-window-get --serviceaccount=default:evidra-marker -n default >/dev/null
SA_T0=$(date +%s)
TOKEN=$(KCTL create token evidra-marker --duration 600s)
SA_AUTH="no"
for try in $(seq 1 30); do
  R=$(docker run --rm --network "$NET" curlimages/curl:8.10.1 -sk \
      -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $TOKEN" \
      "https://$SRV:6443/api/v1/namespaces/default/configmaps/sa-warmup-$NONCE" 2>/dev/null || true)
  [[ "$R" == "404" ]] && { SA_AUTH="yes"; break; }   # 404 = authenticated+granted
  sleep 2                                             # (403/401/000 or anon-403 = not yet)
done
if [[ "$SA_AUTH" == "yes" ]]; then
  pass "k3s SA bearer authenticated after $(( $(date +%s) - SA_T0 ))s"
else
  echo "SPIKE-NOTE(k3d): SA bearer not AUTHed within $(( $(date +%s) - SA_T0 ))s"
fi

# 4. Log retrieval + attribution + stages.
sleep 5
LOG=$(docker exec "$SRV" cat /var/log/kubernetes/audit.log 2>/dev/null || true)
[[ -n "$LOG" ]] || fail "audit log empty/unreadable"
if [[ "$LOG" == *"evidra-window-$NONCE-ungranted"* && "$LOG" == *"evidra-window-$NONCE-start"* ]]; then
  pass "nonce preserved in audit requestURI (both markers)"
else
  echo "SPIKE-NOTE(k3d): nonce NOT visible in requestURI -> use Audit-ID correlation"
fi
[[ "$LOG" == *'"username":"system:admin"'* ]] \
  && pass "cert marker identity attributed" || fail "marker identity missing from log"
if [[ "$SA_AUTH" == "yes" ]]; then
  [[ "$LOG" == *"system:serviceaccount:default:evidra-marker"* ]] \
    && pass "SA warm-up probe attributed" || fail "SA probe expected but not in log"
fi
printf '%s' "$LOG" | python3 -c '
import json, sys
ids = {}
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        e = json.loads(line)
    except Exception:
        continue
    ids.setdefault(e["auditID"], set()).add(e["stage"])
term = sum(1 for s in ids.values() if s & {"ResponseComplete", "Panic"})
print(f"SPIKE-OK(k3d): {len(ids)} auditIDs observed, {term} terminal")'
pass "k3d mechanism proven on this host"
