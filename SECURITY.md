# Security Policy

## Supported Versions

Security fixes target `main` and the latest tagged release. Older tags are not
maintained unless a separate support agreement says otherwise.

## Reporting A Vulnerability

Use GitHub private vulnerability reporting if it is enabled for this
repository. If it is not enabled, open a minimal public issue asking for a
private maintainer contact and do not include exploit details, credentials, or
private run artifacts.

Useful reports include:

- affected version or commit
- reproduction steps
- expected impact
- whether private run data, API keys, or hosted control-plane behavior is
  involved

## Scope

Security-sensitive areas include:

- hosted API authentication and authorization
- browser-exposed secrets
- private report and artifact access
- runner command execution
- scenario fixtures that may leak credentials
- Docker and CI build contexts

Please do not file public issues containing live credentials, private customer
data, or exploit payloads.
