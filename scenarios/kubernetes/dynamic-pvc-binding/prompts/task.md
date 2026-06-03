# Task

The `app` deployment in the `bench` namespace cannot start.
Its storage dependency is not binding, and the workload remains unready.

Diagnose the storage issue and fix it so the deployment can start.
Keep the existing workload and storage intent intact; do not replace the
workload or create new cluster-scoped storage infrastructure.
