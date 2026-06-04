<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
  ~
  ~ Adapted from GSD Core (https://github.com/open-gsd/gsd-core),
  ~ MIT License, Copyright (c) its contributors.
  ~ Adapted sections: red-green-refactor TDD cycle, verify gate concept,
  ~ fresh-context dispatch principle.
  ~ Rewritten sections: all weft plumbing (pick/verify verdict as data,
  ~ bead-based gate enforcement, jj change-scoped commits).
-->

# TDD and verify discipline

Every weft executor agent that implements a non-trivial feature or fix MUST
follow the red-green-refactor cycle and submit to the verify gate before
sealing a pick.

## Red-green-refactor cycle

TDD applies whenever the task has a clear behavioral specification: if you can
write `expect(fn(input)).toBe(output)` before writing `fn`, TDD is warranted.
Mechanical tasks (configuration, simple CRUD, formatting) do not require a TDD
bead.

### Phase 1 — RED

Write a test that describes the expected behavior. Run it. It MUST fail.

- If the test passes without implementation, stop: the feature already exists
  or the test is wrong. Do not proceed.
- Commit the failing test in its own jj change:
  `test(<scope>): add failing test for <feature>`

### Phase 2 — GREEN

Write the minimal code to make the test pass. Run it. It MUST pass.

- Do not gold-plate. Only make the test green.
- Commit the implementation:
  `feat(<scope>): implement <feature>`

### Phase 3 — REFACTOR

If the implementation has obvious structural problems, clean it up. Verify
tests still pass. Commit only if changes were made:
`refactor(<scope>): clean up <feature>`

Each TDD pick results in 2–3 atomic jj changes (RED, GREEN, optional REFACTOR).

## Verify gate

The verify gate is the checkpoint that confirms a pick satisfies its bead's
acceptance criteria before `weft pick seal` is allowed to proceed.

### Verdict as data

`weft pick verify` runs the gate and emits a structured verdict on stdout:

```json
{"ok": true, "verb": "pick.verify", "data": {"pass": true,  "bead": "weft-abc.1", "change": "sqpuoqvx"}}
{"ok": true, "verb": "pick.verify", "data": {"pass": false, "bead": "weft-abc.1", "change": "sqpuoqvx", "reason": "..."}}
```

`weft pick verify` exits `0` whenever the engine ran successfully — including
when the gate verdict is `false` (the `pass` field in `data`). A nonzero exit
means the engine itself failed (e.g. exit `1` for invocation error, exit `2`
for a hard `jj`/`bd` failure), NOT that the gate said no. The verdict is
**data consumed by the engine**, not written to any review-output file.
Orchestrators branch on `.data.pass`; agents never inspect a separate artifact
to learn the gate result.

### Gate sequence validation

When `tdd_mode` is active in `weft.yaml`, the gate enforces RED/GREEN commit
sequence before allowing seal:

1. Checks the jj log for a `test(...)` (RED) commit in the change.
2. Checks for a `feat(...)` (GREEN) commit.
3. If either is absent, the gate fails with a structured diagnostic and the
   bead remains in `active` phase.

### Fail-fast rules

| Situation | Gate action |
|-----------|-------------|
| RED test passes unexpectedly | FAIL — feature may already exist |
| GREEN test still fails | FAIL — implementation incomplete |
| No RED commit in jj log (tdd_mode=true) | FAIL — discipline violated |
| All checks pass | PASS → `weft pick seal` proceeds |

## Fresh-context dispatch principle

Orchestrator context and agent context MUST be kept separate.

### Thin orchestrator

`weft shed` is a thin coordinator. It reads bead metadata, selects dispatch
model (via `model:*` label — see `bead-change-spine.md`), and spawns executor
agents. It does NOT perform implementation, read agent definition files inline,
or accumulate large technical context.

### Fresh context per agent

Each agent spawned by `weft shed` receives a clean context window. This
prevents context rot — the degradation of reasoning quality as the context
window fills with stale, partially-relevant information. Agents are specialized
and receive only the context necessary for their specific bead.

The principle: **orchestrator context stays clean; agents are spawned fresh.**

### Context budget rules

- Orchestrators MUST NOT read agent definition files into their own context.
- Orchestrators MUST NOT inline large file contents into agent prompts.
- Agents receive: the bead spec, relevant plan rows, and targeted file excerpts
  (never whole-directory dumps).
- Read depth scales with available context window: prefer targeted probe
  extractions over full-file reads.
