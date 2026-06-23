# Vendored: Superpowers visual companion

- **Upstream:** https://github.com/obra/superpowers — `skills/brainstorming/scripts/`
- **Upstream commit:** `1681f58a3fb528791991253faec6bc9a8763a208` (obra/superpowers
  `main`; "Phase C: alphabetize README platform listings + spec"). This is the
  newest `main` commit whose `skills/brainstorming/scripts/` is byte-identical to
  the vendored files: `helper.js` matches verbatim, and the other four differ only
  by the weft modifications listed below. The SHA is a commit pointer verified
  against the file blobs, not by date — Superpowers rebases its history, so re-pin
  by matching content, not timestamps. The next `main` commit to touch these
  scripts, `6ec8686` ("Phase D: cross-runtime tweaks"), changes
  `frame-template.html` and so diverges.
- **Proximate source:** vendored via the dev-flow plugin's
  `skills/brainstorming/scripts/` copy (a `fzymgc-house-skills` commit,
  `c2bfff6fc27a`), which is itself byte-identical to obra/superpowers at the
  commit above.
- **License:** MIT (retained; see NOTICE).

## weft modifications

- Scratch dir rebranded: `.superpowers/brainstorm/` → `.weft/sketch/` and
  `/tmp/brainstorm[-id]` → `/tmp/weft-sketch[-id]` (start-server.sh, server.cjs);
  stop-server.sh's cleanup comment updated `.superpowers/` → `.weft/`.
- Visible chrome rebranded to `weft · sketch` (frame-template.html title + h1;
  server.cjs placeholder page title + h1).
- Internal identifiers left unchanged (not user-facing): `BRAINSTORM_*` env var
  names and the `window.brainstorm` content-frame API.

## Security hardening (weft-9i3) — functional divergence from upstream

These are deliberate behavioral changes, not rebranding. Candidates to upstream
to obra/superpowers; kept here until then.

- **`helper.js` — XSS:** the choice indicator is built with `textContent` +
  `createElement` instead of `innerHTML` string concatenation, so a choice
  label whose text contains markup renders as inert text and can never inject
  live DOM.
- **`stop-server.sh` — unvalidated `kill`:** the PID read from
  `state/server.pid` is validated as a positive integer **and** confirmed to be
  our own `node … server.cjs` process before any signal is sent. This prevents a
  malformed value (`0`, `-1`) from broadening `kill` to the caller's process
  group, and a stale/reused PID from signaling an unrelated process.
