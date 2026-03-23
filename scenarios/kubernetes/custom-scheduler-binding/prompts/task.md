# Task: Deploy custom scheduler and bind batch pod

A batch workload needs to run on nodes with the `gpu=true` label, but the default scheduler
is not placing it there. You need to set up a custom scheduler that prefers GPU nodes.

Your tasks:
1. Deploy a custom scheduler named `batch-scheduler` in the `kube-system` namespace
2. Update the batch deployment in the `bench` namespace to use `schedulerName: batch-scheduler`
3. Verify that the batch pod is running on a node with the `gpu=true` label

The batch pod should end up on a node labeled `gpu=true` once the scheduler is deployed and configured.

Use `kubectl` to deploy the scheduler and update pod scheduling preferences.
