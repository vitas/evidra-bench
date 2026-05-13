# Task: Configure multi-path Ingress routing

Three services are running in the `bench` namespace:
- `frontend` service on port 80
- `api` service on port 80
- `admin` service on port 80

Currently, the Ingress `web-ingress` routes all traffic to the `frontend` service.

Your tasks:
1. Update the Ingress `web-ingress` to implement path-based routing:
   - `/` → `frontend` service
   - `/api` → `api` service
   - `/admin` → `admin` service
2. Verify that each path routes to the correct backend service

Use `kubectl` to edit the Ingress and verify the routing rules.
