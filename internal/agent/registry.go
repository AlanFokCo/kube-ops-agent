package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// SpecToMetaMap converts Spec to a map for JSON/LLM context (name, description, focus_area, interval).
func SpecToMetaMap(sp Spec) map[string]any {
	return map[string]any{
		"name":        sp.Name,
		"description": sp.Description,
		"focus_area":  sp.ReportSection,
		"interval":    sp.IntervalSecond,
	}
}

// Spec describes an executable Agent, Go version of Python AgentRegistry.AgentSpec.
type Spec struct {
	Name           string   `yaml:"agent_name" json:"name"`
	SkillName      string   `yaml:"name" json:"skill_name"`
	Description    string   `yaml:"description" json:"description"`
	SkillDir       string   `json:"skill_dir"`
	IntervalSecond int      `yaml:"interval_seconds" json:"interval_seconds"`
	ReportSection  string   `yaml:"report_section" json:"report_section"`
	SubSkills      []string `json:"sub_skills,omitempty"`
	ReadOnly       bool     `json:"read_only"` // default true, read-only kubectl only
}

// Registry discovers all Agents from skills directory.
type Registry interface {
	Specs() []Spec
	SpecByName(name string) (Spec, bool)
}

// GetSummarySkillDir returns summary skill dir path, corresponds to Python get_summary_skill_dir().
// Returns empty string if summary/SKILL.md or summary/agent.yaml missing.
func GetSummarySkillDir(skillsRoot string) string {
	summaryDir := filepath.Join(skillsRoot, "summary")
	if _, err := os.Stat(filepath.Join(summaryDir, "SKILL.md")); err == nil {
		return summaryDir
	}
	if _, err := os.Stat(filepath.Join(summaryDir, "agent.yaml")); err == nil {
		return summaryDir
	}
	return ""
}

// RegistryOption configures Registry behavior.
type RegistryOption func(*registryOptions)

type registryOptions struct {
	onlyIntervalPositive bool // only register Agents with interval_seconds > 0
}

// WithIntervalPositiveOnly only registers Agents with interval_seconds > 0.
func WithIntervalPositiveOnly() RegistryOption {
	return func(o *registryOptions) {
		o.onlyIntervalPositive = true
	}
}

type fileRegistry struct {
	specs map[string]Spec
}

var skillMdFrontmatterRe = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)

// parseSKILLMDFrontmatter parses SKILL.md YAML frontmatter.
func parseSKILLMDFrontmatter(content []byte) (map[string]any, error) {
	matches := skillMdFrontmatterRe.FindSubmatch(content)
	if len(matches) < 2 {
		return nil, nil
	}
	var m map[string]any
	if err := yaml.Unmarshal(matches[1], &m); err != nil {
		return nil, err
	}
	return m, nil
}

// discoverSubSkills scans skillDir/sub-skills/*.md, returns relative paths, aligned with Python.
func discoverSubSkills(skillDir string) []string {
	subDir := filepath.Join(skillDir, "sub-skills")
	entries, err := os.ReadDir(subDir)
	if err != nil {
		return nil
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		paths = append(paths, filepath.Join("sub-skills", e.Name()))
	}
	return paths
}

func NewFileRegistry(skillsRoot string, opts ...RegistryOption) (Registry, error) {
	info, err := os.Stat(skillsRoot)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skills root is not a dir: %s", skillsRoot)
	}

	opt := &registryOptions{}
	for _, f := range opts {
		f(opt)
	}

	specs := make(map[string]Spec)

	err = filepath.WalkDir(skillsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		dir := filepath.Dir(path)
		base := filepath.Base(path)

		var cfg struct {
			Name           string   `yaml:"name"`
			Description    string   `yaml:"description"`
			IntervalSecond int      `yaml:"interval_seconds"`
			ReportSection  string   `yaml:"report_section"`
			SubSkills      []string `yaml:"sub_skills"`
			ReadOnly       *bool    `yaml:"read_only"`
		}

		if base == "agent.yaml" {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return err
			}
		} else if base == "SKILL.md" {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fm, err := parseSKILLMDFrontmatter(data)
			if err != nil || fm == nil {
				return nil
			}
			// Compatible with Python: prefer agent_name, then name
			if v, ok := fm["agent_name"]; ok {
				if s, ok := v.(string); ok && s != "" {
					cfg.Name = s
				}
			}
			if cfg.Name == "" {
				if v, ok := fm["name"]; ok {
					if s, ok := v.(string); ok && s != "" {
						cfg.Name = s
					}
				}
			}
			if v, ok := fm["description"]; ok {
				if s, ok := v.(string); ok {
					cfg.Description = s
				}
			}
			if v, ok := fm["interval_seconds"]; ok {
				switch n := v.(type) {
				case int:
					cfg.IntervalSecond = n
				case float64:
					cfg.IntervalSecond = int(n)
				}
			}
			if v, ok := fm["report_section"]; ok {
				if s, ok := v.(string); ok {
					cfg.ReportSection = s
				}
			}
			if v, ok := fm["sub_skills"]; ok {
				if arr, ok := v.([]any); ok {
					for _, a := range arr {
						if s, ok := a.(string); ok {
							cfg.SubSkills = append(cfg.SubSkills, s)
						}
					}
				}
			}
			if v, ok := fm["read_only"]; ok {
				if b, ok := v.(bool); ok {
					cfg.ReadOnly = &b
				}
			}
		} else {
			return nil
		}

		if cfg.Name == "" {
			return nil
		}

		// If agent.yaml and SKILL.md in same dir, agent.yaml first; else use SKILL.md only
		if base == "SKILL.md" {
			agentPath := filepath.Join(dir, "agent.yaml")
			if _, err := os.Stat(agentPath); err == nil {
				return nil // when agent.yaml exists, let it handle to avoid duplicate
			}
		}

		// _discover_sub_skills: if sub_skills not set, scan sub-skills/*.md
		if len(cfg.SubSkills) == 0 {
			cfg.SubSkills = discoverSubSkills(dir)
		}

		if opt.onlyIntervalPositive && cfg.IntervalSecond <= 0 {
			return nil
		}

		readOnly := true
		if cfg.ReadOnly != nil {
			readOnly = *cfg.ReadOnly
		}
		spec := Spec{
			Name:           cfg.Name,
			SkillName:      cfg.Name,
			Description:    cfg.Description,
			SkillDir:       dir,
			IntervalSecond: cfg.IntervalSecond,
			ReportSection:  firstNonEmpty(cfg.ReportSection, filepath.Base(dir)),
			SubSkills:      cfg.SubSkills,
			ReadOnly:       readOnly,
		}
		specs[spec.Name] = spec
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &fileRegistry{specs: specs}, nil
}

func (r *fileRegistry) Specs() []Spec {
	out := make([]Spec, 0, len(r.specs))
	for _, s := range r.specs {
		out = append(out, s)
	}
	return out
}

func (r *fileRegistry) SpecByName(name string) (Spec, bool) {
	s, ok := r.specs[name]
	return s, ok
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

