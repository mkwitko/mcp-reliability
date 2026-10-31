# Proposal: `_meta` trace context scenario for modelcontextprotocol/conformance

Date: 2026-10-30. Status: draft. Nothing here has been sent. No issue or PR was opened.

Read at: `modelcontextprotocol/conformance` `c37eec8` (2026-10-01, version `0.2.0-alpha.12`, TypeScript, npm, vitest); `modelcontextprotocol/modelcontextprotocol` `68619e4` (draft spec). Scratch clones are in the session scratchpad (`.../scratchpad/conformance`, `.../scratchpad/spec`).

Prior art check: `gh issue list -R modelcontextprotocol/conformance --search "traceparent OR opentelemetry OR \"trace context\" OR 414" --state all` returns nothing. No `src/seps/sep-414.yaml` exists (`src/seps/` has 837, 1932, 1933, 2106, 2164, 2207, 2243, 2260, 2322, 2350, 2352, 2468, 2549, 2575, 2640, 2663). The `request-metadata` scenario (SEP-2575) checks only the `io.modelcontextprotocol/*` keys.

## 1. What the spec says (exact text)

`docs/specification/draft/basic/index.mdx`, lines 419-427 (heading at 419, normative text 421-424, rationale 426-427):

```
**OpenTelemetry trace context:**

As an exception to the prefix requirement above, the keys `traceparent`, `tracestate`, and
`baggage` are reserved for [OpenTelemetry](https://opentelemetry.io/) trace context propagation.
When present, their values MUST follow [W3C Trace Context](https://www.w3.org/TR/trace-context/)
and [W3C Baggage](https://www.w3.org/TR/baggage/) formats respectively.

This exception exists to maintain compatibility with existing implementations and
[OpenTelemetry semantic conventions for MCP](https://opentelemetry.io/docs/specs/semconv/gen-ai/mcp/).
```

Also the reserved-keys table row (line 358): `traceparent`, `tracestate`, `baggage` | "OpenTelemetry trace context propagation" | "OpenTelemetry trace context (below)". SEP-414 is `docs/seps/414-request-meta.mdx` lines 53-55: "When OTel trace context is propagated via `_meta`, the keys `traceparent`, `tracestate`, and `baggage` follow W3C ... value formats."

### What is and is not normative

This matters for check severity, because the conformance repo's rule is "MUST / MUST NOT -> FAILURE; SHOULD / SHOULD NOT -> WARNING" (`AGENTS.md`, "Check conventions"), and CI treats WARNING as failure.

| Requested check | Normative text | Proposed severity |
|-----------------|----------------|-------------------|
| Client: `_meta.traceparent`, when sent, is W3C formatted | Yes: "When present, their values MUST follow W3C Trace Context" (line 423) | FAILURE |
| Client: `tracestate`, `baggage`, when sent, are W3C formatted | Yes, same sentence | FAILURE |
| Server: MUST NOT reject unknown `_meta` keys | **No explicit sentence found.** I searched the draft for `ignore`, `unrecogni`, `unknown`, `reject` near `_meta`: nothing says a server must tolerate unknown `_meta` keys. Support is indirect: `_meta` exists "to allow clients and servers to attach additional metadata" (line ~323), key names are open-ended with a vendor prefix rule, and third-party extensions use their own prefix (line ~361). | WARNING at most, and say so in the issue. Ask the maintainers whether they want it. |
| Server: accepts a request carrying a valid `traceparent` | Implied by the reservation, no keyword | WARNING |
| Server: does not echo or propagate request trace context in the result `_meta` | **No text at all.** | INFO only, or drop |

The task description called "server MUST NOT reject unknown `_meta` keys" a MUST. The text does not say that. The proposal below downgrades it and flags it as an open question. The only MUST that exists is the W3C format of the three reserved keys when present.

## 2. How the conformance repo is built (relevant parts)

- Language: TypeScript, `vitest`, `tsdown` build. CLI: `npx @modelcontextprotocol/conformance client|server --scenario <name> | --suite <name> --spec-version 2026-07-28`.
- Two kinds of scenario (`src/types.ts:132-151`):
  - `Scenario` (tests a **client**): `start(ctx)` returns `{ serverUrl }`, the harness launches the client under test against a mock server, then `getChecks()` returns `ConformanceCheck[]`. Example: `src/scenarios/client/request-metadata.ts` starts a Node `http` server, parses each POST body, and asserts on `request.params._meta` and on headers (lines ~100-200 in that file). Checks use `addOrUpdateCheck`, worst status latches, and declared-but-never-emitted checks are backfilled as FAILURE (`DECLARED_CHECK_IDS`).
  - `ClientScenario` (tests a **server**): `run(ctx: RunContext)` returns checks; it connects with `ctx.connect()` and calls `conn.request(method, params)` (`src/connection/index.ts:34-70`). On the stateless wire the harness merges defaults into `_meta` and "keys already present in `params._meta` win" (`src/connection/stateless.ts:126-140`), so a scenario can inject `_meta.traceparent` by passing it in `params`.
- A check (`ConformanceCheck`): `id`, `name`, `description`, `status` (`SUCCESS|FAILURE|WARNING|INFO|SKIPPED`), `timestamp`, `specReferences: [{id, url}]`, optional `details`, `errorMessage`.
- Conventions that shape this proposal (`AGENTS.md`): open an issue first; prefer **one scenario with many checks**; same `id` for success and failure; `specReferences` required; severity follows the spec keyword; a missing prerequisite is a failure via `untestableCheck()`, SKIPPED only when legitimately not applicable; each scenario needs a passing example (extend `everything-client`/`everything-server`) and a negative test (a deliberately broken example plus a vitest case); register in `src/scenarios/index.ts`; add `src/seps/sep-414.yaml` (`npx @modelcontextprotocol/conformance new-sep 414`).
- Registration points: client scenarios list `scenariosList` (`src/scenarios/index.ts` around line 320), server scenarios `allClientScenariosList` (around line 170). `source: { introducedIn: DRAFT_PROTOCOL_VERSION }` (`'2026-07-28'`, `src/types.ts:50`) scopes it to the draft spec version.

## 3. Proposed scenarios

Following "fewer scenarios, more checks", propose two scenarios sharing one validator, one for each side, named `trace-context-meta` (client side) and `server-trace-context-meta` (server side). The SEP yaml `src/seps/sep-414.yaml` lists the requirement rows.

### 3.1 Client scenario `trace-context-meta` (a `Scenario`)

Mock server records every request or notification the client sends. For each message whose `params._meta` has any of the three reserved keys, validate the value.

Checks (IDs follow the `sep-NNNN-...` pattern used by the traceability tooling):

| id | status rule |
|----|-------------|
| `sep-414-client-traceparent-format` | FAILURE if any observed `traceparent` is not a string matching W3C Trace Context level 1 (see 3.3); SUCCESS if observed and all valid; SKIPPED if the client never sent one (legitimate: the client has no tracing configured, the spec says "when present") |
| `sep-414-client-tracestate-format` | same, for `tracestate` (list-member syntax, max 32 members) |
| `sep-414-client-baggage-format` | same, for `baggage` |
| `sep-414-client-tracestate-requires-traceparent` | WARNING if `tracestate` is sent without `traceparent` (W3C: tracestate alone is meaningless); INFO class, optional |

Problem to state openly: the stock `everything-client` uses the MCP TypeScript SDK, which does not emit trace context unless the application injects it. So against the reference client these checks would be SKIPPED. For a positive example, `everything-client.ts` would add a branch for this scenario name that sets `_meta.traceparent` on its requests (e.g. through the SDK's request options), and a negative example `examples/clients/typescript/sep-414-bad-traceparent.ts` would send `traceparent: "not-a-traceparent"`. Per `AGENTS.md`, a scenario that is always SKIPPED against its reference client does not prove anything, so this needs the example.

Sketch (their format, abridged):

```ts
// src/scenarios/client/trace-context-meta.ts
import type { ScenarioContext, MockServer } from '../../mock-server';
import {
  Scenario, ScenarioUrls, ConformanceCheck, DRAFT_PROTOCOL_VERSION
} from '../../types';
import { isW3CTraceparent, isW3CTracestate, isW3CBaggage } from './trace-context-validators';

const SPEC_REF = {
  id: 'SEP-414',
  url: 'https://modelcontextprotocol.io/specification/draft/basic/index#meta'
};

export const DECLARED_CHECK_IDS = [
  'sep-414-client-traceparent-format',
  'sep-414-client-tracestate-format',
  'sep-414-client-baggage-format'
] as const;

export class TraceContextMetaScenario implements Scenario {
  name = 'trace-context-meta';
  readonly source = { introducedIn: DRAFT_PROTOCOL_VERSION } as const;
  description = `Tests that, when a client sends OpenTelemetry trace context in _meta, \
the values follow W3C formats (SEP-414). A client that sends no trace context is \
reported as SKIPPED, because the requirement applies only "when present".`;

  private srv: MockServer | null = null;
  private seen: Record<string, Array<{ method: string; value: unknown }>> = {};

  async start(ctx: ScenarioContext): Promise<ScenarioUrls> {
    // reuse the same handlers as ToolsCallScenario, wrapped by a recorder
    this.srv = await ctx.createServer({
      'server/discover': () => ({ /* minimal discover result */ }),
      'tools/list': (params) => { this.record('tools/list', params); return { tools: [/* one tool */] }; },
      'tools/call': (params) => { this.record('tools/call', params); return { content: [{ type: 'text', text: 'ok' }] }; }
    });
    return { serverUrl: this.srv.url };
  }

  private record(method: string, params: any) {
    const meta = params?._meta ?? {};
    for (const key of ['traceparent', 'tracestate', 'baggage']) {
      if (key in meta) (this.seen[key] ??= []).push({ method, value: meta[key] });
    }
  }

  getChecks(): ConformanceCheck[] {
    const spec: Record<string, (v: unknown) => boolean> = {
      traceparent: isW3CTraceparent, tracestate: isW3CTracestate, baggage: isW3CBaggage
    };
    return Object.entries(spec).map(([key, valid]) => {
      const hits = this.seen[key] ?? [];
      const bad = hits.filter((h) => !valid(h.value));
      return {
        id: `sep-414-client-${key}-format`,
        name: `Client${key}Format`,
        description: `When sent, _meta.${key} follows its W3C format`,
        status: hits.length === 0 ? 'SKIPPED' : bad.length ? 'FAILURE' : 'SUCCESS',
        timestamp: new Date().toISOString(),
        errorMessage: bad.length ? `Invalid ${key}: ${JSON.stringify(bad[0].value)} on ${bad[0].method}` : undefined,
        specReferences: [SPEC_REF],
        details: { observed: hits.length, invalid: bad.length }
      } as ConformanceCheck;
    });
  }

  async stop() { await this.srv?.stop(); }
}
```

(The sketch is deliberately abridged: the real file must follow `tools_call.ts` for `createServer` and `stop`. Treat it as shape, not code to paste. I did not compile it.)

### 3.2 Server scenario `server-trace-context-meta` (a `ClientScenario`)

Sends requests with trace context in `_meta` and checks the server still serves them. Uses a read-only method every server has (`tools/list`, or `server/discover` on the stateless wire).

| id | request | status rule |
|----|---------|-------------|
| `sep-414-server-accepts-trace-context` | `tools/list` with `_meta: { traceparent: "00-0af7651916cd43dd8448eb211c80319c-00f067aa0ba902b7-01", tracestate: "rojo=00f067aa0ba902b7", baggage: "userId=alice" }` (the example in the spec, line ~442) | WARNING if the server returns a JSON-RPC error or non-2xx, SUCCESS otherwise. No keyword in the spec; WARNING |
| `sep-414-server-ignores-unknown-meta-keys` | same method with `_meta: { "com.example/conformance-probe": "1" }` (a vendor-prefixed key, valid per the key format rules) | WARNING if rejected. Marked "open question" in the issue because the spec has no sentence for it |
| `sep-414-server-tolerates-invalid-traceparent` (optional, INFO) | `_meta.traceparent = "not-a-traceparent"` | INFO if the request fails, SUCCESS otherwise. This is a robustness probe: W3C Trace Context says a receiver should ignore an invalid header and restart the trace, and the MCP spec puts the MUST on the sender, so a failing response is not a violation. INFO only |
| `sep-414-server-result-meta-no-echo` (optional, INFO) | the first request's result | INFO if `result._meta` contains `traceparent` identical to the request's. No text supports a rule here. I recommend dropping it unless the maintainers want it. |

Sketch:

```ts
// src/scenarios/server/trace-context-meta.ts
export class ServerTraceContextMetaScenario implements ClientScenario {
  name = 'server-trace-context-meta';
  readonly source = { introducedIn: DRAFT_PROTOCOL_VERSION } as const;
  description = `Tests that a server serves requests carrying OpenTelemetry trace context \
in _meta (SEP-414) and does not reject unrelated vendor-prefixed _meta keys.`;

  async run(ctx: RunContext): Promise<ConformanceCheck[]> {
    const conn = await ctx.connect();
    const probe = async (id: string, name: string, desc: string, meta: Record<string, unknown>, failStatus: 'WARNING' | 'INFO') => {
      try {
        await conn.request('tools/list', { _meta: meta }); // caller keys win over harness defaults
        return check(id, name, desc, 'SUCCESS');
      } catch (e) {
        return check(id, name, desc, failStatus, `Server rejected request: ${String(e)}`);
      }
    };
    return [
      await probe('sep-414-server-accepts-trace-context', 'ServerAcceptsTraceContext',
        'Server serves a request whose _meta carries W3C trace context',
        { traceparent: '00-0af7651916cd43dd8448eb211c80319c-00f067aa0ba902b7-01', tracestate: 'rojo=00f067aa0ba902b7', baggage: 'userId=alice' }, 'WARNING'),
      await probe('sep-414-server-ignores-unknown-meta-keys', 'ServerIgnoresUnknownMetaKeys',
        'Server serves a request whose _meta carries an unknown vendor-prefixed key',
        { 'com.example/conformance-probe': '1' }, 'WARNING'),
    ];
  }
}
```

`check(...)` stands for a small local helper building a `ConformanceCheck` with `specReferences: [SPEC_REF]`; the repo style prefers repeated literal blocks to clever helpers (`AGENTS.md`, "Optimize for Ctrl+F on the slug"), so the real file should write the objects out.

Negative example per `AGENTS.md`: `examples/servers/typescript/sep-414-rejects-meta.ts`, a server whose request handler returns `-32602` when `_meta` contains any key outside `io.modelcontextprotocol/*`, plus a vitest case modelled on `src/scenarios/server/negative.test.ts`.

### 3.3 Validators (shared, new file `trace-context-validators.ts`)

- `traceparent` (W3C Trace Context level 1, version 00): `^00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$`, trace-id not all zeros, parent-id not all zeros. Unknown future versions (not `00` and not `ff`) may carry extra fields after the flags (`^[0-9a-f]{2}-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}(-.*)?$`), version `ff` is invalid. Value must be a JSON string (a non-string is a FAILURE).
- `tracestate`: at most 32 comma-separated `key=value` members, lowercase key grammar from the W3C spec, value printable ASCII without `,` or `=`; total length not checked.
- `baggage`: comma-separated `key=value` members with optional `;property`; keys are RFC 7230 tokens. Percent-encoding validated loosely.
- Unit tests in vitest with the W3C spec's own examples (valid and invalid) so the validators are not wrong in the direction of false FAILURE.

## 4. Draft issue text (not sent)

Title: Add a conformance scenario for SEP-414 trace context in `_meta` (`traceparent`, `tracestate`, `baggage`)

Body:

> The draft spec (`docs/specification/draft/basic/index.mdx`, "OpenTelemetry trace context") reserves the `_meta` keys `traceparent`, `tracestate` and `baggage`, and says: "When present, their values MUST follow W3C Trace Context and W3C Baggage formats respectively." (SEP-414.) I could not find a scenario that checks this, and there is no `src/seps/sep-414.yaml`. The existing `request-metadata` scenario covers the `io.modelcontextprotocol/*` keys only. I searched issues for `traceparent`, `opentelemetry`, `trace context` and `414` and found nothing.
>
> Proposal, following the "fewer scenarios, more checks" rule: two scenarios, both `introducedIn: DRAFT_PROTOCOL_VERSION`.
>
> 1. Client side (`Scenario` with a mock server). When the client sends any of the three keys, each value must match its W3C format. Checks: `sep-414-client-traceparent-format`, `-tracestate-format`, `-baggage-format`. FAILURE on a malformed value, SKIPPED when the client sends none (the MUST applies "when present"). This needs a small addition to `everything-client` so that it sends a trace context for this scenario, otherwise the check is always SKIPPED, plus a negative client that sends a malformed `traceparent`.
> 2. Server side (`ClientScenario`). Send `tools/list` with a valid `_meta.traceparent`/`tracestate`/`baggage` and with an unknown vendor-prefixed key (`com.example/...`), and check the server still answers. Per the spec text there is no MUST here, so I propose WARNING, not FAILURE.
>
> Question for maintainers: the spec has no sentence that says a server must tolerate unknown `_meta` keys, or that it must not echo trace context. Do you want the "ignores unknown keys" check (as WARNING) at all, or only the W3C-format MUST? I would drop the echo check unless you want it.
>
> I use these keys in a trace linter (`mcp-reliability`) and see real SDK traffic where `traceparent` is the only reserved key people actually populate, so a format check would catch regressions early. Happy to write the validators, the SEP yaml (`new-sep 414`), and the examples. Is this in scope, and should it go in the `draft` suite or `core`?

(Before sending: replace the last paragraph with something that is true at that time. The "see real SDK traffic" sentence is a claim about our own fixtures; our fixtures are from one Go SDK scenario, so say exactly that or remove it.)

## 5. Open questions and risks

- Which suite: `draft` (spec 2026-07-28) is the natural home. The scenario is not useful before the SDK or the test harness can emit trace context.
- Stateless versus stateful wire: `conn.request` on the stateful wire goes through the SDK client transport, and I did not verify that arbitrary `_meta` keys pass through unchanged there. This must be tested before a PR (the `extraHeaders` comment in `src/connection/index.ts:36-47` shows the stateful wire restricts some per-call overrides, so `_meta` pass-through is a real risk).
- The mock-server handler map shape (`ctx.createServer({...})`) was read from `tools_call.ts`; `server/discover` handling for the draft version in mock servers was not checked.
- A client scenario that is always SKIPPED for real SDKs gives little signal; the value of the client check depends on how many SDKs actually emit `traceparent`. Per our research only OTel-instrumented clients do.
- Maintainers are busy with the draft spec's stateless lifecycle, auth and header work (`docs/research/platforms-and-tooling.md`, A.9); expect a slow answer. Roadmap M3 item 3 exit criteria accept a rejection.
