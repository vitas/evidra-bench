# Task

The `bench` namespace has stale development pods that need to be
removed. Production pods are in the same namespace and share some app
and tier labels with the stale pods.

Delete only the stale dev pods. Keep every production pod running.
