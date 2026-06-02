# Weft — Project Memory

Spec-driven AI development orchestration, woven on **jj** (the loom) and
**beads** (the warp/brain). Clean-room reimplementation of GSD Core's
methodology on purpose-built substrates. See `docs/design.md` for the full
design and `README.md` for the metaphor/vocabulary.

## Status

**Design phase.** No engine yet. The engine will be a **Go binary** (single
static binary, sits next to `bd` and `jj`).

## Hard rules

- **VCS is jj** (colocated). MUST use `jj`, MUST NOT use mutating git commands.
  The `jj:jujutsu` skill governs all VCS operations.
- **beads is the brain.** Planning, the dependency graph (the "warp"), task
  state, and scheduling live in beads. `bd ready` is the scheduler. There is no
  `ROADMAP.md` / `STATE.md` / `SUMMARY.md`.
- **Recovery is change-scoped, never op-restore.** Use `jj abandon <change-id>`
  (bead-driven) and `jj op revert`. `jj op restore` is human-gated only — it
  rewinds the global op log and stales other workspaces (jj-vcs/jj#9208).
- **License:** Apache-2.0 with SPDX headers on all source + functional markdown.

## Conventions (mirrors holomush)

- Go 1.26; `cmd/<binary>/` + `internal/` layout.
- Conventional commits; cocogitto (`cog.toml`) tag-only releases; validated in
  CI, not via local hooks (jj does not fire git hooks reliably).
- `.editorconfig` is authoritative for formatting.

## Vocabulary

- **warp** — the bead dependency graph (the plan).
- **weft** — the woven agent work (changes across the warp).
- **pick** — one woven change (one bead → one jj change).
- **shed** — one parallel wave (the set of ready beads woven together).
