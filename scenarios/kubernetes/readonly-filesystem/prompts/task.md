# Task: Fix a container with read-only root filesystem

The `app` deployment in the `bench` namespace has been secured with
`readOnlyRootFilesystem: true`, but the application is now crashing.

The application needs to write temporary files to `/tmp` and logs to
`/var/log/app`. Fix the deployment so it runs with read-only root filesystem
while still allowing the application to write to the paths it needs.

You have access to `kubectl` with the provided kubeconfig.
Do not disable the read-only root filesystem — it is a security requirement.
