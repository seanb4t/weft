---
description: Interactive UAT over an epic's deliverables — walk each one y/n, diagnose failures, file fix picks under the epic.
argument-hint: "[epic-id]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-verify-work (GSD Core, MIT) -->

# /weft-verify-work

Interactive UAT over an epic's deliverables — phase sub-epic or one-shot
feature epic. Takes a required `[epic-id]` argument identifying the bead whose
shipped work is being verified against its acceptance criteria.

Delegates all orchestration to the `verify-work` workflow body. This command is
intentionally thin: it validates the argument and hands off immediately.

## Invocation

```
/weft-verify-work <epic-id>
```

## What it does

Invokes the `weft/workflows/verify-work.md` workflow, passing `epic-id` as the
driving scope. The workflow:

- enumerates the epic's deliverables via a fallback chain (acceptance criteria
  → closed picks' acceptance → epic goal/description),
- walks each deliverable with the user in a y/n loop (empty = pass),
- records failures verbatim, infers severity from the user's wording,
- dispatches read-only diagnosis agents in parallel to find root causes,
- files `uat-fix` picks under the epic for each diagnosed failure, and
- persists a `verify-work:` note per verdict so interrupted sessions resume
  without re-checking already-passed items.

This skill is the human UAT layer that runs after the machine `weft pick verify`
gate has already passed per pick during `execute`. It does not close the epic.
See `weft/workflows/verify-work.md` for the full specification.

## See also

- `weft/workflows/verify-work.md` — the orchestrator body
- `docs/superpowers/specs/2026-06-09-layer-a-interactive-phased-loop-design.md` — §4
