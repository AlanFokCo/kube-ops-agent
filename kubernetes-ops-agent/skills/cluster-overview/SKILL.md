---
agent_name: ClusterOverview
name: cluster-overview
description: Inspect Kubernetes cluster overview - nodes status, namespaces, and basic resource counts
interval_seconds: 300
report_section: Cluster Overview
read_only: true
---

# Cluster Overview Skill

Inspect the Kubernetes cluster at a high level. Use view_text_file to read sub-skills for detailed checks.

## Inspection Steps

1. **Node Status** (see sub-skills/nodes-brief.md)
   - `kubectl get nodes -o wide`
   - Count Ready vs NotReady, note SchedulingDisabled
   - For NotReady: check `kubectl describe node <name>` Events

2. **Namespaces** (see sub-skills/namespaces.md)
   - `kubectl get namespaces`
   - Count total, list system namespaces (kube-system, kube-public, kube-node-lease)
   - Check for terminating namespaces

3. **Resource Overview**
   - `kubectl get pods -A` - count by status
   - `kubectl top nodes` if metrics-server available
   - `kubectl version --short` - cluster version

4. **Cluster Info**
   - `kubectl cluster-info` - API server, core services
   - `kubectl get componentstatuses` (if still available)

## Output Format

```markdown
## Cluster Overview

### Summary
- Nodes: X total, Y Ready, Z NotReady
- Namespaces: X
- Pods: Running X, Pending Y, Failed Z, CrashLoopBackOff W
- Kubernetes: vX.Y.Z

### Nodes
[Brief status table]

### Namespaces
[Count and notable namespaces]

### Pod Summary
| Status | Count |
|--------|-------|
| Running | X |
| Pending | Y |
| Failed | Z |

### Issues (if any)
- [Concise list of problems found]
```
