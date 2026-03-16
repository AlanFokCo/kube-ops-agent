# Node Taints Sub-Skill

## Taint Effects

| Effect | Meaning |
|--------|---------|
| NoSchedule | New pods cannot be scheduled unless they tolerate |
| PreferNoSchedule | Soft - scheduler avoids but not hard block |
| NoExecute | Evict existing pods that don't tolerate |

## Commands

```bash
kubectl get nodes -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints[*].key
kubectl describe nodes | grep -A 5 "Taints:"
```

## Common Taints

- **node.kubernetes.io/not-ready**: Node not ready
- **node.kubernetes.io/unreachable**: Node unreachable
- **node.kubernetes.io/disk-pressure**: Disk pressure
- **node.kubernetes.io/memory-pressure**: Memory pressure
- **node.kubernetes.io/unschedulable**: Manually cordoned
- **dedicated=GPU**: Custom - GPU nodes

## Output

List nodes with taints: Node | Taint Key | Effect | Impact (e.g., "No pods can schedule")
