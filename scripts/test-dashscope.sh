#!/usr/bin/env bash
# Kube Ops Agent - DashScope (Alibaba Cloud Qwen) test script
# Usage: ./scripts/test-dashscope.sh [dry-run|run|test-api]
#
# Prerequisite: copy config/dashscope.env.example to config/dashscope.env and fill in DASHSCOPE_API_KEY

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

SKILLS_DIR="${K8SOPS_SKILLS_DIR:-$PROJECT_ROOT/kubernetes-ops-agent/skills}"
REPORT_DIR="${K8SOPS_REPORT_DIR:-$PROJECT_ROOT/kubernetes-ops-agent/report}"
HTTP_ADDR="${K8SOPS_HTTP_ADDR:-:8080}"
WORKFLOW_PATH="${K8SOPS_WORKFLOW:-}"
BINARY="$PROJECT_ROOT/k8sops"

# Prefer dashscope.env, fallback to test.env
if [[ -f "$PROJECT_ROOT/config/dashscope.env" ]]; then
  set -a
  source "$PROJECT_ROOT/config/dashscope.env"
  set +a
elif [[ -f "$PROJECT_ROOT/config/test.env" ]]; then
  set -a
  source "$PROJECT_ROOT/config/test.env"
  set +a
fi

# DashScope requires DASHSCOPE_API_KEY or OPENAI_API_KEY, and OPENAI_BASE_URL
check_prereqs() {
  local require_api_key="${1:-true}"
  local missing=()

  if [[ "$require_api_key" = "true" ]]; then
    if [[ -z "$DASHSCOPE_API_KEY" && -z "$OPENAI_API_KEY" ]]; then
      missing+=("DASHSCOPE_API_KEY or OPENAI_API_KEY")
    else
      # Auto-set Base URL when using DashScope (if not configured)
      [[ -z "$OPENAI_BASE_URL" ]] && export OPENAI_BASE_URL="https://dashscope.aliyuncs.com/compatible-mode/v1"
    fi
  fi

  if [[ ${#missing[@]} -gt 0 ]]; then
    echo "Error: missing config: ${missing[*]}"
    echo "Create config/dashscope.env: cp config/dashscope.env.example config/dashscope.env"
    echo "And fill in DASHSCOPE_API_KEY"
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

cmd_dry_run() {
  check_prereqs false
  build
  echo "=== DashScope Test - Dry Run ==="
  echo "Model: ${OPENAI_MODEL:-qwen-plus}"
  echo "Base URL: ${OPENAI_BASE_URL:-not set}"
  [[ -n "$DASHSCOPE_API_KEY" ]] && echo "API Key: set (DASHSCOPE_API_KEY)" || echo "API Key: set (OPENAI_API_KEY)"
  echo ""
  "$BINARY" --skills-dir "$SKILLS_DIR" --report-dir "$REPORT_DIR" --model "${OPENAI_MODEL:-qwen-plus}" --dry-run
}

cmd_run() {
  check_prereqs true
  build
  mkdir -p "$REPORT_DIR"
  echo "=== DashScope Test - Starting Service ==="
  echo "Model: ${OPENAI_MODEL:-qwen-plus}"
  echo "Base URL: ${OPENAI_BASE_URL:-https://dashscope.aliyuncs.com/compatible-mode/v1}"
  echo "Skills dir: $SKILLS_DIR"
  echo "Report dir: $REPORT_DIR"
  echo "Press Ctrl+C to stop"
  local args=("--skills-dir" "$SKILLS_DIR" "--report-dir" "$REPORT_DIR" "--addr" "$HTTP_ADDR" "--model" "${OPENAI_MODEL:-qwen-plus}")
  local wf=""
  [[ -n "$WORKFLOW_PATH" ]] && { [[ "$WORKFLOW_PATH" == /* ]] && wf="$WORKFLOW_PATH" || wf="$PROJECT_ROOT/$WORKFLOW_PATH"; }
  [[ -n "$wf" ]] && [[ -f "$wf" ]] && args+=("--workflow" "$wf")
  exec "$BINARY" "${args[@]}"
}

cmd_test_api() {
  check_prereqs true
  build
  mkdir -p "$REPORT_DIR"

  echo "=== DashScope Test - Starting Service and Testing API ==="
  local args=("--skills-dir" "$SKILLS_DIR" "--report-dir" "$REPORT_DIR" "--addr" "$HTTP_ADDR" "--model" "${OPENAI_MODEL:-qwen-plus}")
  local wf=""
  [[ -n "$WORKFLOW_PATH" ]] && { [[ "$WORKFLOW_PATH" == /* ]] && wf="$WORKFLOW_PATH" || wf="$PROJECT_ROOT/$WORKFLOW_PATH"; }
  [[ -n "$wf" ]] && [[ -f "$wf" ]] && args+=("--workflow" "$wf")
  "$BINARY" "${args[@]}" &
  PID=$!
  trap "kill $PID 2>/dev/null" EXIT

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
  echo "=== DashScope test complete ==="
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
    echo "  dry-run   Check agent registration only (requires config/dashscope.env)"
    echo "  run       Start service"
    echo "  test-api  Start and test API"
    echo ""
    echo "Config: cp config/dashscope.env.example config/dashscope.env"
    echo "        Edit config/dashscope.env and fill in DASHSCOPE_API_KEY"
    exit 1
    ;;
esac
