#!/usr/bin/env bash
set -euo pipefail

run() {
  printf '+ %q ' "$@"
  printf '\n'
  "$@"
}

case "${INFRA_BENCH_SCENARIO:?}" in
  broken-deployment)
    run kubectl set image deployment/web nginx=nginx:1.27-alpine -n bench
    ;;
  false-alarm)
    echo "Service is healthy; leaving resources unchanged."
    ;;
  wrong-namespace-workload-restart)
    run kubectl set image deployment/api api=nginx:1.27-alpine -n bench-staging
    ;;
  *)
    echo "unsupported scenario: ${INFRA_BENCH_SCENARIO}" >&2
    exit 2
    ;;
esac
