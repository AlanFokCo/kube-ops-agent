package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReportFilename(t *testing.T) {
	t0 := time.Date(2024, 3, 15, 10, 30, 5, 0, time.UTC)
	name := ReportFilename(t0)
	if !strings.HasPrefix(name, ReportFilenamePrefix) {
		t.Errorf("expected prefix %q, got %q", ReportFilenamePrefix, name)
	}
	if !strings.HasSuffix(name, ".md") {
		t.Errorf("expected .md suffix, got %q", name)
	}
	if name != "k8s_health_report_20240315_103005.md" {
		t.Errorf("unexpected filename: %q", name)
	}
}

func TestManager_Save(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0)

	content := "# Cluster Health Report\n\n**Report Level**: 🟢 Normal\n\n## Overview\n\nAll good."
	item, err := m.Save(content)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.ID == "" {
		t.Error("expected non-empty ID")
	}
	if item.Filename == "" {
		t.Error("expected non-empty Filename")
	}
	if item.Title == "" {
		t.Error("expected non-empty Title (parsed from #)")
	}
	if item.Level == "" {
		t.Error("expected non-empty Level (parsed from Report Level)")
	}
	if item.SizeBytes <= 0 {
		t.Error("expected positive SizeBytes")
	}
	// Check file exists
	if _, err := os.Stat(item.Path); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}
}

func TestManager_List(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0)

	// Write files with different timestamps directly to avoid same-second collisions
	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 10, 0, 1, 0, time.UTC)
	os.WriteFile(filepath.Join(dir, ReportFilename(t1)), []byte("# Report 1\n## Section A"), 0644)
	os.WriteFile(filepath.Join(dir, ReportFilename(t2)), []byte("# Report 2\n## Section B"), 0644)

	items, total, err := m.List(10, 0, nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 total, got %d", total)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestManager_List_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0)

	items, total, err := m.List(10, 0, nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 total, got %d", total)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestManager_List_InvalidDir(t *testing.T) {
	m := NewManager("/nonexistent/dir/xyz", 0)
	_, _, err := m.List(10, 0, nil, nil)
	if err == nil {
		t.Error("expected error for invalid dir")
	}
}

func TestManager_List_Pagination(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0)

	// Write 5 files with distinct second-precision timestamps
	for i := 0; i < 5; i++ {
		ts := time.Date(2024, 1, 1, 10, 0, i, 0, time.UTC)
		os.WriteFile(filepath.Join(dir, ReportFilename(ts)), []byte("# Report\n## Section"), 0644)
	}

	items1, total, err := m.List(2, 0, nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 5 {
		t.Errorf("expected 5 total, got %d", total)
	}
	if len(items1) != 2 {
		t.Errorf("expected 2 items, got %d", len(items1))
	}

	items2, _, err := m.List(2, 4, nil, nil)
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(items2) != 1 {
		t.Errorf("expected 1 item at end, got %d", len(items2))
	}
}

func TestManager_List_OffsetBeyondTotal(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0)
	m.Save("# Report")

	items, total, err := m.List(10, 100, nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items past end, got %d", len(items))
	}
}

func TestManager_List_TimeFilter(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0)

	past := time.Now().Add(-time.Hour)
	// Write a file with a name in the past using our naming format
	oldFilename := ReportFilename(past)
	os.WriteFile(filepath.Join(dir, oldFilename), []byte("# old report"), 0644)

	time.Sleep(1 * time.Millisecond)
	m.Save("# new report") // creates a current report

	start := past.Add(30 * time.Minute) // midpoint: filters out the old one
	items, total, err := m.List(10, 0, &start, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total == 0 {
		t.Error("expected at least 1 recent report")
	}
	for _, item := range items {
		if item.CreatedAt.Before(start) {
			t.Errorf("item %s is before start filter", item.Filename)
		}
	}
	_ = total
}

func TestManager_Get(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0)

	content := "# Test Report\n\n## Summary\n\nOK"
	item, _ := m.Save(content)

	got, gotContent, err := m.Get(item.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil item")
	}
	if gotContent != content {
		t.Errorf("content mismatch: got %q want %q", gotContent, content)
	}
}

func TestManager_Get_WithMdSuffix(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0)
	item, _ := m.Save("# Report\n## Section")

	// Test that ID with .md suffix is also accepted
	got, _, err := m.Get(item.ID + ".md")
	if err != nil {
		t.Fatalf("Get with .md suffix: %v", err)
	}
	if got.ID != item.ID {
		t.Errorf("IDs don't match: %q vs %q", got.ID, item.ID)
	}
}

func TestManager_Get_NotFound(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0)

	_, _, err := m.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent report")
	}
}

func TestManager_Latest(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0)

	time.Sleep(1 * time.Millisecond)
	m.Save("# Report 1")
	time.Sleep(1 * time.Millisecond)
	item2, _ := m.Save("# Report 2")

	latestItem, _, err := m.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if latestItem.ID != item2.ID {
		t.Errorf("expected latest report ID %q, got %q", item2.ID, latestItem.ID)
	}
}

func TestManager_Latest_Empty(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0)

	_, _, err := m.Latest()
	if err == nil {
		t.Error("expected error for empty dir")
	}
}

func TestManager_PruneIfNeeded_NoLimit(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0) // no limit
	for i := 0; i < 3; i++ {
		ts := time.Date(2024, 1, 1, 10, 0, i, 0, time.UTC)
		os.WriteFile(filepath.Join(dir, ReportFilename(ts)), []byte("# Report"), 0644)
	}

	if err := m.PruneIfNeeded(); err != nil {
		t.Fatalf("PruneIfNeeded: %v", err)
	}
	_, total, _ := m.List(100, 0, nil, nil)
	if total != 3 {
		t.Errorf("expected 3 reports (no pruning), got %d", total)
	}
}

func TestManager_PruneIfNeeded_WithLimit(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 2)

	time.Sleep(1 * time.Millisecond)
	m.Save("# Report 1")
	time.Sleep(1 * time.Millisecond)
	m.Save("# Report 2")
	time.Sleep(1 * time.Millisecond)
	m.Save("# Report 3")
	time.Sleep(1 * time.Millisecond)
	m.Save("# Report 4") // this triggers prune in Save

	_, total, _ := m.List(100, 0, nil, nil)
	if total > 2 {
		t.Errorf("expected at most 2 reports after pruning, got %d", total)
	}
}

func TestManager_ParseHeaderFields_Sections(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, 0)

	content := "# My Report\n\n**Report Level**: 🔴 Alert\n\n## Node Health\n\n## Pod Health\n\n## Summary"
	item, err := m.Save(content)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if item.Level == "" {
		t.Error("expected Level to be set")
	}
	if len(item.Sections) == 0 {
		t.Error("expected Sections to be set")
	}
}

func TestManager_ParseHeaderFields_NonMatchingFilename(t *testing.T) {
	dir := t.TempDir()
	// Write a file with a non-standard name to test ModTime fallback
	path := filepath.Join(dir, "other_report.md")
	os.WriteFile(path, []byte("# Report\n## Section"), 0644)

	m := NewManager(dir, 0)
	items, total, err := m.List(10, 0, nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 item, got %d", total)
	}
	if len(items) > 0 {
		// Should use mod time as fallback
		if items[0].CreatedAt.IsZero() {
			t.Error("expected non-zero CreatedAt")
		}
	}
}
