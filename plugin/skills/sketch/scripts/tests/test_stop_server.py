# weft-owned black-box test for the vendored visual-companion stop-server.sh.
#
# Guards the PID hardening (weft-9i3): stop-server.sh reads a PID from a scratch
# file and signals it. Before hardening it passed that value straight to `kill`
# with no checks, so (a) a malformed value like "0"/"-1"/non-numeric broadened
# the signal target and (b) a stale/reused PID could target an unrelated process.
#
# The script stays bash (a small, upstreamable diff to the vendored mirror); this
# test drives it as a black box. Stdlib only (no pytest needed):
#
#     python3 -m unittest test_stop_server -v
#     uv run --python 3.13 python -m unittest test_stop_server -v
#
# Fake servers are spawned DETACHED (reparented to init, pid 1) to mirror the real
# `nohup … node server.cjs & disown` launch. A tracked child would linger as an
# unreaped zombie when stop-server.sh kills it — and `kill -0` / os.kill(,0) can't
# tell a zombie from a live process — so both the script's wait loop and alive()
# would misread a stopped server as still running. Detaching lets init reap it.
import json
import os
import shlex
import signal
import subprocess
import tempfile
import time
import unittest
from pathlib import Path

STOP_SH = Path(__file__).resolve().parent.parent / "visual-companion" / "stop-server.sh"


def alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except PermissionError:
        return True


def last_json(stdout: str) -> dict:
    for line in reversed(stdout.strip().splitlines()):
        line = line.strip()
        if line.startswith("{"):
            return json.loads(line)
    return {}


class StopServerTest(unittest.TestCase):
    def setUp(self):
        self._pids = []
        # test_malformed_pid_is_rejected re-invokes setUp per case; clean up any
        # prior tempdir so it doesn't leak and trip a ResourceWarning.
        prior = getattr(self, "_tmp", None)
        if prior is not None:
            prior.cleanup()
        self._tmp = tempfile.TemporaryDirectory()
        self.session = Path(self._tmp.name)

    def tearDown(self):
        # Detached fakes are children of init, not us, so there is nothing to
        # wait() on — just make sure none survive the test.
        for pid in self._pids:
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        self._tmp.cleanup()

    def spawn(self, argv0: str) -> int:
        """Start a detached fake process and return its PID.

        A shell backgrounds `sleep` and exits, so the sleep reparents to init —
        exactly like the nohup'd `node server.cjs`. `argv0` sets its argv[0]:
        pass a "node …server.cjs" string to look like our server, or "sleep" to
        look like an unrelated process. See the module header for why detaching
        (vs. a tracked child) matters.
        """
        marker = self.session / "spawned.pid"
        marker.unlink(missing_ok=True)
        subprocess.run(
            ["bash", "-c",
             f"exec -a {shlex.quote(argv0)} sleep 60 & echo $! > {shlex.quote(str(marker))}"],
            check=True,
            start_new_session=True,
        )
        for _ in range(100):  # wait for the grandchild to record its PID
            if marker.exists() and marker.read_text().strip():
                break
            time.sleep(0.02)
        pid = int(marker.read_text().strip())
        marker.unlink(missing_ok=True)
        self._pids.append(pid)
        time.sleep(0.2)  # let it settle so `ps` can see its command line
        return pid

    def write_pidfile(self, contents: str):
        state = self.session / "state"
        state.mkdir(parents=True, exist_ok=True)
        (state / "server.pid").write_text(contents)

    def run_stop(self) -> subprocess.CompletedProcess:
        # start_new_session=True puts the script in its own process group, so a
        # buggy `kill 0` inside it can never reach the test runner.
        return subprocess.run(
            ["bash", str(STOP_SH), str(self.session)],
            capture_output=True,
            text=True,
            start_new_session=True,
            timeout=20,
        )

    # ---- Ownership: a PID that is not our server must never be signaled ------

    def test_unowned_pid_is_not_killed(self):
        victim = self.spawn("sleep")  # a plausible PID-reuse collision, not our server
        self.write_pidfile(str(victim))

        result = self.run_stop()

        self.assertTrue(alive(victim), "stop-server.sh killed a process it does not own")
        self.assertNotEqual(last_json(result.stdout).get("status"), "stopped")

    def test_owned_server_is_stopped(self):
        # A process whose command line looks like the real server (node …server.cjs).
        victim = self.spawn("node /x/server.cjs")
        self.write_pidfile(str(victim))

        result = self.run_stop()
        time.sleep(0.2)

        self.assertFalse(alive(victim), "stop-server.sh failed to stop its own server")
        self.assertEqual(last_json(result.stdout).get("status"), "stopped")

    # ---- Malformed PID values must be rejected, never passed to `kill` -------

    def test_malformed_pid_is_rejected(self):
        for contents in ["0", "-1", "abc", "", "  ", "12 34", "1e3"]:
            with self.subTest(contents=repr(contents)):
                self.setUp()  # fresh session + bystander per case
                try:
                    bystander = self.spawn("sleep")
                    self.write_pidfile(contents)

                    result = self.run_stop()

                    self.assertTrue(alive(bystander),
                                    "a malformed PID broadened the signal target")
                    payload = last_json(result.stdout)
                    self.assertNotEqual(payload.get("status"), "stopped")
                    self.assertEqual(payload.get("status"), "failed")
                    self.assertIn("pid", payload.get("error", "").lower())
                finally:
                    self.tearDown()

    def test_missing_pidfile_reports_not_running(self):
        (self.session / "state").mkdir(parents=True, exist_ok=True)
        result = self.run_stop()
        self.assertEqual(last_json(result.stdout).get("status"), "not_running")


if __name__ == "__main__":
    unittest.main()
