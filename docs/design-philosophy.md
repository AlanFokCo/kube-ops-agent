# Design Philosophy

## 1. Skills as Configuration

Agent capabilities are defined by the **skills directory**; no code changes needed to extend:

- Each subdirectory = one Worker Agent
- `SKILL.md` or `agent.yaml` describes metadata and execution guidance
- Supports `sub-skills/` sub-skills, `interval_seconds`, `report_section`, etc.

**Intent**: Ops can define inspection logic by writing Markdown, lowering onboarding cost.

## 2. Orchestration vs Execution Separation

- **Orchestrator**: Decides "what to do and in what order"
- **Executor**: Handles "how to execute safely and reliably"

The orchestration layer outputs structured plans (JSON); the execution layer handles retry, circuit breaker, and concurrency control. Clear separation of concerns.

## 3. Dual-Track Agent System

Two Worker types:

| Type | Driver | Use Case |
|------|--------|----------|
| ReActAgent | Skills + tool calls | Rule-based, scriptable inspection |
| SelfDrivenWorker | ThinkingAgent | Reasoning, dynamic decision-making |

Orchestrator and Summary also have traditional and SelfDriven variants. Choose by scenario.

## 4. Production-Grade Execution

Executor and Runtime provide:

- **Circuit breaker**: Pause calls after agent failure to avoid cascade
- **Rate limiting**: Configurable kubectl and agent concurrency limits
- **Retry**: Exponential backoff; non-retryable errors fail fast
- **Timeout**: Per-execution and per-step timeouts
- **State persistence**: last_run, etc., supports checkpoint resume

## 5. Read-Only First

- Workers default to `read_only: true`
- kubectl tool supports only read-only commands
- Reduces risk of accidental changes; fits inspection use case

## 6. Observability

- `/health`: Service and circuit breaker status
- `/metrics`: Concurrency, circuit breaker, metrics
- `/api/operations`: Execution history
- Logs: Agent results, plan execution progress

## 7. Extensibility

- **MCP**: Integrate external tools via Model Context Protocol
- **Skills**: Add directory to register new agent
- **Plan format**: Supports extended fields like `steps`, `depends_on`, `skip_agents`
