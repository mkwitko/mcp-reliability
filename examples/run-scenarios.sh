#!/usr/bin/env bash
# Runs each scenario against a local collector and copies the resulting
# newline-delimited OTLP JSON into testdata/. Requires docker compose and Go.
set -euo pipefail
cd "$(dirname "$0")/.."
COMPOSE="docker compose -f examples/collector/docker-compose.yml"
OUT=examples/collector/out
BIN=$(mktemp -d)/example
mkdir -p "$OUT" testdata
chmod 777 "$OUT" # the collector runs as uid 10001 and must create the file
# Build once and run the binary directly: `go run` does not forward SIGINT.
go build -o "$BIN" ./examples/go-sdk-streamable-http
trap '$COMPOSE down >/dev/null 2>&1' EXIT

scenario() { # name server-flags client-flags
  local name=$1 sflags=$2 cflags=$3
  # The exporter holds the file open, so restart the collector per scenario
  # instead of deleting the file underneath it.
  $COMPOSE down >/dev/null 2>&1 || true
  rm -f "$OUT/traces.jsonl" "testdata/$name.wire.jsonl"
  $COMPOSE up -d >/dev/null 2>&1
  sleep 3
  "$BIN" -role server -capture "testdata/$name.wire.jsonl" $sflags &
  local pid=$!
  sleep 1
  "$BIN" -role client $cflags
  kill -INT $pid; wait $pid || true
  sleep 3 # batch timeout + file flush
  cp "$OUT/traces.jsonl" "testdata/$name.jsonl"
  echo "wrote testdata/$name.jsonl ($(wc -l < "testdata/$name.jsonl") lines)"
}

scenario healthy "" ""
scenario broken-propagation "" "-break-propagation"
scenario broken-error-type "-break-error-type" ""
scenario leaked-arguments "-leak-arguments" ""
scenario missing-method-attr "-drop-method-attr" ""
scenario bad-traceparent "" "-bad-traceparent"
