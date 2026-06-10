<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-cfp; do not edit manually; use `/adr update weft-cfp` -->

# v1 interaction model: front-loaded spine; phased/interactive GSD Layer-A loop deferred, not compressed

**Date:** 2026-06-10
**Status:** Accepted
**Decision:** weft-cfp
**Deciders:** Sean Brandt

## Context

GSD's value is an interaction-heavy per-phase loop: `/gsd-new-project` (approve roadmap) then, per phase, `discuss-phase` (shape HOW) -> `plan-phase` -> `execute-phase` -> `verify-work` (interactive UAT) -> `ship`, plus upstream `explore`/`spike`/`sketch`. weft v1 ships only `new-project` + `execute` skills (+ planner/executor/reviewer/resolver agents): it front-loads all interaction into the one-shot `new-project` Q&A, emits a single warp (epic + picks + waves), then weaves autonomously with a machine `pick verify` gate (verdict-as-data, exit 0). The 2026-06 dogfood (a greenfield `new-project` run vs the GSD walkthrough) surfaced this gap and the question: was the reduced interaction a deliberate design choice, or fallout of v1 scoping?

## Decision

Record explicitly: weft v1 deliberately ships the **front-loaded spine** (`new-project` + autonomous `execute` over beads+jj) and **defers** GSD's interactive per-phase surface (`discuss-phase`, `verify-work`/UAT, `explore`/`spike`/`sketch`, the phase rhythm). This was a v1 **scope** decision (seam 5: "GSD ships 67 commands; the weft v1 core is a handful ... DROP for v1; revisit later"), **not** a stance that weft should be a less-interactive tool. The phased/interactive loop is on the roadmap and is restored **additively** over the existing verb surface — tracked in epic `weft-ccy` (discuss skill, verify-work skill, phased planner).

## Rationale

- `design.md` §1 names "Layer A — the spec -> plan -> execute -> **verify** -> ship loop" as the borrowable IP weft **keeps**; only Layer B (.planning/ files) and Layer C (git choreography) are replaced (by beads/jj). The full interactive loop was the stated intent.
- `design.md` §7 marks "fully autonomous, host-less waves" as **optional, not a primary goal** — weft did not adopt an autonomous-first identity.
- There is **no ADR** deciding to compress interaction; the omission lives only in seam 5's effort/scope framing ("revisit later"). It is deferral residue, not a recorded choice.
- The substrate already supports phasing with no engine change: the bead dependency graph + `bd ready` + `weft shed form --epic` + `execute`'s epic-scoped loop ("until the epic's ready set is empty") is already a phased scheduler. The `verify=data` choice was made for composability and does not preclude an interactive UAT **skin** over `pick verify`. So restoration is prompt-layer, and it aligns with the original Layer-A intent rather than contradicting the design.

## Alternatives Considered

- **Declare weft autonomous-first; do not restore the interactive loop.** Rejected: contradicts `design.md` §1 (keep Layer A's full loop incl. verify) and §7 (autonomy is optional). It would redefine weft's identity by accident of v1 scoping.
- **Fork/port GSD's full 67-command surface wholesale.** Rejected: clean-room reimplementation on beads+jj is the premise (`design.md` §1, §3/§4); port additively per seam 5, not as a bulk fork.
- **Leave the boundary undocumented.** Rejected: the dogfood showed users reasonably expect the GSD walkthrough's per-phase rhythm; the v1 boundary and the restoration path must be explicit to set expectations and direct the work.

## Consequences

- **Positive:** sets an explicit expectation — weft v1 = front-loaded planning spine + autonomous weave; the phased/interactive per-phase loop is roadmap, not absent-by-principle. Gives a clear additive restoration path (`weft-ccy`: discuss / verify-work / phased planner), each a thin orchestrator over existing verbs + beads.
- **Negative:** until `weft-ccy` lands, weft's interactive UX is materially thinner than GSD's — no per-phase HOW-shaping, no human UAT walk-through, no spike/sketch/explore. Users wanting that loop must wait or drive the verbs manually.
- **Neutral:** the engine substrate is unchanged; the dep-graph already schedules phases, so restoration touches only the prompt/skill layer and the planner's warp structuring. An opt-in `--phased`/`--interactive` mode can coexist with the current one-shot path.

## Addenda

- (2026-06-09, spec docs/superpowers/specs/2026-06-09-layer-a-interactive-phased-loop-design.md) The consequence "an opt-in --phased/--interactive mode can coexist with the current one-shot path" is superseded. Phasing is AUTO-DISCOVERED by the planner — there is no flag. The one-shot path survives behaviorally as the SINGLE-PHASE DEGENERATE CASE (project epic = phase epic, picks planned immediately, today's epic+picks emission shape), not as a separate mode. Pick-level planning for multi-phase projects is just-in-time per phase, after that phase's discuss.
