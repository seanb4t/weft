<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-ea7; do not edit manually; use `/adr update weft-ea7` -->

# Distribute weft as a Claude Code plugin via in-repo marketplace

**Date:** 2026-06-05
**Status:** Accepted
**Decision:** weft-ea7
**Deciders:** Sean Brandt

## Context

Seam 5 deferred how the `weft/` prompt tree reaches a host runtime, sketching a possible `weft install` transform or `go:embed`. Seam 7 resolves it: the prompt tree is re-expressed in-repo as a native Claude Code plugin under `plugin/`, with a repo-root marketplace manifest, distributed via `claude plugin marketplace add` / `claude plugin install`. There is no runtime transform engine, no `go:embed`, and no loose `.claude/commands` files. Grounded against Claude Code v2.1.165.

## Decision

The `weft/` prompt tree is distributed as a native Claude Code plugin (`plugin/` subdir + repo-root `.claude-plugin/marketplace.json`), registered via an in-repo marketplace; the `weft install` verb pins the install to the binary's release tag. No runtime transform, no `go:embed`.

## Rationale

- The plugin *is* the source — no transform step and no derived artifact to keep in sync.
- `claude plugin validate --strict` is a first-class CI gate, replacing hand-rolled lint.
- The in-repo marketplace gives the plugin Claude Code's standard install/update/uninstall lifecycle.
- `go:embed` would require a transform engine with no concrete user benefit given the native plugin model.

## Alternatives Considered

- **Loose `.claude/commands` files (the seam-5 framing).** No install step, works without the plugin model — but no namespacing, no manifest validation, no versioning, no marketplace lifecycle, and namespace collisions with other projects. Rejected.
- **Runtime transform / `go:embed`.** A single binary carries the prompt bytes (offline without a clone) — but requires building and maintaining a transform engine, and treats prompts as derived artifacts rather than source. Rejected; the `--local <clone>` path covers offline for v1.

## Consequences

- **Positive:** plugin structure enforced by schema validation (no ad-hoc lint); standard `claude plugin update` path (no custom update logic); component namespacing (`/weft:execute`) avoids conflicts with other installed plugins.
- **Negative:** Claude Code is the only supported v1 host (a second host needs a second plugin mapping); install requires the `claude` CLI on PATH and network access (or a `--local` clone).
- **Neutral:** a second host is architecturally possible but explicitly out of scope for v1 (YAGNI); the `plugin/` tree becomes the authoritative prompt source, with the seam-5 `weft/` files as inputs to the restructure.
