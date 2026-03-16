---
agent_name: ClusterResources
name: cluster-resources
description: Inspect cluster resource usage - CPU/memory usage, resource quotas, limits, PVC, node allocatable
interval_seconds: 300
report_section: Cluster Resources
read_only: true
---

# Cluster Resources Skill

Inspect cluster-wide resource usage and quotas. Use sub-skills for usage, quotas, and storage.

## Inspection Steps

1. **Node Usage** (sub-skills/usage.md)
   - `kubectl top nodes` (requires metrics-server)
   - Flag nodes > 80% CPU or memory

2. **Pod Usage** (sub-skills/usage.md)
   - `kubectl top pods -A`
   - Top 10 by CPU, top 10 by memory

3. **Resource Quotas** (sub-skills/quotas.md)
   - `kubectl get resourcequotas -A`
   - Namespaces approaching or exceeding limits

4. **LimitRanges**
   - `kubectl get limitranges -A`
   - Default limits per namespace

5. **Storage** (sub-skills/storage.md)
   - `kubectl get pvc -A`
   - `kubectl get pv`
   - Pending PVCs, storage class issues

6. **Node Allocatable**
   - `kubectl get nodes -o custom-columns=NAME:.metadata.name,CPU:.status.allocatable.cpu,MEM:.status.allocatable.memory`

## Output Format

```markdown
## Cluster Resources

### Node Usage
| Node | CPU | Memory | Notes |

### Top Pods (CPU)
| Namespace/Pod | CPU |

### Top Pods (Memory)
| Namespace/Pod | Memory |

### Resource Quotas
| Namespace | Resource | Used | Hard | % |

### Storage
- PVCs: X total, Y Pending
- Pending: [list with reason]

### Summary
- [Resource pressure, quota exhaustion, recommendations]
```
