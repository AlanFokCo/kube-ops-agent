---
agent_name: PodsHealth
name: pods-health
description: Inspect pod health across namespaces - Running, Pending, Failed, CrashLoopBackOff
interval_seconds: 300
report_section: Pods Health
read_only: true
---

# Pods Health Skill

Inspect pod health across the cluster. Use sub-skills for problem pods and restart analysis.

## Inspection Steps

1. **Status Summary**
   - `kubectl get pods -A --no-headers | awk '{print $4}' | sort | uniq -c`
   - Count Running, Pending, Failed, CrashLoopBackOff, Unknown, Completed

2. **Problem Pods** (sub-skills/problem-pods.md)
   - Non-Running pods: `kubectl get pods -A | grep -v Running`
   - For each: `kubectl describe pod <name> -n <ns>` - extract Events, Reason

3. **Restarts** (sub-skills/restarts.md)
   - `kubectl get pods -A -o wide` - RESTARTS column
   - Flag RESTARTS > 5 as unstable

4. **Container Status**
   - `kubectl get pods -A -o jsonpath='{range .items[?(@.status.containerStatuses[*].ready==false)]}{.metadata.namespace}{"/"}{.metadata.name}{" "}{.status.containerStatuses[*].state}{"\n"}{end}'`

## Output Format

```markdown
## Pods Health

### Summary
| Status | Count |
|--------|-------|
| Running | X |
| Pending | Y |
| Failed | Z |
| CrashLoopBackOff | W |

### Problem Pods (Critical)
| Namespace/Pod | Status | Reason | Age |

### High Restart Pods (>5 restarts)
| Namespace/Pod | Restarts | Node |

### Recommendations
- [Root cause hints from events]
```
