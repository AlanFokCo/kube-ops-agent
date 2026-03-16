# Workflow

## 1. Startup Flow

```mermaid
flowchart TB
    A["main() → runServer()"]
    B["NewEnvironment()"]
    C["NewFileRegistry(skills)"]
    D["NewExecutor()"]
    E["NewOrchestratorAgent() / NewSelfDrivenOrchestrator()"]
    F["NewSummaryAgent() / NewSelfDrivenSummary()"]
    G["Scheduler.Start()"]
    H["setupMCP()"]
    I["HTTP Server listen"]

    A --> B --> C --> D --> E --> F --> G --> H --> I
```

## 2. Simple Mode

For fixed-interval scheduled inspection:

```mermaid
flowchart TB
    A["Scheduler checks every checkInterval seconds"]
    B["Build due list: agents where last_run + interval_seconds <= now"]
    C["runOneRoundSync(due) → Execute due agents concurrently"]
    D["Executor.Execute(agentName) × N"]
    E["State.Save()"]

    A --> B --> C --> D --> E
```

## 3. Intelligent Mode

Orchestrator dynamically generates plans with step dependencies and parallel/sequential execution:

```mermaid
flowchart TB
    A["Scheduler every checkInterval seconds"]
    B["1. Generate plan: SelfDrivenOrch / OrchAgent / Fallback"]
    C["2. Execute by Step: depends_on, mode, Executor.Execute()"]
    D["3. Summarize: SelfDrivenSummary / SummaryAgent"]
    E["4. Write to report/k8s_health_report_*.md"]

    A --> B --> C --> D --> E
```

## 4. Plan Structure (InspectionPlan)

```json
{
  "assessment": "Cluster overall assessment",
  "priority": "critical|high|normal|low",
  "steps": [
    {
      "agents": ["AgentA", "AgentB"],
      "mode": "parallel",
      "focus_areas": ["nodes", "pods"],
      "depends_on": [],
      "condition": "Optional condition description",
      "timeout_seconds": 300
    },
    {
      "agents": ["AgentC"],
      "mode": "sequential",
      "depends_on": ["AgentA"]
    }
  ],
  "skip_agents": ["AgentX"],
  "skip_reasoning": "Reason for skipping"
}
```

## 5. Worker Execution Flow

```mermaid
flowchart TB
    A["Executor.Execute(name, input)"]
    B["Circuit breaker check"]
    C["Acquire concurrency slot"]
    D["Retry loop (max 3)"]
    E{"SelfDrivenWorker.Inspect()"}
    F["Return"]
    G["ReActAgent.Reply()"]
    H["Read SKILL.md, tools, LLM reasoning"]
    I["Record Circuit / Metrics / State"]

    A --> B --> C --> D --> E
    E -->|Success| F
    E -->|Failure| G --> H --> I
```

## 6. HTTP Trigger Flow

```mermaid
flowchart LR
    A["POST /trigger"] --> B["Parse agent_names (optional)"]
    B --> C["Scheduler.RunOneRound()"]
    C --> D["Return {status, agents}"]
```

## 7. Chat Flow

```mermaid
flowchart LR
    A["POST /chat"] --> B["Executor.ExecuteChat()"]
    B --> C["buildChatAgent() → K8sChatAgent"]
    C --> D["System prompt, tools, LLM reasoning"]
    D --> E["Return answer (JSON/SSE)"]
```
