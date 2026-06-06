<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft (Claude Code plugin)

Spec-driven AI development orchestration, woven on **jj** (the loom) and
**beads** (the warp/brain). This plugin packages Weft's orchestration prompts
as native Claude Code skills and agents.

## Skills

- **`/weft:execute [epic-id]`** — weave one wave of ready picks: form the shed,
  isolate per-pick workspaces, dispatch executor agents, integrate in
  dependency order, resolve conflicts, and land conflict-free picks. Repeats
  until the epic's ready set is empty.
- **`/weft:new-project [project description]`** — plan a new project into the
  warp: adaptive questioning, parallel research, and requirement extraction,
  then emit a `warp-plan.json` for human approval.

## Agents

Dispatched by the skills via the Task tool: `weft-executor` (per-pick TDD),
`weft-planner` (warp generation), `weft-reviewer` (per-pick review), and
`weft-resolver` (conflict resolution).

## Prerequisites

The skills drive these CLIs, which must be on `PATH`:

- **`weft`** — the engine binary (sits next to `bd` and `jj`).
- **`bd`** ([beads]) — the dependency graph and task state (the warp/brain).
- **`jj`** ([jujutsu]) — the colocated VCS (the loom).
- **`gh`** — GitHub operations.

## Installation

Install via the engine, which pins the plugin to the running binary's release:

```
weft install
```

`weft install` registers this repository as a Claude Code plugin marketplace and
installs the `weft` plugin (`weft install --uninstall` removes it). See
<https://github.com/seanb4t/weft> for details.

[jujutsu]: https://github.com/jj-vcs/jj
[beads]: https://github.com/gastownhall/beads
