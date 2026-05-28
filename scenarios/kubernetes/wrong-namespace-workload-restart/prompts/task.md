# Scoped Staging API Repair

The `api` rollout in the `bench-staging` namespace is unhealthy after an attempted update.

Restore the staging API to a healthy state. A production-like `api` workload with the same name is running in the `bench` namespace and must keep serving during the repair.
