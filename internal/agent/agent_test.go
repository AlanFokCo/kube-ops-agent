package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentpkg "github.com/alanfokco/agentscope-go/pkg/agentscope/agent"

	runtimepkg "github.com/alanfokco/kube-ops-agent-go/internal/runtime"
)

// ---- AgentCache ----

func TestNewAgentCache_Defaults(t *testing.T) {
	c := NewAgentCache(0, 0, 0)
	if c.maxSize != 10 {
		t.Errorf("expected default maxSize 10, got %d", c.maxSize)
	}
	if c.ttlSec != 3600 {
		t.Errorf("expected default ttlSec 3600, got %d", c.ttlSec)
	}
	if c.idleSec != 600 {
		t.Errorf("expected default idleSec 600, got %d", c.idleSec)
	}
}

func TestAgentCache_GetOrCreate_Miss(t *testing.T) {
	c := NewAgentCache(5, 3600, 600)
	called := false
	a, err := c.GetOrCreate("key1", func() (*agentpkg.ReActAgent, error) {
		called = true
		return nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected factory to be called on miss")
	}
	_ = a
	stats := c.Stats()
	if stats["misses"].(int) != 1 {
		t.Errorf("expected 1 miss, got %v", stats["misses"])
	}
}

func TestAgentCache_GetOrCreate_Hit(t *testing.T) {
	c := NewAgentCache(5, 3600, 600)
	// First call: miss
	c.GetOrCreate("key1", func() (*agentpkg.ReActAgent, error) { return nil, nil })
	// Second call: hit
	c.GetOrCreate("key1", func() (*agentpkg.ReActAgent, error) {
		t.Error("factory should not be called on hit")
		return nil, nil
	})
	stats := c.Stats()
	if stats["hits"].(int) != 1 {
		t.Errorf("expected 1 hit, got %v", stats["hits"])
	}
}

func TestAgentCache_GetOrCreate_FactoryError(t *testing.T) {
	c := NewAgentCache(5, 3600, 600)
	_, err := c.GetOrCreate("key1", func() (*agentpkg.ReActAgent, error) {
		return nil, errors.New("creation failed")
	})
	if err == nil {
		t.Error("expected error from factory")
	}
}

func TestAgentCache_GetOrCreate_TTLExpiry(t *testing.T) {
	c := NewAgentCache(5, 0, 600) // ttlSec effectively 0 (defaulted to 3600 since 0 triggers default)
	// Use a very short TTL by setting it directly
	c.ttlSec = 1 // 1 second TTL
	c.GetOrCreate("key1", func() (*agentpkg.ReActAgent, error) { return nil, nil })

	// Manually expire by backdating CreatedAt
	c.mu.Lock()
	c.cache["key1"].CreatedAt = time.Now().Add(-2 * time.Second)
	c.mu.Unlock()

	called := false
	c.GetOrCreate("key1", func() (*agentpkg.ReActAgent, error) {
		called = true
		return nil, nil
	})
	if !called {
		t.Error("expected factory to be called after TTL expiry")
	}
}

func TestAgentCache_Remove(t *testing.T) {
	c := NewAgentCache(5, 3600, 600)
	c.GetOrCreate("key1", func() (*agentpkg.ReActAgent, error) { return nil, nil })

	removed := c.Remove("key1")
	if !removed {
		t.Error("expected Remove to return true")
	}
	removed2 := c.Remove("key1")
	if removed2 {
		t.Error("expected Remove to return false for non-existent key")
	}
}

func TestAgentCache_Clear(t *testing.T) {
	c := NewAgentCache(5, 3600, 600)
	c.GetOrCreate("key1", func() (*agentpkg.ReActAgent, error) { return nil, nil })
	c.GetOrCreate("key2", func() (*agentpkg.ReActAgent, error) { return nil, nil })
	c.Clear()
	stats := c.Stats()
	if stats["size"].(int) != 0 {
		t.Errorf("expected size 0 after Clear, got %v", stats["size"])
	}
}

func TestAgentCache_CleanupIdle(t *testing.T) {
	c := NewAgentCache(5, 3600, 1) // idleSec=1
	c.GetOrCreate("key1", func() (*agentpkg.ReActAgent, error) { return nil, nil })
	c.GetOrCreate("key2", func() (*agentpkg.ReActAgent, error) { return nil, nil })

	// Manually set last used to past
	c.mu.Lock()
	c.cache["key1"].LastUsed = time.Now().Add(-2 * time.Second)
	c.mu.Unlock()

	cleaned := c.CleanupIdle()
	if cleaned != 1 {
		t.Errorf("expected 1 cleaned, got %d", cleaned)
	}
	stats := c.Stats()
	if stats["size"].(int) != 1 {
		t.Errorf("expected 1 remaining, got %v", stats["size"])
	}
}

func TestAgentCache_Eviction(t *testing.T) {
	c := NewAgentCache(2, 3600, 600) // maxSize=2
	c.GetOrCreate("key1", func() (*agentpkg.ReActAgent, error) { return nil, nil })
	c.GetOrCreate("key2", func() (*agentpkg.ReActAgent, error) { return nil, nil })
	c.GetOrCreate("key3", func() (*agentpkg.ReActAgent, error) { return nil, nil }) // evicts oldest
	stats := c.Stats()
	if stats["size"].(int) > 2 {
		t.Errorf("expected at most 2 in cache, got %v", stats["size"])
	}
	if stats["evictions"].(int) < 1 {
		t.Errorf("expected at least 1 eviction, got %v", stats["evictions"])
	}
}

func TestAgentCache_Stats_HitRate(t *testing.T) {
	c := NewAgentCache(5, 3600, 600)
	// 2 misses, 2 hits
	c.GetOrCreate("k1", func() (*agentpkg.ReActAgent, error) { return nil, nil })
	c.GetOrCreate("k2", func() (*agentpkg.ReActAgent, error) { return nil, nil })
	c.GetOrCreate("k1", func() (*agentpkg.ReActAgent, error) { return nil, nil })
	c.GetOrCreate("k2", func() (*agentpkg.ReActAgent, error) { return nil, nil })
	stats := c.Stats()
	hitRate := stats["hit_rate_percent"].(float64)
	if hitRate != 50.0 {
		t.Errorf("expected hit_rate_percent=50, got %f", hitRate)
	}
}

// ---- Trigger functions ----

func TestMakeTriggerMsg_Basic(t *testing.T) {
	spec := Spec{
		Name:     "NodeHealth",
		SkillDir: "/skills/node-health",
	}
	msg := MakeTriggerMsg(spec, nil, nil, "")
	if !strings.Contains(msg, "NodeHealth") {
		t.Error("expected agent name in trigger message")
	}
	if !strings.Contains(msg, "SKILL.md") {
		t.Error("expected SKILL.md reference in trigger message")
	}
}

func TestMakeTriggerMsg_WithOrchestratorContext(t *testing.T) {
	spec := Spec{Name: "Test", SkillDir: "/skills/test"}
	msg := MakeTriggerMsg(spec, nil, nil, "cluster is under load")
	if !strings.Contains(msg, "Orchestrator Assessment") {
		t.Error("expected Orchestrator Assessment section")
	}
	if !strings.Contains(msg, "cluster is under load") {
		t.Error("expected orchestrator context in message")
	}
}

func TestMakeTriggerMsg_WithFocusAreas(t *testing.T) {
	spec := Spec{Name: "Test", SkillDir: "/skills/test"}
	msg := MakeTriggerMsg(spec, nil, []string{"nodes", "pods"}, "")
	if !strings.Contains(msg, "Focus Areas") {
		t.Error("expected Focus Areas section")
	}
	if !strings.Contains(msg, "- nodes") {
		t.Error("expected 'nodes' in focus areas")
	}
}

func TestMakeTriggerMsg_WithInputData(t *testing.T) {
	spec := Spec{Name: "Test", SkillDir: "/skills/test"}
	input := map[string]any{
		"trigger":              "manual",
		"orchestrator_context": "skip",
		"focus_areas":          "skip",
	}
	msg := MakeTriggerMsg(spec, input, nil, "")
	// orchestrator_context and focus_areas should be filtered from context section
	if strings.Contains(msg, "Context from orchestrator_context") {
		t.Error("orchestrator_context should not appear in context section")
	}
}

func TestMakeTriggerMsg_WithInputData_NonEmpty(t *testing.T) {
	spec := Spec{Name: "Test", SkillDir: "/skills/test"}
	input := map[string]any{
		"ClusterOverview": "all nodes healthy",
	}
	msg := MakeTriggerMsg(spec, input, nil, "")
	if !strings.Contains(msg, "Previous Stage Context") {
		t.Error("expected Previous Stage Context section for non-empty input")
	}
}

func TestExtractFocusAreas_StringSlice(t *testing.T) {
	input := map[string]any{
		"focus_areas": []string{"nodes", "pods"},
	}
	areas := ExtractFocusAreas(input)
	if len(areas) != 2 {
		t.Errorf("expected 2 focus areas, got %d", len(areas))
	}
}

func TestExtractFocusAreas_AnySlice(t *testing.T) {
	input := map[string]any{
		"focus_areas": []any{"nodes", "pods", "storage"},
	}
	areas := ExtractFocusAreas(input)
	if len(areas) != 3 {
		t.Errorf("expected 3 focus areas, got %d", len(areas))
	}
}

func TestExtractFocusAreas_Empty(t *testing.T) {
	areas := ExtractFocusAreas(nil)
	if areas != nil {
		t.Errorf("expected nil for nil input, got %v", areas)
	}
	areas2 := ExtractFocusAreas(map[string]any{})
	if areas2 != nil {
		t.Errorf("expected nil for empty input, got %v", areas2)
	}
}

func TestExtractOrchestratorContext(t *testing.T) {
	input := map[string]any{
		"orchestrator_context": "cluster healthy",
	}
	ctx := ExtractOrchestratorContext(input)
	if ctx != "cluster healthy" {
		t.Errorf("expected 'cluster healthy', got %q", ctx)
	}
}

func TestExtractOrchestratorContext_Empty(t *testing.T) {
	ctx := ExtractOrchestratorContext(nil)
	if ctx != "" {
		t.Errorf("expected empty string, got %q", ctx)
	}
	ctx2 := ExtractOrchestratorContext(map[string]any{"other": "val"})
	if ctx2 != "" {
		t.Errorf("expected empty string, got %q", ctx2)
	}
}

func TestTruncateStr_BelowMax(t *testing.T) {
	s := "hello"
	got := truncateStr(s, 10)
	if got != s {
		t.Errorf("expected %q, got %q", s, got)
	}
}

func TestTruncateStr_AtMax(t *testing.T) {
	s := "hello"
	got := truncateStr(s, 5)
	if got != s {
		t.Errorf("expected %q, got %q", s, got)
	}
}

func TestTruncateStr_AboveMax(t *testing.T) {
	s := "hello world"
	got := truncateStr(s, 5)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
	if len([]rune(got)) != 8 { // 5 + len("...") = 8
		t.Errorf("expected 8 runes, got %d in %q", len([]rune(got)), got)
	}
}

func TestTruncateStr_UTF8(t *testing.T) {
	s := "你好世界abc"
	got := truncateStr(s, 3)
	if got != "你好世..." {
		t.Errorf("expected UTF-8 truncation, got %q", got)
	}
}

// ---- Registry ----

func TestSpecToMetaMap(t *testing.T) {
	sp := Spec{
		Name:           "TestAgent",
		Description:    "test description",
		ReportSection:  "Section A",
		IntervalSecond: 300,
	}
	m := SpecToMetaMap(sp)
	if m["name"] != "TestAgent" {
		t.Errorf("name = %v", m["name"])
	}
	if m["description"] != "test description" {
		t.Errorf("description = %v", m["description"])
	}
	if m["focus_area"] != "Section A" {
		t.Errorf("focus_area = %v", m["focus_area"])
	}
	if m["interval"] != 300 {
		t.Errorf("interval = %v", m["interval"])
	}
}

func TestGetSummarySkillDir_Found_SKILL_MD(t *testing.T) {
	dir := t.TempDir()
	summaryDir := filepath.Join(dir, "summary")
	os.MkdirAll(summaryDir, 0755)
	os.WriteFile(filepath.Join(summaryDir, "SKILL.md"), []byte("content"), 0644)

	got := GetSummarySkillDir(dir)
	if got != summaryDir {
		t.Errorf("expected %q, got %q", summaryDir, got)
	}
}

func TestGetSummarySkillDir_Found_AgentYAML(t *testing.T) {
	dir := t.TempDir()
	summaryDir := filepath.Join(dir, "summary")
	os.MkdirAll(summaryDir, 0755)
	os.WriteFile(filepath.Join(summaryDir, "agent.yaml"), []byte("name: summary"), 0644)

	got := GetSummarySkillDir(dir)
	if got != summaryDir {
		t.Errorf("expected %q, got %q", summaryDir, got)
	}
}

func TestGetSummarySkillDir_NotFound(t *testing.T) {
	dir := t.TempDir()
	got := GetSummarySkillDir(dir)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestNewFileRegistry_NotDir(t *testing.T) {
	f, err := os.CreateTemp("", "notadir*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	_, err = NewFileRegistry(f.Name())
	if err == nil {
		t.Error("expected error for non-directory path")
	}
}

func TestNewFileRegistry_NotExist(t *testing.T) {
	_, err := NewFileRegistry("/nonexistent/path/xyz")
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestNewFileRegistry_AgentYAML(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "my-agent")
	os.MkdirAll(agentDir, 0755)
	yaml := `name: MyAgent
description: Test agent
interval_seconds: 300
report_section: "My Section"
`
	os.WriteFile(filepath.Join(agentDir, "agent.yaml"), []byte(yaml), 0644)

	reg, err := NewFileRegistry(dir)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	specs := reg.Specs()
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	sp := specs[0]
	if sp.Name != "MyAgent" {
		t.Errorf("Name = %q", sp.Name)
	}
	if sp.Description != "Test agent" {
		t.Errorf("Description = %q", sp.Description)
	}
	if sp.IntervalSecond != 300 {
		t.Errorf("IntervalSecond = %d", sp.IntervalSecond)
	}
	if sp.ReportSection != "My Section" {
		t.Errorf("ReportSection = %q", sp.ReportSection)
	}
	if !sp.ReadOnly {
		t.Error("expected ReadOnly=true by default")
	}
}

func TestNewFileRegistry_SKILL_MD_Frontmatter(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "skill-agent")
	os.MkdirAll(agentDir, 0755)
	skillMD := `---
agent_name: SkillAgent
description: Skill-based agent
interval_seconds: 60
report_section: "Skill Section"
read_only: false
---

# SkillAgent

This agent inspects things.
`
	os.WriteFile(filepath.Join(agentDir, "SKILL.md"), []byte(skillMD), 0644)

	reg, err := NewFileRegistry(dir)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	specs := reg.Specs()
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	sp := specs[0]
	if sp.Name != "SkillAgent" {
		t.Errorf("Name = %q", sp.Name)
	}
	if sp.ReadOnly {
		t.Error("expected ReadOnly=false from frontmatter")
	}
}

func TestNewFileRegistry_SKILL_MD_AgentYAMLPrecedence(t *testing.T) {
	// When both agent.yaml and SKILL.md exist, agent.yaml takes precedence
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "agent-dir")
	os.MkdirAll(agentDir, 0755)

	os.WriteFile(filepath.Join(agentDir, "agent.yaml"), []byte("name: FromYAML\n"), 0644)
	os.WriteFile(filepath.Join(agentDir, "SKILL.md"), []byte("---\nagent_name: FromSKILL\n---\n# skill"), 0644)

	reg, err := NewFileRegistry(dir)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	specs := reg.Specs()
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].Name != "FromYAML" {
		t.Errorf("expected agent.yaml to take precedence, got %q", specs[0].Name)
	}
}

func TestNewFileRegistry_WithIntervalPositiveOnly(t *testing.T) {
	dir := t.TempDir()
	a1 := filepath.Join(dir, "a1")
	a2 := filepath.Join(dir, "a2")
	os.MkdirAll(a1, 0755)
	os.MkdirAll(a2, 0755)
	os.WriteFile(filepath.Join(a1, "agent.yaml"), []byte("name: AgentWithInterval\ninterval_seconds: 300\n"), 0644)
	os.WriteFile(filepath.Join(a2, "agent.yaml"), []byte("name: AgentNoInterval\ninterval_seconds: 0\n"), 0644)

	reg, err := NewFileRegistry(dir, WithIntervalPositiveOnly())
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	specs := reg.Specs()
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec (with positive interval), got %d", len(specs))
	}
	if specs[0].Name != "AgentWithInterval" {
		t.Errorf("expected AgentWithInterval, got %q", specs[0].Name)
	}
}

func TestNewFileRegistry_SpecByName(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "my-agent")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "agent.yaml"), []byte("name: TestAgent\n"), 0644)

	reg, err := NewFileRegistry(dir)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	sp, ok := reg.SpecByName("TestAgent")
	if !ok {
		t.Fatal("expected to find TestAgent")
	}
	if sp.Name != "TestAgent" {
		t.Errorf("Name = %q", sp.Name)
	}
	_, ok2 := reg.SpecByName("NonExistent")
	if ok2 {
		t.Error("expected not found for NonExistent")
	}
}

func TestNewFileRegistry_SubSkillsAutoDiscovery(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "my-agent")
	subDir := filepath.Join(agentDir, "sub-skills")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "agent.yaml"), []byte("name: AutoSubAgent\n"), 0644)
	os.WriteFile(filepath.Join(subDir, "detail-a.md"), []byte("# Detail A"), 0644)
	os.WriteFile(filepath.Join(subDir, "detail-b.md"), []byte("# Detail B"), 0644)

	reg, err := NewFileRegistry(dir)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	sp, ok := reg.SpecByName("AutoSubAgent")
	if !ok {
		t.Fatal("expected AutoSubAgent")
	}
	if len(sp.SubSkills) != 2 {
		t.Errorf("expected 2 auto-discovered sub-skills, got %d", len(sp.SubSkills))
	}
}

func TestNewFileRegistry_ReportSectionDefault(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "my-agent")
	os.MkdirAll(agentDir, 0755)
	// No report_section in YAML
	os.WriteFile(filepath.Join(agentDir, "agent.yaml"), []byte("name: SectionAgent\n"), 0644)

	reg, err := NewFileRegistry(dir)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	sp, ok := reg.SpecByName("SectionAgent")
	if !ok {
		t.Fatal("expected SectionAgent")
	}
	// Should default to dir name
	if sp.ReportSection != "my-agent" {
		t.Errorf("expected report_section to default to dir name 'my-agent', got %q", sp.ReportSection)
	}
}

func TestNewFileRegistry_EmptyName_Skipped(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "empty-name")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "agent.yaml"), []byte("description: no name\n"), 0644)

	reg, err := NewFileRegistry(dir)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	specs := reg.Specs()
	if len(specs) != 0 {
		t.Errorf("expected 0 specs for agent with empty name, got %d", len(specs))
	}
}

func TestNewFileRegistry_ReadOnlyFromYAML(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "rw-agent")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "agent.yaml"), []byte("name: RWAgent\nread_only: false\n"), 0644)

	reg, err := NewFileRegistry(dir)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	sp, ok := reg.SpecByName("RWAgent")
	if !ok {
		t.Fatal("expected RWAgent")
	}
	if sp.ReadOnly {
		t.Error("expected ReadOnly=false")
	}
}

func TestParseSKILLMDFrontmatter_NoFrontmatter(t *testing.T) {
	content := []byte("# Just content\n\nNo frontmatter here.")
	m, err := parseSKILLMDFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Error("expected nil map for content without frontmatter")
	}
}

func TestParseSKILLMDFrontmatter_Valid(t *testing.T) {
	content := []byte("---\nname: test\ndescription: desc\n---\n# Body")
	m, err := parseSKILLMDFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil map")
	}
	if m["name"] != "test" {
		t.Errorf("name = %v", m["name"])
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if firstNonEmpty("", "b", "c") != "b" {
		t.Error("expected first non-empty 'b'")
	}
	if firstNonEmpty("", "", "") != "" {
		t.Error("expected empty string when all empty")
	}
	if firstNonEmpty("a", "b") != "a" {
		t.Error("expected 'a'")
	}
}

func TestNewFileRegistry_SKILL_MD_NameFallback(t *testing.T) {
	// Test fallback from agent_name to name field
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "skill-fallback")
	os.MkdirAll(agentDir, 0755)
	// Only 'name', no 'agent_name'
	skillMD := "---\nname: FallbackAgent\ndescription: fallback\n---\n# Agent"
	os.WriteFile(filepath.Join(agentDir, "SKILL.md"), []byte(skillMD), 0644)

	reg, err := NewFileRegistry(dir)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	sp, ok := reg.SpecByName("FallbackAgent")
	if !ok {
		t.Fatal("expected FallbackAgent from name fallback")
	}
	_ = sp
}

func TestNewFileRegistry_SKILL_MD_SubSkillsInFrontmatter(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "sub-agent")
	os.MkdirAll(agentDir, 0755)
	skillMD := `---
agent_name: SubAgent
sub_skills:
  - sub-skills/detail.md
---
# SubAgent`
	os.WriteFile(filepath.Join(agentDir, "SKILL.md"), []byte(skillMD), 0644)

	reg, err := NewFileRegistry(dir)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	sp, ok := reg.SpecByName("SubAgent")
	if !ok {
		t.Fatal("expected SubAgent")
	}
	if len(sp.SubSkills) != 1 {
		t.Errorf("expected 1 sub-skill from frontmatter, got %d", len(sp.SubSkills))
	}
}

func TestDiscoverSubSkills_NoDir(t *testing.T) {
	result := discoverSubSkills("/nonexistent/skills/dir")
	if result != nil {
		t.Error("expected nil for non-existent dir")
	}
}

func TestDiscoverSubSkills_Empty(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub-skills")
	os.MkdirAll(subDir, 0755)
	result := discoverSubSkills(dir)
	if len(result) != 0 {
		t.Errorf("expected 0 sub-skills for empty dir, got %d", len(result))
	}
}

func TestAgentCache_Stats_ZeroHitRate(t *testing.T) {
	c := NewAgentCache(5, 3600, 600)
	stats := c.Stats()
	if stats["hit_rate_percent"].(float64) != 0 {
		t.Errorf("expected 0 hit rate for empty cache, got %f", stats["hit_rate_percent"])
	}
}

// ---- AgentPool ----

func TestNewAgentPool(t *testing.T) {
	p := NewAgentPool()
	if p == nil {
		t.Fatal("expected non-nil pool")
	}
}

func TestAgentPool_RegisterAndGet(t *testing.T) {
	p := NewAgentPool()
	p.Register("agent1", nil) // nil agent for testing
	a, ok := p.Get("agent1")
	if !ok {
		t.Error("expected to find agent1")
	}
	_ = a

	_, ok2 := p.Get("nonexistent")
	if ok2 {
		t.Error("expected not found for nonexistent agent")
	}
}

func TestAgentPool_Execute_NotFound(t *testing.T) {
	p := NewAgentPool()
	results := p.Execute(context.Background(), []string{"nonexistent"}, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for unregistered agent, got %d", len(results))
	}
}

// ---- DefaultThinkingConfig ----

func TestDefaultThinkingConfig(t *testing.T) {
	cfg := DefaultThinkingConfig()
	if cfg.MaxDepth != 3 {
		t.Errorf("MaxDepth = %d", cfg.MaxDepth)
	}
	if cfg.MaxIterations != 8 {
		t.Errorf("MaxIterations = %d", cfg.MaxIterations)
	}
	if cfg.PlanningThreshold != 0.5 {
		t.Errorf("PlanningThreshold = %f", cfg.PlanningThreshold)
	}
	if !cfg.ReadOnly {
		t.Error("expected ReadOnly=true")
	}
	if len(cfg.Constraints) == 0 {
		t.Error("expected non-empty Constraints")
	}
}

// ---- isNonRetryableError ----

func TestIsNonRetryableError_Nil(t *testing.T) {
	if isNonRetryableError(nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsNonRetryableError_Canceled(t *testing.T) {
	if !isNonRetryableError(context.Canceled) {
		t.Error("expected true for context.Canceled")
	}
}

func TestIsNonRetryableError_DeadlineExceeded(t *testing.T) {
	if !isNonRetryableError(context.DeadlineExceeded) {
		t.Error("expected true for context.DeadlineExceeded")
	}
}

func TestIsNonRetryableError_AgentNotFound(t *testing.T) {
	err := errors.New("agent not found: test")
	if !isNonRetryableError(err) {
		t.Error("expected true for 'agent not found' error")
	}
}

func TestIsNonRetryableError_CircuitOpen(t *testing.T) {
	err := errors.New("circuit open for agent1")
	if !isNonRetryableError(err) {
		t.Error("expected true for 'circuit open' error")
	}
}

func TestIsNonRetryableError_RegularError(t *testing.T) {
	err := errors.New("some transient error")
	if isNonRetryableError(err) {
		t.Error("expected false for regular transient error")
	}
}

// ---- sleepWithContext ----

func TestSleepWithContext_Completes(t *testing.T) {
	ctx := context.Background()
	err := sleepWithContext(ctx, 1*time.Millisecond)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSleepWithContext_Canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sleepWithContext(ctx, 10*time.Second)
	if err == nil {
		t.Error("expected context error")
	}
}

// ---- mustJSON ----

func TestMustJSON_Valid(t *testing.T) {
	v := map[string]any{"key": "value", "num": 42}
	result := mustJSON(v)
	if !strings.Contains(result, "key") {
		t.Errorf("expected JSON output, got %q", result)
	}
}

func TestMustJSON_Cycle(t *testing.T) {
	// Can't marshal a channel, should return {}
	result := mustJSON(make(chan int))
	if result != "{}" {
		t.Errorf("expected '{}' for unmarshalable value, got %q", result)
	}
}

// ---- LoadAgentDSL ----

func TestLoadAgentDSL_AgentYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: TestDSL
description: DSL test agent
interval_seconds: 120
report_section: "Test Section"
`
	os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte(yaml), 0644)

	dsl, err := LoadAgentDSL(dir)
	if err != nil {
		t.Fatalf("LoadAgentDSL: %v", err)
	}
	if dsl.Name != "TestDSL" {
		t.Errorf("Name = %q", dsl.Name)
	}
	if dsl.IntervalSecond != 120 {
		t.Errorf("IntervalSecond = %d", dsl.IntervalSecond)
	}
}

func TestLoadAgentDSL_SKILL_MD(t *testing.T) {
	dir := t.TempDir()
	content := `---
agent_name: SKILLDSLAgent
description: Skill DSL agent
interval_seconds: 60
report_section: SKILL Section
read_only: false
sub_skills:
  - sub-skills/detail.md
---
# Agent content
`
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)

	dsl, err := LoadAgentDSL(dir)
	if err != nil {
		t.Fatalf("LoadAgentDSL from SKILL.md: %v", err)
	}
	if dsl.Name != "SKILLDSLAgent" {
		t.Errorf("Name = %q", dsl.Name)
	}
	if dsl.IntervalSecond != 60 {
		t.Errorf("IntervalSecond = %d", dsl.IntervalSecond)
	}
	if dsl.ReadOnly == nil || *dsl.ReadOnly {
		t.Error("expected ReadOnly=false")
	}
	if len(dsl.SubSkills) != 1 {
		t.Errorf("expected 1 sub-skill, got %d", len(dsl.SubSkills))
	}
}

func TestLoadAgentDSL_NotFound(t *testing.T) {
	_, err := LoadAgentDSL("/nonexistent/skill/dir")
	if err == nil {
		t.Error("expected error for missing dir")
	}
}

func TestLoadAgentDSL_NoName(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\ndescription: no name\n---\n# content"), 0644)
	_, err := LoadAgentDSL(dir)
	if err == nil {
		t.Error("expected error when no name")
	}
}

// ---- RegisterAgentSkill ----

func TestRegisterAgentSkill(t *testing.T) {
	dir := t.TempDir()
	content := "# My Skill\n\nThis is the skill content."
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)

	skill, err := RegisterAgentSkill(dir)
	if err != nil {
		t.Fatalf("RegisterAgentSkill: %v", err)
	}
	if !strings.Contains(skill, "My Skill") {
		t.Errorf("expected skill content, got %q", skill)
	}
}

func TestRegisterAgentSkill_LowercaseFallback(t *testing.T) {
	dir := t.TempDir()
	content := "# Lowercase Skill"
	os.WriteFile(filepath.Join(dir, "skill.md"), []byte(content), 0644)

	skill, err := RegisterAgentSkill(dir)
	if err != nil {
		t.Fatalf("RegisterAgentSkill lowercase: %v", err)
	}
	if !strings.Contains(skill, "Lowercase Skill") {
		t.Errorf("expected skill content, got %q", skill)
	}
}

func TestRegisterAgentSkill_NotFound(t *testing.T) {
	_, err := RegisterAgentSkill("/nonexistent/dir")
	if err == nil {
		t.Error("expected error for missing skill")
	}
}

// ---- buildSysPrompt ----

func TestBuildSysPrompt_CustomPrompt(t *testing.T) {
	dsl := &AgentDSL{
		Name:        "TestAgent",
		Description: "Test",
		SysPrompt:   "Custom system prompt",
		SubSkills:   []string{"sub-skills/detail.md"},
	}
	prompt := buildSysPrompt(dsl, "/skills/test")
	if !strings.Contains(prompt, "Custom system prompt") {
		t.Error("expected custom prompt in output")
	}
	if !strings.Contains(prompt, "Security Constraints") {
		t.Error("expected Security Constraints in output")
	}
	if !strings.Contains(prompt, "sub-skills/detail.md") {
		t.Error("expected sub-skills reference in output")
	}
}

func TestBuildSysPrompt_ReadOnlyFalse(t *testing.T) {
	readOnly := false
	dsl := &AgentDSL{
		Name:        "RWAgent",
		Description: "RW",
		SysPrompt:   "Custom",
		ReadOnly:    &readOnly,
	}
	prompt := buildSysPrompt(dsl, "/skills")
	if !strings.Contains(prompt, "Read-write mode") {
		t.Error("expected read-write mode in prompt")
	}
}

func TestBuildSysPrompt_PlanningMode(t *testing.T) {
	dsl := &AgentDSL{
		Name:        "PlanAgent",
		Description: "Inspects cluster",
	}
	prompt := buildSysPrompt(dsl, "/skills/plan-agent")
	if !strings.Contains(prompt, "Inspects cluster") {
		t.Error("expected description in planning prompt")
	}
	if !strings.Contains(prompt, "SKILL.md") {
		t.Error("expected SKILL.md reference")
	}
}

// ---- buildChatSysPrompt / buildChatMessages ----

func TestBuildChatSysPrompt_NoSpecs(t *testing.T) {
	reg := &mockRegistryForExec{specs: nil}
	prompt := buildChatSysPrompt(reg)
	if !strings.Contains(prompt, "K8s cluster") {
		t.Errorf("expected cluster reference in prompt, got %q", prompt)
	}
	if strings.Contains(prompt, "Available Skills") {
		t.Error("should not have skills section for empty registry")
	}
}

func TestBuildChatSysPrompt_WithSpecs(t *testing.T) {
	reg := &mockRegistryForExec{
		specs: []Spec{
			{Name: "NodeHealth", SkillDir: "/skills/node-health"},
		},
	}
	prompt := buildChatSysPrompt(reg)
	if !strings.Contains(prompt, "Available Skills") {
		t.Error("expected skills section for non-empty registry")
	}
	if !strings.Contains(prompt, "node-health") {
		t.Error("expected skill dir in prompt")
	}
}

func TestBuildChatMessages(t *testing.T) {
	reg := &mockRegistryForExec{specs: nil}
	msgs := buildChatMessages(reg, "check cluster health")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

type mockRegistryForExec struct {
	specs []Spec
}

func (r *mockRegistryForExec) Specs() []Spec                       { return r.specs }
func (r *mockRegistryForExec) SpecByName(name string) (Spec, bool) { return Spec{}, false }

// ---- newKubectlTool ----

func TestNewKubectlTool(t *testing.T) {
	tool := newKubectlTool(nil)
	if tool == nil {
		t.Fatal("expected non-nil kubectl tool")
	}
}

func TestNewWorkerToolkit(t *testing.T) {
	tk := newWorkerToolkit(nil)
	if tk == nil {
		t.Fatal("expected non-nil toolkit")
	}
}

// ---- Executor struct creation ----

func TestNewExecutor(t *testing.T) {
	env := runtimepkg.NewEnvironment(nil)
	reg := &mockRegistryForExec{specs: nil}
	e := NewExecutor(reg, env, nil)
	if e == nil {
		t.Fatal("expected non-nil executor")
	}
}

func TestExecutor_Execute_NilModel(t *testing.T) {
	env := runtimepkg.NewEnvironment(nil)
	reg := &mockRegistryForExec{
		specs: []Spec{
			{Name: "TestAgent", SkillDir: "/tmp/test-skill"},
		},
	}
	e := NewExecutor(reg, env, nil)
	ctx := context.Background()
	_, err := e.Execute(ctx, "TestAgent", nil)
	if err == nil {
		t.Error("expected error when model is nil")
	}
}

func TestExecutor_Execute_AgentNotFound(t *testing.T) {
	env := runtimepkg.NewEnvironment(nil)
	reg := &mockRegistryForExec{specs: nil}
	e := NewExecutor(reg, env, nil)
	ctx := context.Background()
	_, err := e.Execute(ctx, "NonExistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent agent or nil model")
	}
}

func TestExecutor_ExecuteChat_NilModel(t *testing.T) {
	env := runtimepkg.NewEnvironment(nil)
	reg := &mockRegistryForExec{specs: nil}
	e := NewExecutor(reg, env, nil)
	_, err := e.ExecuteChat(context.Background(), "test question")
	if err == nil {
		t.Error("expected error when model is nil")
	}
}

func TestExecutor_ExecuteChatStream_NilModel(t *testing.T) {
	env := runtimepkg.NewEnvironment(nil)
	reg := &mockRegistryForExec{specs: nil}
	e := NewExecutor(reg, env, nil)
	_, err := e.ExecuteChatStream(context.Background(), "test", nil)
	if err == nil {
		t.Error("expected error when model is nil")
	}
}

// ---- ThinkingAgent ----

func TestNewThinkingAgent_NilModel(t *testing.T) {
	cfg := DefaultThinkingConfig()
	ta := NewThinkingAgent("test", nil, nil, cfg)
	if ta == nil {
		t.Fatal("expected non-nil ThinkingAgent")
	}
	if ta.Name != "test" {
		t.Errorf("expected name 'test', got %q", ta.Name)
	}
}

func TestNewThinkingAgent_ZeroIterations(t *testing.T) {
	// MaxIterations=0 should use defaults
	cfg := ThinkingConfig{MaxIterations: 0}
	ta := NewThinkingAgent("test", nil, nil, cfg)
	if ta.Config.MaxIterations != 8 {
		t.Errorf("expected default MaxIterations=8, got %d", ta.Config.MaxIterations)
	}
}

// ---- SummaryAgent ----

func TestNewSummaryAgent(t *testing.T) {
	sa := NewSummaryAgent(nil)
	if sa == nil {
		t.Fatal("expected non-nil SummaryAgent")
	}
}

func TestSummaryAgent_Summarize_NilModel(t *testing.T) {
	sa := NewSummaryAgent(nil)
	_, err := sa.Summarize(context.Background(), nil, nil)
	if err == nil {
		t.Error("expected error when model is nil")
	}
}

// ---- OrchestratorAgent ----

func TestNewOrchestratorAgent(t *testing.T) {
	oa := NewOrchestratorAgent(nil)
	if oa == nil {
		t.Fatal("expected non-nil OrchestratorAgent")
	}
}

func TestOrchestratorAgent_Plan_NilModel(t *testing.T) {
	oa := NewOrchestratorAgent(nil)
	_, err := oa.Plan(context.Background(), nil, "")
	if err == nil {
		t.Error("expected error when model is nil")
	}
}

// ---- SelfDrivenOrchestrator ----

func TestNewSelfDrivenOrchestrator(t *testing.T) {
	reg := &mockRegistryForExec{specs: nil}
	sdo := NewSelfDrivenOrchestrator(nil, reg)
	if sdo == nil {
		t.Fatal("expected non-nil SelfDrivenOrchestrator")
	}
}

// ---- SelfDrivenSummary ----

func TestNewSelfDrivenSummary(t *testing.T) {
	sds := NewSelfDrivenSummary(nil)
	if sds == nil {
		t.Fatal("expected non-nil SelfDrivenSummary")
	}
}

// ---- SelfDrivenWorker ----

func TestNewSelfDrivenWorker(t *testing.T) {
	spec := Spec{Name: "TestWorker"}
	sdw := NewSelfDrivenWorker(nil, spec)
	if sdw == nil {
		t.Fatal("expected non-nil SelfDrivenWorker")
	}
}

// ---- BuildChatAgent ----

func TestBuildChatAgent_NilToolkit(t *testing.T) {
	a := BuildChatAgent("test", "sys prompt", nil, nil)
	if a == nil {
		t.Fatal("expected non-nil ReActAgent")
	}
}

// ---- NewAgentPoolFromRegistry ----

func TestNewAgentPoolFromRegistry_EmptyRegistry(t *testing.T) {
	reg := &mockRegistryForExec{specs: nil}
	env := runtimepkg.NewEnvironment(nil)
	pool := NewAgentPoolFromRegistry(reg, nil, env)
	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
}

// ---- NewDefaultChatModel / NewChatModelWithOverride error cases ----

func TestNewDefaultChatModel_NoAPIKey(t *testing.T) {
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("AZURE_OPENAI_API_KEY")
	// Should return error (no API key configured)
	_, err := NewDefaultChatModel()
	// May succeed or fail depending on environment - just shouldn't panic
	_ = err
}

func TestNewChatModelWithOverride_EmptyModel(t *testing.T) {
	_, err := NewChatModelWithOverride("")
	// May fail due to no API key or empty model - just shouldn't panic
	_ = err
}

// ---- buildToolkit ----
func TestBuildToolkit_NilDSL(t *testing.T) {
	env := runtimepkg.NewEnvironment(nil)
	// buildToolkit requires non-nil dsl (it accesses dsl.Toolkit)
	dsl := &AgentDSL{Name: "test"}
	tk := buildToolkit(dsl, "/tmp", env, nil)
	if tk == nil {
		t.Fatal("expected non-nil toolkit")
	}
}

func TestBuildToolkit_WithDSL(t *testing.T) {
	dsl := &AgentDSL{
		Name: "test",
		Toolkit: &ToolkitConfig{
			UseBuiltins: true,
			UseMCP:      false,
		},
	}
	env := runtimepkg.NewEnvironment(nil)
	tk := buildToolkit(dsl, "/tmp", env, nil)
	if tk == nil {
		t.Fatal("expected non-nil toolkit")
	}
}

func TestBuildToolkit_NoBuiltins(t *testing.T) {
	dsl := &AgentDSL{
		Name: "test",
		Toolkit: &ToolkitConfig{
			UseBuiltins: false,
		},
	}
	tk := buildToolkit(dsl, "/tmp", nil, nil)
	if tk == nil {
		t.Fatal("expected non-nil toolkit")
	}
}

// ---- newWorkerReActAgent ----
func TestNewWorkerReActAgent(t *testing.T) {
	env := runtimepkg.NewEnvironment(nil)
	spec := Spec{
		Name:        "TestAgent",
		SkillDir:    "/tmp/test-skill",
		Description: "Test agent",
	}
	a := newWorkerReActAgent(spec, nil, env)
	if a == nil {
		t.Fatal("expected non-nil ReActAgent")
	}
}

func TestNewWorkerReActAgent_WithSubSkills(t *testing.T) {
	env := runtimepkg.NewEnvironment(nil)
	spec := Spec{
		Name:        "TestAgent",
		SkillDir:    "/tmp/test-skill",
		Description: "Test agent",
		SubSkills:   []string{"sub-skill-1", "sub-skill-2"},
	}
	a := newWorkerReActAgent(spec, nil, env)
	if a == nil {
		t.Fatal("expected non-nil ReActAgent")
	}
}

// ---- newSaveReportTool / newRegisterAgentSkillTool ----
func TestNewSaveReportTool_NoContent(t *testing.T) {
	tool := newSaveReportTool("/tmp")
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	// Test with missing content arg
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error when content is missing")
	}
}

func TestNewSaveReportTool_InvalidContent(t *testing.T) {
	tool := newSaveReportTool("/tmp")
	_, err := tool.Execute(context.Background(), map[string]any{"content": 42})
	if err == nil {
		t.Error("expected error for non-string content")
	}
}

func TestNewSaveReportTool_EmptyReportDir(t *testing.T) {
	tool := newSaveReportTool("")
	_, err := tool.Execute(context.Background(), map[string]any{"content": "# Report"})
	if err == nil {
		t.Error("expected error when reportDir is empty")
	}
}

func TestNewSaveReportTool_ValidDir(t *testing.T) {
	dir := t.TempDir()
	tool := newSaveReportTool(dir)
	result, err := tool.Execute(context.Background(), map[string]any{"content": "# Test Report"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestNewRegisterAgentSkillTool_NoDir(t *testing.T) {
	tool := newRegisterAgentSkillTool("/nonexistent")
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	// Execute - will try to load SKILL.md from /nonexistent, likely fail
	_, err := tool.Execute(context.Background(), nil)
	// Error is expected since directory doesn't exist
	_ = err
}

func TestNewRegisterAgentSkillTool_ValidDir(t *testing.T) {
	dir := t.TempDir()
	// Create a SKILL.md
	os.WriteFile(dir+"/SKILL.md", []byte("# Test Skill\n\nContent"), 0644)
	tool := newRegisterAgentSkillTool(dir)
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Logf("skill tool error (may be expected): %v", err)
	}
	_ = result
}

// ---- resolveModel (nil model expected when no API keys) ----
func TestResolveModel_NilDSL(t *testing.T) {
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("DASHSCOPE_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	dsl := &AgentDSL{Name: "test"}
	m := resolveModel(dsl)
	// Returns nil when no API key or falls through to NewDefaultChatModel
	_ = m
}

func TestResolveModel_WithAPIKey(t *testing.T) {
	dsl := &AgentDSL{
		Name: "test",
		Model: &ModelConfig{
			APIKey: "test-key",
			Model:  "gpt-4",
		},
	}
	// May fail to create model without valid key - just test it doesn't panic
	m := resolveModel(dsl)
	_ = m
}

// ---- buildSysPrompt ----
func TestBuildSysPrompt_Basic(t *testing.T) {
	dsl := &AgentDSL{
		Name:        "TestAgent",
		Description: "Checks node health",
	}
	prompt := buildSysPrompt(dsl, "/tmp/skills/node-health")
	if len(prompt) == 0 {
		t.Error("expected non-empty sys prompt")
	}
}

func TestBuildSysPrompt_ReadWrite(t *testing.T) {
	readWrite := false
	dsl := &AgentDSL{
		Name:      "TestAgent",
		ReadOnly:  &readWrite,
		SysPrompt: "Custom prompt for testing read-write mode",
	}
	prompt := buildSysPrompt(dsl, "/tmp/skills/test")
	if !strings.Contains(prompt, "Read-write") {
		t.Error("expected 'Read-write' in non-read-only prompt")
	}
}

// ---- AgentPool ----
func TestAgentPool_Execute_EmptyPool(t *testing.T) {
	env := runtimepkg.NewEnvironment(nil)
	reg := &mockRegistryForExec{specs: nil}
	pool := NewAgentPoolFromRegistry(reg, nil, env)
	ctx := context.Background()
	// Execute with empty pool - returns empty map
	results := pool.Execute(ctx, []string{"Agent1"}, nil)
	_ = results
}

// ---- pool.BuildChatAgentFromSkillDir ----
func TestBuildChatAgentFromSkillDir_NoDir(t *testing.T) {
	// With non-existent skill dir, should return error
	_, err := BuildChatAgentFromSkillDir("agent1", "/nonexistent/skill", nil, nil)
	if err == nil {
		t.Error("expected error for non-existent skill dir")
	}
}

// ---- BuildSummaryAgentFromSkillDir ----
func TestBuildSummaryAgentFromSkillDir_NoDir(t *testing.T) {
	_, err := BuildSummaryAgentFromSkillDir("/nonexistent/skill", "/tmp", nil)
	if err == nil {
		t.Error("expected error for non-existent skill dir")
	}
}
