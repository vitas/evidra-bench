# Task: Diagnose and fix emptyDir tmpfs OOMKill

The `cache-app` deployment in the `bench` namespace is crashing with OOMKill errors.
The pod uses an emptyDir with `medium: Memory` (tmpfs) for caching, but the tmpfs
grows beyond the container memory limit (64Mi), causing the pod to be killed.

Your tasks:
1. Inspect the `cache-app` deployment and identify the memory issue
2. Fix the problem by either:
   - Adding a `sizeLimit` to the emptyDir to prevent unbounded growth, OR
   - Increasing the container memory limit, OR
   - Switching the emptyDir from Memory to Disk
3. Ensure the cache-app pod transitions to Running state

Use `kubectl` to diagnose and fix the deployment.
