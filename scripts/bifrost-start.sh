#!/bin/sh
set -eu

# Start Bifrost AI gateway with all configured providers.
# Requires: docker, API keys in environment (source .env first).
#
# Usage:
#   source .env && export $(grep -v '^#' .env | grep -v '^$' | xargs)
#   ./scripts/bifrost-start.sh
#
# Then use with bench-cli:
#   export INFRA_BENCH_BIFROST_URL=http://localhost:9090/v1
#   bench-cli run --provider bifrost --model openai/gpt-4o-mini ...
#   bench-cli run --provider bifrost --model deepseek/deepseek-chat ...
#   bench-cli run --provider bifrost --model google/gemini-2.5-flash ...

BIFROST_PORT="${BIFROST_PORT:-9090}"
CONTAINER_NAME="bench-bifrost"

echo "Starting Bifrost gateway on port ${BIFROST_PORT}..."

# Stop existing container.
docker rm -f "${CONTAINER_NAME}" 2>/dev/null || true

# Start with native provider keys auto-detected.
docker run -d --name "${CONTAINER_NAME}" \
  -p "${BIFROST_PORT}:8080" \
  -v bifrost-data:/app/data \
  -e OPENAI_API_KEY="${OPENAI_API_KEY:-}" \
  -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" \
  -e GROQ_API_KEY="${GROQ_API_KEY:-}" \
  maximhq/bifrost:v1.4.17

echo "Waiting for Bifrost to start..."
sleep 8

BIFROST_URL="http://localhost:${BIFROST_PORT}"

# Register custom OpenAI-compatible providers via API.
register_provider() {
  local provider="$1" key="$2" base_url="$3"
  if [ -z "$key" ]; then
    echo "  SKIP ${provider} (no API key)"
    return
  fi
  curl -sf -X POST "${BIFROST_URL}/api/providers" \
    -H "Content-Type: application/json" \
    -d "{
      \"provider\": \"${provider}\",
      \"keys\": [{\"name\": \"${provider}-key\", \"value\": \"${key}\", \"models\": [], \"weight\": 1.0}],
      \"network_config\": {\"base_url\": \"${base_url}\"},
      \"custom_provider_config\": {
        \"base_provider_type\": \"openai\",
        \"allowed_requests\": {\"chat_completion\": true, \"chat_completion_stream\": true}
      }
    }" > /dev/null 2>&1
  echo "  OK   ${provider}"
}

echo "Registering providers..."
echo "  OK   openai (auto-detected)"
echo "  OK   anthropic (auto-detected)"

register_provider "deepseek" "${DEEPSEEK_API_KEY:-}" "https://api.deepseek.com"
register_provider "google" "${GEMINI_API_KEY:-}" "https://generativelanguage.googleapis.com/v1beta/openai"
register_provider "qwen" "${DASHSCOPE_API_KEY:-}" "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"

echo ""
echo "Bifrost ready at ${BIFROST_URL}"
echo ""
echo "Usage:"
echo "  export INFRA_BENCH_BIFROST_URL=${BIFROST_URL}/v1"
echo "  bench-cli run --provider bifrost --model openai/gpt-4o-mini ..."
echo "  bench-cli run --provider bifrost --model deepseek/deepseek-chat ..."
echo "  bench-cli run --provider bifrost --model google/gemini-2.5-flash ..."
