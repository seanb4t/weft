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

The red→green→refactor phases are **editing phases inside the single
working-copy change** — they are NOT separate jj commits. The CLAUDE.md
invariant is "pick — one woven change (one bead → one jj change)"; issuing
intermediate `jj commit` calls between phases would fork the bead–change spine
and leave `weft pick seal`'s `jj-change:<id>` label ambiguous (see
`weft/agents/weft-executor.md` § "Commit sequence (TDD path)" and
`weft/references/bead-change-spine.md`). jj auto-snapshots the working copy on
every save, so intermediate states are recoverable via `jj --no-pager evolog`
without explicit per-phase commits.

### Phase 1 — RED

Write a test that describes the expected behavior. Run it. It MUST fail.

- If the test passes without implementation, stop: the feature already exists
  or the test is wrong. Do not proceed.
- This is an editing phase — do NOT commit the failing test separately.

### Phase 2 — GREEN

Write the minimal code to make the test pass. Run it. It MUST pass.

- Do not gold-plate. Only make the test green.
- Editing phase — no intermediate commit.

### Phase 3 — REFACTOR

If the implementation has obvious structural problems, clean it up within the
same working-copy change. Verify tests still pass.

### Seal

The whole pick is sealed exactly once via `weft pick seal <bead>`, which
produces the single jj change (`<type>(<bead-id>): <title>`) and writes the
`jj-change:<id>` spine label. One pick → one sealed change, never 2–3.

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

### Gate checks

Because a pick is a single working-copy change (no per-phase commits), the gate
cannot — and does not — inspect commit topology for RED/GREEN ordering. When
`tdd_mode` is active in `weft.yaml`, the gate verifies the *end state* of the
change instead:

1. The change adds or modifies at least one test exercising the bead's feature.
2. The test suite passes (the GREEN state).

RED-before-GREEN ordering is a process discipline the executor follows within
the single change (Phase 1 above); it is not reconstructable from a one-commit
change, so the gate does not assert it.

### Fail-fast rules

| Situation | Gate action |
|-----------|-------------|
| RED test passes unexpectedly (during Phase 1) | Executor STOPs — feature may already exist |
| GREEN test still fails | FAIL — implementation incomplete |
| No test present in the change (tdd_mode=true) | FAIL — discipline violated |
| Tests present and passing | PASS → `weft pick seal` proceeds |

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
