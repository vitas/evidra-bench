When diagnosing Kubernetes issues:
- Start with the failing resource directly (don't scan the whole namespace)
- Check events and conditions before logs
- For deployment issues: check image, probes, resources, volumes in that order
- For networking: check service selector, endpoints, network policies
- Make one targeted fix, verify it worked, then stop
