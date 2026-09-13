#!/usr/bin/env bash
# TEMPORARY plan-Task-11 portability spike (never committed):
# prove default-CNI NetworkPolicy enforcement on kind AND k3d before
# network-policy-fix is admitted. 3 runs per provider.
set -uo pipefail
KUBECFG="${1:?usage: network-policy-core.sh <kubeconfig>}"
K() { kubectl --kubeconfig "$KUBECFG" "$@"; }

# A previous spike run may still be terminating its namespace: wait it
# out so pods are never created inside a Terminating namespace.
for _ in $(seq 1 60); do
  state=$(K get ns npspike -o jsonpath='{.status.phase}' 2>/dev/null || true)
  [[ "$state" == Terminating ]] || break
  sleep 2
done
K get ns npspike >/dev/null 2>&1 || K create ns npspike >/dev/null
K wait --for=jsonpath='{.status.phase}'=Active ns/npspike --timeout=60s >/dev/null || exit 1
for pair in fe:frontend be:backend db:database; do
  name="${pair%%:*}"; label="${pair##*:}"
  K run "$name" --image=busybox:1.36 -n npspike --labels="app=$label" \
    --command -- sh -c "mkdir -p /www && echo ${label}-ok > /www/index.html && httpd -f -p 80 -h /www" >/dev/null 2>&1
done
K -n npspike wait --for=condition=Ready pod/fe pod/be pod/db --timeout=120s >/dev/null || exit 1
FE=$(K get pod -n npspike fe -o jsonpath='{.status.podIP}')
BE=$(K get pod -n npspike be -o jsonpath='{.status.podIP}')
DB=$(K get pod -n npspike db -o jsonpath='{.status.podIP}')

K apply -n npspike -f - <<'POL'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: default-deny, namespace: npspike}
spec:
  podSelector: {}
  policyTypes: [Ingress]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: allow-fe-be, namespace: npspike}
spec:
  podSelector: {matchLabels: {app: backend}}
  policyTypes: [Ingress]
  ingress:
    - {from: [{podSelector: {matchLabels: {app: frontend}}}], ports: [{port: 80}]}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: allow-be-db, namespace: npspike}
spec:
  podSelector: {matchLabels: {app: database}}
  policyTypes: [Ingress]
  ingress:
    - {from: [{podSelector: {matchLabels: {app: backend}}}], ports: [{port: 80}]}
POL

probe() { # probe <src-pod> <dst-ip> -> ok|blocked
  local out
  out=$(K exec "$1" -n npspike -- sh -c "wget -qO- -T 3 http://$2/ 2>&1" 2>&1 | head -1)
  case "$out" in
    *-ok*) echo ok ;;
    *) echo blocked ;;
  esac
}
r1=$(probe fe "$BE"); r2=$(probe be "$DB"); r3=$(probe fe "$DB")
echo "narrow: fe->be=$r1 be->db=$r2 fe->db=$r3"
K apply -n npspike -f - <<'BROAD'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: broad, namespace: npspike}
spec:
  podSelector: {matchLabels: {app: database}}
  policyTypes: [Ingress]
  ingress:
    - {}
BROAD
sleep 2
r4=$(probe fe "$DB")
echo "broad: fe->db=$r4"
K delete ns npspike --wait=false >/dev/null 2>&1
if [[ "$r1" == ok && "$r2" == ok && "$r3" == blocked && "$r4" == ok ]]; then
  echo "SPIKE: ENFORCED"; exit 0
fi
echo "SPIKE: NOT-ENFORCED (fe->be=$r1 be->db=$r2 fe->db-narrow=$r3 fe->db-broad=$r4)"; exit 1
