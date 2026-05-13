# Task

A security review identified that the `config-manager-sa` service
account in the `bench` namespace has permissions that could allow
privilege escalation beyond its intended scope.

Remediate the issue. The application that uses this service account
must continue to function.
