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

require_png_dimensions() {
  local path="$1"
  local width="$2"
  local height="$3"
  local info

  info="$(file "$path")"
  if [[ "$info" != *"PNG image data, ${width} x ${height}"* ]]; then
    echo "FAIL: $path should be a ${width}x${height} PNG" >&2
    echo "file reported: $info" >&2
    exit 1
  fi
}

require_nginx_location_contains() {
  local location="$1"
  local needle="$2"

  awk -v header="location $location {" -v needle="$needle" '
    BEGIN { status = 1 }
    function trim_left(s) {
      sub(/^[[:space:]]+/, "", s)
      return s
    }
    {
      line = trim_left($0)
      if (!in_block && line == header) {
        in_block = 1
        depth = 1
        if (index($0, needle) > 0) {
          found = 1
        }
        next
      }
      if (in_block) {
        if (index($0, needle) > 0) {
          found = 1
        }
        opens = gsub(/\{/, "{")
        closes = gsub(/\}/, "}")
        depth += opens - closes
        if (depth <= 0) {
          status = found ? 0 : 1
          exit
        }
      }
    }
    END { exit status }
  ' ui/nginx.conf || {
    echo "FAIL: nginx location $location should contain $needle" >&2
    exit 1
  }
}

require_file ui/index.html
require_contains ui/index.html "<title>Evidra Bench - AI SRE Agent Benchmark Reports</title>"
require_contains ui/index.html "<link rel=\"canonical\" href=\"https://bench.evidra.cc/\""
require_contains ui/index.html "<meta name=\"msvalidate.01\" content=\"A234E0924D11FE0B9479E58891ACC420\""
require_contains ui/index.html "<meta property=\"og:title\""
require_contains ui/index.html "<meta property=\"og:image\" content=\"https://bench.evidra.cc/og-bench.png\""
require_contains ui/index.html "<meta name=\"twitter:card\" content=\"summary_large_image\""
require_contains ui/index.html "application/ld+json"
require_contains ui/index.html "AI infrastructure agent benchmark"
require_contains ui/index.html "AI SRE benchmarks"
require_contains ui/index.html "failure-mode breakdowns"
require_contains ui/index.html "MCP server benchmark"
require_file ui/public/og-bench.png
require_png_dimensions ui/public/og-bench.png 1200 630

require_file ui/public/google8a0c3bd916294bb0.html
require_contains ui/public/google8a0c3bd916294bb0.html "google-site-verification: google8a0c3bd916294bb0.html"

require_file ui/public/robots.txt
require_contains ui/public/robots.txt "Sitemap: https://bench.evidra.cc/sitemap.xml"

require_file ui/public/_redirects
require_contains ui/public/_redirects "/bench/reports/kubernetes-mcp-readiness-2026-05 /bench/reports/kubernetes-mcp-readiness-2026-05/index.html 200"
require_contains ui/public/_redirects "/bench/articles/what-ai-sre-benchmarks-should-catch-before-production /bench/articles/what-ai-sre-benchmarks-should-catch-before-production/index.html 200"
require_contains ui/public/_redirects "/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough /bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough/index.html 200"

require_file ui/nginx.conf
require_contains ui/nginx.conf 'try_files $uri $uri/index.html $uri/ /index.html;'
require_contains ui/nginx.conf 'location = /bench {'
require_contains ui/nginx.conf 'location = /bench/ {'
require_contains ui/nginx.conf 'location = /bench/runs {'
require_nginx_location_contains '= /bench/runs' 'X-Robots-Tag "noindex, follow" always;'
if grep -Eq 'location ~ .*insights.*X-Robots-Tag "noindex, follow"|location ~ .*insights' ui/nginx.conf; then
  echo "FAIL: /bench/insights must not match an nginx noindex route" >&2
  exit 1
fi
require_contains ui/nginx.conf 'location = /results {'
require_contains ui/nginx.conf 'location = /results/ {'
require_contains ui/nginx.conf 'return 301 https://bench.evidra.cc/bench/runs;'
require_contains ui/nginx.conf 'location = /sitemap.xml {'
require_contains ui/nginx.conf 'location = /sitemap.xml/ {'
require_contains ui/nginx.conf 'return 301 https://bench.evidra.cc/sitemap.xml;'
require_contains ui/nginx.conf 'location = /robots.txt {'
require_contains ui/nginx.conf 'location = /robots.txt/ {'
require_contains ui/nginx.conf 'return 301 https://bench.evidra.cc/robots.txt;'

require_file ui/public/sitemap.xml
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/bench/reports/kubernetes-mcp-readiness-2026-05</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/bench/articles/what-ai-sre-benchmarks-should-catch-before-production</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/open-infrastructure-agent-benchmarks/</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/kubernetes-ai-agent-benchmark/</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/mcp-server-benchmark/</loc>"
require_contains ui/public/sitemap.xml "<loc>https://bench.evidra.cc/ai-sre-regression-testing/</loc>"
if grep -Eq '<loc>https://bench\.evidra\.cc/bench/(leaderboard|dashboard|skill-impact|regressions|insights|reviews|session|runs|scenarios|compare|mcp-readiness|benchmarks|sample-report)(/|</loc>)' ui/public/sitemap.xml; then
  echo "FAIL: sitemap should not include SPA app routes with root canonical" >&2
  exit 1
fi

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
require_contains ui/public/open-infrastructure-agent-benchmarks/index.html "Evidra Bench and Kubeply Infra-Bench"
require_contains ui/public/open-infrastructure-agent-benchmarks/index.html "<link rel=\"canonical\" href=\"https://bench.evidra.cc/open-infrastructure-agent-benchmarks/\""

require_file ui/public/bench/reports/kubernetes-mcp-readiness-2026-05/index.html
require_contains ui/public/bench/reports/kubernetes-mcp-readiness-2026-05/index.html "<title>Kubernetes MCP Readiness 2026-05 - Evidra Bench</title>"
require_contains ui/public/bench/reports/kubernetes-mcp-readiness-2026-05/index.html "<link rel=\"canonical\" href=\"https://bench.evidra.cc/bench/reports/kubernetes-mcp-readiness-2026-05\""
require_contains ui/public/bench/reports/kubernetes-mcp-readiness-2026-05/index.html "100% final-state pass rate"
require_contains ui/public/bench/reports/kubernetes-mcp-readiness-2026-05/index.html "unsafe-pass"
require_contains ui/public/bench/reports/kubernetes-mcp-readiness-2026-05/index.html "application/ld+json"

require_file ui/public/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough/index.html
require_contains ui/public/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough/index.html "<title>Kubernetes MCP Servers Passed. That Was Not Enough. - Evidra Bench</title>"
require_contains ui/public/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough/index.html "<link rel=\"canonical\" href=\"https://bench.evidra.cc/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough\""
require_contains ui/public/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough/index.html "Did the agent pass safely?"
require_contains ui/public/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough/index.html "application/ld+json"

require_file ui/public/bench/articles/what-ai-sre-benchmarks-should-catch-before-production/index.html
require_contains ui/public/bench/articles/what-ai-sre-benchmarks-should-catch-before-production/index.html "<title>What AI SRE Benchmarks Should Catch Before Production - Evidra Bench</title>"
require_contains ui/public/bench/articles/what-ai-sre-benchmarks-should-catch-before-production/index.html "<link rel=\"canonical\" href=\"https://bench.evidra.cc/bench/articles/what-ai-sre-benchmarks-should-catch-before-production\""
require_contains ui/public/bench/articles/what-ai-sre-benchmarks-should-catch-before-production/index.html "per-failure-mode breakdowns"
require_contains ui/public/bench/articles/what-ai-sre-benchmarks-should-catch-before-production/index.html "application/ld+json"

require_file README.md
require_contains README.md "# Evidra Bench"
require_contains README.md "Evidra Bench is"

echo "SEO assets OK"
