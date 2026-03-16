---
agent_name: GPUHealth
name: gpu-health
description: Inspect GPU health in cluster - nvidia device plugin, GPU capacity/allocatable, GPU pods status
interval_seconds: 300
report_section: GPU Health
read_only: true
---

# GPU Health Skill

Inspect GPU resource availability and health. Use sub-skills for device plugin and GPU pods.

## Inspection Steps

1. **Device Plugin** (sub-skills/device-plugin.md)
   - `kubectl get pods -A | grep -E "nvidia|gpu|device-plugin"`
   - nvidia-device-plugin-daemonset should run on GPU nodes
   - Check pod status, node distribution

2. **Node GPU Capacity** (sub-skills/gpu-pods.md)
   - `kubectl get nodes -o custom-columns=NAME:.metadata.name,GPU-CAP:.status.capacity.nvidia\.com/gpu,GPU-ALLOC:.status.allocatable.nvidia\.com/gpu`
   - Nodes without nvidia.com/gpu = no GPU

3. **GPU Pods**
   - Pods requesting nvidia.com/gpu in limits
   - `kubectl get pods -A -o jsonpath='{range .items[?(@.spec.containers[*].resources.limits.nvidia\.com/gpu)]}{.metadata.namespace}{"/"}{.metadata.name}{"\t"}{.status.phase}{"\t"}{.spec.containers[0].resources.limits.nvidia\.com/gpu}{"\n"}{end}'`

4. **GPU Availability**
   - Compare allocatable vs requested (from pods)

## Output Format

```markdown
## GPU Health

### Device Plugin
- Status: [Running/Not found]
- Pods: [count] on [nodes]

### GPU Nodes
| Node | Capacity | Allocatable |

### GPU Pods
| Namespace/Pod | Phase | GPU Requested |

### Summary
- GPU nodes: X
- Total GPU: Y
- GPU pods: Z
- Issues: [if any]
```
