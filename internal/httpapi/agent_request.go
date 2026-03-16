package httpapi

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

// AgentRequest aligns with Python agentscope-runtime AgentRequest, parses params from input.
type AgentRequest struct {
	Input []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"input"`
}

// ParseAgentRequest parses AgentRequest from body, returns merged text.
func ParseAgentRequest(body io.Reader) (text string, ok bool) {
	var req AgentRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return "", false
	}
	var parts []string
	for _, msg := range req.Input {
		for _, block := range msg.Content {
			if block.Type == "text" && block.Text != "" {
				parts = append(parts, block.Text)
			}
		}
	}
	return strings.Join(parts, " "), len(parts) > 0
}

// ParseParamsFromText parses params from "key: value, key2: value2" format.
func ParseParamsFromText(text string) map[string]string {
	out := make(map[string]string)
	for _, param := range strings.Split(text, ",") {
		param = strings.TrimSpace(param)
		if idx := strings.Index(param, ":"); idx > 0 {
			k := strings.TrimSpace(param[:idx])
			v := strings.TrimSpace(param[idx+1:])
			if k != "" {
				out[k] = v
			}
		}
	}
	return out
}

// GetInt gets int from params, returns def if invalid.
func GetInt(params map[string]string, key string, def int) int {
	if v, ok := params[key]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

// GetFloat gets float64 from params, returns def if invalid.
func GetFloat(params map[string]string, key string, def float64) float64 {
	if v, ok := params[key]; ok {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
	}
	return def
}

// GetBool gets bool from params ("true"/"false"), returns nil if invalid.
func GetBool(params map[string]string, key string) *bool {
	if v, ok := params[key]; ok {
		v = strings.TrimSpace(strings.ToLower(v))
		if v == "true" {
			t := true
			return &t
		}
		if v == "false" {
			f := false
			return &f
		}
	}
	return nil
}

// GetString gets string from params.
func GetString(params map[string]string, key string) string {
	return strings.TrimSpace(params[key])
}
