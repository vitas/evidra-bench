#!/usr/bin/env bash
# code-revision.sh — deterministic identity of the EXECUTABLE artifact.
#
# The qualification ledger must pin what actually compiles into the
# binaries — not the commit sha of the repo tip. Commit shas made the
# ledger structurally unpassable: the commit carrying a ledger is by
# construction later than the qualified commit it records, so the tip
# artifact could never satisfy its own ledger (reviewer round-2
# blocker #2). This digest is the fixed point: it covers exactly the Go
# build inputs (all tracked *.go except _test.go, go.mod, go.sum), so
# ledgers, attestation JSON, docs and CI plumbing can move freely while
# the compiled artifact stays identical — and ANY change to shipped code
# moves it.
#
# Emits "code-<sha256:40>". Deterministic across machines: git blob
# hashes are content-addressed and the ordering is enforced here.
set -euo pipefail
repo=$(cd "$(dirname "$0")/.." && git rev-parse --show-toplevel)
cd "$repo"
git ls-tree -r "${1:-HEAD}" \
  | awk '{ print $3 "\t" $4 }' \
  | { grep -E '\.go$|go\.mod$|go\.sum$' || true; } \
  | grep -v '_test\.go$' \
  | LC_ALL=C sort \
  | sha256sum | awk '{ print "code-" substr($1,1,40) }'
