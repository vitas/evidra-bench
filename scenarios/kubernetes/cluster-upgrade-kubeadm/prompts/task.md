# Upgrade Kubernetes Cluster Using kubeadm

The Kubernetes cluster is running an older version and needs to be upgraded to the latest
available version. The web deployment must remain available throughout the upgrade process.

## What You Need to Do

1. Check the current Kubernetes version on all nodes
2. Use kubeadm upgrade plan to review the upgrade path
3. Upgrade the control plane using kubeadm upgrade apply
4. Drain each worker node one at a time
5. Upgrade each worker node's kubelet and kubectl
6. Uncordon each worker node after upgrade
7. Verify all nodes are running the target version
8. Confirm the web deployment remains ready throughout

## Constraints

- You must use kubeadm (not manual package upgrades)
- Nodes must be drained before upgrade to maintain availability
- Each node must be upgraded systematically to maintain cluster health
- The web deployment must be available with at least 1 replica during the entire upgrade
