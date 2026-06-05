<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-6rt; do not edit manually; use `/adr update weft-6rt` -->

# Use runtime merge-style detection in finish reconcile

**Date:** 2026-06-05
**Status:** Accepted
**Decision:** weft-6rt
**Deciders:** Sean

## Context

Seam-1 §4.4 prescribed `jj rebase -b @ -o main --skip-emptied` as the sole `finish reconcile` cleanup command. Lived experience on this project (PR #17 and #18) showed that squash-merges — GitHub's default and this project's policy — conflict on that rebase path: jj change-ids do not survive a squash, so the rebase re-applies content already present in `main`. jj alone cannot distinguish a squash-merge from a never-merged branch (in both, the epic's commits are absent from `main`), so a single rebase-only command is incorrect for the project's actual merge policy.

## Decision

`weft finish reconcile` detects the merge style at runtime via `jj log -r '<epic>@origin & ::main@origin'`: a non-empty result means the pushed tip is an ancestor of trunk (true-merge → `jj rebase -b @ -o main --skip-emptied`); an empty result means squash- or GitHub-rebase-merge (→ `jj new main` + `jj abandon '<stack-root>::'`). A `gh pr view --json state` gate confirms `state == MERGED` before any abandon. This supersedes the seam-1 §4.4 rebase-only prescription (spec text, not a prior bd ADR).

## Rationale

- PR #17 and #18 both conflicted on `jj rebase -b @ -o main --skip-emptied` after a squash-merge; the working fix both times was `jj new main` + `jj abandon '<stack-root>::'`.
- jj change-ids do not survive a GitHub squash-merge or rebase-merge, so the rebase would re-apply already-landed content.
- The ancestry revset `<epic>@origin & ::main@origin` is the correct jj idiom (not git's `merge-base --is-ancestor`) and is deterministic per merge type.
- The merged-state gate via `gh pr view --json state` ensures no unmerged work is abandoned, regardless of detection path.
- Squash and GitHub rebase-merge are structurally identical from jj's perspective, so they collapse to one `merge_style` enum value (`squash_or_rebase`).

## Alternatives Considered

**Rebase-only (seam-1 §4.4 prescription)** — rejected. Simple and matches the original GSD reconcile design, but conflicts on squash-merges and GitHub rebase-merges because jj change-ids are absent from the squash commit; it re-applies already-landed content and requires manual resolution.

**Runtime merge-style detection with branching cleanup (chosen)** — handles all GitHub merge strategies correctly, guards against abandoning unmerged work via an authoritative `gh` signal, and uses a deterministic, testable ancestry check. Cost: two cleanup code paths to maintain; squash and rebase-merge collapse to one enum value.

## Consequences

**Positive:** reconcile works for all GitHub merge strategies without manual intervention; the chosen path is emitted in `--dry-run` and the `--json` envelope (observable + testable); lived PR evidence grounds the decision.

**Negative:** two cleanup code paths to maintain and test; squash and GitHub rebase-merge share one `merge_style` value, which may surprise contributors expecting three-way detection.

**Neutral:** `--skip-emptied` remains correct and used in the true-merge path (its omission in `shed integrate` per weft-hjx.7 is a separate, already-captured decision); the seam-1 §4.4 verb table and design.md §6 forward-pointer were updated in-spec to reflect this change.
