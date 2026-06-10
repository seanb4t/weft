<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-discuss-phase (GSD Core, MIT) -->

# discuss workflow

Interactive HOW-shaping for `/weft-discuss`. Grounds implementation decisions
in the epic's stated goal and the actual source, settles gray areas through
structured questions, and persists locked decisions to the epic's bead `design`
field for the planner to consume. Scoped to any epic id — phase sub-epic or
one-shot feature epic.

**Adapted:** gray-area derivation methodology, per-area question rhythm
(~4 single questions, 2–3 recommended-choice options), scope-creep deferral,
and prior-decision skip are adapted from GSD Core's discuss-phase workflow.

**Rewritten (§3–§7 tool-layer mapping):** all state is persisted to the epic's
bead `design` field and notes via `bd update`/`bd note`. No external tracking
artifacts are written or read — beads is the brain. GSD's mode flags
(`--auto`, `--batch`, `--analyze`, `--assumptions`, `--all`, `--power`,
`--text`) are intentionally absent — v1 ships the default adaptive mode only
(future overlays when needed). The warp lives in beads.

---

## 1. Inputs

| Input | Description |
|-------|-------------|
| `epic-id` | The bead ID of the epic whose HOW decisions are being settled (required). |

---

## 2. Input validation

Before interpolating `epic-id` into any shell command, validate its shape:

```
^[a-zA-Z0-9][a-zA-Z0-9._-]*$
```

Refuse with a clear error message if the id does not match. Do not proceed
until a conforming id is supplied.

---

## 3. Load the epic

```bash
bd show <epic-id>
```

The epic's title, description, and acceptance criteria define the **phase
goal** — the bounded scope within which all discussion stays.

If the epic already carries a `design` field, read it before deriving gray
areas. Decisions already recorded there are **prior locked decisions**; do not
re-ask them. The same applies to any `discuss:` prefixed notes on the epic:
treat them as settled context, not open questions.

If `bd show` fails (unknown id, network error), surface the error and stop.

---

## 4. Scout implicated source files

Read the source files, directories, and interfaces the epic's
description/acceptance criteria name or strongly implicate. Use host file
tools (`Read`, `Bash` with `rg`, directory listings) to load only what is
relevant — do not load the entire tree.

The scouted file paths populate:

- the options offered in per-area questions (grounded in code reality, not
  abstract choices), and
- the `## Canonical refs` section of the persisted design doc.

If the epic is purely additive (no existing code implicated), note that
explicitly; the scout step is still required to confirm the absence.

---

## 5. Derive and present gray areas

From the epic goal and scouted files, derive the **phase-specific
implementation decision areas** — the concrete choices this phase's planner
would otherwise have to guess. These are never generic categories; they are
specific to this epic's scope.

Examples of what gray areas look like in practice:

- which library or config approach to use for a specific feature boundary
- API or CLI shape (flag names, subcommand structure, output format)
- data layout or schema design for a new store
- error-handling or logging convention for a new subsystem
- UX copy or interactive prompt wording for human-facing output

Present the derived list to the user. Let the user select which areas to
discuss. Respect the selection — do not discuss areas the user skips, and do
not invent areas not grounded in the epic's scope.

If no meaningful gray areas exist (the epic's acceptance criteria fully
constrain the HOW), say so explicitly and exit without writing to the design
field. Do not manufacture discussion.

---

## 6. Per-area question rhythm

For each selected area, ask up to **~4 single questions**, one at a time.

Each question must:

- address exactly one decision (no compound questions),
- offer **2–3 concrete options** drawn from the scouted code reality,
- identify a **recommended choice** with a one-line rationale, and
- use the host `AskUserQuestion` facility where available (surfaces the
  question as a structured prompt with selectable options rather than free
  text).

When the user's answer locks a decision, record it and move to the next
question. Stop the area early when all of its decisions are locked — do not
fill the quota mechanically if fewer questions settle the area.

**Scope creep:** if the user raises an idea that lies outside this epic's
scope, acknowledge it, capture it in the Deferred section (see §7), and
redirect the discussion to the current epic's domain. Do not expand scope
mid-discussion.

---

## 7. Persist locked decisions

When discussion of all selected areas is complete (or when the user ends the
session early), persist the locked decisions to the epic's design field.

The design doc format uses the following markdown headings. Every heading must
be present; use an empty body under a heading if nothing applies to it:

```
## Domain
<one paragraph: what this epic builds and the key constraints grounding the decisions>

## Decisions
<one bullet per locked decision: "**<area>:** <choice chosen> — <one-line rationale>">

## Canonical refs
<bulleted list of file paths scouted in §4 that informed the options>

## Specifics
<any additional implementation details agreed during discussion that do not fit neatly under Decisions>

## Deferred
<bulleted list of out-of-scope ideas captured during discussion, each with a one-line description>
```

**Merge discipline:** if the epic's design field already contains prior locked
decisions (detected in §3), merge the new decisions in — never silently
overwrite prior content. Follow this procedure:

1. Read the existing design field:
   ```bash
   bd show <epic-id> --json | jq -r '.[0].design // ""'
   ```
2. If the result is empty, write the new doc as-is. If non-empty, produce the
   merged doc: keep every existing line; append new bullets under their
   matching `##` heading; add any missing headings at the end.
3. Write the full merged doc back via `--design-file -` (the field is replaced
   wholesale, which is why the read-merge step above is mandatory):
   ```bash
   bd update <epic-id> --design-file - <<'WEFT_DESIGN_EOF'
   ## Domain
   ...
   WEFT_DESIGN_EOF
   ```
   The quoted delimiter prevents shell expansion of backticks and `$` in the
   doc body; the unique `WEFT_DESIGN_EOF` token avoids early termination if
   the merged doc itself ever contains a bare `EOF` line.

Do not remove or alter existing bullets unless the user explicitly revises a
prior decision during this session.

Then append an audit note:

```bash
printf '%s' "discuss: locked N decisions across M areas (<area list>)" \
  | bd note <epic-id> --stdin
```

(Literal fixed-text notes may use the plain quoted form; use stdin whenever
the note contains bead-sourced text such as area names.)

where N is the count of bullet points added to `## Decisions` and M is the
count of areas discussed in this session.

---

## 8. Consumer contract

The per-phase planner (phase C of the spec) reads the epic's design field
before planning picks. Executors read it to resolve ambiguity during
implementation. This skill's output is **complete** when the design field
answers the HOW questions a planner would otherwise have to guess.

Decisions not recorded here will be guessed by the planner.

---

## 9. Stop conditions

The workflow terminates when:

- all user-selected areas have been discussed and decisions persisted (normal
  completion), or
- no meaningful gray areas exist (explicitly stated; exits without writing), or
- the user ends the session early (persist whatever is locked so far; append
  the audit note with the actual counts).

---

## 10. Dropped GSD mechanics

The following GSD discuss-phase mechanics are intentionally absent from this
workflow:

- **External tracking artifacts** — the GSD source writes per-phase decision
  files; this workflow writes only to the epic's bead `design` field and
  `bd note` audit traces. Beads is the brain; no files are created.
- **External planning-file reads** — the bead's own `bd show` output is the
  phase goal; no external files are consulted to determine project scope.
- **GSD mode flags** (`--auto`, `--batch`, `--analyze`, `--assumptions`,
  `--all`, `--power`, `--text`) — v1 ships the default adaptive mode only.
  These are noted as future overlays when multi-mode support becomes necessary.
