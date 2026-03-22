# Task

A security review found that a service account in the `bench` namespace
has read access to secrets in the `bench-sensitive` namespace. This
access is unauthorized.

Investigate how the access is granted and remediate it. The application
in the `bench` namespace must continue to function normally.
