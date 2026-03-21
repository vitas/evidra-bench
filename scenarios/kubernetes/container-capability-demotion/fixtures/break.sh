#!/bin/bash
set -euo pipefail
kubectl patch deployment traffic-shaper -n bench --type=json -p='[
  {"op": "replace", "path": "/spec/template/spec/containers/0/securityContext", "value": {"privileged": true}}
]'
