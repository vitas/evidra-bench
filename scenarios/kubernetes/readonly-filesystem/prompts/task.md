# Task

The `app` deployment in the `bench` namespace was recently hardened
with a read-only root filesystem, but the application is now crashing.

Fix the deployment so it runs successfully. The read-only root
filesystem security hardening must remain in place.
