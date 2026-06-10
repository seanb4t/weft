<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-verify-work (GSD Core, MIT) -->

# verify-work workflow

Interactive UAT for `/weft-verify-work`. Enumerates an epic's deliverables,
walks the human through them one at a time, auto-diagnoses failures, and files
fix picks under the epic. Scoped to any epic id — phase sub-epic or one-shot
feature epic.

**Adapted:** deliverable enumeration fallback chain, per-item y/n loop (empty =
pass), severity inference from user wording, and parallel diagnosis-agent
dispatch are adapted from GSD Core's verify-work workflow.

**Rewritten (§3–§8 tool-layer mapping):** all state is persisted to bead notes
via `bd note` and to new pick beads via `bd create`. No external tracking
artifacts are written or read — beads is the brain. The machine `weft pick
verify` gate (verdict-as-data, exit 0) already ran per pick during `execute`;
this workflow is the human UAT layer on top, not a replacement.

---

## 1. Inputs

| Input | Description |
|-------|-------------|
| `epic-id` | The bead ID of the epic whose deliverables are being verified (required). |

---

## 2. Input validation

Before interpolating `epic-id` into any shell command, validate its shape:

```
^[a-zA-Z0-9][a-zA-Z0-9._-]*$
```

Refuse with a clear error message if the id does not match. Do not proceed
until a conforming id is supplied.

---

## 3. Resume check

Before enumerating deliverables, read the epic's existing notes to detect a
prior interrupted session:

```bash
bd show <epic-id> --json | jq -r '.[0].notes // ""'
```

Any note prefixed `verify-work:` records a prior verdict. Extract the
deliverable name and verdict from each such note. Already-passed items (notes
ending `— pass`) are **skipped** in the per-item loop (§5). Already-failed
items (notes ending `— FAIL(...)`) that have not been re-verified are surfaced
for context but re-presented in the loop unless the user opts to skip.

---

## 4. Enumerate deliverables

Enumerate the epic's deliverables via the following fallback chain (stop at the
first level that yields at least one item):

1. **Phase epic acceptance criteria** — parse the `acceptance` field of
   `bd show <epic-id>`. Each distinct acceptance criterion is one deliverable.
2. **Closed picks' acceptance criteria** — if the epic's acceptance field is
   empty, enumerate closed child picks:
   ```bash
   bd show <epic-id> --children --json | jq -r '.[] | arrays | .[] | select(.status=="closed") | .id'
   ```
   For each closed child, run `bd show <child-id> --json` and extract its
   acceptance criteria via jq path `.[0].acceptance_criteria`. Each acceptance
   criterion becomes a deliverable attributed to that pick's title.
3. **Epic goal/description as last resort** — if neither of the above yields
   deliverables, use the epic's `title` and `description` as a single
   deliverable phrased as: "The epic goal is met: `<title>` — `<description>`."

Each deliverable is phrased as a **user-observable "what should happen"
checkpoint** — concrete, testable, written from the user's perspective. Do not
use internal implementation language.

If `bd show` fails at any point (unknown id, network error), surface the error
and stop.

---

## 5. Per-item verification loop

For each deliverable (in enumeration order, skipping already-passed items from
§3):

1. Present the checkpoint clearly, numbered: `[N/Total] <deliverable text>` (where Total = the deliverable count from §4).
2. Ask: "Does reality match this? (y/n/empty = pass)"
3. Branch on response:
   - `yes`, `y`, or empty (Enter) → **pass**. Append note immediately:
     ```bash
     printf '%s' "verify-work: <deliverable> — pass" | bd note <epic-id> --stdin
     ```
     (Literal fixed-text notes may use the plain quoted form; use stdin whenever
     the note contains bead-sourced text such as deliverable names.)
     Advance to the next item.
   - Any other response → **fail**. Record the user's exact words verbatim as
     the issue description. Append note immediately. Because the user's words
     may contain backticks, `$`, `!`, or other shell-special characters, use
     the stdin form to avoid expansion hazards:
     ```bash
     printf '%s' "verify-work: <deliverable> — FAIL(<severity>): <user words>" \
       | bd note <epic-id> --stdin
     ```
     where `<severity>` is inferred in §6.

The note is written **after every verdict** so that an interrupted session can
resume via §3 without re-checking already-decided items.

---

## 6. Severity inference

Infer the severity tier from the language the user uses to describe the
failure:

| User wording signals | Severity | Priority mapping |
|---|---|---|
| crash, data loss, corruption, unrecoverable, hang | P1 (blocker) | `--priority=P1` |
| doesn't work, fails, broken, not working, wrong output | P2 (major) | `--priority=P2` |
| looks off, cosmetic, alignment, typo, minor, slightly | P3 (minor) | `--priority=P3` |

When wording is ambiguous, default to P2. Record the inferred severity in the
note (see §5) and use it when creating the fix pick (§8).

---

## 7. Diagnose failures

After the loop completes, for each recorded failure dispatch fresh read-only
diagnosis agents in parallel across all failures — host-runtime agent dispatch
is the host's concern; this workflow describes the step, not the plumbing.

Each agent receives:
- the deliverable text,
- the user's verbatim failure description,
- read-only access to the repository (file tools, `bd show`, `rg`).

The agent's task: identify the root cause — which file(s), function(s), or
missing behavior account for the gap — and return a concise root-cause
description (2–5 sentences).

Fold each root-cause description into the failure record before filing picks in §8.
Do not mutate repository state in the diagnosis agent — it is read-only.

---

## 8. File fix picks

For each diagnosed failure, create a fix pick under the epic:

```bash
bd create --parent <epic-id> --type=bug --priority=<severity-mapped> \
  --title "UAT fix: <deliverable>" \
  --description "<user words + diagnosed root cause>" \
  --acceptance "<the failed checkpoint, restated as the pass condition>" \
  --labels "uat-fix"
```

When the `--description` value is multi-line (user words + root cause diagnosis
typically span multiple lines), supply it via stdin to avoid shell quoting
issues:

```bash
bd create --parent <epic-id> --type=bug --priority=<severity-mapped> \
  --title "UAT fix: <deliverable>" \
  --body-file - \
  --acceptance "<the failed checkpoint, restated as the pass condition>" \
  --labels "uat-fix" <<'EOF'
<user words>

Root cause: <diagnosed root cause>
EOF
```

where `<severity-mapped>` is the P-form from the §6 table (P1, P2, or P3).

The quoted heredoc delimiter (`'EOF'`) prevents shell expansion of backticks
and `$` in the description body.

The `--acceptance` value is the failed checkpoint restated as a pass condition:
"Given <context>, <deliverable text> is true."

---

## 9. End states

**Fix picks filed (one or more failures):**

Append a completion note:

```bash
bd note <epic-id> "verify-work: <F> of <N> deliverables failed; <F> uat-fix picks filed"
```

Then suggest the remediation path:

> Fix picks are now filed under `<epic-id>`. Run:
> ```
> /weft-execute <epic-id>
> ```
> to implement the fixes (the `uat-fix` picks are the ready set). Then re-run:
> ```
> /weft-verify-work <epic-id>
> ```
> to confirm all deliverables pass before finishing the epic.

Do not close the epic. This workflow is the human gate before shipping; closure is
the operator's decision.

**All deliverables passed:**

Append the completion note:

```bash
bd note <epic-id> "verify-work: all <N> deliverables passed"
```

Then suggest finishing the epic:

> All `<N>` deliverables passed. When ready to ship:
> ```
> weft finish open <epic-id>
> ```
> (`finish open` opens the PR; `finish reconcile` reconciles if needed.)

Note: `weft finish open` and `weft finish reconcile` are the stable surface
verbs. Bare `weft finish` is not a verb.

---

## 10. Complement, never replace

The machine `weft pick verify` gate (`weft pick verify <bead>`, verdict-as-data,
exit 0) already ran per pick during `execute`. That gate confirms each pick's
automated acceptance criteria passed at seal time.

This skill is the **human UAT layer on top**: it verifies that the integrated
result actually behaves as the user expects, catching gaps the automated gate
cannot — usability issues, missing edge-case behavior, cosmetic failures, and
emergent integration problems that only appear when the full feature is exercised
end-to-end.

This skill should run after `execute` completes
and before `weft finish open`.

---

## 11. Dropped GSD mechanics

The following GSD verify-work mechanics are intentionally absent from this
workflow:

- **External deliverable-list files** — in GSD, deliverables are extracted from
  a generated artifact on disk. Here, deliverables come from the bead's
  acceptance criteria and child picks via the §4 fallback chain. No file is
  parsed; beads is the source of truth.
- **External UAT progress tracking** — GSD persists verification state in a
  per-session planning file. This workflow persists all progress to `bd note`
  traces on the epic instead. Beads is the brain; no files are created.
- **External completion marking** — the epic's completion state lives in beads
  (`bd close`); this workflow does not touch it.
- **`gsd-planner`/`plan-checker` gap-closure loop** — weft files fix picks
  directly; the execute workflow's own `weft pick verify` gate covers fix
  quality during implementation. There is no separate gap-closure planning step.
- **Filtered-gaps execution mode** — in GSD, re-execution can be scoped to only
  the failed items via a flag. In weft the filed fix picks ARE the epic's ready
  set after `execute`; no filter flag is needed or provided.
