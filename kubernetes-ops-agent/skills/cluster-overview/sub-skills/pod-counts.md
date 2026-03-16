# Pod Counts Sub-Skill

## Commands

```bash
kubectl get pods -A --no-headers | awk '{print $4}' | sort | uniq -c
kubectl get pods -A -o wide
```

## Status Types

- **Running** - Normal
- **Pending** - Waiting for scheduling/resources
- **Failed** - Pod failed
- **CrashLoopBackOff** - Container crashing repeatedly
- **Unknown** - State unclear
- **Completed** - Job finished (normal for Jobs)
- **Error** - Container failed to start

## What to Report

Count per status. Flag: Pending > 0, Failed > 0, CrashLoopBackOff > 0 as potential issues. For system namespace (kube-system), ensure CoreDNS, etcd, scheduler, controller-manager pods are Running.
