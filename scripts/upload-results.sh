#!/bin/bash
# Upload local results.jsonl to evidra API in batches.
# Usage: ./scripts/upload-results.sh <evidra-url> <api-key> [results.jsonl]
set -euo pipefail

EVIDRA_URL="${1:?Usage: upload-results.sh <evidra-url> <api-key> [results.jsonl]}"
API_KEY="${2:?Usage: upload-results.sh <evidra-url> <api-key> [results.jsonl]}"
JSONL="${3:-runs/results.jsonl}"
BATCH_SIZE=50

if [ ! -f "$JSONL" ]; then
  echo "File not found: $JSONL"
  exit 1
fi

TOTAL=$(wc -l < "$JSONL" | tr -d ' ')
echo "Uploading $TOTAL records from $JSONL to $EVIDRA_URL (batch size: $BATCH_SIZE)"

UPLOADED=0
FAILED=0

# Split into batches and POST
while IFS= read -r batch; do
  PAYLOAD="{\"runs\": [$batch]}"
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "$EVIDRA_URL/v1/bench/runs/batch" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $API_KEY" \
    -d "$PAYLOAD")

  BATCH_COUNT=$(echo "$batch" | tr ',' '\n' | wc -l | tr -d ' ')

  if [ "$HTTP_CODE" -lt 400 ]; then
    UPLOADED=$((UPLOADED + BATCH_COUNT))
    echo "  [$UPLOADED/$TOTAL] uploaded (HTTP $HTTP_CODE)"
  else
    FAILED=$((FAILED + BATCH_COUNT))
    echo "  [$UPLOADED/$TOTAL] FAILED batch (HTTP $HTTP_CODE)"
  fi
done < <(
  # Read JSONL, join lines with commas in batches
  awk -v bs="$BATCH_SIZE" '
    NR > 1 && (NR-1) % bs == 0 { print buf; buf = $0; next }
    NR == 1 { buf = $0; next }
    { buf = buf "," $0 }
    END { if (buf) print buf }
  ' "$JSONL"
)

echo ""
echo "Done. Uploaded: $UPLOADED, Failed: $FAILED, Total: $TOTAL"
