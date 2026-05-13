# Task: Enable StorageClass volume expansion and resize PVC

The application in the `bench` namespace is using a PersistentVolumeClaim (PVC) named `app-data` with a size of 1Gi.
The logs are filling up the volume and need more space.

The problem: The StorageClass `bench-storage` currently has `allowVolumeExpansion: false`.

Your tasks:
1. Enable volume expansion on the StorageClass `bench-storage`
2. Resize the PVC `app-data` from 1Gi to 5Gi
3. Ensure the web deployment is ready and picks up the new volume size

Use `kubectl` to inspect and modify the StorageClass and PVC.
Do not create new storage resources — fix the existing ones.
