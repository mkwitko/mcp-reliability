# Feasibility: OTel Weaver `registry live-check` for MCP span validation (M4)

Date: 2026-10-30. Time spent: about 15 minutes of the 45-minute box. Not blocked at any point.
Everything below was run, not read, unless marked HYPOTHESIS. Work dir (not in the repo): `/tmp/claude-504429705/-home-mauricio-oliveira-Documentos-Coding-futuro-mcp-reliability/9c14ea2d-4fa7-4d9e-aeeb-db21c3e3a5c0/scratchpad/weaver/` (converter, matchers, sample JSON, reports).

## Verdict

1. **Yes, weaver catches a missing `mcp.method.name`.** On `testdata/missing-method-attr.jsonl` it reports `required_attribute_not_present` (level `violation`) for all 6 server spans that lack the attribute, and exits 1. On `testdata/healthy.jsonl` it reports 0 violations and exits 0.
2. **Condition:** weaver needs a *matcher* to pair a span with `mcp.server` or `mcp.client`, because span names are free-form. The obvious matcher key, "span has `mcp.method.name`", cannot be used for this check, since the span lacks the attribute. We had to match on span kind plus a span-name regex (`tools/…`, `server/…`, `notifications/…`, ...). That is a heuristic we would own.
3. **Nothing cross-span.** `testdata/broken-propagation.jsonl` (disconnected client and server spans) produces output identical to `healthy`: 0 violations, exit 0. The sample carries `trace_id`, `span_id` and `parent_span_id`, but each span is assessed on its own.
4. **Conditions are not evaluated.** `error.type` ("conditionally required if the operation fails") is reported as an `information` finding "not present" on every span, failing or not. So `broken-error-type.jsonl` and `leaked-arguments.jsonl` exit 0 with no violation. MCP-ERR-001/002/003, MCP-PROP-*, MCP-PRIV-001 and MCP-SEM-002 are not replaced.
5. **Input path exists without a Collector:** weaver's `json` sample format can be produced from our `testdata/*.jsonl` with a 30-line script. The M4 hypothesis "Collector replay" also works, with one gotcha (`compression: none`).
6. **Extra value weaver does give:** unknown attribute names (typos), wrong value types, and requirement-level checks, from the registry, not from our hand-written code.
7. **Cost:** the registry resolves its dependency (`semantic-conventions` v1.44.0) from GitHub at run time, so doctor would no longer be offline by default. Output is noisy (about 240 findings on a healthy file, almost all `information`/`improvement`).

Recommendation for the M4 ADR: integrate as an opt-in `doctor --weaver` step (convert, shell out, map `violation` findings into our report), not as a replacement for any current rule. Only MCP-SEM-001 is truly duplicated, and ours is deterministic and offline. The attribute-name and type checks are the real addition. Details in section 7.

## 1. Setup

Image tags (Docker Hub API, `otel/weaver`): `latest`, `main`, `v0.28.0`, `v0.27.0`, `v0.26.1`, ... Used `v0.28.0`.

```
$ docker pull otel/weaver:v0.28.0
$ docker run --rm otel/weaver:v0.28.0 --version
weaver 0.28.0
```

Weaver source cloned for reading: `open-telemetry/weaver` `039f9d2` (2026-10-29), files under `crates/weaver_live_check/`.

Registry: `open-telemetry/semantic-conventions-genai` `6f98aef` (2026-10-30), `model/`. It is a v2 registry (`file_format: definition/2` in `model/mcp/spans.yaml`), which matchers require. Its `manifest.yaml` has one dependency, `https://github.com/open-telemetry/semantic-conventions.git@v1.44.0[model]`, which weaver clones on each run. Span signals defined for MCP: `mcp.client` (kind client) and `mcp.server` (kind server) in `model/mcp/spans.yaml`.

## 2. Input options (`weaver registry live-check --help`, trimmed)

```
--input-source <INPUT_SOURCE>  {file path} | stdin | otlp [default: otlp]
--input-format <INPUT_FORMAT>  text | json (not required for OTLP) [default: json]
--format <FORMAT>              json, yaml, jsonl, or a template name (default ansi)
--v2                           output schema v2 (needed for a v2 registry)
--search-all-attributes        also search attributes of the registry's dependencies
--include-unreferenced         (see gotcha 3)
--fail-on <violation|improvement|information|none>   [default: violation]
--config <path>                .weaver.toml (matchers, finding filters)
--advice-policies <dir>        Rego policies (package live_check_advice)
--otlp-grpc-port <p>  --admin-port <p>  --inactivity-timeout <s>   (otlp source)
```

- `text`: a list of attribute names (`crates/weaver_live_check/data/attributes.txt`); too weak for our need.
- `json`: an array of sample entities, `{"resource": {...}}`, `{"span": {...}}`. `SampleSpan` fields from `src/sample_span.rs:48-90`: `name`, `kind` (`client|server|internal|producer|consumer`), `status {code: unset|ok|error, message}`, `attributes [{name, value, type?}]`, `span_events`, `span_links`, and optional `trace_id`, `span_id`, `parent_span_id`, `trace_state`, `start_time`, `end_time`.
- `otlp`: gRPC listener only. The weaver docs and issue #1563 (OTLP/HTTP) agree; I did not re-check the issue state today. There is no OTLP JSON file reader.

## 3. Converting our fixtures (no Collector)

`otlp2weaver.py` (30 lines; reads Collector `fileexporter` JSON lines, writes the weaver sample array). Kept in the scratch dir, not the repo:

```python
#!/usr/bin/env python3
import json, sys
KIND = {0: "internal", 1: "internal", 2: "server", 3: "client", 4: "producer", 5: "consumer"}
STATUS = {0: "unset", 1: "ok", 2: "error"}
def val(v):
    for k in ("stringValue", "boolValue", "doubleValue"):
        if k in v: return v[k]
    if "intValue" in v: return int(v["intValue"])
    if "arrayValue" in v: return [val(x) for x in v["arrayValue"].get("values", [])]
def attrs(l): return [{"name": a["key"], "value": val(a["value"])} for a in l or []]
out = []
for line in open(sys.argv[1]):
    if not line.strip(): continue
    for rs in json.loads(line).get("resourceSpans", []):
        out.append({"resource": {"attributes": attrs(rs.get("resource", {}).get("attributes"))}})
        for ss in rs.get("scopeSpans", []):
            for sp in ss.get("spans", []):
                st = sp.get("status", {})
                s = {"name": sp["name"], "kind": KIND.get(sp.get("kind", 0), "internal"),
                     "attributes": attrs(sp.get("attributes")),
                     "status": {"code": STATUS.get(st.get("code", 0), "unset"), "message": st.get("message", "")},
                     "trace_id": sp["traceId"], "span_id": sp["spanId"]}
                if sp.get("parentSpanId"): s["parent_span_id"] = sp["parentSpanId"]
                out.append({"span": s})
json.dump(out, sys.stdout, indent=1)
```

```
$ python3 -I otlp2weaver.py testdata/missing-method-attr.jsonl > missing-method-attr.json
```

Output for `missing-method-attr`: 15 sample entities (2 resources, 13 spans). The fixture strips `mcp.method.name` from the 6 server spans (they keep `gen_ai.*`, `error.type`); the 6 client spans keep it. In Go, the doctor already has the loader, so the converter would be about 40 lines of Go and would not need Python.

## 4. Matchers

Weaver compares a span only with a signal a matcher names (`docs/matchers.md`: "A span needs a matcher"). `matchers.toml`:

```toml
[["live-check".matchers]]
id = "mcp.server.by-kind-and-name"
sample_type = "span"
when = 'kind == "server" && name.matches("^(tools/|prompts/|resources/|server/|notifications/|initialize|ping|completion/|logging/)")'
signal = "mcp.server"

[["live-check".matchers]]
id = "mcp.client.by-kind-and-name"
sample_type = "span"
when = 'kind == "client" && name.matches("^(tools/|prompts/|resources/|server/|notifications/|initialize|ping|completion/|logging/)")'
signal = "mcp.client"
```

The span `agent step` (kind internal) correctly gets no match. The regex is ours to maintain and will miss spans whose name does not start with an MCP method (a library that names spans `mcp.tools.call`, for instance). Using `attributes.exists(k, k == "mcp.method.name")` would be cleaner, but then the span missing the attribute is never matched and the Required check never runs: that is exactly the case we want to catch.

## 5. Runs

Common command (local registry clone mounted read-only; `run` is a shell function of mine wrapping this):

```
docker run --rm -u $(id -u):$(id -g) -e HOME=/tmp \
  -v $S/sc:/reg:ro -v $S/weaver:/w \
  otel/weaver:v0.28.0 registry live-check \
    -r /reg/model --v2 --search-all-attributes \
    --config /w/matchers.toml \
    --input-source /w/<fixture>.json --input-format json \
    --format json -o /w/out3/<fixture>
```

The git form of the registry also works, with the same result (6 violations, exit 1):

```
-r 'https://github.com/open-telemetry/semantic-conventions-genai.git[model]'
```

Results (report: `<out>/live_check.json`; diagnostics on stderr):

| Fixture | Exit | Violations | Notes |
|---------|------|------------|-------|
| `healthy` | 0 | 0 | 53 entities, 102 `information`, 136 `improvement` |
| `missing-method-attr` | 1 | 6 | all `required_attribute_not_present` for `mcp.method.name`, on the 6 server spans |
| `broken-propagation` | 0 | 0 | identical to healthy; propagation is invisible |
| `broken-error-type` | 0 | 0 | only `information` (see below) |
| `leaked-arguments` | 0 | 0 | opt-in `gen_ai.tool.call.*` present, not flagged |

Violation lines for `missing-method-attr` (summarised from `live_check.json`, `advice.level == violation`):

```
1 ('server initialize',               'required_attribute_not_present', "Required attribute 'mcp.method.name' is not present.")
1 ('server notifications/initialized','required_attribute_not_present', "Required attribute 'mcp.method.name' is not present.")
1 ('server server/discover',          'required_attribute_not_present', "Required attribute 'mcp.method.name' is not present.")
1 ('server tools/call echo',          'required_attribute_not_present', "Required attribute 'mcp.method.name' is not present.")
1 ('server tools/call fail',          'required_attribute_not_present', "Required attribute 'mcp.method.name' is not present.")
1 ('server tools/call missing',       'required_attribute_not_present', "Required attribute 'mcp.method.name' is not present.")
STATS entities 47 advice_levels {'violation': 6, 'improvement': 130, 'information': 101}
```

The `match_info` on a client span shows the matcher at work: `{'signal': 'mcp.client', 'signal_matcher': 'mcp.client.by-kind-and-name', ... 'unmatched': False}`.

### Attribute name and type checks (mutated healthy file, `typo.json`)

One server `tools/call echo` span given a misspelled key `mcp.method_name` and `gen_ai.tool.name = 42`:

```
gen_ai.tool.name violation   type_mismatch     Attribute 'gen_ai.tool.name' has type 'int'. Type should be 'string'.
mcp.method_name  violation   missing_attribute Attribute 'mcp.method_name' does not exist in the registry.
mcp.method_name  information extends_namespace Attribute key 'mcp.method_name' collides with existing namespace 'mcp'
```

This is the category our hand-written rules do not cover today, and the roadmap goal ("do not rebuild attribute-level semconv validation") holds up for it.

### What weaver does not evaluate

On a healthy span the following appears for most spans, always at `information`, never `violation`:

```
Conditionally required attribute 'error.type' is not present.
Conditionally required attribute 'gen_ai.tool.name' is not present.   (also on non-tool spans)
Conditionally required attribute 'jsonrpc.request.id' is not present.
Recommended attribute 'network.transport' is not present.
```

The registry condition text ("If and only if the operation fails") is a free-text note, so weaver cannot evaluate it. Our MCP-ERR-001 ("failed span lacks `error.type`") needs the span status, which weaver does see in the sample, but only a custom Rego policy could connect them.

Other findings that affect us as a consumer:
- `error.type` value `tool_error` and `-32602` are flagged `information undefined_enum_variant` ("not documented"). The MCP semconv prose says `tool_error` SHOULD be used, but the registry enum omits it. Same for `mcp.method.name` = `server/discover`. A candidate upstream issue for semantic-conventions-genai (not verified against open issues).
- `rpc.response.status_code`, `mcp.method.name`, `gen_ai.*` are `improvement not_stable` (development stability). Noise for our purposes.

## 6. Gotchas (all hit during the run)

1. `--v2` is required for a v2 registry and for matchers; without a matcher, a span gets no comparison set at all.
2. **Resource attributes** (`service.name`, `telemetry.sdk.*`) are reported as `missing_attribute` violations ("does not exist in the registry") unless `--search-all-attributes` is passed. Without it, `healthy` exits 1.
3. `--include-unreferenced` together with `--search-all-attributes` makes registry resolution fail with "attribute id ... is declared multiple times" (e.g. `gen_ai.usage.input_tokens`), exit 1. Do not use it.
4. The diagnostics are printed on stderr and include several registry warnings (`definition/2 is not yet stable`, missing manifest `file_format`). Use `--format json -o <dir>` and read `live_check.json`; ignore stderr unless exit is non-zero for a reason other than findings. A `--quiet` run was not tried.
5. Docker: run with `-u $(id -u):$(id -g) -e HOME=/tmp`, else the weaver cache and output directory fail to write.
6. The default exit code reflects `--fail-on violation`. With `none` weaver never fails, so we can map findings ourselves.

## 7. OTLP gRPC path (the M4 "Collector replay" hypothesis)

Confirmed working, with one gotcha. Weaver listens (`--input-source otlp`), a Collector with the `otlpjsonfile` receiver reads the fileexporter file and forwards over gRPC:

```
# collector.yaml
receivers:
  otlpjsonfile: { include: [/replay/in.jsonl], start_at: beginning }
exporters:
  otlp: { endpoint: 127.0.0.1:14317, tls: {insecure: true}, compression: none }
service: { pipelines: { traces: { receivers: [otlpjsonfile], exporters: [otlp] } } }

$ docker run -d --name wv --network host ... otel/weaver:v0.28.0 registry live-check \
    -r /reg/model --v2 --search-all-attributes --config /w/matchers.toml \
    --input-source otlp --otlp-grpc-port 14317 --admin-port 14320 --inactivity-timeout 20 \
    --format json -o /w/out4
$ docker run --rm --network host -v .../replay:/replay:ro \
    otel/opentelemetry-collector-contrib:0.140.0 --config /replay/collector.yaml
```

Gotcha: with the Collector default gzip the exporter fails with `rpc error: code = Unimplemented desc = Content is compressed with gzip which isn't supported` and drops the data (weaver then reports 0 entities and exits 0, which is a silent false pass). `compression: none` fixes it.

Result: 62 entities (13 spans, 45 attributes, 2 resources, 2 scopes), 6 violations, same six `mcp.method.name` lines, exit 1. This path additionally gives weaver `instrumentation_scope` samples. It needs a Collector binary, a free port, and a timeout (the run ends on `--inactivity-timeout`), so it is clearly worse than the sample-JSON path for a single local CLI.

## 8. Where `doctor` would call weaver (M4)

Proposed shape, smallest first:

1. `mcp-reliability doctor --weaver` (opt-in, off by default).
2. Doctor already parses the file (loader). It writes the weaver sample JSON to a temp file (the converter above, in Go, about 40 lines), writes an embedded `matchers.toml`, then runs `weaver registry live-check ... --format json -o <tmpdir>` through `os/exec`, and reads `live_check.json`.
3. Map `advice.level == violation` into doctor findings as a new rule family `MCP-WEAVER-*`, with the weaver advice `id` and message as evidence and the span ids from the sample. Drop `information`, and drop `improvement` unless `-v`. Weaver requires weaver on PATH or Docker; doctor should print the exact missing prerequisite rather than skip silently (a Collector-style silent zero-entity run is the failure to guard against: assert `total_entities_by_type.span` equals our span count).
4. Dedupe: when weaver and MCP-SEM-001 both fire on the same span, show one finding (ours), and mark weaver as confirmation.
5. Keep M4 text "hand-written attribute rules removed or reduced where weaver covers them" honest: only MCP-SEM-001 overlaps, and weaver depends on a matcher regex while ours does not. I would not remove it.

Open choices for the ADR:
- Registry source: pin to a commit of `semantic-conventions-genai` and vendor the `model/` folder (offline, but then the dependency on `semantic-conventions` v1.44.0 must be vendored too, which I did not test: `-r` on a local folder still cloned the dependency from GitHub), or fetch at run time (network call; conflicts with README "makes no network calls").
- Weaver distribution: a Docker image (works, `v0.28.0`) or a binary release; I did not check binary release assets.
- Weaver is pre-1.0 and its span support is still moving (research cites #1851 "Support span type in live-check" and #1859 "validate span names against span name templates", both open at 2026-10-21 and 2026-10-26). Matchers are described in the docs as experimental (stderr prints "Experimental! - Found config"). Pin the version and expect config churn.

## 9. Corrections and confirmations for `docs/research/platforms-and-tooling.md`

- B.2 "weaver cannot directly read a fileexporter file": still true. Confirmed workaround 1: a converter to weaver's `json` sample format (this document, section 3). Confirmed workaround 2: Collector replay over gRPC, with `compression: none`.
- B.2 HYPOTHESIS "needs a v2-format registry; I did not confirm whether `model/mcp` is v1 or v2": it is v2 (`file_format: definition/2`), and `--v2` is required.
- B.2 HYPOTHESIS matcher `when = '"mcp.method.name" in attributes'`: works syntactically, but it makes the Required check unreachable for the span missing the attribute. Use kind plus name instead.
- B.2 "no cross-span check": confirmed by experiment, `broken-propagation.jsonl` passes.
- `roadmap` M4 exit criterion "Documented, tested path from a fileexporter file to weaver": the sample JSON route here is the shorter one; the Collector route is not needed.

## 10. Not done

- Did not test `--input-source stdin`.
- Did not test Rego `--advice-policies` for conditional checks (an `error.type` rule using span status), or whether a policy can see more than one sample. README line 112 describes assessment per sample entity.
- Did not test metrics or logs; MCP registry defines `metrics.yaml` (`mcp.client.operation.duration` and similar) but doctor reads traces only.
- Did not run weaver offline with a vendored dependency.
