# Fix Control Plane Static Pod Crash

The Kubernetes cluster is not responding to kubectl commands. The API server has become unavailable
due to a misconfiguration in the kube-apiserver static pod manifest on the control-plane node.

## What You Need to Do

1. Investigate why the API server is not running
2. Locate the kube-apiserver static pod manifest on the control-plane node
3. Find and fix the typo in the --etcd-servers flag
4. Verify that the API server restarts and becomes healthy
5. Confirm that kubectl is working and the web deployment is ready

## Constraints

- You cannot use kubectl initially (API server is down)
- You must directly access the control-plane node using container exec tools
- The fix must be in the manifest file, not temporary patches
