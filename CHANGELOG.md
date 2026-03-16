# Changelog

This document records version changes for Kube Ops Agent.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

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
