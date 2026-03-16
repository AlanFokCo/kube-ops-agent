# Resource Quotas Sub-Skill

## Commands

```bash
kubectl get resourcequotas -A
kubectl describe resourcequotas -A
```

## Quota Structure

- **Hard**: Maximum allowed (e.g., requests.cpu: "10")
- **Used**: Current consumption
- **% Used**: Used/Hard - flag if > 80%

## Common Resources

- requests.cpu, limits.cpu
- requests.memory, limits.memory
- persistentvolumeclaims
- pods, services

## Impact

When quota exceeded: new pods fail with "exceeded quota" in events.

## Output

| Namespace | Resource | Used | Hard | % | Status |
