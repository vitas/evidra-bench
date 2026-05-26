---
title: Browser Session Auth Design
type: plan
status: active
tags:
  - bench
  - auth
  - ui
---

# Browser Session Auth Design

## Goal

Make authenticated browser writes usable in private deployments without
embedding static API keys in the frontend or storing them in browser
JavaScript state.

## Approach

Add a backend session surface:

- `POST /v1/bench/session` accepts an API key once and returns an HttpOnly
  signed session cookie.
- `GET /v1/bench/session` reports whether the current request is authenticated.
- `DELETE /v1/bench/session` clears the session cookie.

The cookie is stateless. It contains tenant and expiry claims signed with
HMAC-SHA256 using the deployment API key as the server-side signing secret.
Changing the deployment API key invalidates existing browser sessions. Bearer
API key auth remains unchanged for CLI, runners, and automation.

## Browser Behavior

The browser API client sends `credentials: "include"` for all bench API
requests, while continuing to strip caller-provided `Authorization` headers.
The UI adds a compact session page where a private deployment user can sign in
with the deployment API key. The key is sent once to the backend and is not
persisted in local storage or React state after the request completes.

## Security Boundary

Cookies are `HttpOnly`, `SameSite=Lax`, and `Path=/`. The backend sets
`Secure` when the request is HTTPS or arrives behind `X-Forwarded-Proto:
https`. This is enough for the private-deployment slice; OAuth/SSO can replace
the API-key login later without changing the review editor or API client.

## Tests

- backend auth tests cover signed cookie issue, verification, expiry, and
  middleware access
- session route tests cover login, status, logout, and invalid API key
- UI API tests cover `credentials: "include"` without reintroducing
  `Authorization`
- UI session helper tests cover status/login/logout request paths

