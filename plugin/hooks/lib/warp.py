# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Weft Contributors

"""Pure helpers for the weft SessionStart orientation hook.

Detection, epic filtering, and warp-status formatting live here as pure
functions so they can be unit-tested without spawning ``bd``. The hook script
(``session-start-weft-orient``) supplies already-parsed ``bd ... --json`` data
and owns all the I/O.

External-input discipline: ``bd ... --json`` is parsed and rendered into the
model's session context, so every record is treated as untrusted — coerced
through :func:`_normalize_epics` (drop anything malformed) and free-text titles
are passed through :func:`_clean` before interpolation.
"""

from __future__ import annotations

from pathlib import Path

# Fixed orientation surfaced in every weft repo — independent of the beads
# plugin's static bd-remember injection (that one carries memories; this one
# carries weft's verb surface + vocabulary).
ORIENTATION = """\
Weft repo — spec-driven AI orchestration on jj (the loom) + beads (the warp/brain).

Entry skills:
  /weft-new-project <desc> — plan a greenfield project into the warp (adaptive Q&A + research → an approved warp-plan).
  /weft-feature <desc>     — lightweight front door for incremental work on this existing codebase (one epic + picks, in minutes).
  /weft-onboard            — make a non-weft repo weft-ready (bd init + a codebase-mapping pass).

Vocabulary: warp = the bead dependency graph (the plan); weft = the woven agent work;
pick = one woven change (one bead → one jj change); shed = one parallel wave of ready picks.
Scheduler: `bd ready` lists the next shed."""

# Bound on rendered epic titles. Titles come from bd (an external warp that a
# `bd dolt pull` could have populated from a shared/hostile remote), so we cap
# and de-control them before they enter the model's additionalContext.
_TITLE_LIMIT = 120


def _clean(text: object, limit: int = _TITLE_LIMIT) -> str:
    """Flatten a bd-supplied string for safe single-line interpolation: drop
    non-printable chars (newlines included), collapse whitespace, bound length."""
    flattened = "".join(ch if ch.isprintable() else " " for ch in str(text))
    collapsed = " ".join(flattened.split())
    if len(collapsed) > limit:
        return collapsed[: limit - 1] + "…"
    return collapsed


def find_beads_root(start: Path | str) -> Path | None:
    """Return the nearest ancestor of ``start`` (inclusive) containing a
    ``.beads/`` directory, or ``None`` if none is found."""
    cur = Path(start).resolve()
    for d in (cur, *cur.parents):
        if (d / ".beads").is_dir():
            return d
    return None


def is_weft_managed(start: Path | str) -> bool:
    """A repo is weft-managed when a ``.beads/`` warp is present (the same
    signal the feature/onboard/new-project skills use)."""
    return find_beads_root(start) is not None


def _normalize_epics(epic_status: object) -> list[dict]:
    """Coerce ``bd epic status --json`` into clean ``{id,title,status,closed,
    total}`` dicts, dropping any record too malformed to render. Defensive by
    design: a bd version skew or locked-DB error payload must degrade, never
    raise (the hook's always-exit-0 contract)."""
    out: list[dict] = []
    if not isinstance(epic_status, list):
        return out
    for rec in epic_status:
        if not isinstance(rec, dict):
            continue
        epic = rec.get("epic")
        if not isinstance(epic, dict):
            continue
        eid = epic.get("id")
        if not eid:
            continue
        try:
            total = int(rec.get("total_children", 0))
            closed = int(rec.get("closed_children", 0))
        except (TypeError, ValueError):
            continue
        out.append(
            {
                "id": eid,
                "title": epic.get("title") or "",
                "status": epic.get("status"),
                "closed": closed,
                "total": total,
            }
        )
    return out


def active_epics(epic_status: object) -> list[dict]:
    """Normalized open epics with outstanding children, most-remaining first (a
    stable proxy for 'where the work is'). Returns ``_normalize_epics`` dicts."""
    out = [
        e
        for e in _normalize_epics(epic_status)
        if e["status"] == "open" and e["closed"] < e["total"]
    ]
    out.sort(key=lambda e: (-(e["total"] - e["closed"]), e["id"]))
    return out


def format_warp_status(
    ready_count: int | None,
    epic_status: object,
    *,
    max_epics: int = 5,
) -> list[str]:
    """Build the dynamic warp-status lines from parsed bd output.

    ``ready_count is None`` means bd was unavailable; the numeric line is
    omitted so the hook degrades to orientation-only rather than lying.
    """
    lines: list[str] = []
    if ready_count is not None:
        noun = "pick" if ready_count == 1 else "picks"
        lines.append(f"Warp status: {ready_count} {noun} ready.")

    epics = active_epics(epic_status)
    if epics:
        lines.append("Active epics (open, incomplete):")
        for e in epics[:max_epics]:
            pct = round(e["closed"] / e["total"] * 100) if e["total"] else 0
            # Both id and title are bd-supplied (untrusted warp) — clean both
            # before they enter the model's additionalContext.
            eid = _clean(e["id"])
            title = _clean(e["title"])
            lines.append(f"  {eid}  {title}  ({e['closed']}/{e['total']}, {pct}%)")
        extra = len(epics) - max_epics
        if extra > 0:
            lines.append(f"  …and {extra} more open epics.")
    return lines


def build_context(warp_lines: list[str]) -> str:
    """Assemble the SessionStart additionalContext: fixed orientation plus any
    dynamic warp-status lines."""
    parts = [ORIENTATION]
    if warp_lines:
        parts.append("\n".join(warp_lines))
    return "\n\n".join(parts)
