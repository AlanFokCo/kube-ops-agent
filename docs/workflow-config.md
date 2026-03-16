# Workflow Configuration

## Overview

**LLM self-planning is preferred by default.** The Scheduler uses static orchestration only when a workflow path is explicitly specified. Use static orchestration when you need fixed execution order or want to reduce LLM calls.

## Configuration Path

- Default: Not specified (use LLM self-planning)
- Optional: Specify YAML path via `--workflow` or `K8SOPS_WORKFLOW` (e.g. `kubernetes-ops-agent/workflow.yaml`)

## Orchestration Priority

1. **Static workflow**: If workflow path is specified and file is parseable, use static orchestration
2. **SelfDrivenOrchestrator**: LLM self-planning (default)
3. **OrchestratorAgent**: LLM generates JSON plan
4. **Fallback**: All agents execute in parallel

## YAML Format

```yaml
assessment: "Overall assessment description"
priority: normal  # critical | high | normal | low
reasoning: "Orchestration rationale"

steps:
  - agents:
      - AgentName1
      - AgentName2
    mode: parallel   # parallel | sequential
    focus_areas:
      - area1
    condition: "Step description"
    depends_on: []   # Names of dependent agents
    timeout_seconds: 300

  - agents:
      - AgentName3
    mode: sequential
    depends_on:
      - AgentName1
```

## Orchestration Rules

- **depends_on**: Agents this step depends on; step runs only after dependencies have results
- **mode**: `parallel` = concurrent, `sequential` = serial
- **skip_agents**: Top-level list of agents to skip
