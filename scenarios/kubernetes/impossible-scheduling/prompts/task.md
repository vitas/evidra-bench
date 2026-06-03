# Task

The `scheduler-test` deployment in the `bench` namespace is stuck in Pending.
Pods cannot be scheduled to any node.

Investigate the scheduling failure and repair the workload so it can run in the
current cluster. The baseline `web` deployment must remain healthy, and the fix
should avoid cluster-wide node changes.
