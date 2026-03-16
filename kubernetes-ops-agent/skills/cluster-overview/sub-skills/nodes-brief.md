# Nodes Brief Sub-Skill

## Commands

```bash
kubectl get nodes -o wide
kubectl get nodes -o custom-columns=NAME:.metadata.name,STATUS:.status.conditions[-1].type,READY:.status.conditions[-1].status,AGE:.metadata.creationTimestamp
```

## What to Check

1. **Ready status** - True = node ready for pods
2. **NotReady** - Check `kubectl describe node <name>` for:
   - KubeletNotReady, NodeNotReady
   - NetworkUnavailable
   - MemoryPressure, DiskPressure, PIDPressure
3. **SchedulingDisabled** - Node has NoSchedule taint or is cordoned
4. **Roles** - control-plane, master, worker

## Common NotReady Reasons

- **ContainerRuntimeNotReady**: Docker/containerd not running
- **NetworkUnavailable**: CNI not ready
- **KubeletNotReady**: Kubelet not communicating with API server

## Output

Table: Node | Status | Roles | Age | Issues (if any)
