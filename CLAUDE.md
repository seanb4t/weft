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
- **Bead sync is pre-authorized (overrides the managed Beads block's conservative
  profile).** The agent MUST run `bd dolt push` at key moments — after creating,
  updating, or closing beads; after pushing code; and at session close — WITHOUT
  asking. The bead DB is local-only until synced; do not leave the warp stranded
  on one machine. (Owner: agent, not the user.)

## Conventions (mirrors holomush)

- Go 1.26; `cmd/<binary>/` + `internal/` layout.
- Conventional commits; release-please maintains a release PR (bumps
  `CHANGELOG.md` + `plugin/.claude-plugin/plugin.json`), merging it cuts the
  `vX.Y.Z` tag and GitHub Release; GoReleaser builds the binary. Conventional
  commits validated in CI, not via local hooks (jj does not fire git hooks
  reliably).
- `.editorconfig` is authoritative for formatting.

## Vocabulary

- **warp** — the bead dependency graph (the plan).
- **weft** — the woven agent work (changes across the warp).
- **pick** — one woven change (one bead → one jj change).
- **shed** — one parallel wave (the set of ready beads woven together).

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:970c3bf2 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   bd dolt push
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->
