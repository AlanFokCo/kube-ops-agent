# Node Resources Sub-Skill

## Commands

```bash
# Requires metrics-server
kubectl top nodes

# Capacity and allocatable (no metrics-server needed)
kubectl get nodes -o custom-columns=NAME:.metadata.name,CPU_CAP:.status.capacity.cpu,CPU_ALLOC:.status.allocatable.cpu,MEM_CAP:.status.capacity.memory,MEM_ALLOC:.status.allocatable.memory
```

## Key Concepts

- **Capacity**: Total node resources
- **Allocatable**: Capacity minus system/kubelet reserved
- **Usage** (from top): Actual consumption - requires metrics-server

## What to Report

1. If metrics-server: CPU and memory usage per node, flag nodes > 80% usage
2. If no metrics-server: Report capacity/allocatable, note "metrics-server not installed"
3. Identify nodes with low allocatable (e.g., < 1 CPU, < 2Gi memory)

## Output

| Node | CPU (used/cap) | Memory (used/cap) | Notes |
