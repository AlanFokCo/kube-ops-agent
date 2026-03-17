# Workflow

## 1. Startup Flow

```
main() → runServer()
        │
        ▼
  NewEnvironment()
        │
        ▼
  NewFileRegistry(skills)
        │
        ▼
  NewExecutor()
        │
        ▼
  NewOrchestratorAgent() / NewSelfDrivenOrchestrator()
        │
        ▼
  NewSummaryAgent() / NewSelfDrivenSummary()
        │
        ▼
  Scheduler.Start()
        │
        ▼
  setupMCP()
        │
        ▼
  HTTP Server listen
```

## 2. Simple Mode

For fixed-interval scheduled inspection:

```
Scheduler checks every checkInterval seconds
        │
        ▼
Build due list: agents where last_run + interval_seconds <= now
        │
        ▼
runOneRoundSync(due) → Execute due agents concurrently
        │
        ▼
Executor.Execute(agentName) × N
        │
        ▼
State.Save()
```

## 3. Intelligent Mode

Orchestrator dynamically generates plans with step dependencies and parallel/sequential execution:

```
Scheduler every checkInterval seconds
        │
        ▼
1. Generate plan: SelfDrivenOrch / OrchAgent / Fallback
        │
        ▼
2. Execute by Step: depends_on, mode, Executor.Execute()
        │
        ▼
3. Summarize: SelfDrivenSummary / SummaryAgent
        │
        ▼
4. Write to report/k8s_health_report_*.md
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

```
Executor.Execute(name, input)
        │
        ▼
Circuit breaker check
        │
        ▼
Acquire concurrency slot
        │
        ▼
Retry loop (max 3)
        │
        ▼
SelfDrivenWorker.Inspect()
       / \
      /   \
     ▼     ▼
 Success  Failure
     │       │
  Return     ▼
       ReActAgent.Reply()
             │
             ▼
       Read SKILL.md, tools, LLM reasoning
             │
             ▼
       Record Circuit / Metrics / State
```

## 6. HTTP Trigger Flow

```
POST /trigger
     │
     ▼
Parse agent_names (optional)
     │
     ▼
Scheduler.RunOneRound()
     │
     ▼
Return {status, agents}
```

## 7. Chat Flow

```
POST /chat
     │
     ▼
Executor.ExecuteChat()
     │
     ▼
buildChatAgent() → K8sChatAgent
     │
     ▼
System prompt, tools, LLM reasoning
     │
     ▼
Return answer (JSON/SSE)
```
