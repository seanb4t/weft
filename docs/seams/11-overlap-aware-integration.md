<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 11 — Overlap-aware integration topology

> Status: **shipped**. Sub-spec of
> [`docs/design.md`](../design.md) §4–6 (weave + ship). Tracked as bead
> `weft-eoe` (design bead). Descends from finding `weft-78k` (the single-wave
> mixed-conflict cascade surfaced by the seam-10 E2E, `weft-w1y.4`). This seam
> changes the engine: it replaces `shed integrate`'s single lexicographic stack
> with a file-overlap **forest** so that an escalated (permanently-conflicted)
> pick cannot cascade-poison independent picks, and adds a `finish` collapse
> step so the landed picks ship as one line while the escalated pick stays
> parked for a human.

## 1. Scope

Make a **single `shed integrate` over a full wave** correct when that wave
contains both a resolvable (heal) conflict pair and an unresolvable (escalate)
conflict pair. Today the only way to weave such a wave is to split it into
multiple `integrate` calls (the seam-10 E2E uses two) so the escalate pair has
nothing stacked above it.

In scope:

- **`shed integrate` — overlap grouping** (§4.1): partition the wave by
  file-overlap into independent groups; stack linearly only *within* a group;
  emit a forest rooted on `trunk()`.
- **`finish open` — collapse step** (§4.4): before pushing, assemble the epic's
  **closed** picks into a single `@`-tipped line, leaving any escalated/open
  pick parked on `trunk()`.
- **Envelope change** (§4.2): `shed.integrate` emits `data.groups` (replacing
  `data.stack`); `data.conflicts` is unchanged.
- **`execute.md` Steps 5–6 update** (§4.5): drop the "escalate-last ordering"
  prose added in `weft-w1y.6` — grouping makes manual ordering unnecessary — and
  describe the forest.
- **The single-wave E2E** (§6): tighten the seam-10 two-call E2E to one
  `integrate` over the whole wave (the deferred `weft-78k` follow-up), now
  deterministic regardless of bd-assigned id order.

Out of scope:

- **Resolving the parked escalated pick.** After a human resolves it, re-driving
  its integrate/land for a follow-up PR is a separate concern (the weave loop
  already treats an escalated pick as terminal-for-now via the `human` label).
- **`conflict open`/`finalize` and `pick land` semantics.** They are unchanged
  (§4.3); grouping only narrows the subtree they already operate on.
- **Non-add/add conflict taxonomy.** File-overlap is the conservative grouping
  criterion (§4.1); refining it (e.g. line-range overlap) is not needed and not
  pursued.

## 2. Grounding

- **`shed integrate`** (`internal/cli/shed.go:158-268`, `newShedIntegrateCmd`):
  `beads := append(...); sort.Strings(beads)` then a `for` loop rebasing each
  change `jj rebase -s <ch> -o <prev>` with `prev` advancing — building one
  linear stack `trunk() <- b0 <- … <- bN`. Its own comment: *"Wave members are
  mutually independent (spec §4.1), so the dep graph imposes no intra-wave order;
  the deterministic tiebreaker is bead-id lexicographic."* Conflicts surface via
  the scoped revset `conflicts() & (<ch> | …)`. `--skip-emptied` is intentionally
  omitted (ADR `weft-hjx.7`) so the linear cursor stays valid.
- **`conflict finalize`** (`internal/cli/conflict.go:125-239`,
  `newConflictFinalizeCmd`): the escalate branch (still-conflicted resolution)
  runs `bd update <bead> --add-label human` and leaves the change conflicted **in
  place** — no reorder. The heal branch `jj squash --from <ws>@ --into <change>`;
  jj auto-rebases and conflict-simplifies descendants (un-cascade), then
  `scopedConflictChanges` re-queries the change's subtree.
- **`pick land`** (`internal/cli/pick.go:56-88`, `newPickLandCmd`): the land gate
  is exactly `changeConflicted(change) == false`, then `bd close --suggest-next`.
  It does not move `main` or push. A cascade-conflicted pick fails this gate.
- **`finish open`** (`internal/cli/finish.go:160-258`, `newFinishOpenCmd`):
  `jj bookmark set <epic> -r @` then `jj git push -b <epic>` then
  `gh pr create --base main`. It ships everything reachable from `@`. The closed
  picks (`finishOpenPreflight` → `[]finishPick{Bead,Title,Change}`) are already
  gathered for the PR body — the exact list the collapse step needs.
- **`jj diff --name-only`** (jj 0.42): verified present empirically — listed in
  `jj diff --help` and emits one path per changed file (exit 0). The template
  alternative `self.diff.files()` does **not** parse in 0.42; `--name-only` is the
  correct idiom. (Recorded on `weft-eoe` as `grounding/empirical`.)
- **jj cascade semantics:** a conflict is stored in the commit and propagates to
  every descendant; jj cannot represent a conflict in git, so a conflicted commit
  cannot be pushed (same class as the author-less-commit rejection that broke CI
  in PR #39). Resolving an ancestor conflict-simplifies descendants.
- **Origin finding** `weft-78k`: in one linear stack an escalated change poisons
  all picks lex-above it; bd assigns ids, so the orchestrator cannot force the
  escalate-prone pick to be lex-last. The seam-10 E2E sidesteps this with two
  `integrate` calls.

## 3. Problem

A *shed* (wave) is a set of mutually independent `bd ready` picks. `shed
integrate` serializes them into one arbitrary lexicographic stack. That stack is
doing two jobs: (a) **detecting** inter-pick conflicts (stacking makes two
same-file picks collide visibly), and (b) **ordering** the changes for landing.

Job (a) is legitimate, but the single linear stack over-couples *independent*
picks: a pick that touches unrelated files still becomes a descendant of every
pick below it. When a conflict is **escalated** — left unresolved for a human —
its change stays conflicted indefinitely, and jj cascades that conflict to every
descendant. Those descendants then fail `pick land`'s `changeConflicted` gate and
cannot be pushed at `finish`, even though they are individually clean.

The only stack position where an escalated change harms nothing is the very top.
Because bd assigns the ids that determine lex order, neither the fixture nor the
orchestrator can guarantee the escalate-prone pick lands there. Hence a
single-wave integrate containing both a heal pair and an escalate pair is
infeasible today; the working pattern is multiple `integrate` calls with the
escalate group last.

The linear stack is the root: it manufactures ancestry between picks that have
none.

## 4. Design

### 4.1 `shed integrate` — file-overlap forest

Two picks can conflict **only if they touch a common file** — jj conflicts are
per-file. Therefore, partitioning the wave by file-overlap is provably safe: two
picks in different partitions cannot collide, so they never need to be stacked
together.

Algorithm:

1. Resolve each pick's sealed change-id (unchanged; `changeOf`).
2. For each change, read its touched file set:
   `jj diff --name-only -r <change>` (the files the pick adds/modifies relative
   to its parent). `--name-only` is verified present in jj 0.42 (§2). Validate
   each change-id against `changeIDPattern` before interpolation (unchanged
   guard).
3. Build **connected components** over the relation *"share ≥1 file"*
   (union-find). Each component is a group of picks that might mutually conflict.
4. **Order deterministically:** sort picks within a group lexicographically by
   change-id; order groups by their lexicographically-smallest member. This keeps
   integration reproducible (the property `sort.Strings` provided).
5. **Build a forest:** rebase each group as its own linear sub-stack rooted on
   `trunk()` — `jj rebase -s <ch> -o <prev>`, where `prev` resets to `trunk()` at
   each group boundary (rather than chaining across groups as today). Retain the
   `--skip-emptied` omission (ADR `weft-hjx.7`) within a group.
6. **Detect conflicts** with the existing scoped revset
   `conflicts() & (<all changes>)`. Because groups are independent, the only
   conflicts reported are genuine intra-group collisions — no cross-group
   cascade is possible.

Degenerate cases fall out naturally: a wave of all-independent picks yields N
singleton groups (N parallel children of `trunk()`, zero conflicts); a wave where
every pick touches one shared file yields a single group (today's behavior,
which is correct because they genuinely might all collide); transitive overlap
(A–B share `x`, B–C share `y`) merges A, B, C into one component.

### 4.2 Envelope

`shed.integrate` `data`:

- **`groups`**: `[][]{bead,change}` — the forest, one inner array per component,
  in deterministic order. Replaces the former flat `stack` field.
- **`conflicts`**: `[]{bead,change}` — **unchanged shape and meaning** (the
  orchestrator's actionable input to `conflict open <bead>`). This is the field
  the resolve loop consumes; keeping it stable means `conflict`/`resume` and the
  orchestrator loop need no envelope changes.

The current code builds the `changeToBead` map (used to map a conflicted
change-id back to its bead) by iterating the flat `stack[]`; after the refactor
it must be rebuilt by iterating **all members across all groups**. The
`conflicts` array is then assembled exactly as today.

The human-text summary lists groups (e.g. `integrated 6 picks in 4 groups: …`).
Per the seam-9 emit-field-drop guard, add a drift test asserting `groups` and
`conflicts` are always present (and `[]`-initialized, never `null`).

### 4.3 `conflict open` / `finalize` / `pick land` — unchanged

These operate on a change and its subtree. With grouping, that subtree is now a
single group instead of the whole wave, but the code is identical:

- **Heal:** `conflict finalize` squashes the resolution into the conflicted
  change; jj conflict-simplifies the group's descendants (un-cascade). Because
  the group is independent, no other group is touched. `scopedConflictChanges`
  already scopes to the change's subtree.
- **Escalate:** leaves the conflicted change in place with the `human` label.
  Across groups nothing depends on it (the forest guarantee), so it cannot poison
  any independent pick. Within its group its blast radius is bounded by jj's
  rebase direction — see the in-group note below.
- **`pick land`:** asserts the change is conflict-free, then `bd close`. Healed
  picks land; an escalated change fails the gate (correct).

**Which change conflicts within a group (and the in-group cascade bound).** A
group is stacked `trunk() <- m0 <- m1 <- …` via `jj rebase -s mᵢ -o mᵢ₋₁`. jj
applies the conflict to the **rebased (upper) change**: `m0` lands cleanly on
`trunk()`, and a later member that contends for the same file becomes conflicted
as the *descendant*. So in a **two-pick group `[A, B]`** the conflicted change is
always `B`, the tail — escalating it leaves nothing stacked above it, exactly the
"escalate-last" outcome, automatically. In a **group of three or more** that all
contend for one file, the heal fixpoint resolves the lowest conflict first
(un-cascading its descendants) and re-queries; if a member must ultimately
escalate while group members remain stacked above it, those members stay
conflicted and cannot land. This residual **in-group cascade is irreducible and
honest**: those picks genuinely contend for the same file, so one being left
unresolved legitimately blocks the others — unlike the *false* cross-group
cascade (independent picks) that the forest eliminates. Finer intra-group
splitting (per-file sub-grouping so non-contending members of a group can still
land) is a possible future refinement and is **out of scope**; the fixture's
conflict groups are all two-pick, so the single-wave E2E (§6) is unaffected.

### 4.4 `finish open` — collapse step

`finish open` ships everything reachable from `@`. With a forest, the closed
picks are spread across group sub-stacks and `@` does not tip a single line that
contains them all. Before `jj bookmark set <epic> -r @`, `finish open` collapses
the **closed** picks into one line:

1. Take the closed picks (already gathered by `finishOpenPreflight` as
   `[]finishPick` with change-ids). Order them **group-major, preserving each
   group's internal stack order** — *not* a global lex sort. Intra-group order
   encodes a real dependency: a healed upper member's resolved content is defined
   relative to the member below it (the heal squashed the merge into the upper
   change as a delta on its parent), so the lower member must remain its ancestor
   in the collapsed line. Groups themselves are independent and may be
   concatenated in any deterministic order (group order from §4.1).
2. For each closed change, in that order, `jj rebase -r <ch> -o <line-tip>` —
   moving **only that change** (not `-s`, so an escalated tail is never dragged),
   with `line-tip` advancing from `trunk()` through the closed changes. Each
   closed change is conflict-free and is placed atop its own group's
   already-placed lower members, so every rebase is clean.
3. Reposition `@` to the top of the assembled line (`jj new <top>`), so the
   existing `bookmark set -r @` + push ships exactly the landed line.

An escalated/open pick is **not** in the closed set, so it is never rebased into
the line; jj re-parents it onto `trunk()` (off the pushed line) where it stays
conflicted for the human. The PR contains the landed picks; the escalated pick is
a local follow-up.

`finish open --dry-run` returns before any jj mutation today
(`finish.go:195-198`), so it already previews without mutating; the collapse step
must be added **after** that early return, leaving the dry-run path a no-op (no
new dry-run implementation task — only an assertion that collapse is not reached
under `--dry-run`).

**`finish reconcile` interaction.** Reconcile has two branches
(`finish.go:405-430`), and the collapse changes the local topology both must cope
with — a parked, still-open escalated pick is **not** part of the merged work and
must survive reconcile:

- **`mergeStyleSquashOrRebase`** — `jj new main` then abandon
  `roots(trunk()..@)`. After collapse `@` tips the landed line; the parked
  escalated change is a *separate* root under `trunk()`, so it is **not** in
  `trunk()..@` and is correctly excluded from the abandon set. This branch is
  expected to handle the forest shape unchanged — the plan must confirm.
- **`mergeStyleMergeCommit`** — `jj rebase -b @ -o main --skip-emptied`. `-b @`
  rebases the whole branch reachable around `@`; the plan **must verify** it does
  not drag the parked escalated sibling into the rebase (and, if it would,
  scope the rebase to the collapsed line). This is the higher-risk branch.

Both are flagged as plan-time verification items in §7.

### 4.5 `execute.md` Steps 5–6

The "escalate-last ordering" guidance added in `weft-w1y.6` (the orchestrator
must order escalated pairs at the top of the stack) becomes obsolete: grouping
removes cross-pick cascade, so the orchestrator no longer reorders anything. Update
Steps 5–6 to describe: one `integrate` yields a forest with conflicts confined to
overlap groups; resolve each group's conflict (heal un-cascades within the group;
escalate leaves the group tail for a human); land the conflict-free picks; the
fixpoint is per-group and needs no global ordering. Both copies also document
the `shed.integrate` envelope as `"stack": […]`; update that to `"groups"` in the
**same** change that lands the engine envelope rename (§4.2), so the prose and the
emitted shape never disagree. Keep the source (`weft/workflows/execute.md`) and
the shipped copy (`plugin/skills/execute/SKILL.md`) in lockstep (seam-7
discipline).

## 5. Determinism & safety

- **Determinism:** within-group order (lex by change) and group order (lex by
  smallest member) are total and reproducible — preserving the guarantee
  `sort.Strings` gave, now per group. The `finish` collapse reuses these orders
  (group-major, intra-group preserved; §4.4), so it is deterministic too.
- **Injection guard:** every change-id is still `changeIDPattern`-validated
  before any revset interpolation (the standing weft idiom).
- **No silent drops:** retain the `--skip-emptied` omission within a group (ADR
  `weft-hjx.7`); an empty member surfaces rather than breaking the cursor.
- **Conservative grouping:** file-overlap may group picks that do not actually
  conflict (they share a file but edit disjoint regions). That is safe — they
  stack cleanly and both land; grouping only needs to guarantee that any two
  picks that *could* conflict are in the same group, which per-file overlap does.

## 6. Testing

1. **Unit (TDD core):** a pure grouping function `overlapGroups(changeFiles
   map[string][]string) [][]string` — table-driven over: all-independent,
   one-shared-file, two disjoint pairs, transitive chain, singleton, empty. Fast,
   no substrate. This is where the design is proven; the engine wiring is thin
   around it.
2. **Integration — the single-wave proof** (`internal/weave`, `//go:build
   integration`): drive **one** `shed integrate` over the full 6-pick fixture
   wave. Assert exactly **2** conflicts (the p2 and p4 colliders) with **no
   cascade**; heal p2, escalate p4; land the 5 conflict-free picks; `finish open`
   collapses the 5 into a line excluding p4's escalated tail; `resume` shows 5
   landed + 1 escalated (`human`-labelled, conflicted, in_flight). This replaces
   the two-call structure in `TestWeaveLoopEndToEnd` and is **deterministic
   regardless of bd-assigned id order** — the property that made the single-wave
   proof infeasible before.
3. **Envelope drift test** (seam-9 guard): assert `shed.integrate` always emits
   `groups` and `conflicts` as present, `[]`-initialized arrays.
4. **`finish` collapse test:** unit-level (mocked runner) assert the collapse
   issues per-change `jj rebase -r` for each closed pick, in group-major /
   intra-group-preserving order, and excludes a non-closed (escalated) change;
   integration-level coexistence covered by (2).

## 7. Risks & open questions

- **`finish reconcile` both branches (§4.4):** the squash branch
  (`roots(trunk()..@)` abandon) is expected to exclude the parked escalated
  sibling for free; the merge-commit branch (`jj rebase -b @ -o main`) is
  higher-risk — verify `-b @` does not drag the escalated sibling, and scope the
  rebase to the collapsed line if it would. Both must be re-verified against the
  forest + parked-escalated shape.
- **`jj rebase -r` re-parenting:** the collapse relies on `-r` moving a single
  change and jj re-parenting its (escalated) descendants onto the group root.
  The plan must confirm jj 0.42 re-parents an escalated descendant cleanly when
  its landed parent is lifted out, leaving the escalated change conflicted on
  `trunk()` (empirically verify; jj handles conflicted commits in rebases, but
  confirm the parked result is pushable-excluded, not dragged).
- **Multiple escalations in one wave:** N escalated picks park as N siblings on
  `trunk()`. The design handles this (each is its own group tail); the E2E covers
  one, but the unit grouping test covers multi-group shapes.

## 8. Decisions worth recording (ADR candidates)

- **Integration topology is a file-overlap forest, not a linear stack** — the
  core decision; supersedes the lexicographic single-stack rationale (ADR
  `weft-hjx.7` context).
- **`finish` linearizes at push time via per-change `jj rebase -r` over the
  closed picks** — landing stays a bead-close; linearization is deferred to
  `finish` so the forest is preserved through the resolve loop.
