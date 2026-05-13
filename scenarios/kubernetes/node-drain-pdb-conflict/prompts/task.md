# Node Drain Blocked by PodDisruptionBudget

## Scenario

A worker node has been cordoned for kernel maintenance. A configmap in the bench namespace marks the node as needing to be drained. However, the web deployment has a PodDisruptionBudget (PDB) that requires at least 2 pods available at all times.

Your task is to:

1. Understand why `kubectl drain` fails (the PDB minAvailable constraint)
2. Either:
   - Scale up the web deployment to 3+ replicas, OR
   - Adjust the PDB to allow the drain to proceed
3. Successfully drain the worker node
4. Verify that:
   - The node is drained (has SchedulingDisabled taint)
   - All pods are still running
   - The web deployment has sufficient replicas available

## Goal

Drain the cordoned node while respecting pod availability constraints.
