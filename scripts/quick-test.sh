#!/usr/bin/env bash
# Kube Ops Agent quick test script
# Usage: ./scripts/quick-test.sh [dry-run|run|test-api]
#
# Planning mode: default LLM self-planning; set K8SOPS_WORKFLOW for Workflow static orchestration

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Config (aligned with main program defaults)
SKILLS_DIR="${K8SOPS_SKILLS_DIR:-$PROJECT_ROOT/kubernetes-ops-agent/skills}"
REPORT_DIR="${K8SOPS_REPORT_DIR:-$PROJECT_ROOT/kubernetes-ops-agent/report}"
HTTP_ADDR="${K8SOPS_HTTP_ADDR:-:8080}"
WORKFLOW_PATH="${K8SOPS_WORKFLOW:-}"
BINARY="$PROJECT_ROOT/k8sops"

# Load env (prefer provider-specific, fallback to test.env)
if [[ -f "$PROJECT_ROOT/config/openai.env" ]]; then
  set -a && source "$PROJECT_ROOT/config/openai.env" && set +a
elif [[ -f "$PROJECT_ROOT/config/anthropic.env" ]]; then
  set -a && source "$PROJECT_ROOT/config/anthropic.env" && set +a
elif [[ -f "$PROJECT_ROOT/config/test.env" ]]; then
  set -a && source "$PROJECT_ROOT/config/test.env" && set +a
fi

# Check prerequisites (require_api_key: dry-run does not need it)
check_prereqs() {
  local require_api_key="${1:-true}"
  local missing=()

  [[ "$require_api_key" = "true" && -z "$OPENAI_API_KEY" && -z "$DASHSCOPE_API_KEY" && -z "$ANTHROPIC_API_KEY" ]] && missing+=("OPENAI_API_KEY, DASHSCOPE_API_KEY, or ANTHROPIC_API_KEY")
  if [[ ${#missing[@]} -gt 0 ]]; then
    echo "Error: missing env vars: ${missing[*]}"
    echo "Set OPENAI_API_KEY, DASHSCOPE_API_KEY, or ANTHROPIC_API_KEY, or create config/test.env"
    echo "Example: cp config/test.env.example config/test.env"
    exit 1
  fi

  if [[ ! -d "$SKILLS_DIR" ]]; then
    echo "Error: skills dir not found: $SKILLS_DIR"
    exit 1
  fi

  if ! command -v kubectl &>/dev/null; then
    echo "Warning: kubectl not found, Chat kubectl queries will be unavailable"
  fi
}

build() {
  if [[ ! -f "$BINARY" ]]; then
    echo "Building..."
    go build -o "$BINARY" ./cmd/k8sops
  fi
}

# dry-run: print registered agents only
cmd_dry_run() {
  check_prereqs false
  build
  echo "=== Dry Run: Registered Agents ==="
  local wf=""
  [[ -n "$WORKFLOW_PATH" ]] && { [[ "$WORKFLOW_PATH" == /* ]] && wf="$WORKFLOW_PATH" || wf="$PROJECT_ROOT/$WORKFLOW_PATH"; }
  [[ -n "$WORKFLOW_PATH" ]] && echo "Planning mode: Workflow ($WORKFLOW_PATH)" || echo "Planning mode: LLM self-planning"
  local args=("--skills-dir" "$SKILLS_DIR" "--report-dir" "$REPORT_DIR" "--dry-run")
  [[ -n "$wf" ]] && [[ -f "$wf" ]] && args+=("--workflow" "$wf")
  "$BINARY" "${args[@]}"
}

# run: start service (foreground)
cmd_run() {
  check_prereqs true
  build
  mkdir -p "$REPORT_DIR"
  echo "Starting service: $HTTP_ADDR"
  echo "Skills dir: $SKILLS_DIR"
  echo "Report dir: $REPORT_DIR"
  local wf=""
  [[ -n "$WORKFLOW_PATH" ]] && { [[ "$WORKFLOW_PATH" == /* ]] && wf="$WORKFLOW_PATH" || wf="$PROJECT_ROOT/$WORKFLOW_PATH"; }
  [[ -n "$WORKFLOW_PATH" ]] && echo "Planning mode: Workflow ($WORKFLOW_PATH)" || echo "Planning mode: LLM self-planning"
  echo "Press Ctrl+C to stop"
  local args=("--skills-dir" "$SKILLS_DIR" "--report-dir" "$REPORT_DIR" "--addr" "$HTTP_ADDR")
  [[ -n "$wf" ]] && [[ -f "$wf" ]] && args+=("--workflow" "$wf")
  exec "$BINARY" "${args[@]}"
}

# test-api: start service and test API
cmd_test_api() {
  check_prereqs true
  build
  mkdir -p "$REPORT_DIR"

  echo "Starting service (background)..."
  local wf=""
  [[ -n "$WORKFLOW_PATH" ]] && { [[ "$WORKFLOW_PATH" == /* ]] && wf="$WORKFLOW_PATH" || wf="$PROJECT_ROOT/$WORKFLOW_PATH"; }
  local args=("--skills-dir" "$SKILLS_DIR" "--report-dir" "$REPORT_DIR" "--addr" "$HTTP_ADDR")
  [[ -n "$wf" ]] && [[ -f "$wf" ]] && args+=("--workflow" "$wf")
  "$BINARY" "${args[@]}" &
  PID=$!
  trap "kill $PID 2>/dev/null" EXIT

  # Resolve address for curl (:8080 -> 127.0.0.1:8080)
  local base_url="http://127.0.0.1${HTTP_ADDR}"

  echo "Waiting for service ready..."
  for i in {1..30}; do
    if curl -s "${base_url}/health" &>/dev/null; then
      break
    fi
    sleep 0.5
  done

  if ! curl -s "${base_url}/health" &>/dev/null; then
    echo "Error: service not ready within 30s"
    exit 1
  fi

  echo ""
  echo "=== Test /health ==="
  curl -s "${base_url}/health" | head -20

  echo ""
  echo "=== Test /ready ==="
  curl -s "${base_url}/ready"

  echo ""
  echo "=== Test /api/system ==="
  curl -s "${base_url}/api/system" | head -15

  echo ""
  echo "=== Test POST /trigger ==="
  curl -s -X POST "${base_url}/trigger" | head -5

  echo ""
  echo "=== Test complete ==="
  echo "Service still running (PID=$PID), press Enter to stop..."
  read -r
}

case "${1:-run}" in
  dry-run)  cmd_dry_run ;;
  run)      cmd_run ;;
  test-api) cmd_test_api ;;
  *)
    echo "Usage: $0 [dry-run|run|test-api]"
    echo ""
    echo "  dry-run   Print registered agents only, do not start service"
    echo "  run       Start service (foreground)"
    echo "  test-api  Start service and test API"
    echo ""
    echo "Env vars: OPENAI_API_KEY, K8SOPS_SKILLS_DIR, K8SOPS_REPORT_DIR, K8SOPS_WORKFLOW"
    echo "Planning: default LLM self-planning; set K8SOPS_WORKFLOW for Workflow"
    exit 1
    ;;
esac
