# Endpoints Sub-Skill

## Commands

```bash
kubectl get endpoints -A
kubectl get endpoints -A -o jsonpath='{range .items[?(@.subsets==nil)]}{.metadata.namespace}{"/"}{.metadata.name}{"\n"}{end}'
kubectl get endpoints -A -o jsonpath='{range .items[?(@.subsets[*].addresses==nil)]}{.metadata.namespace}{"/"}{.metadata.name}{"\n"}{end}'
```

## Endpoint States

- **Has addresses**: Service can route traffic to pods
- **No addresses (empty subsets)**: No ready pods match selector
- **No subsets**: Service has no pods

## Common Causes

- Selector mismatch - no pods with labels
- Pods not Ready - readiness probe failing
- All pods Pending/Failed

## Impact

Service exists but traffic goes nowhere. Clients get connection refused or timeout.

## Output

List services with empty endpoints: Namespace/Service | Backend pods (from selector) | Reason
