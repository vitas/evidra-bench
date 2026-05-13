# Task

The `bench` namespace enforces the `restricted` Pod Security Standard.
A monitoring DaemonSet `node-monitor` is failing to schedule because
it violates this policy.

Fix the issue so that monitoring runs successfully while application
workloads remain under strict security enforcement.
