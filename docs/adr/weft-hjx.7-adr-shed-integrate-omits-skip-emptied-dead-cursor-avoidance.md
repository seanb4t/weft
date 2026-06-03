<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-hjx.7; do not edit manually; use `/adr update weft-hjx.7` -->

# ADR: shed integrate omits --skip-emptied (dead-cursor avoidance)

**Date:** 2026-06-03
**Status:** Accepted
**Decision:** weft-hjx.7
**Deciders:** —

DECISION: weft 'shed integrate' rebases each wave member with 'jj rebase -s <change> -o <prev-tip>' WITHOUT --skip-emptied, diverging from the original spec §4.1 verb table and plan Task 4 (which listed it).

CONTEXT: integrate builds a dep-ordered linear stack by rebasing each sealed member onto the previous tip, advancing a cursor prev=<ch> each step. --skip-emptied abandons any member that rebases to empty; if it fires, prev points at a now-nonexistent change and the next 'jj rebase -o <ch>' fails ('revision not found').

RATIONALE: jj change-ids are stable across rebase, so WITHOUT --skip-emptied every member survives and the linear cursor stays valid. An empty member (rare: a pick whose diff is already upstream) surfaces downstream (resume/land) rather than being silently dropped mid-stack — which is the safer behavior for a wave-integration verb. --skip-emptied remains correct in post-squash-merge reconciliation (finish reconcile), where empties are EXPECTED; that usage is unaffected.

CONSEQUENCES: integrate may leave an empty commit in the stack for an already-upstream member (acceptable; visible). Spec §4.1, plan Task 4, design.md §4, and the in-code comment in internal/cli/shed.go all updated to match. Surfaced by PR #9 review finding weft-fp0.1.
