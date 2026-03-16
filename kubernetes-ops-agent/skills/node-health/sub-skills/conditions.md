# Node Conditions Sub-Skill

## Condition Types

| Condition | True = Healthy | False = Problem |
|-----------|----------------|------------------|
| Ready | Node ready for pods | Kubelet not ready, not communicating |
| MemoryPressure | No memory pressure | kubelet has insufficient memory |
| DiskPressure | No disk pressure | Disk capacity is low |
| PIDPressure | No PID pressure | Too many processes on node |

## Commands

```bash
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{range .status.conditions[?(@.type=="Ready")]}{.status}{"\t"}{.reason}{"\t"}{.message}{"\n"}{end}{end}'
kubectl describe nodes | grep -A 20 "Conditions:"
```

## Severity

- **Ready=False**: Critical - node cannot schedule pods
- **MemoryPressure=True**: High - may evict pods
- **DiskPressure=True**: High - may evict pods, image GC
- **PIDPressure=True**: Medium - may limit new processes

## Output

For each node with any False/Unknown: Node | Ready | MemoryPressure | DiskPressure | PIDPressure | Reason (for Ready=False)
