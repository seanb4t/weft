---
description: Throwaway visual/UI direction — explore 2–4 HTML mockup variants in a live browser companion (side-by-side, click-to-select), capture the chosen direction as bead-native state. Pre-planning; hands off to ui-phase/plan-phase.
argument-hint: "[topic] [epic-id]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-sketch (GSD Core, MIT); visual companion vendored from obra/superpowers (MIT) -->

# sketch workflow

weft's analog of `/gsd-sketch`: throwaway exploration of **visual/UI direction**
before a frontend phase is planned. Mockups are disposable — they live only in
the gitignored `.weft/sketch/` scratch dir (or `/tmp`), never committed. The
durable output is the **chosen direction**, recorded bead-native as a
`bd remember` finding (+ the epic `design` field), the weft analog of GSD's
`sketch-findings` skill. No `.planning/` files.

Mockups render in a **live browser companion** (vendored, zero-dependency Node
server) supporting side-by-side comparison and click-to-select.

> `sketch` is distinct from `explore` (WHAT to build) and `discuss` (HOW to build
> it). It shapes how a UI surface should *look*. It is an optional door, run
> before `ui-phase`.
>
> Runtime: the companion needs `node` on `PATH`. Without it, fall back to static
> HTML in `/tmp` (Phase 2 degraded path).

---

## Phase 1 — Frame the surface

Treat `[topic]` as the UI surface to explore (a screen, component, or flow). If
unclear, one light round of framing: which surface, its key elements, and how
many variants to show (**2–4**). State the variants to the user before
generating them.

---

## Phase 2 — Launch the visual companion

```
COMP="${CLAUDE_PLUGIN_ROOT}/skills/sketch/scripts/visual-companion"
command -v node >/dev/null || echo "degraded: no node — using static HTML fallback"
"$COMP/start-server.sh" --project-dir "$(jj root)"
```

Launch the server **in the background** (it must survive across turns — use the
Bash tool's `run_in_background` on platforms that reap detached processes, then
read `$STATE_DIR/server-info` next turn). Capture `url`, `screen_dir`, and
`state_dir` from the startup JSON. Tell the user to open the URL. The scratch
dir lands under the gitignored `.weft/sketch/<session>/` — never committed.

**Degraded (no `node`):** write each variant as a self-contained HTML file under
`/tmp/weft-sketch-<id>/` and give the user `file://` paths to open. Then skip to
Phase 4 (capture from the user's terminal reply).

---

## Phase 3 — Push variants + iterate

Write each variant as a **content fragment** (no `<html>`/`<head>` — the server
wraps it in the weft frame template) to a fresh, semantically-named file in
`screen_dir`, using the **Write tool** (never `cat`/heredoc). Companion classes:

- `.split` — side-by-side mockups; `.cards` — labelled design cards.
- `.mockup` / `.mockup-header` / `.mockup-body` — a framed preview.
- Wireframe blocks: `.mock-nav`, `.mock-sidebar`, `.mock-content`,
  `.mock-button`, `.mock-input`, `.placeholder`.
- `.options` (add `data-multiselect` to allow multiple) with
  `data-choice` + `onclick="toggleSelect(this)"` for A/B/C choices.

Show **2–4 options max** per screen and state the question on each ("Which layout
reads more clearly?"). End your turn: remind the user of the URL, summarise
what's on screen, and ask them to click a choice and/or reply in the terminal.

Next turn: read `$STATE_DIR/events` (JSONL of clicks; absent → no browser
interaction) **and** the user's terminal text. Iterate with a new file
(`layout-v2.html`) if feedback changes the screen. When moving back to the
terminal, push a `waiting.html` fragment to clear stale content.

---

## Phase 4 — Capture the chosen direction

From the selection events + terminal text, record the chosen direction —
**layout, color palette, typography, spacing**. Persist bead-native:

```
bd remember "Sketch — <surface>: chosen direction — layout: <…>; palette: <…>; typography: <…>; spacing: <…>." --key sketch-<slug>
```

If `[epic-id]` was given, also fold it into the phase epic so `ui-phase` /
`plan-phase` see it the same session (the `design`-field handoff, ADR `weft-b19`):

```
bd update <epic-id> --design "<existing design + the chosen sketch direction>"
```

Do not invent decisions the exploration did not produce. No `.planning/` files;
no committed mockups.

---

## Phase 5 — Stop + hand off

```
"$COMP/stop-server.sh" "$SESSION_DIR"
```

Mockups in `.weft/sketch/` are transient (gitignored; `/tmp` sessions are
auto-deleted). Point onward:

- Lock the full UI contract → `ui-phase <epic-id>`.
- Or go straight to planning → `plan-phase <epic-id>`.

---

## What this workflow does NOT do

- It does not keep or commit mockups — they are throwaway (gitignored `.weft/`
  or `/tmp`).
- It does not write `.planning/` or sidecar durable state — the durable output
  is `bd remember` + the epic `design` field.
- It does not run a build or framework — variants are self-contained HTML the
  companion serves.
- It does not lock a UI contract — that is `ui-phase`. `sketch` only explores
  visual direction.
