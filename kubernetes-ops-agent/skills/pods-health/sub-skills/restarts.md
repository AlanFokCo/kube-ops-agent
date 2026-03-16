# Pod Restarts Sub-Skill

## Commands

```bash
kubectl get pods -A -o wide
kubectl get pods -A -o custom-columns=NAMESPACE:.metadata.namespace,POD:.metadata.name,RESTARTS:.status.containerStatuses[0].restartCount,NODE:.spec.nodeName
```

## Interpretation

| Restarts | Severity | Action |
|----------|----------|--------|
| 0-2 | Normal | May be rolling updates |
| 3-5 | Monitor | Check logs for pattern |
| 6+ | High | Likely crash loop, investigate |

## Follow-up

For high-restart pods: `kubectl logs <pod> -n <ns> --previous` to see why last container exited.

## Output

List pods with RESTARTS > 5: Namespace/Pod | Restarts | Node | (optional: last exit reason from logs)
