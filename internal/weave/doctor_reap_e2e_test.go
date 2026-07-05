// internal/weave/doctor_reap_e2e_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/seanb4t/weft/internal/workspace"
)

// TestDoctorSurfacesCrashAndStrand is the unattended-trust milestone exit test
// (roadmap §7.4): kill an executor mid-pick and strand a workspace — one command
// surfaces both. Components landed in Tasks 1-3 (internal/liveness,
// internal/cli/doctor.go, internal/cli/reap.go), so this test is EXPECTED TO
// PASS; any failure here is a Task 1-3 bug, not a reason to weaken an assertion.
//
// Fixture: an epic with two picks, both `shed isolate`d into jj workspaces
// (both beads in_progress), plus a foreign workspace outside the worktrees root.
//   - "Crash" pick A: workspace left untouched. Backdating jj state is
//     impractical, so staleness is config-driven — [liveness] threshold = "1ms"
//     plus a short sleep pushes both isolated workspaces past the threshold. No
//     jj command runs inside A's/B's workspace after the sleep (that would
//     refresh recency and make a dead executor look live).
//   - "Strand" pick B: bead closed, workspace left behind (the incomplete
//     shed-cleanup shape, seam 3 §5).
//
// Assertions run non-destructive first (doctor, dry-run reap, busy control),
// because the real reap mutates the fixture; the single destructive reap is
// last and collects both dead workspaces while skipping the foreign one.
func TestDoctorSurfacesCrashAndStrand(t *testing.T) {
	requireSubstrate(t)
	r := newScratchRepo(t)

	// --- Fixture: epic with two picks, isolated into two workspaces ---
	epic := r.mustCreateEpic(t, "unattended-trust exit-test epic")
	aBead := r.mustCreateChild(t, epic, "pick A (crashed executor)", "eA")
	bBead := r.mustCreateChild(t, epic, "pick B (stranded workspace)", "eB")

	// isolate sets each bead in_progress BEFORE creating its workspace on trunk().
	r.runWeft(t, "", "shed", "isolate", aBead, bBead)
	aName := workspace.Name(aBead)
	bName := workspace.Name(bBead)

	// "Strand" pick B: close the bead, leave the workspace — an orphan whose bead
	// is no longer in_progress (incomplete shed-cleanup shape).
	r.mustBD(t, "close", bBead)

	// Foreign workspace: a non-bead name whose directory lives OUTSIDE the
	// worktrees root (the Claude Code worktree-agent-* shape). reap must never
	// touch it; doctor flags it distinctly.
	foreignName := "foreign-agent"
	foreignDir := filepath.Join(t.TempDir(), foreignName)
	r.mustJJ(t, "workspace", "add", foreignDir, "--name", foreignName, "-r", "trunk()")

	// "Crash" pick A: leave its workspace untouched. Drive staleness through
	// config, not by mutating jj. After this point no jj command runs inside A's
	// or B's workspace, so their last-activity signals stay frozen at isolate
	// time.
	r.writeConfig(t, "1ms")
	time.Sleep(50 * time.Millisecond)

	// Pre-run snapshot for the I1 non-mutation proxy (doctor + dry-run reap must
	// leave this untouched).
	wsBefore := r.jjWorkspaceNames(t)

	// --- Assertion 1: ONE doctor run surfaces BOTH the crash and the strand ---
	// runWeft fatals on a non-zero exit or ok=false, so a successful call already
	// proves exit code 0 with a well-formed envelope.
	doc := r.runWeft(t, "", "doctor")
	dd := parseDoctor(t, doc.Data)

	strayA, ok := findFinding(dd.Findings, func(f drFinding) bool {
		return f.Category == "stray" && f.Bead == aBead
	})
	if !ok {
		t.Fatalf("doctor(1ms): no stray finding for crashed pick A (%s); findings=%+v", aBead, dd.Findings)
	}
	if strayA.Reason != "stale-activity" {
		t.Fatalf("doctor(1ms): stray A reason = %q, want %q", strayA.Reason, "stale-activity")
	}
	if strayA.Suggest == "" {
		t.Fatalf("doctor(1ms): stray A has empty suggest; want a recovery verb")
	}

	orphanB, ok := findFinding(dd.Findings, func(f drFinding) bool {
		return f.Category == "orphan" && f.Bead == bBead
	})
	if !ok {
		t.Fatalf("doctor(1ms): no orphan finding for stranded pick B (%s); findings=%+v", bBead, dd.Findings)
	}
	if orphanB.Reason != "bead-not-in-progress" {
		t.Fatalf("doctor(1ms): orphan B reason = %q, want %q", orphanB.Reason, "bead-not-in-progress")
	}
	if orphanB.Workspace != bName {
		t.Fatalf("doctor(1ms): orphan B workspace = %q, want %q", orphanB.Workspace, bName)
	}
	if orphanB.Suggest == "" {
		t.Fatalf("doctor(1ms): orphan B has empty suggest; want a recovery verb")
	}

	foreignF, ok := findFinding(dd.Findings, func(f drFinding) bool {
		return f.Category == "foreign" && f.Workspace == foreignName
	})
	if !ok {
		t.Fatalf("doctor(1ms): no foreign finding for %s; findings=%+v", foreignName, dd.Findings)
	}
	if foreignF.Reason != "no-bead" {
		t.Fatalf("doctor(1ms): foreign reason = %q, want %q", foreignF.Reason, "no-bead")
	}

	// --- Assertion 2: dry-run reap lists both, forgets nothing ---
	dry := r.runWeft(t, "", "reap", "--dry-run")
	dr := parseReap(t, dry.Data)
	if !dr.DryRun {
		t.Fatalf("reap --dry-run(1ms): dry_run=false in envelope")
	}
	if !strIn(dr.WouldReap, aBead) || !strIn(dr.WouldReap, bBead) {
		t.Fatalf("reap --dry-run(1ms): would_reap=%v, want both %s and %s", dr.WouldReap, aBead, bBead)
	}
	if len(dr.Reaped) != 0 {
		t.Fatalf("reap --dry-run(1ms): reaped=%v, want empty (mutates nothing)", dr.Reaped)
	}
	if !strIn(dr.Foreign, foreignName) {
		t.Fatalf("reap --dry-run(1ms): foreign=%v, want %s", dr.Foreign, foreignName)
	}
	if got := r.jjWorkspaceNames(t); !sameSet(got, wsBefore) {
		t.Fatalf("reap --dry-run(1ms) mutated the workspace list: before=%v after=%v", wsBefore, got)
	}
	if !isDir(r.workspacePath(t, aBead)) || !isDir(r.workspacePath(t, bBead)) {
		t.Fatalf("reap --dry-run(1ms) removed a workspace dir from disk")
	}

	// --- Assertion 3: busy control at threshold 24h ---
	// A crashed executor within a 24h window looks live; doctor must NOT flag it
	// stray, and reap must keep it. This runs BEFORE the destructive reap so the
	// single 1ms reap below can collect BOTH dead workspaces. The busy control is
	// non-destructive (dry-run) on purpose: a real reap at 24h would still collect
	// the closed-orphan B (its bead-not-in-progress state is threshold-independent),
	// removing B before the destructive step and defeating "reap collects both".
	r.writeConfig(t, "24h")

	docBusy := r.runWeft(t, "", "doctor")
	ddBusy := parseDoctor(t, docBusy.Data)
	if _, found := findFinding(ddBusy.Findings, func(f drFinding) bool {
		return f.Category == "stray" && f.Bead == aBead
	}); found {
		t.Fatalf("doctor(24h): pick A flagged stray though within threshold (busy); findings=%+v", ddBusy.Findings)
	}

	dryBusy := r.runWeft(t, "", "reap", "--dry-run")
	drBusy := parseReap(t, dryBusy.Data)
	if strIn(drBusy.WouldReap, aBead) {
		t.Fatalf("reap --dry-run(24h): busy pick A in would_reap=%v — a live executor must be kept", drBusy.WouldReap)
	}
	if !strIn(drBusy.WouldReap, bBead) {
		t.Fatalf("reap --dry-run(24h): closed-orphan pick B missing from would_reap=%v (threshold-independent)", drBusy.WouldReap)
	}

	// --- Invariant I1 proxy: doctor mutates nothing ---
	// The subprocess harness runs the real weft binary, so it exposes no per-call
	// runner write-log; the literal "doctor issued no bd/jj write calls" cannot be
	// asserted here (noted in the bead report). Behavioral proxy: after two doctor
	// runs and two dry-run reaps, the workspace set and both bead statuses are
	// exactly the pre-run state.
	if got := r.jjWorkspaceNames(t); !sameSet(got, wsBefore) {
		t.Fatalf("doctor/dry-run reap mutated the workspace list (I1 violated): before=%v after=%v", wsBefore, got)
	}
	if s := r.bdStatus(t, aBead); s != "in_progress" {
		t.Fatalf("pick A status is %q after doctor (I1 violated); want in_progress", s)
	}
	if s := r.bdStatus(t, bBead); s != "closed" {
		t.Fatalf("pick B status is %q after doctor (I1 violated); want closed", s)
	}

	// --- Assertion 4: the destructive reap collects both dead, skips the foreign ---
	// A's recency is still frozen at isolate time (no jj command ran in its
	// workspace), so re-narrowing the threshold to 1ms makes the crashed executor
	// reapable again alongside the stranded, already-orphaned B.
	r.writeConfig(t, "1ms")
	reap := r.runWeft(t, "", "reap")
	rp := parseReap(t, reap.Data)
	if !strIn(rp.Reaped, aBead) || !strIn(rp.Reaped, bBead) {
		t.Fatalf("reap(1ms): reaped=%v, want both dead workspaces %s and %s", rp.Reaped, aBead, bBead)
	}
	if strIn(rp.Reaped, foreignName) {
		t.Fatalf("reap(1ms) reaped the foreign workspace: reaped=%v", rp.Reaped)
	}
	if !strIn(rp.Foreign, foreignName) {
		t.Fatalf("reap(1ms): foreign=%v, want %s skipped", rp.Foreign, foreignName)
	}

	// --- Assertion 5: the foreign workspace survives; the dead ones are gone ---
	after := r.jjWorkspaceNames(t)
	if strIn(after, aName) || strIn(after, bName) {
		t.Fatalf("reap(1ms) left a dead workspace registered: %v", after)
	}
	if !strIn(after, foreignName) {
		t.Fatalf("reap(1ms) unregistered the foreign workspace: %v", after)
	}
	if !strIn(after, "default") {
		t.Fatalf("reap(1ms) removed the default workspace: %v", after)
	}
	if !isDir(foreignDir) {
		t.Fatalf("reap(1ms) deleted the foreign workspace dir %s", foreignDir)
	}
	if isDir(r.workspacePath(t, aBead)) || isDir(r.workspacePath(t, bBead)) {
		t.Fatalf("reap(1ms) left a dead workspace dir on disk")
	}
}

// --- parsing types + helpers (local to this file) ---

// drFinding mirrors the doctor `finding` envelope shape (internal/cli/doctor.go).
type drFinding struct {
	Category  string `json:"category"`
	Reason    string `json:"reason"`
	Bead      string `json:"bead"`
	Workspace string `json:"workspace"`
	Change    string `json:"change"`
	Evidence  string `json:"evidence"`
	Suggest   string `json:"suggest"`
}

// doctorData is the data payload of a `weft doctor --json` envelope.
type doctorData struct {
	Findings []drFinding `json:"findings"`
	Warnings []string    `json:"warnings"`
}

// reapData is the data payload of a `weft reap --json` envelope.
type reapData struct {
	Reaped    []string `json:"reaped"`
	WouldReap []string `json:"would_reap"`
	Foreign   []string `json:"foreign"`
	DryRun    bool     `json:"dry_run"`
}

func parseDoctor(t *testing.T, data json.RawMessage) doctorData {
	t.Helper()
	var d doctorData
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("parse doctor data: %v\n%s", err, data)
	}
	return d
}

func parseReap(t *testing.T, data json.RawMessage) reapData {
	t.Helper()
	var d reapData
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("parse reap data: %v\n%s", err, data)
	}
	return d
}

// findFinding returns the first finding satisfying pred, and whether one exists.
func findFinding(fs []drFinding, pred func(drFinding) bool) (drFinding, bool) {
	for _, f := range fs {
		if pred(f) {
			return f, true
		}
	}
	return drFinding{}, false
}

// writeConfig rewrites .weft/config.toml at the repo root, preserving the shed
// and verify blocks and setting [liveness] threshold. The file lives in the
// default workspace, outside the worktrees root, so rewriting it never refreshes
// the isolated picks' last-activity signals.
func (r *scratchRepo) writeConfig(t *testing.T, threshold string) {
	t.Helper()
	cfg := "[shed]\nmax = 10\n\n" +
		"[verify]\ncommand = \"test ! -f .weft-verify-fail\"\n\n" +
		"[liveness]\nthreshold = \"" + threshold + "\"\n"
	if err := os.WriteFile(filepath.Join(r.root, ".weft", "config.toml"), []byte(cfg), 0o600); err != nil {
		t.Fatalf("write .weft/config.toml: %v", err)
	}
}

// jjWorkspaceNames returns the registered jj workspace names, mirroring the
// exact template reap.go/doctor.go use.
func (r *scratchRepo) jjWorkspaceNames(t *testing.T) []string {
	t.Helper()
	cmd := execJJ(r, "workspace", "list", "-T", `name ++ "\n"`)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("jj workspace list: %v\nstderr: %s", err, stderr.String())
	}
	return splitLinesTrim(stdout.String())
}

// bdStatus returns a bead's status via `bd show --json` (used by the I1 proxy).
func (r *scratchRepo) bdStatus(t *testing.T, bead string) string {
	t.Helper()
	cmd := execBD(r, "show", bead, "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bd show %s: %v\nstderr: %s", bead, err, stderr.String())
	}
	// bd show --json writes a clean JSON array to stdout (separate buffers keep
	// chatter off it), so parse it directly — routing through lastJSONLine would
	// wrongly extract the inner object from the array.
	var arr []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &arr); err != nil {
		t.Fatalf("bd show %s: parse json: %v\nstdout: %s", bead, err, stdout.String())
	}
	if len(arr) == 0 {
		t.Fatalf("bd show %s: no results", bead)
	}
	return arr[0].Status
}

// execJJ builds a jj command in the repo root (peer of execBD in harness_test.go).
func execJJ(r *scratchRepo, args ...string) *exec.Cmd {
	cmd := exec.Command("jj", append([]string{"--no-pager"}, args...)...)
	cmd.Dir = r.root
	return cmd
}

// splitLinesTrim splits on newlines, trimming whitespace and dropping empties.
func splitLinesTrim(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			out = append(out, ln)
		}
	}
	return out
}

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// strIn reports whether ss contains s.
func strIn(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// sameSet reports whether a and b hold the same string set (order-independent).
func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}
