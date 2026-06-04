---
description: Plan a new project into the warp — adaptive questions, research, then emit a warp-plan.json the human approves.
argument-hint: "[project description]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-new-project (GSD Core, MIT) -->

# /weft-new-project

Turn a project idea into a warp. Given a `[project description]` (or asked
interactively if none is provided), this command drives adaptive questioning
and parallel research to extract a coherent set of requirements, then
dispatches the `weft-planner` agent to produce a `warp-plan.json`. The
human approves the dry-run preview before the warp is materialised into
beads.

## Intent

This command is a **thin entry point**. It takes the optional
`[project description]` argument, grounds the session in project context, and
delegates all orchestration to the workflow body in
`weft/workflows/new-project` (the thin orchestrator). There is no logic to
duplicate here — the workflow is the authority.

## Argument

`[project description]` — a free-form description of the project to plan. If
omitted, the workflow opens with adaptive questioning to elicit scope, goals,
constraints, and technical preferences before proceeding.

## Invocation

```
/weft-new-project [project description]
```

Delegates immediately to the `new-project` workflow with the provided
description (or none) as the initial context. Do not proceed beyond this
delegation; all decisions, research, planning, and approval gating are the
workflow's responsibility.
