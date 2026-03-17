package agent

import (
	"fmt"
	"path/filepath"
	"strings"
)

// MakeTriggerMsg builds structured trigger message in make_trigger_msg style.
// Includes orchestrator_context, focus_areas, input_data, aligned with Python worker_agent.make_trigger_msg.
func MakeTriggerMsg(spec Spec, inputData map[string]any, focusAreas []string, orchestratorContext string) string {
	var parts []string

	if orchestratorContext != "" {
		parts = append(parts, `## Orchestrator Assessment

The Orchestrator Agent has analyzed the cluster and provides this context:

`+orchestratorContext+`

---
`)
	}

	if len(focusAreas) > 0 {
		var list []string
		for _, a := range focusAreas {
			list = append(list, "- "+a)
		}
		parts = append(parts, `## Focus Areas

Based on the orchestrator's analysis, please pay special attention to:
`+strings.Join(list, "\n")+`

---
`)
	}

	if len(inputData) > 0 {
		var ctxParts []string
		for k, v := range inputData {
			if k == "orchestrator_context" || k == "focus_areas" {
				continue
			}
			ctxParts = append(ctxParts, fmt.Sprintf("### Context from %s:\n%v", k, truncateStr(fmt.Sprintf("%v", v), 2000)))
		}
		if len(ctxParts) > 0 {
			parts = append(parts, `## Previous Stage Context

`+strings.Join(ctxParts, "\n\n")+`

---
`)
		}
	}

	skillPath := filepath.Join(spec.SkillDir, "SKILL.md")
	base := fmt.Sprintf(`Please execute the [%s] inspection skill.

%s## Your Tasks

1. **Read your skill file**: Use view_text_file to read %s
2. **Create execution plan**: Based on the skill and any focus areas, create a specific plan
3. **Execute the plan**: Run the necessary kubectl commands systematically
4. **Analyze results**: Identify issues, patterns, and provide intelligent analysis
5. **Output report**: Generate a comprehensive Markdown report

Begin by reading your skill file and creating your execution plan.`, spec.Name, strings.Join(parts, ""), skillPath)
	return base
}

// ExtractFocusAreas extracts focus_areas from input.
func ExtractFocusAreas(input map[string]any) []string {
	if v, ok := input["focus_areas"]; ok {
		if arr, ok := v.([]string); ok {
			return arr
		}
		if arr, ok := v.([]any); ok {
			var out []string
			for _, a := range arr {
				if s, ok := a.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}

// ExtractOrchestratorContext extracts orchestrator_context from input.
func ExtractOrchestratorContext(input map[string]any) string {
	if v, ok := input["orchestrator_context"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func truncateStr(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
