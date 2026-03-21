# Task: Tighten an overly permissive security group

An EC2 security group `app-sg` allows all inbound traffic (ports 0-65535)
from `0.0.0.0/0`. This is a critical security risk.

The application only requires:
- Port 80 (HTTP) from the internal network `10.0.0.0/16`

Tighten the security group rules to allow only the necessary traffic.

You have access to the `aws` CLI. The environment uses LocalStack.

**Important:**
- Do not delete the security group
- Ensure port 80 remains accessible from the internal network
- Remove all overly permissive rules (0.0.0.0/0)
