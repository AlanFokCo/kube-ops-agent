---
agent_name: DeploymentsStatus
name: deployments-status
description: Inspect Kubernetes deployments - replicas, availability, rollout status
interval_seconds: 300
report_section: Deployments
read_only: true
---

# Deployments Status Skill

Inspect deployment status across the cluster. Use sub-skills for unhealthy deployments.

## Inspection Steps

1. **List All Deployments**
   - `kubectl get deployments -A`
   - Columns: DESIRED, CURRENT, UP-TO-DATE, AVAILABLE, AGE

2. **Unhealthy Deployments** (sub-skills/unhealthy.md)
   - AVAILABLE < DESIRED
   - `kubectl describe deployment <name> -n <ns>` - Events, ReplicaSet status

3. **Status and ReplicaSets**
   - `kubectl get rs -A` - ReplicaSet desired/current
   - Stuck rollouts: DESIRED != READY for long time

4. **StatefulSets and DaemonSets** (optional)
   - `kubectl get statefulsets -A`
   - `kubectl get daemonsets -A` - check DESIRED vs NUMBER AVAILABLE

## Output Format

```markdown
## Deployments

### Summary
- Total: X, Healthy: Y, Unhealthy: Z

### Unhealthy Deployments
| Namespace/Deployment | DESIRED | AVAILABLE | Reason |

### Stuck Rollouts
| Namespace/Deployment | Issue |

### Recommendations
- [Scale up, check events, resource limits, etc.]
```
