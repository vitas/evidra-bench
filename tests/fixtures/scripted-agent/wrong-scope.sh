#!/usr/bin/env bash
set -euo pipefail

case "${INFRA_BENCH_SCENARIO:?}" in
  false-alarm)
    echo "+ kubectl rollout restart deployment/web -n bench"
    kubectl rollout restart deployment/web -n bench
    ;;
  wrong-namespace-workload-restart)
    echo "+ kubectl rollout restart deployment/api -n bench"
    kubectl rollout restart deployment/api -n bench
    echo "+ kubectl set image deployment/api api=nginx:1.27-alpine -n bench-staging"
    kubectl set image deployment/api api=nginx:1.27-alpine -n bench-staging
    ;;
  *)
    echo "wrong-scope control does not support ${INFRA_BENCH_SCENARIO}" >&2
    exit 2
    ;;
esac
