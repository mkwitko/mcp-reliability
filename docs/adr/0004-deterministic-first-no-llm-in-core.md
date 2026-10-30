# 0004: Deterministic checks first, no LLM in core

Status: accepted, 2026-10-30

## Context
Findings are only useful to maintainers if they are reproducible and traceable to normative text. LLM output is non-deterministic and can invent violations. Traces may contain secrets, so sending them to a model is a privacy risk.

## Decision
The core only runs deterministic rules. No network calls, no LLM dependency. An optional AI layer (explanations, triage) is deferred to M5, only if evidence shows demand; if built, it is opt-in, separate from core, and bring-your-own-key (BYOK).

## Consequences
- Same input gives same output; golden-file tests are possible.
- Less "smart" output at first; messages must be good by themselves.
- AI layer may never ship.

## Alternatives
- LLM-based analysis in core: rejected for reproducibility and privacy.
- Hosted AI service: rejected (no SaaS, no backend).
