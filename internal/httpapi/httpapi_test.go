package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alanfokco/kube-ops-agent-go/internal/agent"
	mcp "github.com/alanfokco/kube-ops-agent-go/internal/mcp"
	"github.com/alanfokco/kube-ops-agent-go/internal/operations"
	"github.com/alanfokco/kube-ops-agent-go/internal/report"
	"github.com/alanfokco/kube-ops-agent-go/internal/runtime"
	"github.com/alanfokco/kube-ops-agent-go/internal/scheduler"
)

// ---- ParseAgentRequest ----

func TestParseAgentRequest_Valid(t *testing.T) {
	body := `{"input": [{"content": [{"type": "text", "text": "check nodes"}]}]}`
	text, ok := ParseAgentRequest(strings.NewReader(body))
	if !ok {
		t.Error("expected ok=true")
	}
	if text != "check nodes" {
		t.Errorf("text = %q", text)
	}
}

func TestParseAgentRequest_MultipleMessages(t *testing.T) {
	body := `{"input": [
		{"content": [{"type": "text", "text": "check nodes"}]},
		{"content": [{"type": "text", "text": "and pods"}]}
	]}`
	text, ok := ParseAgentRequest(strings.NewReader(body))
	if !ok {
		t.Error("expected ok=true")
	}
	if !strings.Contains(text, "check nodes") || !strings.Contains(text, "and pods") {
		t.Errorf("text = %q", text)
	}
}

func TestParseAgentRequest_SkipsNonText(t *testing.T) {
	body := `{"input": [{"content": [{"type": "image", "text": "ignored"}, {"type": "text", "text": "real"}]}]}`
	text, ok := ParseAgentRequest(strings.NewReader(body))
	if !ok {
		t.Error("expected ok=true")
	}
	if text != "real" {
		t.Errorf("text = %q", text)
	}
}

func TestParseAgentRequest_InvalidJSON(t *testing.T) {
	_, ok := ParseAgentRequest(strings.NewReader("not json"))
	if ok {
		t.Error("expected ok=false for invalid JSON")
	}
}

func TestParseAgentRequest_EmptyText(t *testing.T) {
	body := `{"input": [{"content": [{"type": "text", "text": ""}]}]}`
	_, ok := ParseAgentRequest(strings.NewReader(body))
	if ok {
		t.Error("expected ok=false for empty text")
	}
}

// ---- ParseParamsFromText ----

func TestParseParamsFromText(t *testing.T) {
	params := ParseParamsFromText("agent_name: TestAgent, limit: 10, offset: 5")
	if params["agent_name"] != "TestAgent" {
		t.Errorf("agent_name = %q", params["agent_name"])
	}
	if params["limit"] != "10" {
		t.Errorf("limit = %q", params["limit"])
	}
	if params["offset"] != "5" {
		t.Errorf("offset = %q", params["offset"])
	}
}

func TestParseParamsFromText_Empty(t *testing.T) {
	params := ParseParamsFromText("")
	if len(params) != 0 {
		t.Errorf("expected empty params, got %v", params)
	}
}

func TestParseParamsFromText_NoColon(t *testing.T) {
	params := ParseParamsFromText("nocolon")
	if len(params) != 0 {
		t.Errorf("expected empty params for no colon, got %v", params)
	}
}

// ---- GetInt ----

func TestGetInt(t *testing.T) {
	params := map[string]string{"count": "42", "invalid": "abc"}
	if got := GetInt(params, "count", 0); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
	if got := GetInt(params, "invalid", 99); got != 99 {
		t.Errorf("expected default 99, got %d", got)
	}
	if got := GetInt(params, "missing", 5); got != 5 {
		t.Errorf("expected default 5, got %d", got)
	}
}

// ---- GetFloat ----

func TestGetFloat(t *testing.T) {
	params := map[string]string{"rate": "3.14", "bad": "not-a-float"}
	if got := GetFloat(params, "rate", 0); got != 3.14 {
		t.Errorf("expected 3.14, got %f", got)
	}
	if got := GetFloat(params, "bad", 1.0); got != 1.0 {
		t.Errorf("expected default 1.0, got %f", got)
	}
	if got := GetFloat(params, "missing", 2.5); got != 2.5 {
		t.Errorf("expected default 2.5, got %f", got)
	}
}

// ---- GetBool ----

func TestGetBool(t *testing.T) {
	params := map[string]string{"active": "true", "disabled": "false", "bad": "maybe"}
	b := GetBool(params, "active")
	if b == nil || !*b {
		t.Error("expected true for 'active'")
	}
	b2 := GetBool(params, "disabled")
	if b2 == nil || *b2 {
		t.Error("expected false for 'disabled'")
	}
	b3 := GetBool(params, "bad")
	if b3 != nil {
		t.Error("expected nil for 'maybe'")
	}
	b4 := GetBool(params, "missing")
	if b4 != nil {
		t.Error("expected nil for missing key")
	}
}

// ---- GetString ----

func TestGetString(t *testing.T) {
	params := map[string]string{"name": "  TestAgent  "}
	got := GetString(params, "name")
	if got != "TestAgent" {
		t.Errorf("expected 'TestAgent', got %q", got)
	}
	if got2 := GetString(params, "missing"); got2 != "" {
		t.Errorf("expected empty string for missing key, got %q", got2)
	}
}

// ---- TokenManager ----

func TestNewTokenManager_Empty(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("K8SOPS_API_TOKEN")
	os.Unsetenv("API_TOKENS")
	tm := NewTokenManager(nil)
	if tm == nil {
		t.Fatal("expected non-nil TokenManager")
	}
	if tm.HasTokens() {
		t.Error("expected no tokens initially")
	}
}

func TestNewTokenManager_ExtraTokens(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("K8SOPS_API_TOKEN")
	os.Unsetenv("API_TOKENS")
	tm := NewTokenManager([]string{"token1", "token2", ""})
	if tm.TokenCount() != 2 {
		t.Errorf("expected 2 tokens, got %d", tm.TokenCount())
	}
}

func TestNewTokenManager_FromEnv(t *testing.T) {
	os.Setenv("API_TOKEN", "env-token")
	os.Setenv("API_TOKENS", "tok1, tok2, tok3")
	defer func() {
		os.Unsetenv("API_TOKEN")
		os.Unsetenv("API_TOKENS")
	}()
	tm := NewTokenManager(nil)
	if !tm.IsValid("env-token") {
		t.Error("expected env-token to be valid")
	}
	if !tm.IsValid("tok1") {
		t.Error("expected tok1 to be valid")
	}
	if !tm.IsValid("tok2") {
		t.Error("expected tok2 to be valid")
	}
}

func TestTokenManager_AddRemove(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("K8SOPS_API_TOKEN")
	os.Unsetenv("API_TOKENS")
	tm := NewTokenManager(nil)
	tm.AddToken("new-token")
	if !tm.IsValid("new-token") {
		t.Error("expected new-token to be valid after Add")
	}
	tm.RemoveToken("new-token")
	if tm.IsValid("new-token") {
		t.Error("expected new-token to be invalid after Remove")
	}
}

func TestTokenManager_AddToken_Empty(t *testing.T) {
	tm := NewTokenManager(nil)
	tm.AddToken("")
	if tm.HasTokens() {
		t.Error("adding empty token should not add anything")
	}
}

func TestTokenManager_Rotate(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("K8SOPS_API_TOKEN")
	os.Unsetenv("API_TOKENS")
	tm := NewTokenManager([]string{"old-token"})
	tm.RotateToken("old-token", "new-token")
	if tm.IsValid("old-token") {
		t.Error("old-token should be invalid after rotation")
	}
	if !tm.IsValid("new-token") {
		t.Error("new-token should be valid after rotation")
	}
}

func TestTokenManager_IsValid_Empty(t *testing.T) {
	tm := NewTokenManager([]string{"tok"})
	if tm.IsValid("") {
		t.Error("empty token should not be valid")
	}
}

func TestNewTokenManager_K8SOpsAPIToken(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("API_TOKENS")
	os.Setenv("K8SOPS_API_TOKEN", "k8sops-token")
	defer os.Unsetenv("K8SOPS_API_TOKEN")
	tm := NewTokenManager(nil)
	if !tm.IsValid("k8sops-token") {
		t.Error("expected k8sops-token to be valid from K8SOPS_API_TOKEN env")
	}
}

// ---- AuthRateLimiter ----

func TestNewAuthRateLimiter_Defaults(t *testing.T) {
	rl := NewAuthRateLimiter(0, 0, 0)
	if rl.maxFails != 5 {
		t.Errorf("maxFails = %d", rl.maxFails)
	}
}

func TestAuthRateLimiter_NotBlocked(t *testing.T) {
	rl := NewAuthRateLimiter(5, time.Minute, 300)
	if rl.IsBlocked("1.2.3.4") {
		t.Error("IP should not be blocked initially")
	}
}

func TestAuthRateLimiter_BlockedAfterFailures(t *testing.T) {
	rl := NewAuthRateLimiter(3, time.Minute, 300)
	rl.RecordFailure("1.2.3.4")
	rl.RecordFailure("1.2.3.4")
	rl.RecordFailure("1.2.3.4")
	rl.RecordFailure("1.2.3.4") // > maxFails
	if !rl.IsBlocked("1.2.3.4") {
		t.Error("IP should be blocked after >3 failures")
	}
}

func TestAuthRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewAuthRateLimiter(2, time.Minute, 300)
	rl.RecordFailure("1.1.1.1")
	rl.RecordFailure("1.1.1.1")
	rl.RecordFailure("1.1.1.1")
	if rl.IsBlocked("2.2.2.2") {
		t.Error("different IP should not be blocked")
	}
}

// ---- extractToken ----

func TestExtractToken_XAPIKey(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", "my-token")
	got := extractToken(r)
	if got != "my-token" {
		t.Errorf("expected 'my-token', got %q", got)
	}
}

func TestExtractToken_BearerAuth(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer secret-token")
	got := extractToken(r)
	if got != "secret-token" {
		t.Errorf("expected 'secret-token', got %q", got)
	}
}

func TestExtractToken_None(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	got := extractToken(r)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExtractToken_PreferXAPIKey(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", "api-key-token")
	r.Header.Set("Authorization", "Bearer auth-token")
	got := extractToken(r)
	if got != "api-key-token" {
		t.Errorf("X-API-Key should take precedence, got %q", got)
	}
}

// ---- isExempt ----

func TestIsExempt_Exact(t *testing.T) {
	exempt := map[string]bool{"/health": true, "/ready": true}
	if !isExempt("/health", exempt) {
		t.Error("expected /health to be exempt")
	}
	if isExempt("/api/data", exempt) {
		t.Error("expected /api/data to not be exempt")
	}
}

func TestIsExempt_WildcardPrefix(t *testing.T) {
	exempt := map[string]bool{"/api/*": true}
	if !isExempt("/api/data", exempt) {
		t.Error("expected /api/data to be exempt with /api/*")
	}
	if !isExempt("/api/v2/users", exempt) {
		t.Error("expected /api/v2/users to be exempt with /api/*")
	}
	if isExempt("/other", exempt) {
		t.Error("expected /other to not be exempt")
	}
}

// ---- defaultExemptPaths ----

func TestDefaultExemptPaths(t *testing.T) {
	exempt := defaultExemptPaths([]string{"/custom", ""})
	if !exempt["/health"] {
		t.Error("expected /health in defaults")
	}
	if !exempt["/ready"] {
		t.Error("expected /ready in defaults")
	}
	if !exempt["/custom"] {
		t.Error("expected /custom added")
	}
	if exempt[""] {
		t.Error("empty string should not be added")
	}
}

// ---- clientIP ----

func TestClientIP_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "1.2.3.4:8080"
	ip := clientIP(r)
	if ip != "1.2.3.4" {
		t.Errorf("expected '1.2.3.4', got %q", ip)
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.1, 192.168.1.1")
	ip := clientIP(r)
	if ip != "10.0.0.1" {
		t.Errorf("expected '10.0.0.1', got %q", ip)
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", " 10.0.0.2 ")
	ip := clientIP(r)
	if ip != "10.0.0.2" {
		t.Errorf("expected '10.0.0.2', got %q", ip)
	}
}

// ---- authCheck ----

func TestAuthCheck_NoTokensConfigured(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("K8SOPS_API_TOKEN")
	os.Unsetenv("API_TOKENS")
	tm := NewTokenManager(nil)
	r := httptest.NewRequest("GET", "/api/data", nil)
	result := authCheck(r, tm, defaultExemptPaths(nil), nil)
	if !result.OK {
		t.Error("expected OK when no tokens configured")
	}
}

func TestAuthCheck_ExemptPath(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("K8SOPS_API_TOKEN")
	os.Unsetenv("API_TOKENS")
	tm := NewTokenManager([]string{"tok"})
	r := httptest.NewRequest("GET", "/health", nil)
	result := authCheck(r, tm, defaultExemptPaths(nil), nil)
	if !result.OK {
		t.Error("expected OK for exempt /health path")
	}
}

func TestAuthCheck_ValidToken(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("K8SOPS_API_TOKEN")
	os.Unsetenv("API_TOKENS")
	tm := NewTokenManager([]string{"valid-token"})
	r := httptest.NewRequest("GET", "/api/data", nil)
	r.Header.Set("X-API-Key", "valid-token")
	result := authCheck(r, tm, defaultExemptPaths(nil), nil)
	if !result.OK {
		t.Errorf("expected OK with valid token, got: %v", result)
	}
}

func TestAuthCheck_InvalidToken(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("K8SOPS_API_TOKEN")
	os.Unsetenv("API_TOKENS")
	tm := NewTokenManager([]string{"valid-token"})
	r := httptest.NewRequest("GET", "/api/data", nil)
	r.Header.Set("X-API-Key", "wrong-token")
	result := authCheck(r, tm, defaultExemptPaths(nil), nil)
	if result.OK {
		t.Error("expected NOT OK with invalid token")
	}
	if result.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", result.StatusCode)
	}
}

func TestAuthCheck_BlockedIP(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("K8SOPS_API_TOKEN")
	os.Unsetenv("API_TOKENS")
	tm := NewTokenManager([]string{"tok"})
	rl := NewAuthRateLimiter(1, time.Minute, 300)
	rl.RecordFailure("1.2.3.4")
	rl.RecordFailure("1.2.3.4") // triggers block

	r := httptest.NewRequest("GET", "/api/data", nil)
	r.RemoteAddr = "1.2.3.4:9090"
	result := authCheck(r, tm, defaultExemptPaths(nil), rl)
	if result.OK {
		t.Error("expected NOT OK for blocked IP")
	}
	if result.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", result.StatusCode)
	}
}

// ---- AuthMiddleware ----

func TestAuthMiddleware_NoTokens(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("K8SOPS_API_TOKEN")
	os.Unsetenv("API_TOKENS")
	tm := NewTokenManager(nil)
	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), tm, nil, nil)

	r := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("K8SOPS_API_TOKEN")
	os.Unsetenv("API_TOKENS")
	tm := NewTokenManager([]string{"my-token"})
	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), tm, nil, nil)

	r := httptest.NewRequest("GET", "/api/test", nil)
	r.Header.Set("X-API-Key", "my-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_Unauthorized(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("K8SOPS_API_TOKEN")
	os.Unsetenv("API_TOKENS")
	tm := NewTokenManager([]string{"my-token"})
	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), tm, nil, nil)

	r := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// ---- HTTP Handler Tests ----

type testRegistry struct {
specs []agent.Spec
}

func (r *testRegistry) Specs() []agent.Spec                        { return r.specs }
func (r *testRegistry) SpecByName(name string) (agent.Spec, bool) { return agent.Spec{}, false }

func newTestDeps(t *testing.T) *Deps {
dir := t.TempDir()
env := runtime.NewEnvironment(nil)
reg := &testRegistry{
specs: []agent.Spec{
{Name: "NodeHealth", IntervalSecond: 300},
},
}
return &Deps{
Env:       env,
Registry:  reg,
ReportMgr: report.NewManager(dir, 0),
OpsMgr:    operations.NewManager(),
ReportDir: dir,
StartTime: time.Now(),
}
}

func setupTestRouter(d *Deps) *gin.Engine {
gin.SetMode(gin.TestMode)
r := gin.New()
RegisterRoutesWithDeps(r, d)
return r
}

func TestHandleHealth(t *testing.T) {
d := newTestDeps(t)
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/health", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
if !strings.Contains(w.Body.String(), "status") {
t.Error("expected status in response")
}
}

func TestHandleReady_NoScheduler(t *testing.T) {
d := newTestDeps(t)
d.Scheduler = nil
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/ready", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
if !strings.Contains(w.Body.String(), "not_ready") {
t.Error("expected not_ready reason in response")
}
}

func TestHandleMetrics(t *testing.T) {
d := newTestDeps(t)
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/metrics", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}

func TestHandleSystem(t *testing.T) {
d := newTestDeps(t)
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/system", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
body := w.Body.String()
if !strings.Contains(body, "version") {
t.Error("expected version in system response")
}
}

func TestHandleReportsList(t *testing.T) {
d := newTestDeps(t)
// Save a test report
d.ReportMgr.Save("# Test Report\n## Section")
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/reports", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
if !strings.Contains(w.Body.String(), "success") {
t.Error("expected success in response")
}
}

func TestHandleReportsList_WithParams(t *testing.T) {
d := newTestDeps(t)
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/reports?limit=5&offset=0", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}

func TestHandleReportDetail_NoID(t *testing.T) {
d := newTestDeps(t)
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/report", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusBadRequest {
t.Errorf("expected 400 for missing id, got %d", w.Code)
}
}

func TestHandleReportDetail_Latest_Empty(t *testing.T) {
d := newTestDeps(t)
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/report?latest=true", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusNotFound {
t.Errorf("expected 404 when no reports, got %d", w.Code)
}
}

func TestHandleReportDetail_Latest(t *testing.T) {
d := newTestDeps(t)
d.ReportMgr.Save("# My Report\n## Section")
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/report?latest=true", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
}
}

func TestHandleReportDetail_GetByID(t *testing.T) {
d := newTestDeps(t)
item, _ := d.ReportMgr.Save("# My Report")
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/report?id="+item.ID, nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
}
}

func TestHandleReportDetail_Markdown(t *testing.T) {
d := newTestDeps(t)
d.ReportMgr.Save("# My Report")
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/report?latest=true&format=markdown", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
ct := w.Header().Get("Content-Type")
if !strings.Contains(ct, "markdown") {
t.Errorf("expected markdown content type, got %q", ct)
}
}

func TestHandleReportPOST(t *testing.T) {
d := newTestDeps(t)
r := setupTestRouter(d)

body := `{"content": "# New Report\n\nContent"}`
w := httptest.NewRecorder()
req := httptest.NewRequest("POST", "/api/report", strings.NewReader(body))
req.Header.Set("Content-Type", "application/json")
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
}
}

func TestHandleReportPOST_NoContentType(t *testing.T) {
d := newTestDeps(t)
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("POST", "/api/report", strings.NewReader("content"))
r.ServeHTTP(w, req)
if w.Code != http.StatusBadRequest {
t.Errorf("expected 400, got %d", w.Code)
}
}

func TestHandleOperations_NilMgr(t *testing.T) {
d := newTestDeps(t)
d.OpsMgr = nil
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/operations", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}

func TestHandleOperations_WithMgr(t *testing.T) {
d := newTestDeps(t)
now := time.Now()
d.OpsMgr.Record("agent1", true, now, now, 1.0, "")
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/operations", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}

func TestHandleMCPTools_NilRegistry(t *testing.T) {
d := newTestDeps(t)
d.MCPReg = nil
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/mcp-tools", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}

func TestAgentNames(t *testing.T) {
specs := []agent.Spec{
{Name: "Agent1"},
{Name: "Agent2"},
}
names := agentNames(specs)
if len(names) != 2 {
t.Fatalf("expected 2 names, got %d", len(names))
}
if names[0] != "Agent1" || names[1] != "Agent2" {
t.Errorf("names = %v", names)
}
}

func TestHandleHealth_Shutting_Down(t *testing.T) {
d := newTestDeps(t)
d.Env.Graceful.RequestShutdown()
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/health", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
if !strings.Contains(w.Body.String(), "shutting_down") {
t.Error("expected shutting_down status")
}
}

func TestHandleSystem_WithMCPReg(t *testing.T) {
d := newTestDeps(t)
d.MCPReg = mcp.NewRegistry()
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/system", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
if !strings.Contains(w.Body.String(), "mcp") {
t.Error("expected mcp in response when MCPReg set")
}
}

func TestRegisterRoutes(t *testing.T) {
gin.SetMode(gin.TestMode)
engine := gin.New()
env := runtime.NewEnvironment(nil)
reg := &testRegistry{}
dir := t.TempDir()
RegisterRoutes(engine, env, reg, nil, dir, 0)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/health", nil)
engine.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}

func TestHandleTrigger_NoSpecs(t *testing.T) {
d := newTestDeps(t)
d.Registry = &testRegistry{specs: nil}
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("POST", "/trigger", nil)
req.Header.Set("Content-Type", "application/json")
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
if !strings.Contains(w.Body.String(), "No agents") {
t.Error("expected 'No agents' message")
}
}

func TestHandleMCPTools_Initialized(t *testing.T) {
d := newTestDeps(t)
// Create a registry with tools
mcpReg := mcp.NewRegistry()
d.MCPReg = mcpReg
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/mcp-tools", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}

func TestHandleOperations_History(t *testing.T) {
d := newTestDeps(t)
now := time.Now()
d.OpsMgr.Record("agent1", true, now, now, 1.0, "")
d.OpsMgr.Record("agent1", false, now, now, 2.0, "err")
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/operations?type=history&agent_name=agent1&limit=10", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
}
if !strings.Contains(w.Body.String(), "history") {
t.Error("expected history type in response")
}
}

func TestHandleOperations_Summary(t *testing.T) {
d := newTestDeps(t)
now := time.Now()
d.OpsMgr.Record("agent1", true, now, now, 1.0, "")
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/operations?type=summary", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
if !strings.Contains(w.Body.String(), "summary") {
t.Error("expected summary type in response")
}
}

func TestHandleReady_WithScheduler(t *testing.T) {
d := newTestDeps(t)
env := runtime.NewEnvironment(nil)
reg := &testRegistry{}
sched := scheduler.NewWithOptions(
scheduler.ModeSimple, reg, nil, env, "", 5*time.Second, 0, nil, nil, nil, nil,
)
d.Scheduler = sched
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/ready", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
if !strings.Contains(w.Body.String(), "ready") {
t.Error("expected 'ready' in response")
}
}

func TestHandleSystem_WithReports(t *testing.T) {
d := newTestDeps(t)
d.ReportMgr.Save("# Report 1")
d.ReportMgr.Save("# Report 2")
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/system", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
if !strings.Contains(w.Body.String(), "reports") {
t.Error("expected reports in response")
}
}

func TestHandleReportsList_WithStartEnd(t *testing.T) {
d := newTestDeps(t)
d.ReportMgr.Save("# Report")
r := setupTestRouter(d)

now := time.Now()
past := now.Add(-time.Hour).Unix()
future := now.Add(time.Hour).Unix()
w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/reports?start_time="+strings.TrimSpace(strings.ReplaceAll(time.Unix(past, 0).Format("20060102"), " ", ""))+"&end_time="+strings.TrimSpace(""), nil)
_ = req
req = httptest.NewRequest("GET", fmt.Sprintf("/api/reports?start_time=%d&end_time=%d", past, future), nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}

func TestHandleReportDetail_InvalidID(t *testing.T) {
d := newTestDeps(t)
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/report?id=nonexistent", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusNotFound {
t.Errorf("expected 404 for invalid ID, got %d", w.Code)
}
}

// ---- GinAuthMiddleware ----

func TestGinAuthMiddleware_Pass(t *testing.T) {
os.Unsetenv("API_TOKEN")
os.Unsetenv("K8SOPS_API_TOKEN")
os.Unsetenv("API_TOKENS")
tm := NewTokenManager([]string{"test-token"})
middleware := GinAuthMiddleware(tm, nil, nil)

gin.SetMode(gin.TestMode)
r := gin.New()
r.Use(middleware)
r.GET("/test", func(c *gin.Context) { c.JSON(http.StatusOK, "ok") })

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/test", nil)
req.Header.Set("X-API-Key", "test-token")
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}

func TestGinAuthMiddleware_Unauthorized(t *testing.T) {
os.Unsetenv("API_TOKEN")
os.Unsetenv("K8SOPS_API_TOKEN")
os.Unsetenv("API_TOKENS")
tm := NewTokenManager([]string{"test-token"})
middleware := GinAuthMiddleware(tm, nil, nil)

gin.SetMode(gin.TestMode)
r := gin.New()
r.Use(middleware)
r.GET("/test", func(c *gin.Context) { c.JSON(http.StatusOK, "ok") })

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/test", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusUnauthorized {
t.Errorf("expected 401, got %d", w.Code)
}
}

func TestGinAuthMiddleware_ExemptPath(t *testing.T) {
os.Unsetenv("API_TOKEN")
os.Unsetenv("K8SOPS_API_TOKEN")
os.Unsetenv("API_TOKENS")
tm := NewTokenManager([]string{"test-token"})
middleware := GinAuthMiddleware(tm, []string{"/health"}, nil)

gin.SetMode(gin.TestMode)
r := gin.New()
r.Use(middleware)
r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, "ok") })

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/health", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200 for exempt path, got %d", w.Code)
}
}

func TestGinAuthMiddleware_RateLimited(t *testing.T) {
os.Unsetenv("API_TOKEN")
os.Unsetenv("K8SOPS_API_TOKEN")
os.Unsetenv("API_TOKENS")
tm := NewTokenManager([]string{"test-token"})
rl := NewAuthRateLimiter(1, time.Minute, 300)
rl.RecordFailure("192.0.2.1")
rl.RecordFailure("192.0.2.1") // triggers block

middleware := GinAuthMiddleware(tm, nil, rl)

gin.SetMode(gin.TestMode)
r := gin.New()
r.Use(middleware)
r.GET("/test", func(c *gin.Context) { c.JSON(http.StatusOK, "ok") })

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/test", nil)
req.RemoteAddr = "192.0.2.1:1234"
r.ServeHTTP(w, req)
if w.Code != http.StatusTooManyRequests {
t.Errorf("expected 429, got %d", w.Code)
}
}

func TestHandleTrigger_WithSpecs_NilExecutor(t *testing.T) {
	// When no scheduler and executor is nil, trigger returns no-agents error
	d := newTestDeps(t)
	d.Registry = &testRegistry{specs: nil}
	r := setupTestRouter(d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/trigger", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for no-agents trigger, got %d", w.Code)
	}
}

func TestHandleTrigger_NoMatchingAgents(t *testing.T) {
d := newTestDeps(t)
d.Registry = &testRegistry{
specs: []agent.Spec{{Name: "NodeHealth"}},
}
r := setupTestRouter(d)

body := `{"input":[{"content":[{"type":"text","text":"agent_names: UnknownAgent"}]}]}`
w := httptest.NewRecorder()
req := httptest.NewRequest("POST", "/trigger", strings.NewReader(body))
req.Header.Set("Content-Type", "application/json")
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
if !strings.Contains(w.Body.String(), "No matching agents") {
t.Errorf("expected 'No matching agents', got: %s", w.Body.String())
}
}

func TestHandleTrigger_WithScheduler(t *testing.T) {
	d := newTestDeps(t)
	// Use empty registry so RunOneRound has nothing to execute (avoids nil-executor panic)
	d.Registry = &testRegistry{specs: []agent.Spec{}}
	sEnv := runtime.NewEnvironment(nil)
	d.Scheduler = scheduler.NewWithOptions(
		scheduler.ModeSimple, &testRegistry{}, nil, sEnv, "", 5*time.Second, 0, nil, nil, nil, nil,
	)
	r := setupTestRouter(d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/trigger", nil)
	r.ServeHTTP(w, req)
	// Empty registry returns "No agents" - but scheduler is set
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
func TestHandleReportPOST_MissingContent(t *testing.T) {
d := newTestDeps(t)
r := setupTestRouter(d)

body := `{"other": "field"}`
w := httptest.NewRecorder()
req := httptest.NewRequest("POST", "/api/report", strings.NewReader(body))
req.Header.Set("Content-Type", "application/json")
r.ServeHTTP(w, req)
if w.Code != http.StatusBadRequest {
t.Errorf("expected 400 for missing content, got %d", w.Code)
}
}

func TestHandleReportPOST_WithInputField(t *testing.T) {
d := newTestDeps(t)
r := setupTestRouter(d)

body := `{"input":[{"content":[{"type":"text","text":"# Report via input field"}]}]}`
w := httptest.NewRecorder()
req := httptest.NewRequest("POST", "/api/report", strings.NewReader(body))
req.Header.Set("Content-Type", "application/json")
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
}
}

func TestHandleReportsList_POST(t *testing.T) {
d := newTestDeps(t)
d.ReportMgr.Save("# Test Report")
r := setupTestRouter(d)

body := `{"input":[{"content":[{"type":"text","text":"limit: 5, offset: 0"}]}]}`
w := httptest.NewRecorder()
req := httptest.NewRequest("POST", "/api/reports", strings.NewReader(body))
req.Header.Set("Content-Type", "application/json")
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}

func TestHandleOperations_POST(t *testing.T) {
d := newTestDeps(t)
now := time.Now()
d.OpsMgr.Record("agent1", true, now, now, 1.0, "")
r := setupTestRouter(d)

body := `{"input":[{"content":[{"type":"text","text":"type: summary"}]}]}`
w := httptest.NewRecorder()
req := httptest.NewRequest("POST", "/api/operations", strings.NewReader(body))
req.Header.Set("Content-Type", "application/json")
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}

func TestHandleOperations_HistoryWithFilter(t *testing.T) {
d := newTestDeps(t)
now := time.Now()
d.OpsMgr.Record("agent1", true, now, now, 1.0, "")
d.OpsMgr.Record("agent2", false, now, now, 2.0, "err")
r := setupTestRouter(d)

w := httptest.NewRecorder()
req := httptest.NewRequest("GET", "/api/operations?type=history&success_only=true", nil)
r.ServeHTTP(w, req)
if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}
