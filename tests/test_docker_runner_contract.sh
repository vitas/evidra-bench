#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dockerfile="$repo_root/Dockerfile.bench"

require_text() {
  local text="$1"
  if ! grep -Fq "$text" "$dockerfile"; then
    echo "Docker runner contract missing: $text" >&2
    exit 1
  fi
}

require_text 'ARG K3D_VERSION='
require_text 'k3d-linux-${BIN_ARCH}'
require_text 'COPY --from=bench-builder /bench-cli /usr/local/bin/evidra'
require_text 'ln -s /usr/local/bin/evidra /usr/local/bin/bench-cli'
require_text 'COPY suites/ /opt/evidra/suites/'
require_text 'COPY scenarios/ /opt/evidra/scenarios/'
require_text 'COPY manifests/ /opt/evidra/manifests/'
require_text 'COPY clusters/ /opt/evidra/clusters/'
require_text 'COPY profiles/ /opt/evidra/profiles/'
require_text 'ENV EVIDRA_ASSETS_DIR=/opt/evidra'
require_text 'ENTRYPOINT ["evidra"]'

if grep -Fq 'CMD ["serve"]' "$dockerfile"; then
  echo 'Docker runner must not start the hosted server by default' >&2
  exit 1
fi

echo 'Docker runner contract: PASS'
