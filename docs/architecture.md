# Architecture

## Overview

Kube Ops Agent uses a **multi-agent collaboration** architecture, splitting Kubernetes cluster inspection into **planning, execution, and summarization** phases, each handled by different agent types. The design centers on **Plan (Inspection Plan)**—the planning layer produces structured plans, the execution layer schedules Workers by plan, and the summarization layer generates unified reports.

See [Plan-Centric LLM Self-Driven Architecture](plan-centric-architecture.md) for details.

## System Layers

```
┌──────────────────────────────────────────────────────┐
│  HTTP API Layer                                      │
│  /health, /trigger, /chat, /api/reports, ...         │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  Scheduler Layer                                     │
│  Timed trigger / Intelligent planning / Simple       │
│  interval / Plan execution loop                      │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  Orchestrator Layer                                  │
│  LLM self-planning or Workflow static orchestration  │
│  → InspectionPlan                                    │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  Executor Layer                                      │
│  Execute Worker / Retry / Circuit breaker /          │
│  Concurrency control                                 │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  Worker Layer (ReAct / SelfDriven)                   │
│  kubectl / Skill invocation / LLM reasoning /        │
│  Read-only constraints                               │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  Runtime Layer                                       │
│  Circuit breaker / Rate limiting / State persistence │
│  / Metrics                                           │
└──────────────────────────────────────────────────────┘
```

## Core Components

### 1. Registry

- **Role**: Discover and register agents from skills directory
- **Implementation**: `FileRegistry` scans subdirectories under `skills_dir` containing `agent.yaml` or `SKILL.md`
- **Output**: `Spec` list (Name, Description, IntervalSecond, ReportSection, SubSkills, ReadOnly)
- **Discovery rules**: `agent.yaml` takes precedence over `SKILL.md` frontmatter in same dir; auto-scans `sub-skills/*.md` when `sub_skills` not configured

### 2. Scheduler

- **Role**: Trigger inspection by schedule or rules, drive the full flow: planning → execution → summarization
- **Modes**:
  - **Simple**: Execute when `interval_seconds` expires, no planning, direct concurrent execution of due agents
  - **Intelligent**: Check every `checkInterval` seconds, generate InspectionPlan and execute by steps
- **Planning sources** (intelligent mode): Workflow file (if specified) > SelfDrivenOrchestrator > OrchestratorAgent > Fallback full parallel
- **Code**: `internal/scheduler/scheduler.go`

### 3. Orchestrator

- **Role**: Generate `InspectionPlan` from task and available Workers
- **Types**:
  - **SelfDrivenOrchestrator**: Self-driven planning based on ThinkingAgent, preferred by default
  - **OrchestratorAgent**: Traditional LLM single call, outputs InspectionPlan JSON
- **Output**: `InspectionPlan` (assessment, priority, steps, depends_on, mode, skip_agents, etc.)
- **Code**: `internal/agent/orchestrator_self_driven.go`, `internal/agent/orchestrator.go`

### 4. Executor

- **Role**: Execute single Worker with production-grade guarantees
- **Capabilities**: Retry (max 3, exponential backoff), circuit breaker, concurrency slot limit, timeout
- **Worker types**: Prefer SelfDrivenWorker (Thinking-driven), fallback to ReActAgent (skill-driven) on failure
- **Code**: `internal/agent/executor.go`

### 5. Summary Agent

- **Role**: Aggregate Worker outputs into Markdown health report
- **Types**: `SelfDrivenSummary` (preferred) or `SummaryAgent` (traditional)
- **Input**: Worker text outputs, organized by `report_section`
- **Output**: Markdown files under `report_dir`
- **Code**: `internal/agent/summary_self_driven.go`, `internal/agent/summary.go`

### 6. Runtime Environment

- **Role**: Unified management of concurrency, rate limiting, circuit breaker, state, metrics
- **Components**: ConcurrencyController, CircuitBreaker, RateLimiter, StatePersistence, KubectlLimit
- **Code**: `internal/runtime/`

## Data Flow

```
Skills directory (skills/)
          │
          ▼
    Registry.Specs()
          │
          ▼
    Scheduler.loop()
         / \
        /   \
       ▼     ▼
  Simple    Intelligent
  mode      mode
  │         │
  │         ▼
  │    Generate InspectionPlan
  │    (Workflow / SelfDrivenOrch / Orch / Fallback)
  │         │
  │         ▼
  │    Execute by Step (depends_on, mode)
  │         │
  └────►    │
            ▼
      Executor.Execute(agentName) × N
            │
            ▼
      Worker.Reply() → results
            │
            ▼
      Summary.Summarize(results) → Markdown
            │
            ▼
      Write to report/k8s_health_report_*.md
```

## Extension Points

| Extension | Method | Notes |
|-----------|--------|-------|
| **Skills / Agent** | Add subdirectory under `skills/` | See [Developer Guide](developer-guide.md) |
| **Planning** | `--workflow` or LLM default | See [Usage Guide](usage-guide.md) |
| **MCP** | `K8SOPS_MCP_CONFIG` | Mount external tools to Agent Toolkit |
| **LLM** | `OPENAI_MODEL` | Switch model for planning and execution |
