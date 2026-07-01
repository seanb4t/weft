<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft

**Spec-driven AI development orchestration, woven on [jj] and [beads].**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

> Status: **design** — see [docs/design.md](docs/design.md). No engine yet.

Weft is a meta-prompting / context-engineering system for AI coding agents. It
takes the methodology proven by [GSD](https://github.com/open-gsd/gsd-core) —
a tight, spec-driven loop that keeps the main context clean by doing heavy work
in fresh subagent contexts — and rebuilds it on purpose-built substrates instead
of homegrown ones:

- **The warp — [beads]** (`bd`) is the brain. It owns planning, the dependency
  graph, task state, and scheduling. `bd ready` *is* the scheduler. There is no
  `ROADMAP.md` / `STATE.md` / `SUMMARY.md`.
- **The weft — execution.** Agent work woven across the warp in parallel passes
  (waves), each pass laying down one change per ready bead.
- **The loom — [jj]** (Jujutsu) is the substrate. It holds the warp under
  tension, lets passes run in parallel sheds (isolated workspaces), and tolerates
  a dropped pick without unravelling the fabric (first-class conflicts).

The name says the design: **beads holds the structure; jj does the weaving.**

## Why

GSD invented a file-based tracker (`.planning/`) and a git-commit choreography
because it had no real tools underneath. beads is the purpose-built tracker; jj
is the purpose-built VCS. Weft keeps GSD's *methodology* and deletes the two
subsystems GSD only built to compensate.

## Vocabulary

| Term | Meaning |
|------|---------|
| **warp** | the bead dependency graph — the plan, held under tension |
| **weft** | the woven work — agent changes laid across the warp |
| **pick** | a single woven change (one bead → one jj change) |
| **shed** | a parallel wave — the set of ready beads woven together |

## Development

Working on weft — or dogfooding the weave on any bd-managed repo — has three
fresh-clone gotchas. A clean git tree is **not** a clean environment.

**Get the `weft` CLI on PATH.** `weft install` registers the Claude Code plugin
but does not put the binary on PATH. Build and link it yourself:

```
go install ./cmd/weft            # into $GOBIN / ~/go/bin
# or: go build -o ~/.local/bin/weft ./cmd/weft
```

`weft install` also reports the running binary's path (and a symlink hint) in the
envelope's `next` field.

**The verify gate ships committed.** `.weft/config.toml` is tracked with a
`[verify].command` (build + test), so `weft pick verify` works out of the box.
Local engine state under `.weft/` stays gitignored — only the config is tracked.

**Resetting the warp is not a re-checkout.** beads' Dolt database persists in the
shared Dolt server *and* on the git remote (`refs/dolt/data`), so deleting the
clone dir and re-cloning re-adopts the **same** warp — the fresh checkout comes
back with the old beads, not an empty plan. To actually reset:

```
bd init --reinit-local                       # new local identity over .beads/
bd init --reinit-local --discard-remote      # also drop remote history (TTY confirm)
# non-interactive (agents/CI) needs the destroy-token; weft's prefix is 'weft':
bd init --reinit-local --discard-remote --destroy-token=DESTROY-weft
```

The first `bd dolt push` after `--discard-remote` is a history-replacing
force-push. For throwaway test isolation, point `BEADS_DIR` at a temp dir
instead; `bd dolt clean-databases` drops stale test DBs from the shared server.
See `bd init-safety` for the full flag contract.

## License

Apache License 2.0. See [LICENSE](LICENSE).

Methodology and several command/agent prompts are adapted from GSD Core (MIT);
see [docs/design.md](docs/design.md) for attribution.

[jj]: https://github.com/jj-vcs/jj
[beads]: https://github.com/gastownhall/beads
