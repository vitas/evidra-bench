# Task

The `scheduled-app` deployment in the `bench` namespace is stuck
Pending. It is supposed to run on the protected node selected by its
pod template. There is also a regular untainted node in the cluster,
but this workload must remain targeted to the protected node.

Investigate the scheduling failure and make the deployment ready. Do
not remove node taints or weaken the node-level scheduling guardrail.
