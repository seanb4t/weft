# Vendored: Superpowers visual companion

- **Upstream:** https://github.com/obra/superpowers — `skills/brainstorming/scripts/`
- **Upstream commit:** unknown (vendored from local dev-flow copy; see Proximate source)
- **Proximate source:** the dev-flow plugin's brainstorming/scripts/ copy present
  in this environment (`c2bfff6fc27a`).
- **License:** MIT (retained; see NOTICE).

## weft modifications

- Scratch dir rebranded: `.superpowers/brainstorm/` → `.weft/sketch/` and
  `/tmp/brainstorm[-id]` → `/tmp/weft-sketch[-id]` (start-server.sh, server.cjs);
  stop-server.sh's cleanup comment updated `.superpowers/` → `.weft/`.
- Visible chrome rebranded to `weft · sketch` (frame-template.html title + h1;
  server.cjs placeholder page title + h1).
- Internal identifiers left unchanged (not user-facing): `BRAINSTORM_*` env var
  names and the `window.brainstorm` content-frame API.
- No functional/protocol changes.
