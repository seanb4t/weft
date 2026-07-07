---
description: Read-only warp progress readout — runs `weft status` and narrates what's done vs what's left, calling out blocked picks and the aggregate. No arg for a whole-warp overview; an epic-id for a per-epic drill-down.
argument-hint: "[epic-id]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- native weft binary surface — no GSD command maps to this; no new semantics -->

# status workflow

Presents as `/weft:status`. A read-only, point-in-time narration of the
`weft status` binary subcommand — the binary computes every count from beads
live; this skill only shells out and narrates the result in plain language.
It reimplements none of the counting.

---

## Steps

1. Run the binary, passing through the optional argument:

   ```bash
   weft status $1
   ```

   No argument prints a whole-warp overview — every epic with its picks
   counted by status plus an aggregate. An epic id drills into that one
   epic's picks grouped by status.

2. Narrate the output for the user:
   - Lead with the headline: how much is **done** (closed) vs **remaining**
     (in_progress + open + blocked), per epic and in aggregate.
   - Call out any **blocked** picks by name — they are the likely next
     conversation, not just a count.
   - In whole-warp mode, note which epic(s) are furthest along and which
     haven't started.
   - In drill-down mode, walk the status groups in the order the binary
     emits them (closed, in_progress, blocked, open).
   - If the warp is empty, say so plainly — nothing to report yet.

---

## What this workflow does NOT do

- It does not compute or re-derive counts — `weft status` is the sole
  source of truth, backed by beads.
- It does not read any repo-local steering documents — beads is the only
  source, so this skill ships unchanged for any weft-managed repo.
- It does not mutate warp state — it is a read-only, point-in-time snapshot.
