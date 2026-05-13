#!/usr/bin/env bash
set -euo pipefail

range="${1:-}"

if [[ -z "$range" ]]; then
  if [[ -n "${GITHUB_BASE_REF:-}" ]]; then
    git fetch --no-tags --depth=50 origin \
      "${GITHUB_BASE_REF}:refs/remotes/origin/${GITHUB_BASE_REF}"
    range="origin/${GITHUB_BASE_REF}..HEAD"
  elif [[ -n "${GITHUB_EVENT_BEFORE:-}" && "${GITHUB_EVENT_BEFORE}" != "0000000000000000000000000000000000000000" ]]; then
    range="${GITHUB_EVENT_BEFORE}..HEAD"
  else
    range="HEAD"
  fi
fi

missing=0

commit_list() {
  if [[ "$range" == *".."* ]]; then
    git rev-list --no-merges "$range"
  else
    git rev-parse "$range"
  fi
}

while IFS= read -r commit; do
  if ! git log -1 --format=%B "$commit" | grep -Eiq '^Signed-off-by: .+ <.+@.+>$'; then
    echo "FAIL: commit $commit is missing a Signed-off-by trailer" >&2
    missing=1
  fi
done < <(commit_list)

if [[ "$missing" -ne 0 ]]; then
  echo "Add a DCO sign-off with: git commit -s" >&2
  exit 1
fi

echo "DCO sign-off check passed for $range"
