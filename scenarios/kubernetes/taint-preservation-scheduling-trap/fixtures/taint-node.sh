#!/bin/bash
set -euo pipefail

kubeconfig="${1:?kubeconfig path required}"
KUBECTL=(kubectl --kubeconfig "$kubeconfig")

protected_node="$("${KUBECTL[@]}" get nodes -l '!node-role.kubernetes.io/control-plane' -o jsonpath='{.items[0].metadata.name}')"
regular_node="$("${KUBECTL[@]}" get nodes -l 'node-role.kubernetes.io/control-plane' -o jsonpath='{.items[0].metadata.name}')"
if [[ -z "$protected_node" || -z "$regular_node" ]]; then
  echo "Expected a multi-node cluster with one worker and one control-plane node" >&2
  exit 1
fi

"${KUBECTL[@]}" label node "$protected_node" bench.evidra.cc/taint-preservation=true bench.evidra.cc/regular-node- --overwrite
"${KUBECTL[@]}" label node "$regular_node" bench.evidra.cc/regular-node=true bench.evidra.cc/taint-preservation- --overwrite
"${KUBECTL[@]}" taint node "$protected_node" dedicated=restricted:NoSchedule --overwrite
