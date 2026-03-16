# Developer Guide

This document is for developers extending Kube Ops Agent—how to add agents, write skills, and related best practices.

## Table of Contents

- [Extending Agents](#extending-agents)
- [Extending Skills](#extending-skills)
- [Sub-Skills](#sub-skills)
- [Summary Agent Customization](#summary-agent-customization)
- [MCP Tool Integration](#mcp-tool-integration)
- [Testing and Debugging](#testing-and-debugging)

---

## Extending Agents

### Method 1: Via Skills Directory (Recommended)

**No Go code changes**—add a subdirectory under `kubernetes-ops-agent/skills/` to auto-register a new agent.

#### 1. Create Skill Directory Structure

```
skills/
└── my-new-agent/
    ├── agent.yaml      # Metadata (alternative to SKILL.md)
    └── SKILL.md        # Main skill description
```

#### 2. Define agent.yaml

```yaml
name: MyNewAgent                    # Agent name, must be unique in directory
description: Inspect XXX health status  # Short description for Orchestrator selection
interval_seconds: 300               # Scheduled inspection interval (seconds); 0 = no schedule
report_section: My Section          # Section title in report
sub_skills:                         # Optional: sub-skill path list
  - sub-skills/detail-a.md
  - sub-skills/detail-b.md
read_only: true                     # Default true, read-only kubectl only
```

#### 3. Or Use SKILL.md Frontmatter

If you prefer not to maintain `agent.yaml`, use YAML frontmatter at the top of `SKILL.md`:

```markdown
---
agent_name: MyNewAgent
name: my-new-agent
description: Inspect XXX health status
interval_seconds: 300
report_section: My Section
read_only: true
---

# My New Agent Skill

(Skill body)
```

**Note**: If both `agent.yaml` and `SKILL.md` exist in the same directory, `agent.yaml` takes precedence; `SKILL.md` is used for skill content only.

### Registration Rules

- **Discovery**: `FileRegistry` scans subdirectories under `skills_dir` containing `agent.yaml` or `SKILL.md`
- **Naming**: `agent_name` or `name` must be non-empty and unique within the skills root
- **Filtering**: With `--interval-positive-only`, only agents with `interval_seconds > 0` are registered

### Verification

```bash
# Print registered agents only, do not start service
k8sops --dry-run
```

Output should include the new agent's name, skill path, and interval.

---

## Extending Skills

### SKILL.md Structure

`SKILL.md` is the main guidance for agent execution; the LLM reads it to plan inspection steps.

#### Recommended Structure

```markdown
# [Agent Name] Skill

(One-line responsibility description)

## Inspection Steps

1. **Step one** (see sub-skills/xxx.md)
   - `kubectl get ...`
   - Check points

2. **Step two**
   - Commands and check logic

## Output Format

```markdown
## [Section Title]

### Summary
- Key metrics

### Details
[Tables or lists]

### Issues (if any)
- Problem list
```
```

### Writing Guidelines

| Guideline | Description |
|-----------|-------------|
| **Command examples** | Provide runnable kubectl commands to reduce LLM trial-and-error |
| **Checklist** | Clarify "what to check" and "how to judge normal vs abnormal" |
| **Output format** | Use Markdown template to constrain report structure for Summary aggregation |
| **Sub-skill references** | Split complex domains into `sub-skills/*.md`; keep main skill concise |

### Read-Only Constraints

- Default `read_only: true`; only `kubectl get`, `describe`, `logs`, `top` allowed
- Forbidden: `apply`, `delete`, `patch`, `edit`, `create`, `exec`, etc.
- Constraints are injected in system prompt; LLM and tool layer both validate

---

## Sub-Skills

Sub-skills split the main skill into finer inspection areas for the LLM to reference as needed.

### Directory Structure

```
my-agent/
├── agent.yaml
├── SKILL.md
└── sub-skills/
    ├── namespaces.md
    ├── nodes-brief.md
    └── pod-counts.md
```

### Sub-Skill Content Template

```markdown
# [Topic] Sub-Skill

## Commands

```bash
kubectl get namespaces
kubectl get namespaces -o custom-columns=...
```

## What to Check

1. **Metric one** - Normal/abnormal criteria
2. **Metric two** - Check points

## Common Issues

- **Issue type**: Possible causes and troubleshooting

## Output

What the report should include for this sub-domain.
```

### Reference Methods

- **agent.yaml**: `sub_skills: [sub-skills/namespaces.md, ...]`
- **SKILL.md**: In main skill, write "see sub-skills/namespaces.md"
- **Auto-discovery**: If `sub_skills` not configured, Registry auto-scans `sub-skills/*.md`

---

## Summary Agent Customization

Summary Agent aggregates Worker outputs into the final report. Its skill is at `skills/summary/`.

### Customize Report Structure

Edit `skills/summary/SKILL.md`:

1. **Report Structure**: Adjust section order and titles
2. **Executive Summary**: Define summary writing rules
3. **Recommendations**: Define format by severity (Critical/High/Medium)

### Mapping to Workers

Summary input is Worker text output, organized by `report_section` or agent name. After adding a Worker, add the corresponding section in Summary's Report Structure.

---

## MCP Tool Integration

External tools (custom CLI, internal APIs) can be integrated via Model Context Protocol.

### Configuration

1. Prepare MCP server config, e.g. `config/mcp_servers.yaml`
2. Set env: `K8SOPS_MCP_CONFIG=config/mcp_servers.yaml`
3. Enable MCP in agent's `agent.yaml`:

```yaml
toolkit:
  use_mcp: true
  mcp_tools: []   # Empty = use all; or list tool names
```

### Tool Discovery

MCP tools are injected into the Agent Toolkit; the LLM can choose to call them based on the task. See MCP config examples in the project root.

---

## Testing and Debugging

### Local Verification

```bash
# 1. Check agent registration
k8sops --dry-run

# 2. Manually trigger one inspection (service must be running)
curl -X POST http://localhost:8080/trigger

# 3. View reports
ls kubernetes-ops-agent/report/
```

### Logging

- Set `--log-level DEBUG` for more detailed execution logs
- Plan generation, step execution, circuit breaker status are logged to stdout

### Troubleshooting

| Issue | Check |
|-------|-------|
| Agent not registered | Verify `agent_name`/`name` non-empty, `skills_dir` correct |
| Execution timeout | Adjust `K8SOPS_AGENT_TIMEOUT` or step-level `timeout_seconds` |
| Circuit breaker open | Check `/health`, wait for recovery or restart |
| Report missing section | Ensure Summary Report Structure includes corresponding `report_section` |
