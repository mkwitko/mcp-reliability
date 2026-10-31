#!/usr/bin/env bash
# Captures the openllmetry (opentelemetry-instrumentation-mcp) scenarios into
# testdata/. Uses its own collector instance (port 4319, out-openllmetry/,
# compose project col-openllmetry) and MCP server port 8766, so it can run
# next to examples/run-scenarios.sh. Requires docker compose and uv.
set -euo pipefail
cd "$(dirname "$0")/../.."
export COLLECTOR_PORT=4319 COLLECTOR_OUT=./out-openllmetry
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:$COLLECTOR_PORT
COMPOSE="docker compose -p col-openllmetry -f examples/collector/docker-compose.yml"
OUT=examples/collector/out-openllmetry
PYDIR=examples/openllmetry-python
PY=$PYDIR/.venv/bin/python # run python directly: SIGINT must reach the server
URL=http://127.0.0.1:8766/
mkdir -p "$OUT" testdata
chmod 777 "$OUT" # the collector runs as uid 10001 and must create the file
(cd "$PYDIR" && uv sync --locked -q)
trap '$COMPOSE down >/dev/null 2>&1' EXIT

scenario() { # name client-flags...
  local name=$1; shift
  # The exporter holds the file open, so restart the collector per scenario.
  $COMPOSE down >/dev/null 2>&1 || true
  rm -f "$OUT/traces.jsonl"
  $COMPOSE up -d >/dev/null 2>&1
  sleep 3
  $PY $PYDIR/server.py --port 8766 &
  local pid=$!
  sleep 3
  $PY $PYDIR/client.py --url $URL "$@"
  kill -INT $pid; wait $pid || true
  sleep 3 # batch timeout + file flush
  cp "$OUT/traces.jsonl" "testdata/$name.jsonl"
  echo "wrote testdata/$name.jsonl ($(wc -l < "testdata/$name.jsonl") lines)"
}

scenario openllmetry-default
scenario openllmetry-with-meta --meta
scenario openllmetry-legacy-transport --legacy-transport
