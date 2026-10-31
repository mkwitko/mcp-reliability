#!/usr/bin/env bash
# Builds modelcontextprotocol/typescript-sdk#3115 (branch feat/protocol-instrumentation)
# at a pinned commit and packs core/client/server into ./vendor for capture only.
# Needs git, node and pnpm. Usage: build-tarballs.sh [workdir]
set -euo pipefail
SHA=5a073867a783066cc673ff07f7943aebe6165787
HERE=$(cd "$(dirname "$0")" && pwd)
WORK=${1:-$(mktemp -d)}/ts-sdk-pr
git clone -q --depth 1 -b feat/protocol-instrumentation https://github.com/modelcontextprotocol/typescript-sdk "$WORK"
got=$(git -C "$WORK" rev-parse HEAD)
[ "$got" = "$SHA" ] || { echo "branch moved: got $got, fixtures were captured at $SHA" >&2; exit 1; }
(cd "$WORK" && pnpm install --frozen-lockfile && pnpm -r build)
mkdir -p "$HERE/vendor"
for p in core client server; do (cd "$WORK/packages/$p" && pnpm pack --pack-destination "$HERE/vendor"); done
