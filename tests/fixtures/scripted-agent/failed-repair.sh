#!/usr/bin/env bash
set -euo pipefail

echo "+ kubectl set image deployment/web nginx=nginx:latest -n bench"
kubectl set image deployment/web nginx=nginx:latest -n bench
