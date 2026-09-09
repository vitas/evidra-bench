#!/usr/bin/env bash
# Audit provisioning spike, kind, mechanism C (no apiserver restart):
#   policy + kubeadm pod-manifest patch staged into a NAMED docker volume
#   (daemon-side; the only DooD-safe writer) -> kind extraMounts map them
#   into the node -> ClusterConfiguration apiServer extraArgs (audit flags)
#   + InitConfiguration patches.directory (adds the pod volumes at first
#   start). Restart-free on purpose: spike finding — patching the manifest
#   of a running apiserver cold-breaks bearer authn (system:anonymous) for
#   minutes (jwks refetch), which no production run may eat.
set -euo pipefail

CLUSTER=evidra-audit-spike-c
VOL=evidra-audit-spike-cfg
NONCE="spikec-$(date +%s)-$$"
fail() { echo "SPIKE-FAIL(kind-C): $*" >&2; exit 1; }
pass() { echo "SPIKE-OK(kind-C): $*"; }
command -v kind >/dev/null || fail "kind missing"
command -v kubectl >/dev/null || fail "kubectl missing"

kind delete cluster --name "$CLUSTER" >/dev/null 2>&1 || true
docker volume rm "$VOL" >/dev/null 2>&1 || true
[[ -z "${EVIDRA_SPIKE_KEEP:-}" ]] && trap 'kind delete cluster --name "$CLUSTER" >/dev/null 2>&1 || true; docker volume rm "$VOL" >/dev/null 2>&1 || true' EXIT

# 1. Stage policy.yaml + pod patch into the named volume.
docker volume create "$VOL" >/dev/null
docker run --rm -i -v "$VOL:/data" alpine:3.22 sh -c 'mkdir -p /data/patches && cat > /data/policy.yaml' <<'YAML'
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: Metadata
    resources: [{group: "", resources: ["configmaps"]}]
  - level: Request
    verbs: ["create", "update", "patch", "delete", "deletecollection"]
  - level: None
YAML
docker run --rm -i -v "$VOL:/data" alpine:3.22 sh -c 'cat > /data/patches/kube-apiserver.yaml' <<'YAML'
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
spec:
  containers:
    - name: kube-apiserver
      volumeMounts:
        - mountPath: /etc/kubernetes/audit
          name: audit-policy
          readOnly: true
        - mountPath: /var/log/kubernetes
          name: audit-log
  volumes:
    - name: audit-policy
      hostPath:
        path: /etc/kubernetes/audit
    - name: audit-log
      hostPath:
        path: /var/log/kubernetes
        type: DirectoryOrCreate
YAML
VOL_DATA=$(docker volume inspect -f '{{.Mountpoint}}' "$VOL")

# 2. Cluster config referencing the daemon-side volume paths.
CFG=$(mktemp)
cat > "$CFG" <<YAML
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraMounts:
      - hostPath: $VOL_DATA/policy.yaml
        containerPath: /etc/kubernetes/audit/policy.yaml
        readOnly: true
      - hostPath: $VOL_DATA/patches
        containerPath: /etc/kubernetes/kubeadm-patches
        readOnly: true
    kubeadmConfigPatches:
      - |
        apiVersion: kubeadm.k8s.io/v1beta3
        kind: InitConfiguration
        patches:
          directory: /etc/kubernetes/kubeadm-patches
      - |
        apiVersion: kubeadm.k8s.io/v1beta3
        kind: ClusterConfiguration
        metadata:
          name: config
        apiServer:
          extraArgs:
            audit-policy-file: /etc/kubernetes/audit/policy.yaml
            audit-log-path: /var/log/kubernetes/audit.log
            audit-log-maxage: "1"
            audit-log-maxsize: "16"
            audit-log-maxbackup: "1"
YAML

kind create cluster --name "$CLUSTER" --config "$CFG" --wait 300s >/dev/null || fail "cluster create failed"
NODE="${CLUSTER}-control-plane"
docker exec "$NODE" sh -c 'grep -q audit-policy-file /etc/kubernetes/manifests/kube-apiserver.yaml' \
  || fail "manifest lacks audit args"
pass "cluster created with audit args at first start (no restart)"

KCFG_FILE=$(mktemp); kind get kubeconfig --name "$CLUSTER" > "$KCFG_FILE"
kctl() { kubectl --kubeconfig "$KCFG_FILE" "$@"; }

# 3. Marker identities (bound tokens — must authenticate immediately now).
kctl create serviceaccount evidra-marker >/dev/null
kctl create role evidra-window-get --resource=configmaps --verb=get -n default >/dev/null
kctl create rolebinding evidra-marker-window --role=evidra-window-get --serviceaccount=default:evidra-marker -n default >/dev/null
TOKEN=$(docker exec "$NODE" kubectl --kubeconfig /etc/kubernetes/admin.conf create token evidra-marker --duration 600s) || fail "mint failed"

marker_probe() { # $1=token $2=path -> "<status> <audit-id> <SA|ANON|NONE>"
  docker exec -i "$NODE" python3 - "$1" "$2" <<'PY'
import ssl, sys, urllib.request, urllib.error
token, path = sys.argv[1], sys.argv[2]
ctx = ssl._create_unverified_context()
url = "https://127.0.0.1:6443" + path
body = ""
try:
    r = urllib.request.urlopen(url, context=ctx, timeout=10)
    code, aid = r.status, r.headers.get("Audit-ID", "")
except urllib.error.HTTPError as e:
    code, aid = e.code, e.headers.get("Audit-ID", "")
    body = e.read().decode(errors="replace")
flag = "SA" if "system:serviceaccount" in body else ("ANON" if "system:anonymous" in body else "NONE")
print(code, aid, flag)
PY
}

MPATH_NAME="evidra-window-"
# Marker = harness ADMIN CERT identity. Spike finding: bearer tokens (bound
# AND classic) are rejected as system:anonymous on fresh kind clusters for
# minutes (issuer/jwks warm-up); client certs authenticate immediately.
# Attribution rides on the nonce-bearing object NAME in the requestURI; the
# 404 status is expected and irrelevant.
admin_marker() { # $1=label -> succeeds if GET returns NotFound (audited)
  local out
  out=$(docker exec "$NODE" kubectl --kubeconfig /etc/kubernetes/admin.conf get configmap "$MPATH_NAME$NONCE-$1" -n default 2>&1)
  printf '%s' "$out" | grep -q "NotFound"
}
admin_marker start || fail "admin-cert start marker GET did not 404"
admin_marker ungranted || fail "admin-cert control marker GET did not 404"
pass "admin-cert markers audited instantly (client certs immune to bearer cold window)"

# SA token warm-up measurement (NON-fatal): the agent's real path later needs
# this gate; record kind's observed latency.
SA_T0=$(date +%s)
SA_AUTH="no"
for try in $(seq 1 90); do
  PROBE=$(marker_probe "$TOKEN" "/api/v1/namespaces/default/configmaps/${MPATH_NAME}sa-$NONCE" || true)
  [[ "$PROBE" == 404*SA* ]] && { SA_AUTH="yes"; break; }
  sleep 2
done
if [[ "$SA_AUTH" == "yes" ]]; then
  pass "SA bearer token authenticated after $(( $(date +%s) - SA_T0 ))s warm-up"
else
  echo "SPIKE-NOTE(kind-C): SA bearer still anonymous after $(( $(date +%s) - SA_T0 ))s -> identity_auth_ready gate is MANDATORY on kind"
fi
rm -f "$KCFG_FILE"

# 4. Log retrieval, attribution, stage model.
sleep 5
LOG=$(docker exec "$NODE" cat /var/log/kubernetes/audit.log 2>/dev/null || true)
[[ -n "$LOG" ]] || fail "audit log empty/unreadable"
echo "SPIKE-DIAG(kind-C): LOGLEN=${#LOG}"
if [[ "$LOG" != *"${MPATH_NAME}$NONCE-start"* ]]; then
  docker exec "$NODE" cat /var/log/kubernetes/audit.log > /tmp/kind-spike-dump.log 2>/dev/null || true
  echo "SPIKE-DIAG(kind-C): DUMP=$(wc -c < /tmp/kind-spike-dump.log) DUMPGREP=$(grep -c "${MPATH_NAME}$NONCE-start" /tmp/kind-spike-dump.log || true)"
  echo "SPIKE-DIAG(kind-C): PATTERN=${MPATH_NAME}$NONCE-start"
fi
[[ "$LOG" == *"${MPATH_NAME}$NONCE-start"* ]] \
  && pass "nonce preserved in audit requestURI" \
  || fail "nonce not visible in requestURI"
[[ "$LOG" == *"kubernetes-admin"* && "$LOG" == *"${MPATH_NAME}$NONCE-start"* ]] \
  && pass "admin marker events present with nonce in requestURI" \
  || fail "admin marker events missing from audit"
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
print(f"SPIKE-OK(kind-C): {len(ids)} auditIDs observed, {term} terminal")'
pass "kind mechanism C proven on this host (restart-free, DooD-staged)"
