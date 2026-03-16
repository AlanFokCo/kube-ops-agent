---
agent_name: NetworkHealth
name: network-health
description: Inspect container network health - CoreDNS, Services, Endpoints, Pod IPs, CNI pods, NetworkPolicies
interval_seconds: 300
report_section: Network Health
read_only: true
---

# Network Health Skill

Inspect pod networking and cluster DNS. Use sub-skills for DNS, CNI, and endpoints.

## Inspection Steps

1. **DNS (CoreDNS)** (sub-skills/dns.md)
   - `kubectl get pods -n kube-system -l k8s-app=kube-dns`
   - `kubectl get svc -n kube-system kube-dns`
   - CoreDNS pods must be Running for cluster DNS

2. **CNI** (sub-skills/cni.md)
   - `kubectl get pods -A | grep -E "calico|flannel|cilium|weave|canal|kube-router"`
   - CNI DaemonSet pods = one per node

3. **Endpoints** (sub-skills/endpoints.md)
   - `kubectl get endpoints -A`
   - Endpoints with empty addresses = service has no ready pods

4. **Services**
   - `kubectl get svc -A` - ClusterIP, NodePort, LoadBalancer
   - Check for pending LoadBalancer (external IP not assigned)

5. **NetworkPolicies**
   - `kubectl get networkpolicies -A`
   - Note any that might block traffic

## Output Format

```markdown
## Network Health

### DNS (CoreDNS)
- Pods: X/Y Running
- Service: ClusterIP [ip]

### CNI
- Type: [calico/flannel/cilium/...]
- Pods: X/Y Running (expected: 1 per node)

### Services
- Total: X
- LoadBalancer pending: [list]

### Empty Endpoints
| Namespace/Service | Issue |

### NetworkPolicies
- Count: X
```
