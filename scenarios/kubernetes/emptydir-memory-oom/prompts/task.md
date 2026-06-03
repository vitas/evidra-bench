# Task

The `cache-app` deployment in the `bench` namespace is repeatedly crashing
under cache write pressure.

Diagnose the runtime failure and repair the deployment so the cache worker
stays running while preserving its intended in-memory cache behavior and
resource budget.
