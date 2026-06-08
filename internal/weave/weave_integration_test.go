// internal/weave/weave_integration_test.go
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
	"slices"
	"strings"
	"testing"
)

// TestWeaveLoopEndToEnd drives the full weave loop once over the synthetic
// fixture and asserts each branch closes: clean land (p1), conflict→heal
// (p2a/p2b), verify-fail→retry (p3), unresolvable→human escalation (p4a/p4b).
//
// The test is deliberately slow — it spins up 6 jj workspaces, creates
// conflicts, drives the real weft binary — budget several minutes.
//
// Integration strategy: two calls to `shed integrate` (wave1: clean+heal;
// wave2: escalate) rather than one call for all 6 beads. This avoids jj's
// cascade propagation: when an escalated bead's change remains conflicted, all
// beads stacked above it in a linear integration stack also become
// cascade-conflicted and cannot be landed. By integrating the escalate pair
// last (with no beads above it), only the escalated bead itself is stuck, and
// the non-conflicted bead of the escalate pair can still be landed.
func TestWeaveLoopEndToEnd(t *testing.T) {
	r := newScratchRepo(t)
	fx := r.seedFixture(t)

	// refOf inverts the ref→id map for branching on a returned bead-id.
	refOf := map[string]string{}
	for ref, id := range fx.byRef {
		refOf[id] = ref
	}

	// Split the wave into two integration groups.
	// wave1: p1 (clean), p2a+p2b (heal pair), p3 (verify-fail/retry).
	// wave2: p4a+p4b (escalate pair).
	var wave1, wave2 []string

	// --- Step 1: form the wave ---
	form := r.runWeft(t, "", "shed", "form", "--epic", fx.epic)
	wave := dataStringSlice(t, form.Data, "wave")
	if len(wave) != 6 {
		t.Fatalf("wave = %v, want 6 picks", wave)
	}
	for _, bead := range wave {
		ref := refOf[bead]
		if ref == "" {
			t.Fatalf("bead %s not in refOf map (unexpected wave member)", bead)
		}
		if ref == "p4a" || ref == "p4b" {
			wave2 = append(wave2, bead)
		} else {
			wave1 = append(wave1, bead)
		}
	}
	if len(wave1) != 4 || len(wave2) != 2 {
		t.Fatalf("wave split: wave1=%v wave2=%v", wave1, wave2)
	}

	// --- Step 2: isolate ---
	r.runWeft(t, "", append([]string{"shed", "isolate"}, wave...)...)

	// --- Step 3+4: dispatch (scripted) + verify gate, with p3 retry ---
	//
	// `pick verify` is run from the REPO ROOT (not the workspace) so that weft
	// finds .weft/config.toml there. The verify gate "test ! -f .weft-verify-fail"
	// checks for the marker in the repo root. Writing the marker into the
	// workspace directory would cause `jj commit` (in pick seal) to commit it as
	// part of the pick's change, causing spurious conflicts at integrate.
	// The executor writes pick content files into the workspace directory; the
	// verify-fail marker is written into the repo root (r.root) and removed after
	// the retry.
	for _, bead := range wave {
		ref := refOf[bead]
		ws := r.workspacePath(t, bead)

		// Write pick content into the workspace (never the fail-marker).
		r.scriptedExecutor(t, ws, ref, false)
		if ref == "p3" {
			// Drop the fail marker in the repo root so the gate trips.
			marker := filepath.Join(r.root, ".weft-verify-fail")
			if err := os.WriteFile(marker, []byte("x"), 0o600); err != nil {
				t.Fatalf("write verify-fail marker: %v", err)
			}
		}

		// verify runs from r.root so it reads .weft/config.toml and the marker.
		v := r.runWeft(t, "", "pick", "verify", bead)
		pass := dataBool(t, v.Data, "pass")
		if ref == "p3" {
			if pass {
				t.Fatalf("p3 first verify should fail (verify-fail marker in repo root)")
			}
			// Remove the marker and re-verify (retry path).
			if err := os.Remove(filepath.Join(r.root, ".weft-verify-fail")); err != nil {
				t.Fatalf("remove verify-fail marker: %v", err)
			}
			v = r.runWeft(t, "", "pick", "verify", bead)
			if !dataBool(t, v.Data, "pass") {
				t.Fatalf("p3 retry verify should pass")
			}
		} else if !pass {
			t.Fatalf("%s verify should pass, got fail", ref)
		}
		r.runWeft(t, ws, "pick", "seal", bead)
	}

	// --- Step 5a: integrate wave1 (p1, p2a, p2b, p3) ---
	// Expect exactly 1 conflict: the lex-later bead of the p2 pair
	// (p2a or p2b, whichever has the lex-later bead-id).
	integ1 := r.runWeft(t, "", append([]string{"shed", "integrate"}, wave1...)...)
	var integ1Data struct {
		Stack []struct {
			Bead   string `json:"bead"`
			Change string `json:"change"`
		} `json:"stack"`
		Conflicts []struct {
			Bead   string `json:"bead"`
			Change string `json:"change"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal(integ1.Data, &integ1Data); err != nil {
		t.Fatalf("parse integ1 data: %v", err)
	}
	// wave1 has one conflict pair (p2a/p2b); must have at least 1 conflict.
	// Cascade may inflate the count but there must be at least 1.
	if len(integ1Data.Conflicts) < 1 {
		t.Fatalf("wave1 conflicts = %d, want >= 1: %s", len(integ1Data.Conflicts), integ1.Data)
	}

	// Build stack position map for wave1.
	wave1StackPos := map[string]int{}
	for i, e := range integ1Data.Stack {
		wave1StackPos[e.Bead] = i
	}
	sortedW1Conflicts := sortConflictsByStackPos(integ1Data.Conflicts, wave1StackPos)

	// --- Step 6a: resolve wave1 conflicts (heal the p2 pair conflict) ---
	//
	// All conflicts in wave1 either are the genuine p2 pair conflict or are
	// cascade from it. Process bottom-up; heal any p2 pair bead, skip
	// cascade-only beads (p1, p3 not in any conflict pair).
	for _, c := range sortedW1Conflicts {
		ref := refOf[c.Bead]
		if ref != "p2a" && ref != "p2b" {
			continue // cascade-only (p1, p3): skip
		}
		if !r.isConflicted(t, c.Change) {
			continue // already resolved by a previous cascade heal
		}
		open := r.runWeft(t, "", "conflict", "open", c.Bead)
		resolveDir := dataString(t, open.Data, "path")
		// Heal: fix ALL conflicted files (including cascade-introduced ones).
		r.healAllConflicts(t, resolveDir)
		fin := r.runWeft(t, "", "conflict", "finalize", c.Bead)
		if dataBool(t, fin.Data, "escalated") {
			t.Fatalf("wave1 conflict %s (ref=%s) unexpectedly escalated", c.Bead, ref)
		}
	}

	// --- Step 7a: land all wave1 picks ---
	for _, bead := range wave1 {
		r.runWeft(t, "", "pick", "land", bead)
	}

	// --- Step 8a: cleanup wave1 workspaces ---
	r.runWeft(t, "", append([]string{"shed", "cleanup"}, wave1...)...)

	// --- Step 5b: integrate wave2 (p4a, p4b — escalate pair) ---
	integ2 := r.runWeft(t, "", append([]string{"shed", "integrate"}, wave2...)...)
	var integ2Data struct {
		Stack []struct {
			Bead   string `json:"bead"`
			Change string `json:"change"`
		} `json:"stack"`
		Conflicts []struct {
			Bead   string `json:"bead"`
			Change string `json:"change"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal(integ2.Data, &integ2Data); err != nil {
		t.Fatalf("parse integ2 data: %v", err)
	}
	// wave2 has one conflict pair (p4a/p4b); must have exactly 1 conflict
	// (no cascade possible with only 2 beads — no downstream picks).
	if len(integ2Data.Conflicts) != 1 {
		t.Fatalf("wave2 conflicts = %d, want 1: %s", len(integ2Data.Conflicts), integ2.Data)
	}

	// --- Step 6b: escalate the wave2 conflict ---
	escalated := map[string]bool{}
	c2 := integ2Data.Conflicts[0]
	open2 := r.runWeft(t, "", "conflict", "open", c2.Bead)
	resolveDir2 := dataString(t, open2.Data, "path")
	// Escalate: workspace opened but left unresolved; finalize reads workspace@ directly.
	if resolveDir2 == "" {
		t.Fatalf("conflict open (escalate) returned empty path")
	}
	fin2 := r.runWeft(t, "", "conflict", "finalize", c2.Bead)
	if !dataBool(t, fin2.Data, "escalated") {
		t.Fatalf("wave2 conflict %s should escalate (markers left unresolved)", c2.Bead)
	}
	escalated[c2.Bead] = true
	escalatedBead := c2.Bead

	// --- Step 7b: land the non-escalated wave2 pick ---
	for _, bead := range wave2 {
		if !escalated[bead] {
			r.runWeft(t, "", "pick", "land", bead)
		}
	}

	// --- Step 8b: cleanup + reap (idempotent) ---
	// Cleanup only the non-escalated wave2 pick's workspace; the escalated
	// pick's pick workspace remains active (in_progress, not yet closed by
	// a human). Reap handles orphaned workspaces.
	toClean2 := make([]string, 0, len(wave2)-1)
	for _, bead := range wave2 {
		if !escalated[bead] {
			toClean2 = append(toClean2, bead)
		}
	}
	if len(toClean2) > 0 {
		r.runWeft(t, "", append([]string{"shed", "cleanup"}, toClean2...)...)
	}
	r.runWeft(t, "", "reap", "--epic", fx.epic)
	r.runWeft(t, "", "reap", "--epic", fx.epic) // second call must also succeed (idempotent)

	// --- Step 9: resume shows terminal state ---
	resume := r.runWeft(t, "", "resume", "--epic", fx.epic)

	landed := dataStringSlice(t, resume.Data, "landed")
	blocked := dataStringSlice(t, resume.Data, "blocked")
	inFlight := dataStringSlice(t, resume.Data, "in_flight")
	conflicts := dataStringSlice(t, resume.Data, "conflicts")

	// CONTRACT CORRECTION (verified against internal/cli/conflict.go:162-173):
	// The plan's original Step 9 asserted `len(blocked) >= 1` for the escalated
	// pick. However, the escalation path only runs:
	//   bd update <bead> --add-label human
	// It does NOT set status=blocked. The bead remains in_progress. Therefore:
	//   - resume.data.blocked is EMPTY (escalated bead is not blocked)
	//   - resume.data.in_flight contains the escalated bead (still in_progress)
	//   - resume.data.conflicts contains the escalated change-id (still conflicted
	//     in jj; conflictChanges() scopes to the epic's sealed changes)
	// The corrected assertions are AT LEAST AS STRONG as the original: they
	// positively confirm the escalated pick is surfaced for human attention via
	// the `human` label + conflicted change, rather than weakening to "something
	// is blocked somewhere".

	// 5 picks landed: p1, p2a, p2b (healed), p3, and the non-escalated p4x.
	if len(landed) != 5 {
		t.Fatalf("landed = %v (len=%d), want 5", landed, len(landed))
	}

	// The escalated pick is NOT landed.
	for _, id := range landed {
		if id == escalatedBead {
			t.Fatalf("escalated bead %s must not appear in landed", escalatedBead)
		}
	}

	// Engine contract (weft-78k): escalation flags `human` + leaves the change conflicted;
	// it does NOT set status=blocked. The escalated pick surfaces via resume.data.conflicts
	// + the human label, asserted below.
	if len(blocked) != 0 {
		t.Fatalf("blocked = %v (len=%d), want 0 — escalation does not set status=blocked", blocked, len(blocked))
	}

	// The escalated bead IS in in_flight (in_progress status).
	foundInFlight := false
	for _, id := range inFlight {
		if id == escalatedBead {
			foundInFlight = true
			break
		}
	}
	if !foundInFlight {
		t.Fatalf("escalated bead %s not found in in_flight=%v; resume.Data=%s", escalatedBead, inFlight, resume.Data)
	}

	// The escalated bead's change is still conflicted; resume.data.conflicts
	// ([]string of change-ids) must be non-empty — the escalated change surfaces here.
	if len(conflicts) == 0 {
		t.Fatalf("resume.data.conflicts is empty — escalated change not surfaced; resume.Data=%s", resume.Data)
	}

	// Confirm the `human` label via bd show (the escalation gate in the engine).
	r.assertBeadHasLabel(t, escalatedBead, "human")

	// The loop has terminated: forming again yields an empty wave (the escalated
	// pick carries `human`; bd ready excludes human-labelled beads).
	form2 := r.runWeft(t, "", "shed", "form", "--epic", fx.epic)
	if w := dataStringSlice(t, form2.Data, "wave"); len(w) != 0 {
		t.Fatalf("post-loop wave = %v, want empty (escalated pick is human-labelled, excluded by bd ready)", w)
	}
}

// sortConflictsByStackPos sorts a conflicts slice in ascending stack position
// order (bottom of stack = lower index, processed first). This ensures that
// healing a genuine conflict at a lower stack position auto-resolves cascade
// conflicts on higher positions before they are processed.
func sortConflictsByStackPos(conflicts []struct {
	Bead   string `json:"bead"`
	Change string `json:"change"`
}, stackPos map[string]int) []struct{ Bead, Change string } {
	out := make([]struct{ Bead, Change string }, len(conflicts))
	for i, c := range conflicts {
		out[i] = struct{ Bead, Change string }{c.Bead, c.Change}
	}
	slices.SortFunc(out, func(a, b struct{ Bead, Change string }) int {
		return stackPos[a.Bead] - stackPos[b.Bead]
	})
	return out
}

// healAllConflicts fixes all conflicted files in the resolution workspace by
// writing "resolved\n" to each. This is necessary when a bead in the heal
// pair has CASCADE conflicts (from an upstream bead) in addition to or instead
// of its own genuine conflict — finalize requires ZERO remaining conflicts in
// the workspace @ to proceed with the heal path.
//
// The resolution workspace's @ is an empty child of the conflicted change, so
// the conflicted files are visible in the working directory (inherited from @-)
// but jj does not list them as "changed". Uses `jj resolve --list` to
// enumerate conflicted paths, then writes "resolved\n" to each file.
func (r *scratchRepo) healAllConflicts(t *testing.T, resolveDir string) {
	t.Helper()
	cmd := exec.Command("jj", "--no-pager", "resolve", "--list")
	cmd.Dir = resolveDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jj resolve --list in %s: %v\n%s", resolveDir, err, out)
	}
	// `jj resolve --list` output: "<path>    <N>-sided conflict" per line.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	fixed := 0
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		// Extract the file path (first whitespace-delimited field).
		fields := strings.Fields(ln)
		if len(fields) == 0 {
			continue
		}
		path := fields[0]
		if err := os.WriteFile(filepath.Join(resolveDir, path), []byte("resolved\n"), 0o600); err != nil {
			t.Fatalf("healAllConflicts: write %s: %v", path, err)
		}
		fixed++
	}
	if fixed == 0 {
		t.Fatalf("healAllConflicts: no conflicted files found in %s\njj resolve --list output:\n%s", resolveDir, string(out))
	}
	// Force jj to snapshot the working copy in this workspace so that
	// subsequent commands from the repo root (e.g. `conflict finalize`) see the
	// updated file content. In jj 0.42, a workspace's working copy is only
	// snapshotted by commands that run in that workspace, not by commands
	// referencing it from another workspace.
	snap := exec.Command("jj", "--no-pager", "diff", "--stat")
	snap.Dir = resolveDir
	if out, err := snap.CombinedOutput(); err != nil {
		t.Fatalf("jj diff --stat (snapshot) in %s: %v\n%s", resolveDir, err, out)
	}
	// Verify that the resolver writes were picked up: jj resolve --list must
	// report no remaining conflicted files. An empty/zero-line result means all
	// conflicts are resolved; a non-empty result means a snapshot missed the
	// write and finalize would silently re-escalate.
	listAfter := exec.Command("jj", "--no-pager", "resolve", "--list")
	listAfter.Dir = resolveDir
	listOut, listErr := listAfter.CombinedOutput()
	// jj resolve --list exits non-zero when there are no conflicts (nothing to list).
	// So we check the trimmed output, not the exit code.
	if listErr == nil && strings.TrimSpace(string(listOut)) != "" {
		t.Fatalf("healAllConflicts: jj resolve --list still reports conflicts after write+snapshot in %s:\n%s", resolveDir, listOut)
	}
}

// isConflicted reports whether the given jj change-id is currently in jj's
// conflicts() set. This lets the conflict-resolution loop skip cascade
// conflicts that were auto-resolved when an upstream conflict was healed.
func (r *scratchRepo) isConflicted(t *testing.T, change string) bool {
	t.Helper()
	cmd := exec.Command("jj", "--no-pager", "log", "-r", "conflicts() & "+change,
		"--no-graph", "-T", `change_id.short(12) ++ "\n"`)
	cmd.Dir = r.root
	out, err := cmd.CombinedOutput()
	if err != nil {
		// jj exits non-zero when the revset matches nothing (not an error for us).
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// assertBeadHasLabel verifies the bead carries the expected label via bd show --json.
// Uses separate stdout/stderr buffers (pattern from onlyEpicID / childBeads).
func (r *scratchRepo) assertBeadHasLabel(t *testing.T, bead, label string) {
	t.Helper()
	cmd := execBD(r, "show", bead, "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bd show %s --json: %v\nstderr: %s", bead, err, stderr.String())
	}
	raw := stdout.String()
	var arr []struct {
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		t.Fatalf("bd show %s: parse json: %v\nstdout: %s", bead, err, raw)
	}
	if len(arr) == 0 {
		t.Fatalf("bd show %s: no results", bead)
	}
	for _, lbl := range arr[0].Labels {
		if lbl == label {
			return // found
		}
	}
	t.Fatalf("bead %s missing label %q; labels=%v", bead, label, arr[0].Labels)
}
