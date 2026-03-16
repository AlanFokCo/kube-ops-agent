# CNI Sub-Skill

## Commands

```bash
kubectl get pods -A | grep -E "calico|flannel|cilium|weave|canal|kube-router|tigera"
kubectl get daemonsets -A | grep -E "calico|flannel|cilium"
```

## CNI Types

| CNI | DaemonSet name pattern |
|-----|-------------------------|
| Calico | calico-node, calico-kube-controllers |
| Flannel | kube-flannel-ds |
| Cilium | cilium |
| Weave | weave-net |
| Canal | canal |

## Expected

- DaemonSet pods = number of nodes (or nodes with specific labels)
- All pods Running

## Common Issues

- **Not all nodes**: Some nodes may have taints, check DaemonSet tolerations
- **CrashLoopBackOff**: CNI config mismatch, node driver issue

## Output

CNI type | DaemonSet | Pods: X/Y Running | Nodes covered
