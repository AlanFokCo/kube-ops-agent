package report

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Item is report file metadata, aligned with Python ReportSummary/ReportDetail.
type Item struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	CreatedAt time.Time `json:"created_at"`
	Timestamp float64   `json:"timestamp,omitempty"` // Unix timestamp, aligned with Python ReportSummary
	Path      string    `json:"-"`
	Title     string    `json:"title,omitempty"`
	Level     string    `json:"level,omitempty"` // 🟢 Normal / 🟡 Attention / 🔴 Alert
	Sections  []string  `json:"sections,omitempty"`
	SizeBytes int64     `json:"size_bytes,omitempty"`
}

const (
	ReportTimestampFormat = "20060102_150405"
	ReportFilenamePrefix = "k8s_health_report_"
)

// ReportFilename returns the standard report filename for the given time.
func ReportFilename(t time.Time) string {
	return ReportFilenamePrefix + t.Format(ReportTimestampFormat) + ".md"
}

var (
	filenamePattern = regexp.MustCompile(`k8s_health_report_(\d{8}_\d{6})\.md`)
	titlePattern    = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	levelPattern    = regexp.MustCompile(`\*\*Report Level\*\*:\s*(🟢\s*Normal|🟡\s*Attention|🔴\s*Alert)`)
	sectionPattern  = regexp.MustCompile(`(?m)^##\s+(.+)$`)
)

// Manager manages report files (Markdown) on disk.
type Manager struct {
	root       string
	maxReports int // 0=unlimited
}

// NewManager creates report manager. maxReports=0 means unlimited.
func NewManager(root string, maxReports int) *Manager {
	return &Manager{root: root, maxReports: maxReports}
}

// PruneIfNeeded deletes oldest reports if exceeding max_reports.
func (m *Manager) PruneIfNeeded() error {
	if m.maxReports <= 0 {
		return nil
	}
	items, total, err := m.List(10000, 0, nil, nil)
	if err != nil || total <= m.maxReports {
		return err
	}
	// items sorted by time desc (newest first), keep first maxReports, delete rest
	for i := m.maxReports; i < len(items); i++ {
		_ = os.Remove(items[i].Path)
	}
	return nil
}

// List returns reports sorted by time desc with pagination and time filter.
func (m *Manager) List(limit, offset int, start, end *time.Time) ([]Item, int, error) {
	entries, err := os.ReadDir(m.root)
	if err != nil {
		return nil, 0, err
	}
	var items []Item
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		full := filepath.Join(m.root, e.Name())
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		t := m.parseTimestampFromFilename(e.Name(), info.ModTime())
		if start != nil && t.Before(*start) {
			continue
		}
		if end != nil && t.After(*end) {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".md")
		item := Item{ID: id, Filename: e.Name(), CreatedAt: t, Timestamp: float64(t.Unix()), Path: full, SizeBytes: info.Size()}
		m.parseHeaderFields(full, &item)
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	total := len(items)
	if offset > total {
		return []Item{}, total, nil
	}
	endIdx := offset + limit
	if endIdx > total {
		endIdx = total
	}
	return items[offset:endIdx], total, nil
}

// Get returns report content by ID. Supports .md suffix (auto-stripped).
func (m *Manager) Get(id string) (*Item, string, error) {
	id = strings.TrimSuffix(id, ".md")
	path := filepath.Join(m.root, id+".md")
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	t := m.parseTimestampFromFilename(filepath.Base(path), info.ModTime())
	item := &Item{
		ID:        id,
		Filename:  filepath.Base(path),
		CreatedAt: t,
		Timestamp: float64(t.Unix()),
		Path:      path,
		SizeBytes: info.Size(),
	}
	m.parseHeaderFieldsFromContent(string(data), item)
	return item, string(data), nil
}

func (m *Manager) parseTimestampFromFilename(name string, fallback time.Time) time.Time {
	matches := filenamePattern.FindStringSubmatch(name)
	if len(matches) < 2 {
		return fallback
	}
	t, err := time.Parse("20060102_150405", matches[1])
	if err != nil {
		return fallback
	}
	return t
}

func (m *Manager) parseHeaderFields(path string, item *Item) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	// Read first 4KB for header parsing
	content := string(data)
	if len(content) > 4096 {
		content = content[:4096]
	}
	m.parseHeaderFieldsFromContent(content, item)
}

func (m *Manager) parseHeaderFieldsFromContent(content string, item *Item) {
	if m := titlePattern.FindStringSubmatch(content); len(m) >= 2 {
		item.Title = strings.TrimSpace(m[1])
	}
	if m := levelPattern.FindStringSubmatch(content); len(m) >= 2 {
		item.Level = strings.TrimSpace(m[1])
	}
	for _, m := range sectionPattern.FindAllStringSubmatch(content, -1) {
		if len(m) >= 2 {
			item.Sections = append(item.Sections, strings.TrimSpace(m[1]))
		}
	}
}

// Save saves content as new report, returns metadata.
func (m *Manager) Save(content string) (*Item, error) {
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return nil, err
	}
	now := time.Now()
	filename := ReportFilename(now)
	path := filepath.Join(m.root, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return nil, err
	}
	id := strings.TrimSuffix(filename, ".md")
	info, _ := os.Stat(path)
	sizeBytes := int64(0)
	if info != nil {
		sizeBytes = info.Size()
	}
	item := &Item{
		ID:        id,
		Filename:  filename,
		CreatedAt: now,
		Timestamp: float64(now.Unix()),
		Path:      path,
		SizeBytes: sizeBytes,
	}
	m.parseHeaderFieldsFromContent(content, item)
	_ = m.PruneIfNeeded()
	return item, nil
}

// Latest returns the most recent report.
func (m *Manager) Latest() (*Item, string, error) {
	items, _, err := m.List(1, 0, nil, nil)
	if err != nil {
		return nil, "", err
	}
	if len(items) == 0 {
		return nil, "", os.ErrNotExist
	}
	return m.Get(items[0].ID)
}

