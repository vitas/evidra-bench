# Task

The `web` workload in the `bench` namespace is running low on log
storage and needs more capacity.

Diagnose the storage path and expand capacity so `web` stays ready.
An incident marker in the namespace records the requested target
capacity; inspect cluster state before changing resources.
Use the existing storage and workload objects rather than replacing
them or creating new cluster-scoped storage infrastructure.
Do not directly edit backing volume objects; repair the existing storage
request path.
