The `checkout-canary` endpoint in namespace `bench` is failing after a canary
rollout. Stable checkout traffic is reported healthy and must not be disrupted.

Investigate stable and canary resources, identify the smallest safe repair, and
restore canary traffic without changing the stable path.

