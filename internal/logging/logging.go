package logging

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Setup configures global logging; level is DEBUG/INFO/WARNING/ERROR. Format aligned with Python logging_config.
func Setup(level string) error {
	s := stringsToLower(level)
	if s == "warning" {
		s = "warn"
	}
	lvl, err := logrus.ParseLevel(s)
	if err != nil {
		lvl = logrus.InfoLevel
	}
	logrus.SetLevel(lvl)
	logrus.SetOutput(os.Stdout)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		DisableColors:   false,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	return nil
}

func stringsToLower(s string) string {
	b := []byte(s)
	for i := 0; i < len(b); i++ {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// LogAgentResult logs structured Agent execution result.
func LogAgentResult(agentName string, success bool, durationSeconds float64, extra string) {
	fields := logrus.Fields{
		"agent":            agentName,
		"duration_seconds": durationSeconds,
		"status":           "success",
		"extra":            extra,
	}
	if !success {
		fields["status"] = "failure"
	}
	if success {
		logrus.WithFields(fields).Info("[AgentExecutor] agent executed")
	} else {
		logrus.WithFields(fields).Warn("[AgentExecutor] agent executed")
	}
}

// LogPlanExecution logs plan step execution.
func LogPlanExecution(stepIndex, totalSteps, agentsInStep int, mode string, durationSeconds float64, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	logrus.WithFields(logrus.Fields{
		"step":              stepIndex,
		"total_steps":       totalSteps,
		"agents_in_step":    agentsInStep,
		"mode":              mode,
		"duration_seconds":  durationSeconds,
		"status":            status,
	}).Info("[PlanExecutor] step executed")
}
