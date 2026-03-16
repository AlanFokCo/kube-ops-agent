# Kube Ops Agent Helm Chart

Deploy Kube Ops Agent in Kubernetes. Default: **LLM self-planning**; optional Workflow static orchestration.

## Prerequisites

- Kubernetes 1.24+
- Helm 3+
- Built and pushed Docker image (or use default `kube-ops-agent:1.0.0`)

## Installation

### Method 1: Pass API Key directly

```bash
helm upgrade --install k8sops ./helm/kube-ops-agent \
  --namespace kube-ops-agent --create-namespace \
  --set openai.apiKey=YOUR_OPENAI_API_KEY \
  --set image.repository=your-registry/kube-ops-agent \
  --set image.tag=1.0.0
```

### Method 2: Use existing Secret (recommended for production)

```bash
# Create Secret first
kubectl create secret generic k8sops-openai \
  --from-literal=api-key=YOUR_OPENAI_API_KEY \
  -n kube-ops-agent

# Reference at install
helm upgrade --install k8sops ./helm/kube-ops-agent \
  --namespace kube-ops-agent --create-namespace \
  --set openai.existingSecret=k8sops-openai \
  --set openai.apiKey="" \
  --set image.repository=your-registry/kube-ops-agent
```

Note: When using `existingSecret`, set `openai.apiKey` to empty to avoid creating a new Secret.

### Method 3: Use Makefile

```bash
export OPENAI_API_KEY=your-key
make docker-build
make helm-install
```

## Planning Mode

- **Default**: LLM self-planning (`workflow.enabled: false`)
- **Workflow**: Enable `workflow.enabled: true` for static orchestration, fewer LLM calls

```bash
# Enable built-in Workflow
helm upgrade --install k8sops ./helm/kube-ops-agent \
  --set openai.apiKey=xxx \
  --set workflow.enabled=true \
  --set workflow.configMap=default
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `openai.apiKey` | OpenAI API Key | "" |
| `openai.existingSecret` | Existing Secret name | "" |
| `openai.model` | LLM model | gpt-4o-mini |
| `image.repository` | Image repository | kube-ops-agent |
| `image.tag` | Image tag | 1.0.0 |
| `skills.useDefaultSkills` | Use built-in example skills | true |
| `workflow.enabled` | Enable Workflow static orchestration | false |
| `workflow.configMap` | Workflow ConfigMap (default=built-in) | "" |
| `config.simpleMode` | Simple interval mode (no planning) | false |
| `report.persistentVolumeClaim.create` | Create report PVC | true |
| `report.persistentVolumeClaim.size` | Report storage size | 1Gi |
| `ingress.enabled` | Enable Ingress | false |

## Access Service

```bash
# Port forward
kubectl port-forward svc/k8sops-kube-ops-agent 8080:8080 -n kube-ops-agent

# Access
curl http://localhost:8080/health
curl -X POST http://localhost:8080/trigger
```

## Uninstall

```bash
helm uninstall k8sops -n kube-ops-agent
# Or
make helm-uninstall
```
