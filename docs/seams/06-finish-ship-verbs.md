<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 6 — `weft finish` ship verbs

> Status: **design-reviewer READY** (round 2). Sub-spec of
> [`docs/design.md`](../design.md) §6 (Ship) and §9. Tracked as bead
> `weft-hjx.9` (child of `weft-hjx`). Promotes the `finish open|reconcile` verbs
> scoped — but left unbuilt — in [seam 1](01-command-surface.md) §4.4. Round-1
> findings fixed inline (merge-detection revset §6.1.3; PR-body single-read +
> helper §5; `merge_style` enum §8; `bookmark set -r @` clarification §4.1; ADR
> materialization §9; `picks.title` envelope §8). No implementation exists yet.

## 1. Scope

The two epic-level ship verbs that take a finished epic off the loom and into a
PR, then reconcile the local jj state after a human merges:

- `weft finish open <epic>` — push the epic's woven stack and open a GitHub PR
  whose body is assembled from the epic's closed beads.
- `weft finish reconcile <epic>` — after the PR merges, clean up the local jj
  topology (abandon the now-redundant local stack, drop the bookmark).

This unblocks porting the deferred `/weft-finish` and `/weft-verify` prompts
(seam 5), which need these verbs to exist.

**Out of scope:** the human merge itself (`main` is protected; `gh pr merge` is
human-gated — the engine opens and reconciles *around* the merge, never performs
it); the `/review-pr` gate (host/prompt layer); `weft install` (§8); any
non-squash workflow policy change.

## 2. Grounding

- **seam-1 §4.4** scoped both verbs at the command-surface level (coarse,
  bead-driven). This seam promotes them to an implementable spec.
- **design.md §6 (Ship)** and **§5.1 (Audit)**: the PR body is generated from
  the epic's closed beads, each carrying its change-id — there is no
  `SUMMARY.md`. "Beads is the brain."
- **GSD `gsd-ship` (deepwiki open-gsd/gsd-core)**: a *deterministic script that
  calls `gh` directly* (not agent-delegated). Flow: preflight (verified, clean
  tree, on a feature branch, remote configured, `gh` installed + authed) → push
  → assemble a rich PR body → write body to a **temp file** (shell-arg limits) →
  `gh pr create --title --body-file --base [--draft]`. Weft's engine **is** that
  deterministic layer (design.md §7), so `finish` calls `gh` directly; the body
  source is the epic's closed beads rather than `.planning` markdown.
- **Lived evidence (this project, PR #17 & #18)**: seam-1 §4.4's prescribed
  reconcile command `jj rebase -b @ -o main --skip-emptied` **conflicts on
  squash-merges** — the rebase re-applies content already present in `main` via
  the squash commit. The working approach both times was `jj new main` +
  `jj abandon <stack-root>::`. Also: `gh pr merge --delete-branch` did **not**
  reliably delete the remote branch (#18 required a `gh api -X DELETE`).

## 3. Verb surface

```
weft finish open <epic>       [--draft] [--dry-run] [--json] [--pick <path>]
weft finish reconcile <epic>  [--dry-run] [--json] [--pick <path>]
```

Both are coarse, idempotent, and honor the uniform output contract (seam-1 §3 /
`internal/envelope`): default text, `--json` envelope `{ok,verb,data,next}`,
`--pick` field projection.

## 4. `finish open <epic>`

### 4.1 Steps

1. **Preflight** (all failures → `exit.Invocationf`, exit 1, with a specific
   message — never a cryptic mid-push `gh` error):
   - working tree clean (`jj st` reports no changes);
   - the epic's woven stack is resolvable: `@` is a descendant of `trunk()` with
     at least one non-empty change to ship (the bookmark is *set* in step 2, so
     it need not pre-exist);
   - an `origin` remote is configured;
   - `gh` is installed and authenticated (`gh auth status`).
2. `jj bookmark set <epic> -r @` → `jj git push -b <epic>`. (`-r @` is correct
   here: it sets the bookmark *at* the working-copy tip. The seam-1 §4.4 "never
   `-r @`" warning is about `jj rebase -b @` chain-truncation, not `bookmark
   set` — different command, different flag semantics.)
3. **Assemble the PR body** from the epic's closed beads (§5), written to a
   temp file via the existing `writeTempPayload` idiom (`plan.go`) — `0600`,
   cleaned up after.
4. `gh pr create --title <derived> --body-file <tmp> --base main` (add
   `--draft` when `--draft` is passed).
5. **`--dry-run`** mutates nothing: it emits the push plan, the derived title,
   the assembled body, and the exact `gh pr create` command (symmetry with
   `plan emit --dry-run`).

### 4.2 PR title

Derived from the epic, not synthesized prose: `"<epic-title> (<epic-id>)"`
(e.g. `Drain weft-yga: seam follow-up hardening (weft-yga)`). Deterministic,
greppable, no LLM call.

### 4.3 Idempotency / re-run

If a PR already exists for the bookmark (`gh pr view <epic>` resolves an open
PR), `finish open` re-pushes the bookmark (picking up new commits) and reports
the **existing** PR URL rather than erroring — so iterating on review feedback
(push fixes → re-run) is safe and a no-op when nothing changed.

## 5. PR-body assembly (from closed beads)

The body is generated, never hand-written (design.md §5.1 audit). Source: the
epic's **closed** child beads, read in **one** `bd list --parent <epic>
--status closed --json` call that yields the full bead objects — each pick's
**title** (the conventional-commit subject) *and* its `labels` (from which the
`jj-change:<id>` value is extracted). Note: the existing `resume` helpers are
close but neither suffices alone — `beadIDsByStatus(epic, "closed")` returns
only `[]string` of ids, and `epicChanges()` extracts only the change-id labels
(dropping titles). `finish open` therefore needs a small helper (extend
`epicChanges` to a `closedPicks(epic) []struct{Bead, Title, Change string}`, or
a new reader) that parses title + `jj-change` label together from that single
`bd list` read. Do **not** implement it as `beadIDsByStatus` + per-bead
`bd show` (N+1, and title-less reads silently produce incomplete bodies).

Structure (Markdown):

- **Summary** — one line: `N picks woven for <epic-id> — <epic-title>.`
- **Picks** — one bullet per closed bead: `` - `<bead-id>` <conventional-commit
  subject> (`<change-id>`) ``. The subject is read from the bead title /
  description; the change-id ties each line to its woven jj change (the audit
  trail).
- **Trailer** — the standard generated-by attribution line.

Empty-epic guard: if the epic has zero closed beads, `finish open` refuses
(exit 1, "nothing woven to ship") rather than opening an empty PR.

## 6. `finish reconcile <epic>`

### 6.1 Steps

1. **Merged-state gate** (safety — never abandon unmerged work):
   `gh pr view <epic> --json state,mergeCommit`. Proceed only if
   `state == "MERGED"`; otherwise `exit.Invocationf` (exit 1, "PR for <epic> is
   not merged (<state>) — refusing to reconcile"). jj alone cannot distinguish a
   squash-merge from a never-merged branch (in both, the epic's commits are
   absent from `main`), so an authoritative `gh` signal is required.
2. `jj git fetch` (advances `main@origin`; the local `main` bookmark stays
   stale — compare against `trunk()` / `main@origin`).
3. **Detect merge style** — is the epic's pushed tip an ancestor of
   `main@origin`? The concrete check (jj's ancestry idiom is `X & ::Y`, not
   git's `merge-base --is-ancestor`): the pushed tip is the remote-tracking ref
   `<epic>@origin`; it is an ancestor of trunk iff

   ```
   jj log -r '<epic>@origin & ::main@origin' --no-graph -T 'commit_id'
   ```

   returns a non-empty result. (`::main@origin` is the inclusive ancestor set of
   the fetched trunk.) Branch on the result:
   - **Non-empty → merge-commit / true-merge** → the pushed commits are
     reachable from trunk; `jj rebase -b @ -o main --skip-emptied` abandons them
     as empty cleanly.
   - **Empty → squash-merge or GitHub rebase-merge** → the content landed under
     a new commit id and jj change-ids never reach GitHub, so a rebase would
     re-apply and conflict. Use `jj new main` + `jj abandon '<stack-root>::'`
     (stack root via `roots(trunk()..@)`). Squash and rebase-merge are
     structurally indistinguishable here and take the same path, so they collapse
     to one `merge_style` value (§8).
4. **Drop the bookmark** — `jj bookmark delete <epic>` (the squash path's
   abandon already deletes it; this is the idempotent backstop). If the remote
   branch survives (`gh pr merge --delete-branch` is unreliable), delete it via
   `gh api -X DELETE repos/{owner}/{repo}/git/refs/heads/<epic>`.
5. **`--dry-run`** emits the detected merge style and the exact planned commands
   without mutating.

### 6.2 Safety net

A mistaken abandon is recoverable: `jj abandon` is reversible via `jj op log`
(`jj op revert <op-id>`) and `jj evolog`. The merged-state gate (6.1.1) is the
primary guard; recoverability is the belt-and-suspenders backstop. (Recovery is
change-scoped per the project hard rules — never `jj op restore`.)

## 7. Error handling & exit codes

Matches every sibling verb (seam-1 §3):

- **exit 1 (`Invocationf`)** — preflight failures (unclean tree, no resolvable
  stack/bookmark, no remote, `gh` unauthenticated), empty epic, and the
  reconcile merged-state gate (PR not merged).
- **exit 2 (`Hardf`)** — any subprocess (`jj`/`bd`/`gh`) that fails to start or
  returns non-zero, surfacing its stderr.

## 8. `--json` envelopes

`finish.open`:

```json
{ "epic": "...", "bookmark": "...", "pushed": true,
  "pr_url": "https://github.com/.../pull/N", "pr_exists": false,
  "picks": [{"bead": "...", "title": "...", "change": "..."}], "dry_run": false }
```

`finish.reconcile`:

```json
{ "epic": "...", "merged": true, "merge_style": "squash_or_rebase",
  "abandoned": ["<change-id>", "..."], "bookmark_deleted": true,
  "remote_branch_deleted": false, "remote_branch_warning": "", "dry_run": false }
```

`remote_branch_warning` is present-but-empty (`""`) on success and on the
spec-sanctioned silent 404 (§6.1.4 best-effort); it carries a diagnostic string
only when the remote-branch deletion fails for a non-404 reason (slug
resolution, auth/429/5xx, parse), which is surfaced without aborting reconcile.

`merge_style` is a closed enum with exactly two values, one per detection branch
(§6.1.3): `"merge_commit"` (pushed tip is an ancestor of trunk → rebase path)
and `"squash_or_rebase"` (not an ancestor → `new`+`abandon` path; squash and
GitHub rebase-merge are structurally indistinguishable and share this value).

All list fields are initialized `[]T{}` (never `var x []T`) so an empty result
serializes as `[]`, not `null` — the engine output contract (a documented weft
memory + existing convention). Every list-emitting verb gets an empty-output
test asserting `"<key>": []`.

## 9. Decisions to capture as ADRs

1. **`finish reconcile` supersedes seam-1 §4.4's `rebase`-only command** with
   runtime merge-style detection (rebase for true-merges; `new`+`abandon` for
   squash/rebase-merge). Grounded in lived PR #17/#18 conflicts.
2. **The engine wraps `gh`** (its third external CLI, after `bd` and `jj`) for
   `finish`. Justified: `gh` is a deterministic CLI, not agent dispatch (the one
   thing the engine must not do — design.md §7); it mocks identically through
   `run.Runner`; it is the faithful translation of GSD's `gsd-ship` script.

Both ADRs are materialized by the `capture-adrs` lifecycle step (run after this
spec is READY), which mints a bead id + a `docs/adr/<id>-<slug>.md` file per
decision — matching the project's ADR precedent (`weft-hjx.7` for the
`--skip-emptied` omission, `weft-lc7` for the conflict model). This `## 9`
list is the input to that step, not the ADR record itself.

## 10. Testing

Per the weft test conventions (assert on the specific envelope field via
`json.Unmarshal` into a shape struct, never `strings.Contains` on whole output;
verify fail-first):

- `finish open`: preflight refusals (each → exit 1); PR-body assembled from
  closed beads (decode `data.picks`); `--dry-run` mutates nothing (no `jj git
  push` / `gh pr create` in `r.calls`); empty-epic refusal; idempotent re-run
  reports existing PR; empty `picks` serializes as `[]`.
- `finish reconcile`: merged-state gate refuses when `state != MERGED` (exit 1,
  no `jj abandon`/`rebase` in `r.calls`); merge-style detection picks `rebase`
  vs `new`+`abandon` per ancestry; `--dry-run` mutates nothing; `abandoned`
  serializes as `[]` when empty.

## 11. Out of scope / deferred

- The human merge (`gh pr merge`) — protected `main`, human-gated.
- `/weft-finish` and `/weft-verify` prompt ports — unblocked by this seam, done
  in the seam-5 port flow once these verbs land.
- Non-squash merge *policy* — detection handles all styles, but the project
  policy remains squash-merge to protected `main`.
- `weft install` (§8 sub-seam) — unrelated distribution transform.
