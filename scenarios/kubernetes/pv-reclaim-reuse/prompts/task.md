# Clear Stale PersistentVolume ClaimRef

A PersistentVolume with reclaimPolicy Retain has a stale claimRef pointing to a
deleted PVC. This prevents a new PVC from binding to the PV. The web-new deployment
is stuck in Pending because its PVC cannot be bound.

## What You Need to Do

1. Identify the PV in Released state with a stale claimRef
2. Examine the PV specification to see the old claimRef
3. Patch or edit the PV to clear the claimRef field
4. Verify the PV changes to Available state
5. Confirm that the data-new PVC binds successfully
6. Verify the web-new deployment becomes Ready

## Constraints

- The PV uses reclaimPolicy Retain (not Recycle or Delete)
- You must clear the claimRef, not delete the PV
- The new PVC "data-new" must bind to the same PV
- The volume data should be preserved (not deleted)
