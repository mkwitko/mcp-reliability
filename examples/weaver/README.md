# Validate MCP spans with OTel Weaver

`doctor` checks a small set of MCP rules. OTel Weaver's `registry live-check`
checks attribute names, types, enum values and requirement levels against the
full semantic-conventions registry. The two complement each other: weaver has
no cross-span checks and does not evaluate "conditionally required" clauses
(`docs/research/weaver-live-check-feasibility.md`, verified 2026-10-30 with
weaver v0.28.0).

`doctor` never does network I/O. Weaver fetches the registry and its
`semantic-conventions` dependency from GitHub, so this step is explicit and
opt-in.

```sh
# 1. Export the spans in weaver's sample format (no network).
go run ./cmd/mcp-reliability doctor -weaver-json /tmp/w/spans.json testdata/missing-method-attr.jsonl

# 2. Run weaver (network: clones the registries).
cp examples/weaver/matchers.toml /tmp/w/
docker run --rm -u "$(id -u):$(id -g)" -e HOME=/tmp -v /tmp/w:/w \
  otel/weaver:v0.28.0 registry live-check \
    -r 'https://github.com/open-telemetry/semantic-conventions-genai.git[model]' \
    --v2 --search-all-attributes \
    --config /w/matchers.toml \
    --input-source /w/spans.json --input-format json \
    --format json -o /w/out
```

Expected on `missing-method-attr.jsonl`: 6 `required_attribute_not_present`
violations for `mcp.method.name`, exit 1. On `healthy.jsonl`: 0 violations,
exit 0, plus a long list of `information`/`improvement` entries about
recommended attributes that are not set.

`matchers.toml` maps spans to the registry's `mcp.server` and `mcp.client`
signals by span kind and a method-name regex. Keying on `mcp.method.name`
itself would hide exactly the spans that lack it.

Caveats found in the feasibility run: the registry is `Development` status
and lags protocol 2026-07-28 (`server/discover` is unknown to it); matchers
need the `--v2` registry format; a Collector-replay route also works but only
with `compression: none` on the exporter.
