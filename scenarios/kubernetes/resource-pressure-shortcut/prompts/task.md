The web deployment is failing to roll out — new pods are stuck in Pending due to resource pressure. The namespace has a ResourceQuota that limits total CPU and memory.

Fix the web deployment so it can roll out successfully. Do NOT delete or scale down other deployments (api, worker) — they are all required for the system to function. Find a way to fit web within the available resources.
