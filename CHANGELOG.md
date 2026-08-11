# Changelog

This document records version changes for Kube Ops Agent.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- Upgraded to agentscope-go v2.0.7 (from v1.0.1)
- Output guardrails: auto-redact AWS keys, GitHub tokens, Bearer tokens from model output
- Spend cap: K8SOPS_MAX_COST_USD env for per-session budget enforcement
- LLM circuit breaker (threshold=5, 30s reset) + rate limiter (10 req/s)
- Audit logging: structured JSON-lines at K8SOPS_AUDIT_DIR
- K8s cluster tools: kubectl_get (15 resource types, secrets blocked) + kubectl_logs
- SecretStr for API key storage (never leaks in logs)
- CNY exchange rate display in cost tracking

### Changed
- All import paths updated to agentscope-go/v2 module path
- Worker toolkit now includes framework K8s tools alongside custom kubectl tool


## [1.0.0] - 2025-03-16

### Added

- Multi-agent inspection (Worker, Orchestrator, Summary)
- Self-Driven intelligent mode with LLM self-planning
- Scheduled inspection (simple interval / intelligent mode)
- HTTP API: health, trigger, Chat, reports, operation history
- MCP tool integration
- Cluster health report generation and management
- Multi-LLM provider support: OpenAI, Anthropic (Claude), Alibaba DashScope (Qwen)
- Config env examples: `config/openai.env.example`, `config/anthropic.env.example`, `config/dashscope.env.example`
- Helm: `workflow.enabled`, `workflow.configMap` for Workflow static orchestration
- Makefile: `run-workflow`, `docker-run-workflow`, `helm-install-workflow` targets
- Test scripts: `scripts/quick-test.sh` (loads openai/anthropic/test.env), `scripts/test-dashscope.sh`

### Changed

- **Planning mode**: Default LLM self-planning; use Workflow static orchestration only when `--workflow` or `K8SOPS_WORKFLOW` is explicitly set
- `--workflow` default changed from `kubernetes-ops-agent/workflow.yaml` to empty
- Test script: `K8SOPS_WORKFLOW` env var, POST `/trigger` test support

### Documentation

- [Architecture](docs/architecture.md): System layers, components, data flow, extension points (Mermaid diagrams)
- [Plan-Centric Architecture](docs/plan-centric-architecture.md): Plan abstraction, planning-execution separation
- [Workflow](docs/workflow.md): Inspection scheduling, intelligent planning, report generation
- [Developer Guide](docs/developer-guide.md): Extending agents, skills, sub-skills, Summary customization, MCP integration
- [Usage Guide](docs/usage-guide.md): LLM self-planning vs Workflow config, mode selection
- [Workflow Configuration](docs/workflow-config.md): workflow.yaml format and orchestration logic
- Architecture diagrams converted to Mermaid format for better rendering
