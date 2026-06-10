---
description: Shape HOW a phase gets built — adaptive gray-area questions whose locked decisions land in the epic's bead design field for the planner.
argument-hint: "[epic-id]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-discuss-phase (GSD Core, MIT) -->

# /weft-discuss

Shape the implementation HOW for any epic — phase sub-epic or one-shot feature
epic. Takes a required `[epic-id]` argument identifying the bead whose design
decisions are being settled.

Delegates all orchestration to the `discuss` workflow body. This command is
intentionally thin: it validates the argument and hands off immediately.

## Invocation

```
/weft-discuss <epic-id>
```

## What it does

Invokes the `weft/workflows/discuss.md` workflow, passing `epic-id` as the
driving scope. The workflow:

- loads the epic via `bd show <epic-id>` to establish the phase goal and
  read any prior locked decisions,
- scouts the source files the epic implicates,
- derives phase-specific gray areas and lets the user select which to discuss,
- walks each selected area with up to ~4 single questions (concrete options,
  recommended choice, one-line rationale),
- deflects scope creep to a Deferred section, and
- persists locked decisions to the epic's `design` field via
  `bd update <epic-id> --design-file -` (the stdin form).

The per-phase planner (phase C) and executors consume the design field to
answer HOW questions that would otherwise be guessed. The skill's output is
complete when the design field covers the implementation decisions a planner
would need. See `weft/workflows/discuss.md` for the full specification.

## See also

- `weft/workflows/discuss.md` — the orchestrator body
- `docs/superpowers/specs/2026-06-09-layer-a-interactive-phased-loop-design.md` — §3
