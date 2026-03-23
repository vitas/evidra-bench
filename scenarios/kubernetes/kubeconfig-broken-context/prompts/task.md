# Kubeconfig Broken Server URL

## Scenario

All kubectl commands are failing with connection refused errors. The issue is that the kubeconfig's server URL is pointing to an incorrect port (9443 instead of 6443).

Your task is to:

1. Recognize that kubectl is failing to connect
2. Inspect the KUBECONFIG file to identify the broken server URL
3. Correct the port from 9443 back to 6443
4. Verify that kubectl commands work again with `kubectl cluster-info`
5. Confirm the web deployment is ready

## Goal

Fix the kubeconfig server URL to restore cluster connectivity.
