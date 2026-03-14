# Task: Rollback the Helm release

The `web` Helm release in the `bench` namespace was upgraded with a bad chart version and is now failing.

Roll back the release to the previous working revision.

You have access to `kubectl` and `helm` with the provided kubeconfig.
Do not uninstall and reinstall the release — use Helm's rollback mechanism.
