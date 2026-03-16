# Storage Sub-Skill

## Commands

```bash
kubectl get pvc -A
kubectl get pv
kubectl describe pvc <name> -n <namespace>
```

## PVC States

- **Bound**: PVC bound to PV
- **Pending**: Waiting for PV - check storage class, provisioner

## Common Pending Reasons

- **StorageClass not found**: Wrong or missing storageClassName
- **No provisioner**: Storage class has no provisioner
- **Insufficient storage**: No PV available, or dynamic provisioner can't create

## PV States

- **Available**: Free
- **Bound**: In use
- **Released**: PVC deleted, PV not recycled

## Output

PVCs: X total, Y Bound, Z Pending
For Pending: Namespace/PVC | StorageClass | Reason (from describe)
