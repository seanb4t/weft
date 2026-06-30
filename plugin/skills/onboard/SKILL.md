---
description: Make an existing non-weft repo weft-ready — bd init, one codebase-mapping pass seeding bead memories, then route to feature or new-project. No planning of its own.
argument-hint: ""
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-map-codebase (GSD Core, MIT), compressed to one pass + bead-backed memories -->

# onboard workflow

The third door: make an existing repo that has never run weft **weft-ready**. Runs
`bd init`, maps the codebase in one pass, seeds the map as `bd remember` memories (which
the beads `bd prime` hook surfaces every future session), then hands off to `feature` or
`new-project`. It does no planning of its own — it creates no epic or picks.

Use `feature` for incremental work on an already-managed repo; use `new-project` for a
brand-new project.

---

## Phase 0 — Precondition (idempotency)

Check whether this repo already has a **local** beads database:

```
test -d .beads && echo managed || echo unmanaged
```

Gate on the `.beads/` directory's presence — **not** `bd list`, which silently falls back
to the global shared beads DB when no local `.beads/` exists and would make an unmanaged
repo look "managed." If `.beads/` is present, the repo is already weft-managed — do not
re-onboard. Tell the user and point them onward: `feature` (incremental work on existing
code) or `new-project` (greenfield build). Exit.

Only proceed when `.beads/` is absent (the repo has never run weft).

---

## Phase 1 — bd init

Initialise beads for this repo:

```
bd init --non-interactive -p <prefix>
```

Choose `<prefix>` from the repository directory name (a short, lowercase slug); if the
directory name is ambiguous or unsuitable, ask the user for a prefix. `bd init` creates a
local-only beads DB under `.beads/` (no Dolt remote — the user wires sync later). Confirm
`.beads/` now exists before proceeding.

### Decide whether to track the agent-interaction audit log

`bd init` commits `.beads/interactions.jsonl` — bd's append-only per-session interaction
log — as a **tracked** file, and its generated `.beads/.gitignore` does not exclude it.
Left tracked, the log churns independently in every pick workspace, so `weft shed
integrate` hits spurious 2-sided conflicts on `.beads/interactions.jsonl` that have nothing
to do with the woven code (weft-1qq).

This is repo-dependent — decide with the user:

- **Default — exclude it from the warp** (recommended for any repo that will weave). The log
  stays on disk as local telemetry but never enters a pick or the warp. Order matters: jj
  only untracks a path that an ignore rule already matches, so ignore *then* untrack:

  ```
  grep -qxF interactions.jsonl .beads/.gitignore || echo interactions.jsonl >> .beads/.gitignore
  jj file untrack .beads/interactions.jsonl
  ```

  After this `jj file list 'glob:.beads/**'` no longer lists `interactions.jsonl`, and the
  file remains on disk untracked.

- **Keep it tracked** only if the repo deliberately wants the interaction history in version
  control. Then leave it as-is and accept that weaves may need to resolve recurring
  `.beads/interactions.jsonl` conflicts — surface this trade-off to the user before choosing it.

---

## Phase 2 — Codebase map (one Explore pass)

Dispatch **one** `Explore` subagent to map the existing code in a single pass. Instruct it
to investigate four axes and return a digest with one short section per axis (a few bullet
points each — not exhaustive prose):

- **Stack & integrations** — languages, runtimes, frameworks, key dependencies, external
  services/APIs, build/run/test commands.
- **Architecture & structure** — top-level layout, layers/boundaries, entry points, where
  the important code lives.
- **Conventions & testing** — code style, naming, error-handling patterns, the test
  framework and how tests are organised/run.
- **Concerns** — notable tech debt, known fragile areas, or hazards a contributor should
  know.

Hold the returned digest in context for Phase 3. Never dispatch more than one recon pass.

---

## Phase 3 — Seed bead-backed memory

Persist the digest as durable `bd remember` memories — one per axis, with stable keys — so
the beads `bd prime` hook surfaces them every future session (beads is the brain; no
CONTEXT.md, no `.planning/` files):

```
bd remember "<stack & integrations digest>"     --key weft-map-stack
bd remember "<architecture & structure digest>"  --key weft-map-arch
bd remember "<conventions & testing digest>"      --key weft-map-conventions
bd remember "<concerns digest>"                    --key weft-map-concerns
```

Then seed one orientation memory so future sessions know how to drive weft:

```
bd remember "weft repo: incremental work on existing code -> /weft-feature; greenfield build -> /weft-new-project; vocab — warp (the bead dependency graph/plan), weft (the woven work), pick (one bead -> one jj change), shed (one parallel wave)." --key weft-orientation
```

Keep each memory concise (a few lines); they are orientation, not full documentation.

---

## Phase 4 — Hand off

The repo is now weft-ready: `.beads/` is initialised and the codebase map + orientation are
seeded as memories. onboard plans nothing itself. Present both exits and let the user pick:

- **`feature`** — incremental work on the existing code (mints one epic + picks; minutes).
- **`new-project`** — a greenfield/first build planned from scratch.

Closing note: weft's VCS verbs (`execute`/`shed`/`pick`) require a colocated jj repo. If
this repo is not yet jj-colocated, set that up via the `jj-init` skill before running them.
(VCS setup is out of scope for onboard.)

---

## What this workflow does NOT do

- It does not plan — no epic, no picks. That is `feature` / `new-project`.
- It does not run the four-axis map as four separate agents (GSD's model) — one Explore
  pass, deliberately compressed.
- It does not write `.planning/` files or CLAUDE.md prose — the map lives in `bd remember`
  memories, surfaced by `bd prime`.
- It does not set up jj colocation or a Dolt remote — `jj-init` and later sync wiring own
  those.
