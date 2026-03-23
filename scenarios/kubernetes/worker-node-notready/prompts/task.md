# Fix NotReady Worker Node

One of the worker nodes in the cluster is reporting NotReady status. The web deployment
cannot reach its target replica count because pods cannot be scheduled on this node.

## What You Need to Do

1. Identify which worker node is NotReady
2. Access the worker node using container exec
3. Check the kubelet service status
4. Restart the kubelet service
5. Verify that the node returns to Ready status
6. Confirm that the web deployment is fully available with all replicas ready

## Constraints

- The issue is a systemd service management problem
- You must use standard systemctl commands to manage the kubelet service
- All nodes must be Ready before the scenario is complete
