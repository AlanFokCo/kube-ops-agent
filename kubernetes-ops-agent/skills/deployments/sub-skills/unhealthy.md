# Unhealthy Deployments Sub-Skill

## Commands

```bash
kubectl get deployments -A
kubectl describe deployment <name> -n <namespace>
kubectl get rs -n <namespace> -l <deployment-selector>
```

## Common Causes (AVAILABLE < DESIRED)

1. **Insufficient resources** - Pending pods, check node capacity
2. **ImagePullBackOff** - Image not found or pull secret missing
3. **CrashLoopBackOff** - Container crashing
4. **PodDisruptionBudget** - PDB blocking eviction during rollout
5. **Readiness probe failing** - Pods not ready, not counted in AVAILABLE

## Events to Check

From `kubectl describe deployment`:
- **ScalingReplicaSet** - normal scaling
- **FailedCreate** - pod creation failed (resource quota, limit range)
- **FailedScheduling** - no nodes fit

## Output

For each unhealthy deployment: Namespace/Name | DESIRED/AVAILABLE | Reason (from events or ReplicaSet) | Suggested fix
