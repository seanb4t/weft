<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-88z; do not edit manually; use `/adr update weft-88z` -->

# Collapse command and workflow pairs into skills for the weft plugin

**Date:** 2026-06-05
**Status:** Accepted
**Decision:** weft-88z
**Deciders:** Sean Brandt

## Context

The seam-5 prompt tree (`weft/`) uses a GSD-inherited thin-command→workflow indirection: a `weft/commands/weft-X.md` invokes `weft/workflows/X.md` as a separate file. Claude Code's plugin model introduces the *skill* as a component that is both invocation entrypoint and body (`SKILL.md` with frontmatter + body), making the two-file pattern redundant. The command/workflow pairs in `weft/` are 1:1.

## Decision

Each 1:1 thin-command/workflow pair from seam 5 collapses into a single `plugin/skills/X/SKILL.md`; the `argument-hint` from the command frontmatter is carried into the SKILL frontmatter. The plugin exposes skills (`/weft:execute`, `/weft:new-project`), not a separate commands+workflows split.

## Rationale

- Claude Code docs recommend `skills/` over `commands/` for new plugins.
- The command/workflow separation has no benefit here: no workflow is shared across multiple command entrypoints.
- A skill is both entrypoint and body — the two-file pattern existed only for historical GSD reasons, not structural necessity.
- A single file reduces the surface `claude plugin validate --strict` must cover.

## Alternatives Considered

- **Keep separate `commands/` and `workflows/` in the plugin.** Preserves GSD upstream structural parity and is easier to diff against GSD source — but reintroduces artificial indirection with no demonstrated reuse benefit, and leaves two files to validate and keep in sync. Rejected.

## Consequences

- **Positive:** one file per capability instead of two — simpler authoring and review; aligns with Claude Code's recommended plugin component model.
- **Negative:** if a future capability needs the same workflow body invoked from multiple entrypoints, the collapsed skill must be re-split.
- **Neutral:** the GSD thin-command→workflow pattern remains in `weft/` (seam 5) but is not carried into the `plugin/` tree.
