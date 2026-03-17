package logging

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestSetup_Levels(t *testing.T) {
	levels := []string{"DEBUG", "INFO", "WARNING", "ERROR", "debug", "info", "warn"}
	for _, lvl := range levels {
		if err := Setup(lvl); err != nil {
			t.Errorf("Setup(%q) returned error: %v", lvl, err)
		}
	}
	// Reset to default
	Setup("INFO")
}

func TestSetup_InvalidLevel(t *testing.T) {
	// Invalid level should default to Info (no error returned)
	if err := Setup("INVALID_LEVEL"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLogAgentResult_Success(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.DebugLevel)
	defer Setup("INFO")

	LogAgentResult("agent1", true, 1.5, "")
	if buf.Len() == 0 {
		t.Error("expected log output")
	}
}

func TestLogAgentResult_Failure(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.DebugLevel)
	defer Setup("INFO")

	LogAgentResult("agent1", false, 0.5, "some error")
	if buf.Len() == 0 {
		t.Error("expected log output for failure")
	}
}

func TestLogPlanExecution(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.DebugLevel)
	defer Setup("INFO")

	LogPlanExecution(1, 3, 2, "parallel", 5.0, true)
	if buf.Len() == 0 {
		t.Error("expected log output for plan execution")
	}
	buf.Reset()
	LogPlanExecution(2, 3, 1, "sequential", 2.0, false)
	if buf.Len() == 0 {
		t.Error("expected log output for failed plan execution")
	}
}

func TestStringsToLower(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"DEBUG", "debug"},
		{"Info", "info"},
		{"warning", "warning"},
		{"", ""},
		{"MiXeD", "mixed"},
	}
	for _, tt := range tests {
		got := stringsToLower(tt.input)
		if got != tt.want {
			t.Errorf("stringsToLower(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
