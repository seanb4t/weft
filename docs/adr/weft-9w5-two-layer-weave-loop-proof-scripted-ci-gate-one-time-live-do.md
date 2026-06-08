<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-9w5; do not edit manually; use `/adr update weft-9w5` -->

# Two-layer weave-loop proof: scripted CI gate + one-time live dogfood

**Date:** 2026-06-08
**Status:** Accepted
**Decision:** weft-9w5
**Deciders:** Sean Brandt

## Context

Proving the weave loop end-to-end (seam 10) must cover two distinct failure surfaces: (1) verb choreography + JSON-envelope correctness — the engine verbs composed across every branch — and (2) the prompt-driven orchestration (execute.md + real agents) navigating those same branches. These fail in different ways: a green verb-composition test says nothing about whether the prompt drives the verbs correctly, and a single live run is non-deterministic and cannot serve as a regression gate.

## Decision

The proof uses two deliberately non-redundant layers. A scripted Go integration gate (subprocess-binary E2E, real jj+bd, scripted executor/resolver stand-ins) is the deterministic CI regression artifact proving verb composition + branch closure. A one-time cmux-driven live dogfood runs /weft-execute with real agents over the same fixture, proving the prompt navigates the branches; its output is findings beads, not a CI gate. Neither layer subsumes the other.

## Rationale

Verb choreography and prompt-driven orchestration are independent failure surfaces; one layer cannot honestly cover both. The scripted gate must be deterministic + CI-able to be a regression artifact, which a live LLM-driven run can never be. The live run must exercise the real prompt+agents, which a scripted stand-in deliberately does not. The non-redundancy is the point: a green gate with a broken prompt still fails the live run.

## Alternatives Considered

Single scripted gate only — fully CI-able + deterministic, but never proves execute.md + real agents navigate the branches; prompt drift goes undetected. Single live run only — exercises the real orchestrator, but is non-deterministic, not a regression gate, and cannot be required to pass in CI. Both rejected: each leaves one of the two failure surfaces unproven.

## Consequences

Positive: the regression gate is deterministic + CI-able; the live run captures real-agent behavior and surfaces gap beads. Negative: two proofs to author + coordinate; live-run findings are captured manually, not automated. Neutral: the layer split is encoded in spec §2 and constrains how future seams extend proof coverage.
