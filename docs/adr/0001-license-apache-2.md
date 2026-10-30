# 0001: License under Apache-2.0

Status: accepted, 2026-10-30

## Context
The project targets contributions to and adoption alongside OpenTelemetry and MCP SDK projects, which are Apache-2.0 (OTel, openllmetry) or MIT. Upstream PRs (M3) and possible donation to a neutral foundation are easier with a compatible license. Apache-2.0 adds an explicit patent grant.

## Decision
License all code and docs in this repo under Apache-2.0, copyright "2026 project contributors".

## Consequences
- Compatible with OTel and most MCP SDK licenses; code can move upstream.
- Requires NOTICE/attribution handling if we vendor code.
- Not copyleft: others can build closed products on it. Accepted.

## Alternatives
- MIT: simpler, no patent grant. Rejected for patent clarity.
- AGPL/BSL: would deter contributors and upstream adoption. Rejected; no SaaS plan to protect.
