# Task: Author and apply a Kubernetes audit policy

The cluster's API server currently has no audit policy configured, which means
security-relevant API requests (reading secrets, exec into pods, privilege
changes) are not being logged anywhere.

Your tasks:

1. Confirm the API server is not currently using an audit policy
2. Author an audit policy YAML that captures at least:
   - `RequestResponse` level for secrets access in all namespaces
   - `RequestResponse` level for pod exec / attach / portforward
   - `Metadata` level for all other requests
3. Place the audit policy on the control-plane node where the API server can
   read it (typically `/etc/kubernetes/audit/policy.yaml`)
4. Configure the API server to use the policy (via `--audit-policy-file` and
   `--audit-log-path` flags in the static pod manifest)
5. Wait for the API server to restart and verify the cluster is healthy
6. Confirm audit logs are being written

The baseline `web` deployment must remain healthy throughout.

**Hint:** In a Kind cluster the control-plane node is a Docker container.
You can use `docker exec` on the control-plane node to place files and
inspect the API server manifest at `/etc/kubernetes/manifests/kube-apiserver.yaml`.
