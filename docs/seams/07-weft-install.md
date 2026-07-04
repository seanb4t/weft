<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 7 — `weft install` (Claude Code plugin distribution)

> Status: **shipped**. Sub-spec of
> [`docs/design.md`](../design.md) §8 distribution and §9, and the deferred
> sub-seam noted in [seam 5](05-gsd-markdown-ports.md) §8 ("the `weft install`
> transform rules … and whether install is a `weft` verb or a separate tool").
> Tracked as bead `weft-hjx.10` (child of `weft-hjx`). Implemented and released
> (`weft install`).

## 1. Scope

How the Weft prompt tree reaches a host runtime. Two deliverables:

- **The weft plugin** — the `weft/` prompt tree (authored in seam 5) is
  re-expressed in-repo as a **native Claude Code plugin** under `plugin/`, with
  a repo-root marketplace manifest. This is the bulk of the work and is
  *markdown authoring*, not engine code.
- **`weft install` verb** — a thin, version-pinned wrapper over Claude Code's
  own `claude plugin` CLI that registers the repo as a marketplace and installs
  the plugin, pinned to the running binary's release so the engine verbs and the
  prompts that call them stay in lockstep.

Claude Code is the v1 (and only) host. A second host would be a second plugin
mapping; nothing here precludes it, but no untestable adapter is built (YAGNI).

**Out of scope (v1):** any runtime transform engine or `go:embed` of the prompt
tree (the plugin *is* the source — there is nothing to transform at install
time); non-Claude hosts; plugin `hooks`, `mcpServers`, `lspServers`,
`outputStyles`, `channels`, `userConfig` (Weft needs none); an offline binary
that carries the plugin bytes (the `--local` path covers "I have a clone").

## 2. Grounding

Recorded as `bd note`s on `weft-hjx.10` (probe + context7 + deepwiki + the live
`claude plugin` CLI + a `claude-code-guide` live-docs sweep). The load-bearing
facts, all verified against **Claude Code v2.1.165** (`code.claude.com/docs`,
2026-06-05):

- **A plugin is the native unit.** `<plugin-root>/.claude-plugin/plugin.json`
  declares `name` (only required field, kebab-case), optional `version`,
  `description`, `author`, `license`, `keywords`, and `./`-relative component
  paths (`commands`, `agents`, `skills`, …). No `../` traversal.
- **Components are namespaced `/<plugin>:<name>`.** A plugin named `weft` exposes
  `skills/execute/SKILL.md` as `/weft:execute`. The source `weft-` filename
  prefix is therefore dropped (else `/weft:weft-execute`).
- **A skill is an invocable procedure *with* arguments and bundled files** —
  `skills/<name>/SKILL.md` (YAML frontmatter: `description`, `argument-hint`,
  `arguments`, `allowed-tools`, …) plus reference files addressed by
  `${CLAUDE_SKILL_DIR}` (own dir) or `${CLAUDE_PLUGIN_ROOT}` (shared). Docs
  recommend `skills/` over `commands/` for new plugins.
- **Marketplace.** `<repo>/.claude-plugin/marketplace.json` lists `plugins[]`,
  each with a `source`. For a plugin in a subdir of the same repo the source is
  the relative string `"./plugin"` (resolves to `<repo>/plugin`; works for
  git-added marketplaces). `strict` defaults to `true` (keeps `plugin.json`
  authoritative for components); §3.3 sets it explicitly for self-documentation.
- **Install CLI (non-interactive).** `claude plugin marketplace add <url|path|owner/repo[@ref]>`
  then `claude plugin install <plugin>@<marketplace> --scope user|project|local`
  (default `user`). `claude plugin uninstall <plugin> --scope <s> -y`
  (`-y`/`--yes` required without a TTY). `claude plugin validate <path> --strict`
  is the CI gate. `claude plugin update`, `enable`, `disable`, `list` exist.
- **Versioning / release.** `claude plugin tag` cuts a **`{name}--v{version}`**
  git tag (e.g. `weft--v1.4.0`), validating `plugin.json` ↔ marketplace entry
  agree. A marketplace `source` ref pins to a tag/branch/sha. If `version` is set
  in both `plugin.json` and the marketplace entry, `plugin.json` wins.
- **Install is cached, never in-place** (`~/.claude/plugins/cache/<mkt>/<plugin>/<ver>/`),
  so the plugin must be self-contained (no `../`).

## 3. The weft plugin (authoring)

### 3.1 Repo layout

```
weft/                                    # repo root (the Go engine)
├── .claude-plugin/
│   └── marketplace.json                 # one entry: weft → source "./plugin"
├── plugin/
│   ├── .claude-plugin/
│   │   └── plugin.json                  # name "weft", version, Apache-2.0
│   ├── skills/
│   │   ├── execute/SKILL.md             # → /weft:execute
│   │   └── new-project/SKILL.md         # → /weft:new-project
│   ├── agents/
│   │   ├── executor.md  planner.md
│   │   ├── resolver.md  reviewer.md     # → /weft:executor … (Task name kept; see §3.4)
│   ├── references/
│   │   ├── jj-agent-safety.md
│   │   ├── bead-change-spine.md
│   │   └── tdd-verify-discipline.md     # shared, cited via ${CLAUDE_PLUGIN_ROOT}/references/…
│   ├── README.md
│   └── NOTICE                            # GSD Core (MIT) attribution
├── cmd/  internal/                       # Go engine (adds internal/install, run.Claude)
└── … (cog.toml, .editorconfig, etc.)
```

Two `.claude-plugin/` dirs is the standard monorepo shape: the **repo-root** one
holds the marketplace; the **`plugin/`** one holds the plugin manifest.

### 3.2 Source → plugin mapping

The seam-5 tree is already Claude-native markdown, so this is restructuring, not
content transform. SPDX headers and the GSD provenance comments are **kept** (the
plugin files are checked-in Apache-2.0 source).

| seam-5 source | → plugin | rule |
|---|---|---|
| `weft/commands/weft-X.md` **+** `weft/workflows/X.md` | `plugin/skills/X/SKILL.md` | **collapse** the 1:1 thin-command/workflow pair into one skill (a skill is both entrypoint and body); carry the command's `argument-hint` into SKILL frontmatter |
| `weft/agents/weft-Y.md` | `plugin/agents/Y.md` | filename drops `weft-` (→ `/weft:Y`); frontmatter `name` kept (§3.4) |
| `weft/references/Z.md` | `plugin/references/Z.md` | shared reference; skills cite `${CLAUDE_PLUGIN_ROOT}/references/Z.md` |
| — | `plugin/.claude-plugin/plugin.json` | generated manifest (§3.3) |
| — | `.claude-plugin/marketplace.json` | generated manifest (§3.3) |
| — | `plugin/{README.md,NOTICE}` | docs + attribution |

Per-skill/agent authoring worksheets (the exact frontmatter, the
workflow→skill-body edits, the `weft/workflows/…` reference rewrites →
`${CLAUDE_PLUGIN_ROOT}` / Skill-tool invocations) are produced when each file is
authored — mirroring seam 5's per-prompt worksheets (seam 5 §8).

### 3.3 Manifests

`plugin/.claude-plugin/plugin.json`:

```json
{
  "$schema": "https://json.schemastore.org/claude-code-plugin-manifest.json",
  "name": "weft",
  "version": "0.0.0",
  "description": "Spec-driven AI dev orchestration on jj + beads — woven waves of ready picks.",
  "author": { "name": "Weft Contributors" },
  "homepage": "https://github.com/seanb4t/weft",
  "repository": "https://github.com/seanb4t/weft",
  "license": "Apache-2.0",
  "keywords": ["jj", "beads", "orchestration", "gsd"]
}
```

`.claude-plugin/marketplace.json`:

```json
{
  "$schema": "https://json.schemastore.org/claude-code-marketplace.json",
  "name": "weft",
  "description": "The Weft plugin marketplace.",
  "plugins": [
    {
      "name": "weft",
      "source": "./plugin",
      "description": "Spec-driven AI dev orchestration on jj + beads.",
      "license": "Apache-2.0",
      "category": "development",
      "strict": true
    }
  ]
}
```

`version` is carried **only** in `plugin.json` (it wins over a marketplace
entry, so setting it in both invites silent skew). The release process keeps it
in lockstep with the binary (§4.2).

### 3.4 Agent naming

Agent **files** drop the prefix (`agents/executor.md` → `/weft:executor` if ever
slash-listed), but each agent's frontmatter **`name` is kept verbatim**
(`weft-executor`, …) because skills dispatch agents by that name via the
Task/subagent tool, and a distinct, collision-proof name matters more there than
slash ergonomics. Whether to also rename the Task-facing name is a decision
deferred to §9.

### 3.5 Validation discipline

The plugin is gated exactly as seam 5 gated its prompts, plus the native
validator:

- `claude plugin validate ./plugin --strict` and
  `claude plugin validate . --strict` (the marketplace) — **CI gate**; `--strict`
  turns unrecognized fields / missing metadata into failures.
- The seam-5 grep-discipline checks still apply: no `.planning/` / `worktree` /
  `gsd-tools` / `ROADMAP` / `SUMMARY` references; SPDX header + NOTICE present;
  skills call only stable `weft`/`bd`/`jj`/`gh` verbs.

## 4. `weft install` verb

A root subcommand (`func (a *App) newInstallCmd()`, registered like `finish`).
It writes **no files** — the plugin lives in the repo/marketplace; the verb only
drives the `claude plugin` CLI.

```
weft install [--scope user|project|local] [--ref <git-ref>] [--local <path>]
             [--uninstall] [--dry-run] [--json]
```

### 4.1 Steps (install path)

1. **Validate inputs.** `--ref` and `--local` are interpolated into the
   marketplace `add` argument; allowlist-validate both before any subprocess
   (§7), per the standing injection guard.
2. **Prereq.** Resolve `claude` on `PATH` (`run.Claude` probe). Absent →
   `exit.Hardf` with the two commands the user can run by hand (the verb cannot
   proceed without the host CLI). `--dry-run` skips this and just prints.
3. **Resolve the source + ref.**
   - default (git marketplace): source `seanb4t/weft`, ref `weft--v<version>`
     where `<version>` is the binary's release version (§4.2);
   - `--ref <r>`: override the ref (branch/tag/sha) — for pre-release/dev;
   - `--local <path>`: source is the local clone `<path>` (must contain
     `.claude-plugin/marketplace.json`); no ref. Offline/dev.
4. **Register the marketplace.** `claude plugin marketplace add <source>[@<ref>]`
   (idempotent — re-add updates).
5. **Install the plugin.** `claude plugin install weft@weft --scope <scope>`.
6. **Emit** the envelope (§8).

### 4.2 Version-pin lockstep (the reason the verb exists)

The verb's value over hand-running two `claude plugin` lines is that it pins the
prompt version to *its own* binary, so a given `weft` binary always installs the
prompts authored against the verbs it ships.

- Release cuts **two** tags at the same version `X.Y.Z`: the cocogitto binary tag
  `vX.Y.Z` and the plugin tag `weft--vX.Y.Z` (via `claude plugin tag`, which also
  validates the manifests agree). `plugin.json.version` is bumped to `X.Y.Z` in
  the same release commit.
- `weft --version` reports `X.Y.Z`; `weft install` pins
  `seanb4t/weft@weft--vX.Y.Z`.
- **Dev / untagged builds** (version `0.0.0` / a `+dirty` suffix): the
  `weft--v0.0.0` tag won't exist. The verb refuses to silently float — it errors
  with guidance to pass `--ref main` (or a specific ref) or `--local`. (No silent
  fallback to `main`; that would break the lockstep guarantee invisibly.)

### 4.3 Uninstall

`weft install --uninstall` → `claude plugin uninstall weft --scope <scope> -y`
(the `-y` is mandatory without a TTY). Marketplace removal is left to the user /
`claude plugin marketplace remove weft` (removing it would break other scopes).

### 4.4 Idempotency / re-run / update

A re-run re-pins and re-installs. `install` is idempotent; `marketplace add` on
an already-registered name is **expected** to update-or-no-op, but that re-add
semantic is not yet confirmed against the live CLI (§2 grounded the add syntax,
not the duplicate behavior). The verb therefore tolerates an already-registered
marketplace: if `marketplace add` errors on a duplicate, fall back to
`marketplace remove weft` + re-add (or `marketplace update weft`). An integration
test pins whichever path the CLI actually takes (§10). Updating to a newer weft
is "install the newer binary, run `weft install`" (it pins the new tag) — or
`claude plugin update weft`. The verb does not track its own state; Claude Code's
installed-plugin registry is the source of truth (no hand-rolled manifest —
contrast GSD's `gsd-file-manifest.json`, which we don't need because we place no
files).

## 5. `run.Claude` — the 4th wrapped CLI

`internal/run` gains `Claude(r Runner, args ...string) (Result, error)`,
mirroring `JJ`/`BD`/`GH`. It wraps the `claude` binary. This extends the
gh-as-3rd-CLI decision (ADR `weft-yuj`): like `bd`/`jj`/`gh` it is a
deterministic CLI the engine shells, **not** agent dispatch (the engine never
dispatches agents; §design.md §7). `internal/install` holds the verb logic and
depends only on the `run.Runner` interface, so it is unit-testable with the
existing fake runner.

## 6. Error handling & exit codes

Reuses the engine's `exit` contract (`internal/exit`):

- **`CodeInvocation` (1):** bad `--scope` value; `--ref`/`--local` failing the
  allowlist; dev/untagged version with no `--ref`/`--local`; `--local` path with
  no `.claude-plugin/marketplace.json`.
- **`CodeHard` (2):** `claude` not on `PATH`; any `claude plugin …` subprocess
  exiting non-zero (surfaced with its stderr, per the silent-failure discipline
  — never swallowed).

## 7. Input validation (injection guard)

`--ref` and `--local` reach the `claude plugin marketplace add` argument vector.
Exec is arg-sliced (`run.Claude`), so there is no OS-shell injection, but a
leading-dash value could be misparsed as a flag and a stray value could point the
marketplace at an unintended source. Per the standing guard idiom
(`changeIDPattern`/`epicIDPattern`, conflict.go/finish.go; engram memory
`weft-cli-validate-user-id-before-revset-or-gh-api`):

- `--ref`: a git-ref allowlist (`refPattern`) — leading alphanumeric, then
  `[A-Za-z0-9._/-]` (covers `main`, `weft--v1.2.3`, 40-hex shas; rejects leading
  `-`, spaces, shell/path metacharacters beyond `/.-`).
- `--local`: must resolve to an existing directory containing
  `.claude-plugin/marketplace.json`; reject a leading `-`.

Validation runs **before** any `run.Claude` call.

## 8. `--json` envelope

`weft install` emits the standard `{ok, verb, data, next}` envelope
(`internal/envelope`). Every `data` key is always present (the engine output
contract; empty string / `false` rather than omitted); the envelope-level `next`
follows the shared type's `omitempty` (envelope.go) and is always populated here
with the restart hint. `--dry-run` mutates nothing and reports the commands it
would run.

```json
{
  "ok": true,
  "verb": "install",
  "data": {
    "plugin": "weft",
    "marketplace": "weft",
    "source": "seanb4t/weft",
    "ref": "weft--v1.4.0",
    "scope": "user",
    "uninstall": false,
    "registered": true,
    "installed": true,
    "commands": ["claude plugin marketplace add seanb4t/weft@weft--v1.4.0",
                 "claude plugin install weft@weft --scope user"],
    "dry_run": false
  },
  "next": "Restart Claude Code to load the weft plugin; try /weft:execute."
}
```

`commands[]` is initialized `[]string{}` (never nil → never JSON `null`; the
non-nil-slice contract). On the uninstall path `data.uninstall` is `true` and
`commands` carries the single `uninstall` invocation.

## 9. Decisions to capture as ADRs

- **Weft is distributed as a Claude Code plugin via an in-repo marketplace** —
  not loose `.claude/commands` files, not a runtime transform/`go:embed`. This
  supersedes the seam-5 §8 framing of a "`weft install` transform". (Headline
  decision.)
- **Command+workflow pairs collapse into skills.** The GSD thin-command→workflow
  indirection is dropped because a Claude Code skill is both entrypoint and body.
- **`run.Claude` is the 4th wrapped CLI**, extending ADR `weft-yuj`
  (gh-as-3rd-CLI): a deterministic CLI, not agent dispatch.
- **Version-pin lockstep via the `weft--vX.Y.Z` plugin tag**, cut at the same
  version as the binary; dev builds refuse to float silently.
- **Agent Task-name retention** (§3.4) — keep `weft-executor` etc. as the
  dispatch name vs. renaming to the namespaced form. (Smaller; may fold into the
  collapse ADR.)

## 10. Testing

The **plugin** is gated by `claude plugin validate … --strict` + the seam-5
grep-discipline checks (§3.5) — no `go test` for the markdown.

The **verb** is unit-tested with the fake `run.Runner` (no real `claude`),
asserting on the exact argument vectors per the envelope-field convention
(`json.Unmarshal` into a struct, never whole-output `strings.Contains`):

- default install pins `seanb4t/weft@weft--v<version>` and runs the two
  `claude plugin` calls in order, scope passthrough;
- `--ref` / `--local` override the source and skip the version-tag derivation;
- `--scope` validation (reject an unknown scope → exit 1);
- `--ref` / `--local` allowlist (injection fixtures: leading `-`, spaces,
  metacharacters → exit 1, no subprocess);
- dev/untagged version with no `--ref`/`--local` → exit 1 with guidance;
- `claude` absent → exit 2; a non-zero `claude plugin` exit → exit 2 with stderr
  surfaced;
- `--uninstall` runs `uninstall … -y` and nothing else;
- re-run / duplicate marketplace: a non-zero `marketplace add` (already
  registered) triggers the remove-then-add (or `marketplace update`) fallback,
  not a hard failure — an integration check pins the real CLI's re-add semantic;
- `--dry-run` runs no subprocess and reports `commands[]`;
- empty-output / envelope-shape test (`commands: []` non-null).

## 11. Out of scope / deferred

- **Non-Claude hosts** (Codex, etc.) — a future plugin mapping + a small adapter;
  not built until there's a host to test against.
- **Offline binary that carries the plugin** (`go:embed` the tree + a bundled
  local marketplace) — `--local <clone>` covers the offline case for v1.
- **Plugin `hooks`, `mcpServers`, `userConfig`, `channels`, `outputStyles`** —
  Weft needs none today.
- **`weft install --update` / health-check / `weft doctor`** — `claude plugin
  update`/`list` cover it; revisit if the thin verb proves too thin.
