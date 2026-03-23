# Task: Remediate vulnerable container images

The `web` deployment in the `bench` namespace is running a container image
with known critical vulnerabilities.

Your tasks:

1. Identify which image the deployment is using
2. Scan it for vulnerabilities (trivy is available in the cluster as trivy-operator, or you can use `trivy image` from the CLI)
3. Determine the critical CVEs present
4. Update the deployment to use a patched version of the same image (stay on the same major version line if possible)
5. Verify the deployment rolls out successfully with the new image

The deployment must remain functional after the update — do not change
anything other than the container image.
