# GPU Pods Sub-Skill

## Commands

```bash
kubectl get nodes -o custom-columns=NAME:.metadata.name,GPU:.status.capacity.nvidia\.com/gpu
kubectl get pods -A -o jsonpath='{range .items[?(@.spec.containers[*].resources.limits.nvidia\.com/gpu)]}{.metadata.namespace}{"/"}{.metadata.name}{" "}{.status.phase}{" "}{.spec.containers[0].resources.limits.nvidia\.com/gpu}{"\n"}{end}'
```

## Pod GPU Request

Pods request GPU via `resources.limits.nvidia.com/gpu`.

## What to Report

1. Nodes with nvidia.com/gpu capacity
2. Pods with nvidia.com/gpu in limits - namespace, name, phase, GPU count
3. Pending GPU pods - may indicate insufficient GPU
4. Total GPU requested vs allocatable

## Output

| Namespace/Pod | Phase | GPU Requested | Node (if Running) |
