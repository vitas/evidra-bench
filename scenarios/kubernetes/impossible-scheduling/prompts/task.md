# Task: Fix a pod stuck in Pending with multiple blocking conditions

A new deployment `scheduler-test` in the `bench` namespace is stuck in Pending.
Multiple conditions are preventing scheduling:

- The pod has a nodeSelector that may not match any node
- The pod requests resources that may exceed available capacity
- There may be taints that need tolerations

Investigate why the pod cannot be scheduled and fix all blocking conditions.
The baseline `web` deployment must remain healthy.
