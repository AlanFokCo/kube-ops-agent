# DNS Sub-Skill

## Commands

```bash
kubectl get pods -n kube-system -l k8s-app=kube-dns
kubectl get svc -n kube-system kube-dns
kubectl get endpoints -n kube-system kube-dns
```

## Components

- **CoreDNS** (or kube-dns): Pods in kube-system
- **kube-dns service**: ClusterIP, typically 10.96.0.10
- **Endpoints**: Must have addresses for DNS to work

## Health Check

- All CoreDNS pods Running
- kube-dns service has ClusterIP
- kube-dns endpoints has addresses (from CoreDNS pods)

## Impact

If CoreDNS down: pods cannot resolve cluster DNS names (e.g., svc.namespace.svc.cluster.local).

## Output

CoreDNS pods: X/Y Running | Service: ClusterIP [ip] | Endpoints: [ready/total]
