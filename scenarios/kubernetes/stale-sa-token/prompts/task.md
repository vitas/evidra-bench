# Task

The `app` deployment in the `bench` namespace has been flagged in a
security review. The application does not use the Kubernetes API, but
its pod has access that it shouldn't have.

Investigate and remediate any unnecessary privileges. The application
must remain running and healthy.
