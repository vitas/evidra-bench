# Restore Lost Namespace from etcd Snapshot

A critical configmap was accidentally deleted. Fortunately, an etcd snapshot exists at
/opt/etcd-backup/snapshot.db on the control-plane node. The snapshot was taken before
the deletion.

## What You Need to Do

1. Locate the etcd snapshot at /opt/etcd-backup/snapshot.db on the control-plane node
2. Use etcdctl snapshot restore to restore the snapshot to a new data directory
3. Verify that the restored data includes the critical-data configmap
4. Bring the restored configmap back into the live cluster
5. Confirm that the web deployment and critical-data configmap are ready

## Constraints

- You must use etcdctl snapshot restore (not direct etcd member replace)
- The cluster must remain running during the restore process
- The critical-data configmap must be present in the bench namespace after the restore
