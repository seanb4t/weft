---
description: Feasibility experiments — decompose a technical question into Given/When/Then hypotheses, run throwaway jj-change experiments, record VALIDATED/INVALIDATED/PARTIAL verdicts as a spike bead + a bd remember finding. Optional epic context.
argument-hint: "[question] [epic-id]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-spike (GSD Core, MIT), rewritten to throwaway-jj + bead-backed state -->

# spike workflow

Focused feasibility experiments before committing to an approach. weft's analog
of `/gsd-spike`. Each experiment is a **Given/When/Then** hypothesis run in a
**throwaway jj change** that is abandoned afterward — the code is disposable, the
**verdict** is what survives, as a `spike` bead + a `bd remember` finding.

Run loose (pre-project: "is X feasible before I commit?") or with an optional
`[epic-id]` (mid-flow: "can this phase's approach work?", feeding `plan-phase`).

> jj note: all jj commands use `--no-pager`; experiment code is never kept. The
> working copy is always a commit and auto-snapshotted, so your in-flight work is
> never lost across the spike.

---

## Phase 1 — Intake + decompose

Treat the optional `[question]` as the seed; one light round of framing
questions if the technical uncertainty is unclear. Decompose it into **2–5
independent experiments**, each phrased as a **Given/When/Then** hypothesis:

```
Given <starting condition>
When  <the thing we try>
Then  <the observable result that would confirm/deny feasibility>
```

State the experiments to the user before running them. (No `--quick`/`--text`
mode flags in v1 — default adaptive mode only, consistent with `discuss`.)

---

## Phase 2 — Run each experiment in a throwaway jj change

Before touching jj, **save the current working-copy change id** so you can
restore the user afterward:

```
SAVED=$(jj --no-pager log -r @ --no-graph -T 'change_id.short(12)')
```

Then, for each experiment:

```
jj --no-pager new main          # loose; epic-scoped: jj --no-pager new <epic-tip>
# ... write the minimal experiment code, run it ...
```

For the epic-scoped case, resolve `<epic-tip>` from the epic's most recent landed
pick — its `jj-change:<id>` label — or a bookmark at the epic tip. Observe the
result against the Given/When/Then and record the verdict — **VALIDATED**,
**INVALIDATED**, or **PARTIAL** — with the concrete evidence (what ran, what
happened). Then discard the experiment:

```
jj --no-pager abandon @
```

After all experiments, **return the user to where they started**:

```
jj --no-pager edit "$SAVED"
```

Never keep experiment code; the verdict + evidence captured in Phase 3 is the
durable artifact.

---

## Phase 3 — Persist the verdicts

Record one **`spike` bead** capturing the question, every Given/When/Then, each
verdict, and the evidence:

```
ID=$(printf '%s' "<for each experiment: the Given/When/Then + VERDICT + evidence>" \
  | bd create --type chore --labels spike --title "Spike: <question>" --stdin --json | jq -r .id)
```

(`bd create` requires `--title`; the per-experiment Given/When/Then + verdicts go
in the description via `--stdin`.)

If invoked with `[epic-id]`, parent the spike bead under it
(`bd create … --parent <epic-id> …`) and fold the validated decision into the
epic's `design` field so `discuss`/`plan-phase` see it **this session**:

```
bd update <epic-id> --design "<settled technical choice + why, from the spike>"
```

Close the spike bead (the investigation is done):

```
bd close "$ID" --reason "spike concluded: <one-line verdict summary>"
```

Record the validated finding as a durable memory for **future** sessions
(`bd prime` injects memories at session start, so this surfaces next session —
the epic `design` field above is the same-session path):

```
bd remember "spike: <library/approach> <VALIDATED|INVALIDATED|PARTIAL> for <use-case> — <evidence>" --key spike-<slug>
```

---

## Phase 4 — Conclude + hand off

Report the verdicts. A **VALIDATED** spike unblocks planning — run `plan-phase`
(epic-scoped) or `feature`/`new-project`. An **INVALIDATED** one redirects the
approach — back to `explore`/`discuss` with the finding in hand.

---

## What this workflow does NOT do

- It does not keep experiment code — every experiment change is abandoned; only
  the verdict survives.
- It does not write `.planning/spikes/` files or a wrap-up skill — the durable
  outputs are the `spike` bead + `bd remember`.
- It does not plan or implement the validated approach — that is
  `plan-phase`/`feature`.
- It does not leave the user's working copy moved — it restores the saved `@`.
