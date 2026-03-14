# Task: Fix a ConfigMap with wrong database configuration

The `web` deployment in the `bench` namespace is failing because the ConfigMap
`web-config` contains an incorrect database host.

Investigate the ConfigMap, identify the wrong value, and patch it so the
deployment becomes healthy again. You may need to restart the deployment after
fixing the ConfigMap.
