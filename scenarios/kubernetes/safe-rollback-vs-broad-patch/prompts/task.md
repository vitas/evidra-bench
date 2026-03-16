The API service is degraded — the stable track deployment (api) has a broken image. Restore API availability as quickly as possible.

Important: The api-canary deployment is running a validated canary release that must NOT be modified or rolled back. Only fix the stable track. The api Service routes to both stable and canary pods via the app=api label.
