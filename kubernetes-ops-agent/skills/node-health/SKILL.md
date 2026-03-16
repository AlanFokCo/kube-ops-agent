---
agent_name: NodeHealth
name: node-health
description: Inspect Kubernetes node health - Ready status, conditions (MemoryPressure, DiskPressure, PIDPressure), taints, capacity
interval_seconds: 300
report_section: Node Health
read_only: true
---

# Node Health Skill

Inspect the health status of Kubernetes nodes in detail. Use sub-skills for conditions, taints, and resources.

## Inspection Steps

1. **Node Conditions** (sub-skills/conditions.md)
   - Ready, MemoryPressure, DiskPressure, PIDPressure
   - `kubectl describe nodes` or jsonpath for conditions

2. **Taints** (sub-skills/taints.md)
   - NoSchedule, NoExecute, PreferNoSchedule
   - Impact on pod scheduling

3. **Resources** (sub-skills/resources.md)
   - `kubectl top nodes` - CPU/memory usage
   - Capacity vs Allocatable - reserved system resources

4. **Node Details**
   - `kubectl get nodes -o wide` - OS, kernel, container runtime
   - Check for node.kubernetes.io/not-ready, node.kubernetes.io/unreachable

## Output Format

```markdown
## Node Health

### Summary
- Total: X, Ready: Y, NotReady: Z
- Nodes with pressure: [count]

### Node Status Table
| Node | Ready | MemPressure | DiskPressure | PIDPressure | Taints |
|------|-------|-------------|--------------|-------------|-------|

### Problem Nodes
| Node | Issue | Detail |

### Resource Usage (if metrics-server)
| Node | CPU | Memory |

### Recommendations
- [Actionable items based on findings]
```
