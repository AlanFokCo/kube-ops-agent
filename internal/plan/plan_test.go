package plan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFile(t *testing.T) {
	path := filepath.Join("..", "..", "kubernetes-ops-agent", "workflow.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skip("workflow.yaml not found")
	}
	p, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if len(p.Steps) == 0 {
		t.Fatal("expected steps")
	}
	if p.Priority == "" {
		t.Error("expected priority")
	}
}
