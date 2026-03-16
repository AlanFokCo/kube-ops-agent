# GPU Device Plugin Sub-Skill

## Commands

```bash
kubectl get pods -A | grep -E "nvidia|gpu|device-plugin"
kubectl get daemonsets -A | grep -i gpu
kubectl describe daemonset nvidia-device-plugin-daemonset -n kube-system
```

## Expected Components

- **nvidia-device-plugin-daemonset** or **nvidia-device-plugin-ds** - DaemonSet
- Runs on all nodes with GPU (or nodes with GPU label)
- Pods must be Running for GPU to be allocatable

## Common Issues

- **No plugin found**: Cluster may not have GPU, or plugin not installed
- **Plugin CrashLoopBackOff**: Driver incompatibility, check node logs
- **Plugin not on GPU node**: Check node labels, tolerations

## Output

Plugin name | Pod count | Status | Nodes with plugin | Any non-Running pods
