#!/usr/bin/env bash
# Runs the MCP TypeScript SDK scenarios against the local collector and copies
# the newline-delimited OTLP JSON into testdata/. Mirrors the Python run.sh.
# Requires docker compose, node/npm and Go. The pr3115 scenario also needs
# pr3115/vendor/*.tgz (see pr3115/build-tarballs.sh) and is skipped without them.
set -euo pipefail
cd "$(dirname "$0")/../.."
COMPOSE="docker compose -f examples/collector/docker-compose.yml"
OUT=examples/collector/out
TSDIR=examples/typescript-sdk-streamable-http
GOBIN=$(mktemp -d)/example
mkdir -p "$OUT" testdata
chmod 777 "$OUT" # the collector runs as uid 10001 and must create the file
(cd "$TSDIR" && npm ci --silent --no-audit --no-fund)
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

# node runs directly (not via npm) so SIGINT reaches the server and flushes spans.
TS_SERVER="node $TSDIR/server.mjs"
TS_CLIENT="node $TSDIR/client.mjs"
scenario typescript-sdk-default "$TS_SERVER" "$TS_CLIENT"
scenario cross-go-client-typescript-server "$TS_SERVER" "$GOBIN -role client"
scenario cross-typescript-client-go-server "$GOBIN -role server" "$TS_CLIENT"

if ls "$TSDIR"/pr3115/vendor/*.tgz >/dev/null 2>&1; then
  (cd "$TSDIR/pr3115" && npm install --silent --no-audit --no-fund) # no lockfile: tarball hashes are local
  scenario typescript-sdk-pr3115 "node $TSDIR/pr3115/server.mjs" "node $TSDIR/pr3115/client.mjs"
else
  echo "skipping typescript-sdk-pr3115: run $TSDIR/pr3115/build-tarballs.sh first"
fi
