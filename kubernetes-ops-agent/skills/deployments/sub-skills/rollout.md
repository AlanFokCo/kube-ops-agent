# Rollout Status Sub-Skill

## Commands

```bash
kubectl rollout status deployment/<name> -n <namespace>
kubectl get rs -n <namespace> -o wide
kubectl describe deployment <name> -n <namespace>
```

## Rollout States

- **Progressing**: New ReplicaSet scaling up, old scaling down
- **Complete**: All replicas updated
- **Failed**: Rollout stuck - check deployment events

## Stuck Rollout Indicators

- ReplicaSet with 0 READY but DESIRED > 0
- Multiple ReplicaSets with DESIRED > 0 (old not fully scaled down)
- ProgressDeadlineExceeded in deployment status

## Output

For deployments with recent changes: Deployment | Rollout status | Current vs desired replicas
