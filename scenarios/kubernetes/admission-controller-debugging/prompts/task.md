# Admission Controller Webhook Service Down

## Scenario

Users report that pod creation is failing with timeout errors. Your task is to:

1. Identify the ValidatingAdmissionWebhook configuration that is causing the issue
2. Investigate why the webhook is timing out (the backend service is down)
3. Decide whether to:
   - Restore the webhook service by scaling it back up, OR
   - Change the webhook's failurePolicy to allow, OR
   - Both
4. Verify that pods can be created successfully again

## Goal

Restore pod creation capability while maintaining security (the webhook exists for a reason — understand it before disabling it).
