# Namespaces Sub-Skill

## Commands

```bash
kubectl get namespaces
kubectl get namespaces -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,AGE:.metadata.creationTimestamp
```

## What to Check

1. **Total count** - Number of namespaces
2. **System namespaces** - kube-system, kube-public, kube-node-lease should be Active
3. **Terminating** - Namespaces stuck in Terminating may indicate finalizer issues
4. **Age** - Very old namespaces vs recently created

## Common Issues

- **Terminating namespace**: Often due to stuck finalizers; check `kubectl get namespace <name> -o yaml` for finalizers
- **Missing kube-system**: Critical - cluster DNS/scheduler may be broken

## Output

Report: Total count, list of system namespaces with status, any Terminating namespaces, notable custom namespaces.
