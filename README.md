# Kube Ops Agent

[![CI](https://github.com/alanfokco/kube-ops-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/alanfokco/kube-ops-agent/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/alanfokco/kube-ops-agent/branch/main/graph/badge.svg)](https://codecov.io/gh/alanfokco/kube-ops-agent)

AI Agent-based Kubernetes cluster inspection, health reporting, and intelligent Q&A. Supports multi-agent collaboration, **LLM self-planning**, scheduled inspection, MCP integration, and HTTP API for cluster state queries and conversational ops.

📖 [Documentation](docs/README.md) - Architecture, developer guide, usage, LLM planning and Workflow config

## Features

- **Multi-Agent Collaboration**: Worker agents run inspections, Orchestrator plans dynamically, Summary agent aggregates reports
- **LLM Self-Planning**: Default LLM plans execution order from task and available agents; optional Workflow static orchestration
- **Intelligent Mode**: Self-Driven mode, step-by-step execution by Plan with dependency and parallelism control
- **Scheduled Inspection**: Configurable scheduling, simple interval or intelligent mode
- **Chat Assistant**: Natural language Q&A via `/chat`, read-only kubectl queries
- **MCP Integration**: Model Context Protocol for external tools
- **Health Reports**: Auto-generated Markdown cluster health reports with history

## Prerequisites

- Go 1.25+
- Accessible Kubernetes cluster (`kubectl` configured)
- OpenAI API Key (for LLM)

## Quick Start

### Install

```bash
git clone https://github.com/alanfokco/kube-ops-agent.git
cd kube-ops-agent

make build
# Or: go build -o k8sops ./cmd/k8sops
```

Built-in example skills (`kubernetes-ops-agent/skills/`) include:

- **Agent config**: Each agent dir has `agent.yaml` (metadata) and `SKILL.md` (main skill)
- **Sub-skills**: `sub-skills/*.md` for detailed check guidance
- **Planning mode**: Default LLM self-planning; optional `workflow.yaml` (use `--workflow` for static orchestration)

### Docker

```bash
make docker-build
make docker-run   # Default LLM self-planning
# Or
make docker-run-workflow   # Workflow static orchestration
```

### Kubernetes (Helm)

```bash
# After building and pushing image
make docker-build
make helm-install   # Default LLM self-planning
# Or
make helm-install-workflow   # Enable Workflow static orchestration

# Or manual
helm upgrade --install k8sops ./helm/kube-ops-agent \
  --namespace kube-ops-agent --create-namespace \
  --set openai.apiKey=YOUR_OPENAI_KEY \
  --set image.repository=your-registry/kube-ops-agent \
  --set image.tag=1.0.0
```

### Configuration

```bash
export OPENAI_API_KEY="your-api-key"           # Required
export OPENAI_MODEL="gpt-4o-mini"              # Optional
export K8SOPS_SKILLS_DIR="kubernetes-ops-agent/skills"
export K8SOPS_REPORT_DIR="kubernetes-ops-agent/report"
export K8SOPS_WORKFLOW=""                       # Empty=LLM planning; path=Workflow
export K8SOPS_MCP_CONFIG="./config/mcp_servers.yaml"   # Optional
```

### Run

```bash
./k8sops --dry-run   # List registered agents
./k8sops             # Start service (default :8080)
```

### Quick Test

```bash
# OpenAI
cp config/openai.env.example config/openai.env
# Edit config/openai.env, fill in OPENAI_API_KEY

# Or Anthropic Claude
cp config/anthropic.env.example config/anthropic.env
# Edit config/anthropic.env, fill in ANTHROPIC_API_KEY

# Or generic
cp config/test.env.example config/test.env
# Edit config/test.env, fill in OPENAI_API_KEY

./scripts/quick-test.sh dry-run   # Check agent registration only
./scripts/quick-test.sh run       # Start service
./scripts/quick-test.sh test-api  # Start and test API
```

### DashScope (Alibaba Cloud Qwen) Test

```bash
cp config/dashscope.env.example config/dashscope.env
# Edit config/dashscope.env, fill in DASHSCOPE_API_KEY

./scripts/test-dashscope.sh dry-run   # Check agent registration only
./scripts/test-dashscope.sh run      # Start service
./scripts/test-dashscope.sh test-api # Start and test API
```

## Planning Mode

| Mode | Trigger | Description |
|------|---------|-------------|
| **LLM Self-Planning** | Default (no `--workflow`) | SelfDrivenOrchestrator generates plan from task |
| **Workflow Static Orchestration** | `--workflow path/to/workflow.yaml` | Predefined YAML for fixed execution order |

See [Usage Guide](docs/usage-guide.md).

## CLI Arguments

| Argument | Env Var | Default | Description |
|----------|---------|---------|-------------|
| `--report-dir` | K8SOPS_REPORT_DIR | kubernetes-ops-agent/report | Report directory |
| `--skills-dir` | K8SOPS_SKILLS_DIR | kubernetes-ops-agent/skills | Skills root |
| `--workflow` | K8SOPS_WORKFLOW | "" | Workflow YAML path (empty=LLM planning) |
| `--addr` | K8SOPS_HTTP_ADDR | :8080 | HTTP listen address |
| `--model` | OPENAI_MODEL | gpt-4o-mini | LLM model |
| `--no-intelligent` | - | false | Simple interval mode |
| `--dry-run` | - | false | Print registered agents only, do not start |

## HTTP API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/ready` | GET | Readiness |
| `/metrics` | GET | Metrics |
| `/trigger` | POST | Manual trigger inspection |
| `/chat` | POST | Chat Q&A (JSON/SSE stream) |
| `/api/reports` | GET/POST | Report list |
| `/api/report` | GET/POST | Report detail |
| `/api/operations` | GET/POST | Operation history |
| `/api/system` | GET | System status |
| `/mcp-tools` | GET/POST | MCP tools list |

### Authentication

If `API_TOKEN`, `API_TOKENS`, or `K8SOPS_API_TOKEN` is set, include in request:

- `X-API-Key: <token>` or
- `Authorization: Bearer <token>`

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build binary |
| `make run` | Run (LLM self-planning) |
| `make run-workflow` | Run with Workflow |
| `make dry-run` | List registered agents |
| `make docker-build` | Build Docker image |
| `make docker-run` | Run container (LLM planning) |
| `make docker-run-workflow` | Run container (Workflow) |
| `make helm-install` | Helm install (LLM planning) |
| `make helm-install-workflow` | Helm install with Workflow |
| `make helm-uninstall` | Helm uninstall |

## Skills Directory Structure

Each subdirectory under `skills` is a Worker Agent. Include:

- `agent.yaml` or `SKILL.md`: Agent metadata (name, description, interval_seconds, report_section)
- `SKILL.md`: Skill description and execution guidance
- `sub-skills/`: Optional sub-skill Markdown files

See [Developer Guide](docs/developer-guide.md) for extension.

## License

Apache License 2.0 - see [LICENSE](LICENSE).
