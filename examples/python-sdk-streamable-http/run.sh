#!/usr/bin/env bash
# Runs the MCP Python SDK v2 scenarios against the local collector and copies
# the newline-delimited OTLP JSON into testdata/. Mirrors examples/run-scenarios.sh.
# Requires docker compose, uv and Go.
set -euo pipefail
cd "$(dirname "$0")/../.."
COMPOSE="docker compose -f examples/collector/docker-compose.yml"
OUT=examples/collector/out
PYDIR=examples/python-sdk-streamable-http
PY=$PYDIR/.venv/bin/python # run python directly: SIGINT must reach the server
GOBIN=$(mktemp -d)/example
mkdir -p "$OUT" testdata
chmod 777 "$OUT" # the collector runs as uid 10001 and must create the file
(cd "$PYDIR" && uv sync --locked -q)
go build -o "$GOBIN" ./examples/go-sdk-streamable-http
trap '$COMPOSE down >/dev/null 2>&1' EXIT

scenario() { # name server-cmd client-cmd
  local name=$1 server=$2 client=$3
  # The exporter holds the file open, so restart the collector per scenario.
  $COMPOSE down >/dev/null 2>&1 || true
  rm -f "$OUT/traces.jsonl"
  $COMPOSE up -d >/dev/null 2>&1
  sleep 3
  $server &
  local pid=$!
  sleep 3
  $client
  kill -INT $pid; wait $pid || true
  sleep 3 # batch timeout + file flush
  cp "$OUT/traces.jsonl" "testdata/$name.jsonl"
  echo "wrote testdata/$name.jsonl ($(wc -l < "testdata/$name.jsonl") lines)"
}

scenario python-sdk-default "$PY $PYDIR/server.py" "$PY $PYDIR/client.py"
scenario cross-go-client-python-server "$PY $PYDIR/server.py" "$GOBIN -role client"
scenario cross-python-client-go-server "$GOBIN -role server" "$PY $PYDIR/client.py"
