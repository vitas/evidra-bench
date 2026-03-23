# Task: Write Falco rules to detect sensitive file access

Falco is running in the cluster (namespace: `falco`) for runtime security monitoring.

A suspicious pod `suspect-app` has been deployed to the `bench` namespace. It is
periodically reading `/etc/shadow` and writing to `/etc/passwd` inside the container.

Your tasks:

1. Investigate the suspicious pod and confirm the malicious behavior
2. Write a custom Falco rule that detects:
   - Any container reading `/etc/shadow`
   - Any container writing to `/etc/passwd`
3. Deploy the rule to Falco (via ConfigMap, Helm values, or Falco's rules directory)
4. Verify that Falco generates alerts when the suspicious behavior occurs

The rule should have priority `WARNING` or higher and output the container name,
image, and file path in the alert message.

The baseline `web` deployment must remain healthy throughout.
