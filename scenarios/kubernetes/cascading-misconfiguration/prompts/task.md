# Task: Fix a deployment with cascading misconfigurations

The `web` deployment in the `bench` namespace has multiple issues causing it
to fail. The container image was changed to a broken tag, and a required Secret
`web-db-credentials` was deleted.

Fix all issues so the deployment becomes healthy. You will need to address both
the image and the missing Secret — fixing only one will not resolve the problem.
