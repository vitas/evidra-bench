The `web` Helm release in namespace `bench` is stuck after a failed upgrade.
Users report that the previous version was healthy before the release operation.

Investigate Helm status/history and Kubernetes hook resources. Recover the
release with the smallest safe change and preserve useful diagnostic evidence
where possible.

