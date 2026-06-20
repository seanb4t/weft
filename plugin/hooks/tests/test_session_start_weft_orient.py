# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Weft Contributors

"""End-to-end tests for the session-start-weft-orient hook.

Runs the script as a subprocess (via the test interpreter, bypassing the uv
shebang — the script is stdlib-only). Detection and silence need no bd; the
live-warp, timeout, and degradation paths are exercised with a hermetic fake
`bd` placed on PATH.
"""

from __future__ import annotations

import json
import os
import stat
import subprocess
import sys
import time
from pathlib import Path

import pytest

HOOK = Path(__file__).resolve().parent.parent / "session-start-weft-orient"


def run_hook(
    cwd: str,
    *,
    stdin: str | None = None,
    extra_path: str | None = None,
    replace_path: str | None = None,
    env: dict | None = None,
):
    """Invoke the hook. `extra_path` prepends to PATH; `replace_path` replaces it
    outright (keeping the rest of os.environ intact); `env` overlays extra vars."""
    proc_env = dict(os.environ)
    if env:
        proc_env.update(env)
    if replace_path is not None:
        proc_env["PATH"] = replace_path
    elif extra_path:
        proc_env["PATH"] = f"{extra_path}{os.pathsep}{proc_env['PATH']}"
    payload = stdin if stdin is not None else json.dumps({"cwd": cwd})
    return subprocess.run(
        [sys.executable, str(HOOK)],
        input=payload,
        capture_output=True,
        text=True,
        timeout=20,
        env=proc_env,
        cwd=cwd,
    )


def context_of(proc) -> str:
    return json.loads(proc.stdout)["hookSpecificOutput"]["additionalContext"]


def _weft_repo(tmp_path: Path) -> Path:
    (tmp_path / ".beads").mkdir()
    return tmp_path


def _empty_bin(tmp_path: Path) -> str:
    """A bin dir with no `bd` — for PATH replacement so bd is unresolvable."""
    bindir = tmp_path / "emptybin"
    bindir.mkdir()
    return str(bindir)


def _fake_bd(tmp_path: Path, ready, epic_status, *, sleep: float = 0.0) -> str:
    """Write a fake `bd` onto a bin dir; return that dir for PATH injection.

    `ready`/`epic_status` may be JSON-serializable values, or a raw string
    emitted verbatim (to simulate garbage/non-list output). `sleep` stalls every
    invocation, to exercise the probe timeout.
    """
    def emit(value):
        body = value if isinstance(value, str) else json.dumps(value)
        return f"cat <<'EOF'\n{body}\nEOF"

    bindir = tmp_path / "bin"
    bindir.mkdir()
    script = bindir / "bd"
    script.write_text(
        "#!/bin/sh\n"
        f"sleep {sleep}\n"
        'case "$*" in\n'
        # The heredoc terminator EOF must stand alone on its line, so `;;` goes
        # on the following line — not appended after EOF.
        f"  *'ready --json'*) {emit(ready)}\n  ;;\n"
        f"  *'epic status --json'*) {emit(epic_status)}\n  ;;\n"
        "  *) echo '[]' ;;\n"
        "esac\n"
    )
    script.chmod(script.stat().st_mode | stat.S_IEXEC | stat.S_IXGRP | stat.S_IXOTH)
    return str(bindir)


def test_non_weft_dir_is_silent(tmp_path: Path):
    proc = run_hook(str(tmp_path))
    assert proc.returncode == 0
    assert proc.stdout.strip() == ""


def test_malformed_stdin_is_silent(tmp_path: Path):
    proc = run_hook(str(_weft_repo(tmp_path)), stdin="not json")
    assert proc.returncode == 0
    assert proc.stdout.strip() == ""


def test_weft_repo_emits_orientation_without_bd(tmp_path: Path):
    # .beads present but bd unresolvable (PATH replaced with an empty bin dir,
    # rest of os.environ preserved) -> orientation surfaces; warp numbers degrade.
    repo = _weft_repo(tmp_path)
    proc = run_hook(str(repo), replace_path=_empty_bin(tmp_path))
    assert proc.returncode == 0
    ctx = context_of(proc)
    assert "/weft-feature" in ctx
    assert "Vocabulary" in ctx
    assert "Warp status:" not in ctx  # degraded — no live numbers


def test_empty_stdin_falls_back_to_cwd(tmp_path: Path):
    # Empty stdin is a distinct branch from malformed JSON: data={} -> cwd from
    # os.getcwd() (which the subprocess inherits as the weft repo) -> still emits.
    repo = _weft_repo(tmp_path)
    proc = run_hook(str(repo), stdin="", replace_path=_empty_bin(tmp_path))
    assert proc.returncode == 0
    assert "/weft-feature" in context_of(proc)


def test_weft_repo_with_fake_bd_shows_live_status(tmp_path: Path):
    repo = _weft_repo(tmp_path)
    ready = [{"id": f"weft-{i}"} for i in range(5)]
    epics = [
        {
            "epic": {"id": "weft-zzz", "title": "Demo epic", "status": "open"},
            "total_children": 4,
            "closed_children": 1,
            "eligible_for_close": False,
        }
    ]
    proc = run_hook(str(repo), extra_path=_fake_bd(tmp_path, ready, epics))
    assert proc.returncode == 0
    ctx = context_of(proc)
    assert "5 picks ready" in ctx
    assert "weft-zzz" in ctx
    assert "(1/4, 25%)" in ctx
    out = json.loads(proc.stdout)["hookSpecificOutput"]
    assert out["hookEventName"] == "SessionStart"


def test_garbage_bd_json_degrades_to_orientation(tmp_path: Path):
    # bd emits valid JSON that is NOT a list (e.g. an error object) -> the hook's
    # isinstance guards degrade to orientation-only rather than crashing.
    repo = _weft_repo(tmp_path)
    bindir = _fake_bd(tmp_path, {"error": "locked db"}, "not json at all")
    proc = run_hook(str(repo), extra_path=bindir)
    assert proc.returncode == 0
    ctx = context_of(proc)
    assert "/weft-feature" in ctx
    assert "Warp status:" not in ctx


def test_slow_bd_hits_timeout_and_degrades(tmp_path: Path):
    # A wedged warp: bd sleeps far longer than the per-probe budget. With a tiny
    # timeout the hook must kill the probe and degrade quickly (proving timeout=
    # is wired), not block for the full sleep.
    repo = _weft_repo(tmp_path)
    bindir = _fake_bd(tmp_path, [{"id": "x"}], [], sleep=5)
    start = time.monotonic()
    proc = run_hook(
        str(repo), extra_path=bindir, env={"WEFT_HOOK_BD_TIMEOUT_S": "0.3"}
    )
    elapsed = time.monotonic() - start
    assert proc.returncode == 0
    assert "Warp status:" not in context_of(proc)  # degraded
    assert elapsed < 3, f"timeout not enforced (took {elapsed:.1f}s of a 5s sleep)"


def test_debug_flag_writes_stderr_on_degraded_bd(tmp_path: Path):
    # WEFT_HOOK_DEBUG surfaces why a probe degraded (bd unresolvable on the
    # empty PATH here) on stderr — without touching stdout or the exit code.
    repo = _weft_repo(tmp_path)
    proc = run_hook(
        str(repo), replace_path=_empty_bin(tmp_path), env={"WEFT_HOOK_DEBUG": "1"}
    )
    assert proc.returncode == 0
    assert "/weft-feature" in context_of(proc)  # stdout unaffected
    assert "session-start-weft-orient:" in proc.stderr  # diagnostic present


@pytest.mark.parametrize(
    "payload",
    [
        "",
        "not json",
        "{}",
        '{"cwd": 12345}',  # non-string cwd
        '{"cwd": "/nonexistent/path/xyz"}',
        '{"other": "x"}',  # no cwd key
        '{"cwd": null}',
    ],
)
def test_always_exits_zero(tmp_path: Path, payload: str):
    # The orientation hook must never fail a session, whatever the payload.
    proc = run_hook(str(tmp_path), stdin=payload, replace_path=_empty_bin(tmp_path))
    assert proc.returncode == 0
