# Audit provisioning feasibility — spike findings

ADR 0001 rollout step 1 gate. Question: can the bench runner, operating
Docker-outside-of-Docker (docker.sock mounted into a container; cluster nodes
are siblings on the daemon), provision **API audit** on kind and k3d, and
retrieve the evidence inside a bounded window? Answer: **yes on Docker
Desktop** (this doc), Linux CI evidence appended below (throwaway draft PR,
closed after capture).

Scripts: `kind-attempt-c.sh`, `k3d-attempt-a.sh` (self-contained, self-cleaning).

## Proven recipe

### kind (mechanism C — restart-free)
1. Stage `policy.yaml` + `patches/kube-apiserver.yaml` (strategic-merge pod
   patch adding `audit-policy`/`audit-log` hostPath volumes) into a **named
   docker volume** — the only DooD-safe way to put files where the daemon
   can mount them from.
2. `kind create cluster` config: `extraMounts` the volume directory
   (`{{.Mountpoint}}`, a daemon path) to node `/etc/kubernetes/audit` and
   the patches dir to `/etc/kubernetes/kubeadm-patches`;
   `kubeadmConfigPatches`: `InitConfiguration.patches.directory` +
   `ClusterConfiguration.apiServer.extraArgs` audit flags.
   The pod patch file MUST be named `kube-apiserver.yaml` (kubeadm
   convention) or it is silently ignored.
3. Read logs with `docker exec <node> cat /var/log/kubernetes/audit.log`
   (offset-based tail). No apiserver restart, no host path writes.

### k3d
Named volume mounted via `--volume <vol>:/etc/kubernetes/audit@server:0` +
`--k3s-arg "--kube-apiserver-arg=<flag>@server:0"` (this k3d build has no
`--kube-apiserver-arg` wrapper flag). k3s embeds the API server: file paths
are container-FS-direct; same exec-cat retrieval.

## Hard findings (shape the production design)

1. **`/healthz` cannot be a marker.** It is served pre-authentication: it is
   audited as `system:anonymous` even when a valid bearer is sent, and no
   RBAC grant changes that. Markers must be authenticated RESOURCE
   requests.
2. **Bearer auth is cold on kind for minutes; client certs are not.**
   Fresh-cluster observation: bound AND classic Secret SA tokens were both
   rejected as `system:anonymous` for ≥194 s (still cold when the probe
   loop gave up), while client-certificate identities authenticate
   immediately. k3s accepted bound tokens in ~0–1 s. Consequences:
   - window markers ride the **harness certificate identity** (nonce in the
     ConfigMap *name* is the correlation key; 404 is the expected result);
   - every SA-token identity (agent, evidence-reader) requires an
     **`identity_auth_ready` gate** (an authenticated 403/404 probe,
     explicitly rejecting anonymous ones) before any windowed claim is
     trusted — otherwise the runner would silently misattribute agent
     traffic as anonymous.
3. **Stage model confirmed empirically**: real stages are
   `RequestReceived`/`ResponseStarted`/`ResponseComplete`/`Panic`; 859/868
   (kind) and 586/606 (k3d) auditIDs reached terminal stage during the
   capture; multi-stage events share one `auditID` → dedup key must be
   `(auditID, stage)`.
4. **`Audit-ID` response header is present on both providers** — usable as
   a marker correlation aid alongside the nonce name.
5. **Nested file bind mounts don't propagate**: staging `policy.yaml` via a
   kind `extraMounts` FILE path creates a node-visible file that the static
   pod's hostPath-directory bind does NOT see (0-byte placeholder). Mount
   whole directories (volume dir), never single files across the
   node→pod boundary.
6. **kubeadm v1beta3 has no `apiServer.extraVolumes`** (mechanism A dead
   end); patching the running apiserver manifest (mechanism B) additionally
   cold-breaks bearer auth for ~5+ minutes post-restart (jwks refetch) —
   restart-free provisioning is not just nicer, it is required for evidence
   validity.
7. Tooling traps found (relevant for the collector): `docker exec` without
   `-i` drops heredoc stdin; `pipefail` + `printf '%s' "$BIGVAR" | grep -q`
   false-fails on match (early-exit SIGPIPE = 141); command substitution
   must not mix informational stdout with token material.

## Retention/rotation

Policies used `--audit-log-maxsize=16 --audit-log-maxbackup=1
--audit-log-maxage=1d`. Rotation detection (inode/size regression) must
mark the source `incomplete: log_rotated`; v1 does not follow rotated files.

## Verdict

Mechanism **C (kind) + named-volume k3s args** are adopted as the production
provisioning path, with the identity findings (1, 2) folded into
`pkg/audit` design: cert markers, SA readiness gate, `(auditID, stage)`
model, requestURI nonce correlation, Audit-ID header recorded when present.

## Linux CI evidence (real ubuntu-latest, not Docker Desktop)

Throwaway draft PR #67 (`DO NOT MERGE — audit feasibility spike`, closed
after capture). Green run:
<https://github.com/vitas/evidra-bench/actions/runs/34421243843>
(ubuntu-24.04, kind leg + k3d leg). Outcome: **BOTH LEGS PASS** with the
pinned `kindest/node:v1.31.2`.

Runner-only findings (DD could not show these):

1. **Default kind on recent runners ships kindest/node:v1.37.0, where
   v1beta3 `kubeadmConfigPatches` are NOT rendered into the apiserver
   manifest** — `apiServer.extraArgs` was silently dropped ("manifest lacks
   audit args") and provisioning failed. Pinning the node image
   (`EVIDRA_SPIKE_NODE_IMAGE`, default `kindest/node:v1.31.2`) is REQUIRED
   for determinism; production provisioning must pin both kind version and
   node image, never inherit runner defaults.
2. **The kind bearer cold window is real Linux behavior, not a Docker
   Desktop artifact**: SA bearer tokens (bound) were still
   `system:anonymous` after 186 s on ubuntu-latest; k3s accepted them in
   ~4 s. The `identity_auth_ready` gate is therefore mandatory for every
   bearer identity on kind, with generous (>=10 min) timeout budget — and
   cert-identity window markers remain the correct design.
3. Stage model re-confirmed at scale on real Linux: 805/814 (kind) and
   742/763 (k3d) auditIDs reached terminal stage within the window.
