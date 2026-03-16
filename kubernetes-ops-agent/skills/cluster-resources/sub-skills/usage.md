# Resource Usage Sub-Skill

## Commands

```bash
kubectl top nodes
kubectl top pods -A
kubectl top pods -A --sort-by=cpu
kubectl top pods -A --sort-by=memory
```

## Requirements

- **metrics-server** must be installed for `kubectl top` to work
- If not: report "metrics-server not installed", use capacity/allocatable instead

## Thresholds

| Usage | Severity |
|-------|----------|
| < 70% | Normal |
| 70-85% | Monitor |
| > 85% | High - risk of eviction/scheduling failure |

## Output

For nodes: Node | CPU (used/total) | Memory (used/total) | Flag if > 80%
For pods: Top 10 by CPU, top 10 by memory
