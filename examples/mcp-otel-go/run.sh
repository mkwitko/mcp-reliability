#!/usr/bin/env bash
# E1/H1 for github.com/olgasafonova/mcp-otel-go. Own collector (port 4320), server port 8767.
# Usage: examples/mcp-otel-go/run.sh   (requires docker compose and Go)
set -euo pipefail
cd "$(dirname "$0")/../.."
COMPOSE="docker compose -p col-mcpotel -f examples/collector/docker-compose.yml"
export COLLECTOR_PORT=4320 COLLECTOR_OUT=./out-mcpotel
OUT=examples/collector/out-mcpotel
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4320
BIN=$(mktemp -d)/mcpotel
mkdir -p "$OUT" testdata
chmod 777 "$OUT"
(cd examples/mcp-otel-go && go build -o "$BIN" .)
trap '$COMPOSE down >/dev/null 2>&1' EXIT

scenario() { # name client-flags
  local name=$1 cflags=$2
  $COMPOSE down >/dev/null 2>&1 || true
  rm -f "$OUT/traces.jsonl"
  $COMPOSE up -d >/dev/null 2>&1
  sleep 3
  "$BIN" -role server &
  local pid=$!
  sleep 1
  "$BIN" -role client $cflags
  kill -INT $pid; wait $pid || true
  sleep 3
  cp "$OUT/traces.jsonl" "testdata/$name.jsonl"
  echo "wrote testdata/$name.jsonl ($(wc -l < "testdata/$name.jsonl") lines)"
}

scenario mcp-otel-go-default ""
scenario mcp-otel-go-client-inject "-inject"
