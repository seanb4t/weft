---
description: Weave a wave of ready picks — form the shed, isolate, dispatch executors, integrate, resolve conflicts, land.
argument-hint: "[epic-id]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-execute-phase (GSD Core, MIT) -->

# /weft-execute

Weave one wave of ready picks for the given epic. Takes an optional
`[epic-id]` argument; if omitted, operates on the active epic inferred from
context.

Delegates all orchestration to the `execute` workflow body. This command is
intentionally thin: it validates the argument and hands off immediately.

## Invocation

```
/weft-execute [epic-id]
```

## What it does

Invokes the `weft/workflows/execute.md` workflow, passing `epic-id` as the
driving scope. The workflow runs the full weave loop:

- forms the next ready wave (`weft shed form`),
- isolates per-pick workspaces (`weft shed isolate`),
- dispatches a fresh `weft-executor` agent per pick,
- gates each pick with `weft pick verify`,
- integrates the wave in dependency order (`weft shed integrate`),
- resolves any conflicts (`weft conflict open` / `weft conflict finalize`),
- lands conflict-free picks (`weft pick land`),
- cleans up workspaces (`weft shed cleanup`, `weft reap`),
- and projects state for the next wave (`weft resume`).

The loop repeats until the epic's ready set is empty. See
`weft/workflows/execute.md` for the full specification.

## See also

- `weft/workflows/execute.md` — the orchestrator body
- `weft/agents/weft-executor.md` — per-pick executor agent
- `weft/agents/weft-reviewer.md` — per-pick review agent (dispatched by verify)
- `weft/agents/weft-resolver.md` — conflict resolution agent
- `docs/seams/01-command-surface.md` — stable verb surface
- `docs/seams/03-workspace-lifecycle.md` — workspace isolation semantics
- `docs/seams/04-conflict-resolution.md` — conflict open/finalize contract
