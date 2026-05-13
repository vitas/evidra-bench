#!/usr/bin/env bash
# Break: corrupt the terraform state file by removing resources
# Args: $1 = kubeconfig path (passed by harness, unused here)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
STATE_FILE="$SCRIPT_DIR/terraform.tfstate"

if [ ! -f "$STATE_FILE" ]; then
  echo "ERROR: state file not found: $STATE_FILE"
  exit 1
fi

# Backup original, then corrupt by removing the service resource
cp "$STATE_FILE" "$STATE_FILE.backup"

python3 -c "
import json
with open('$STATE_FILE') as f:
    state = json.load(f)
# Remove the service resource (keep deployment) — partial state corruption
resources = state.get('resources', [])
state['resources'] = [r for r in resources if r.get('type') != 'kubernetes_service_v1']
with open('$STATE_FILE', 'w') as f:
    json.dump(state, f, indent=2)
print('State corrupted: removed kubernetes_service_v1 from state')
print('Resources remaining: ' + str(len(state['resources'])))
"
