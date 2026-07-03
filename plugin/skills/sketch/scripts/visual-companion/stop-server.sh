#!/usr/bin/env bash
# Stop the brainstorm server and clean up
# Usage: stop-server.sh <session_dir>
#
# Validates the recorded PID (a positive integer, confirmed to be our own
# `node … server.cjs` process) before signaling, then stops the server. Only
# deletes the session directory if it's under /tmp (ephemeral); persistent
# directories (.weft/) are kept so mockups can be reviewed later.

SESSION_DIR="$1"

if [[ -z "$SESSION_DIR" ]]; then
  echo '{"error": "Usage: stop-server.sh <session_dir>"}'
  exit 1
fi

STATE_DIR="${SESSION_DIR}/state"
PID_FILE="${STATE_DIR}/server.pid"

if [[ ! -f "$PID_FILE" ]]; then
  echo '{"status": "not_running"}'
  exit 0
fi

pid=$(cat "$PID_FILE")

# Validate the PID before signaling. A raw `kill "$pid"` on a value like "0" or
# "-1" broadens the signal to the caller's whole process group / every process
# the user owns. Require a plain positive integer; reject empty, zero, negative,
# non-numeric, or multi-token values outright.
if [[ ! "$pid" =~ ^[1-9][0-9]*$ ]]; then
  echo '{"status": "failed", "error": "invalid pid in server.pid; refusing to signal"}'
  exit 1
fi

# Already gone? The pidfile is stale — clean up and report, never signal.
if ! kill -0 "$pid" 2>/dev/null; then
  rm -f "$PID_FILE" "${STATE_DIR}/server.log"
  echo '{"status": "not_running"}'
  exit 0
fi

# Confirm ownership before signaling. A stale-then-reused PID could name an
# unrelated process; only signal our own visual-companion server (launched as
# `node … server.cjs`). Refuse to touch anything else.
if ! ps -p "$pid" -o command= 2>/dev/null | grep -q 'server\.cjs'; then
  echo "{\"status\": \"failed\", \"error\": \"pid ${pid} is not the visual-companion server; refusing to signal\"}"
  exit 1
fi

# Our server, alive — stop gracefully, escalate to SIGKILL if still up.
kill "$pid" 2>/dev/null || true

# Wait for graceful shutdown (up to ~2s)
for i in {1..20}; do
  if ! kill -0 "$pid" 2>/dev/null; then
    break
  fi
  sleep 0.1
done

# If still running, escalate to SIGKILL
if kill -0 "$pid" 2>/dev/null; then
  kill -9 "$pid" 2>/dev/null || true

  # Give SIGKILL a moment to take effect
  sleep 0.1
fi

if kill -0 "$pid" 2>/dev/null; then
  echo '{"status": "failed", "error": "process still running"}'
  exit 1
fi

rm -f "$PID_FILE" "${STATE_DIR}/server.log"

# Only delete ephemeral /tmp directories
if [[ "$SESSION_DIR" == /tmp/* ]]; then
  rm -rf "$SESSION_DIR"
fi

echo '{"status": "stopped"}'
