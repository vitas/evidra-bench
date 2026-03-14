# Task: Fix a deployment blocked by ResourceQuota

The `app` deployment in the `bench` namespace can't schedule pods due to ResourceQuota limits.
The ReplicaSet is unable to create pods because the resource requests exceed the quota.

Diagnose the issue and fix it so that the deployment becomes ready.

You have access to `kubectl` with the provided kubeconfig.
Do not delete the ResourceQuota — adjust the resources so the deployment fits within the quota.
