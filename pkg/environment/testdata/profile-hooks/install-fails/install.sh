#!/bin/sh
set -eu

# Write partial state before failing.
echo "partial" > "${EVIDRA_WORK_DIR}/partial.marker"

echo "about to fail" >&2
exit 1
