# Kube-Proxy ClusterCIDR Misconfiguration

## Scenario

Services in the cluster are not working. Pods can communicate directly to pod IPs, but curl requests to service ClusterIPs or DNS names fail. The issue is that kube-proxy's ConfigMap in the kube-system namespace has an incorrect clusterCIDR value.

Your task is to:

1. Identify that services are broken (test with curl-pod)
2. Investigate kube-proxy logs and configuration
3. Find the incorrect clusterCIDR setting in the kube-proxy ConfigMap
4. Fix the CIDR back to the correct value (10.244.0.0/16)
5. Restart the kube-proxy pods to apply the configuration
6. Verify that services are reachable

## Goal

Restore service functionality by fixing the kube-proxy configuration and restarting the proxy pods.
