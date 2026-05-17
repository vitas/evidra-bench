#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

require_file() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "FAIL: missing $path" >&2
    exit 1
  fi
}

require_contains() {
  local path="$1"
  local needle="$2"
  if ! grep -Fq "$needle" "$path"; then
    echo "FAIL: $path should contain $needle" >&2
    exit 1
  fi
}

require_file ui/index.html
require_contains ui/index.html "<title>Evidra Bench - AI Infrastructure Agent Benchmark</title>"
require_contains ui/index.html "<link rel=\"canonical\" href=\"https://bench.evidra.cc/\""
require_contains ui/index.html "<meta name=\"msvalidate.01\" content=\"A234E0924D11FE0B9479E58891ACC420\""
require_contains ui/index.html "<meta property=\"og:title\""
require_contains ui/index.html "<meta property=\"og:image\" content=\"https://bench.evidra.cc/og-bench.png\""
require_contains ui/index.html "<meta name=\"twitter:card\" content=\"summary_large_image\""
require_contains ui/index.html "application/ld+json"
require_contains ui/index.html "AI infrastructure agent benchmark"
require_contains ui/index.html "MCP server benchmark"
require_file ui/public/og-bench.png

require_file ui/public/google8a0c3bd916294bb0.html
require_contains ui/public/google8a0c3bd916294bb0.html "google-site-verification: google8a0c3bd916294bb0.html"

require_file ui/public/robots.txt
require_contains ui/public/robots.txt "Sitemap: https://bench.evidra.cc/sitemap.xml"

require_file ui/public/sitemap.xml
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/bench/reports/kubernetes-mcp-readiness-2026-05</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/open-infrastructure-agent-benchmarks/</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/kubernetes-ai-agent-benchmark/</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/mcp-server-benchmark/</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/ai-sre-regression-testing/</loc>"

for page in \
  ui/public/kubernetes-ai-agent-benchmark/index.html \
  ui/public/mcp-server-benchmark/index.html \
  ui/public/ai-sre-regression-testing/index.html
do
  require_file "$page"
  require_contains "$page" "<link rel=\"canonical\""
  require_contains "$page" "<meta name=\"description\""
  require_contains "$page" "https://bench.evidra.cc/bench/sample-report"
  require_contains "$page" "Evidra Bench"
  require_contains "$page" "application/ld+json"
done

require_file ui/public/open-infrastructure-agent-benchmarks/index.html
require_contains ui/public/open-infrastructure-agent-benchmarks/index.html "Evidra Bench"
require_contains ui/public/open-infrastructure-agent-benchmarks/index.html "infrastructure agent benchmark"
require_contains ui/public/open-infrastructure-agent-benchmarks/index.html "Evidra Bench vs Harbor task datasets"
require_contains ui/public/open-infrastructure-agent-benchmarks/index.html "<link rel=\"canonical\" href=\"https://bench.evidra.cc/open-infrastructure-agent-benchmarks/\""

require_file README.md
require_contains README.md "# Evidra Bench"
require_contains README.md "Evidra Bench is"

echo "SEO assets OK"
