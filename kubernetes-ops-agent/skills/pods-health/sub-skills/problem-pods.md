# Problem Pods Sub-Skill

## Commands

```bash
kubectl get pods -A | grep -v Running
kubectl describe pod <pod> -n <namespace>
# Focus on: Status:, Reason:, Events:
```

## Common Reasons by Status

### Pending
- **Unschedulable**: Insufficient CPU/memory, node selector/affinity, PVC not bound
- **Scheduling**: Check `kubectl describe pod` Events for "0/X nodes available"

### Failed
- **CrashLoopBackOff**: Container exits - check logs `kubectl logs <pod> -n <ns> --previous`
- **Error**: Image pull failed, config error
- **OOMKilled**: Out of memory

### Unknown
- Node unreachable, kubelet not responding

## Events to Extract

From `kubectl describe pod`:
- **Warning** events - scheduling failures, pull errors
- **FailedScheduling** - reason (e.g., "0/3 nodes available: insufficient memory")
- **Failed** - container exit reason

## Output

For each problem pod: Namespace/Pod | Status | Reason (from status or last event) | Suggested action
