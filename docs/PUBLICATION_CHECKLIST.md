---
title: Publication Checklist
type: governance
status: active
tags:
  - bench
  - oss
  - publication
---

# Publication Checklist

Do not make the existing private repository public without a clean-history
decision. Current history contains old generated run artifacts and internal
planning docs that are no longer tracked on `main` but still exist in git
history.

## Before Visibility Change

- Confirm current `main` has no uncommitted changes.
- Run `make lint`.
- Run `make test`.
- Run `bash tests/test_secret_hygiene.sh`.
- Run `bash tests/test_artifact_hygiene.sh`.
- Confirm `docs/plans/`, `CLAUDE.md`, raw `runs/`, databases, and secrets are
  not tracked.
- Publish from a clean export or orphan history, not the existing private
  repository history.

## Clean Export Path

Use the clean export script from the private working copy:

```bash
scripts/create-public-export.sh ../evidra-bench-public
```

Then review the export before pushing it to a public repository:

```bash
cd ../evidra-bench-public
git status --short
git log --oneline --decorate
git ls-files docs/plans CLAUDE.md runs/results.jsonl
make lint
make test
```

Expected result:

- one initial public baseline commit
- no tracked internal plans
- no tracked raw run artifacts
- no tracked private agent instructions

## GitHub Settings

After the clean public repository exists:

- enable required CI on the default branch
- disable force-pushes and branch deletion on the default branch
- require signed commits or DCO checks according to the project policy
- enable private vulnerability reporting when available
- enable secret scanning and push protection when available
- enable Dependabot alerts and security updates
- keep release publishing limited to tags

## After Publication

- Verify README badges resolve against the public repository.
- Verify `go install github.com/vitas/evidra-bench/cmd/bench-cli@latest` works
  after the first public tag.
- Verify Docker image labels point to the public source repository.
- Publish one small reproducible public report as the first credibility anchor.
