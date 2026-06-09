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
	"strings"
	"testing"
)

// TestWeaveLoopEndToEnd drives the full weave loop once over the synthetic
// fixture using a SINGLE call to `shed integrate` over all 6 picks.
//
// Integration strategy: the fixture's file-overlap forest confines conflicts to
// the two collider pairs (p2a/p2b and p4a/p4b); no cross-group cascade leaks
// because the pairs touch disjoint files. One integrate call therefore surfaces
// exactly 2 conflicts. The test heals p2, escalates p4, lands 5 picks, and
// confirms that `finish open` ships those 5 (escalated pick is parked off the
// merged line).
//
// The test is deliberately slow — it spins up 6 jj workspaces, creates
// conflicts, drives the real weft binary — budget several minutes.
func TestWeaveLoopEndToEnd(t *testing.T) {
	requireSubstrate(t)
	r := newScratchRepo(t)
	fx := r.seedFixture(t)

	// refOf inverts the ref→id map for branching on a returned bead-id.
	refOf := map[string]string{}
	for ref, id := range fx.byRef {
		refOf[id] = ref
	}

	// --- Step 1: form the wave ---
	form := r.runWeft(t, "", "shed", "form", "--epic", fx.epic)
	wave := dataStringSlice(t, form.Data, "wave")
	if len(wave) != 6 {
		t.Fatalf("wave = %v, want 6 picks", wave)
	}
	for _, bead := range wave {
		if refOf[bead] == "" {
			t.Fatalf("bead %s not in refOf map (unexpected wave member)", bead)
		}
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

	// --- Step 5: integrate the WHOLE wave (one call) ---
	integ := r.runWeft(t, "", append([]string{"shed", "integrate"}, wave...)...)
	var integData struct {
		Groups [][]struct {
			Bead   string `json:"bead"`
			Change string `json:"change"`
		} `json:"groups"`
		Conflicts []struct {
			Bead   string `json:"bead"`
			Change string `json:"change"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal(integ.Data, &integData); err != nil {
		t.Fatalf("parse integrate data: %v", err)
	}
	// The forest confines conflicts to the two overlap pairs — exactly 2, no cascade.
	if len(integData.Conflicts) != 2 {
		t.Fatalf("conflicts = %d, want exactly 2 (p2 + p4 colliders, no cross-group cascade): %s",
			len(integData.Conflicts), integ.Data)
	}
	// The wave is woven as a forest, not one linear stack: the two collider pairs
	// land in separate overlap groups (alongside the independent singletons), so
	// there is more than one group and every pick appears in exactly one group.
	if len(integData.Groups) < 2 {
		t.Fatalf("groups = %d, want >= 2 (forest, not one linear stack): %s",
			len(integData.Groups), integ.Data)
	}
	groupedPicks := 0
	for _, g := range integData.Groups {
		groupedPicks += len(g)
	}
	if groupedPicks != 6 {
		t.Fatalf("groups cover %d picks, want all 6: %s", groupedPicks, integ.Data)
	}

	// --- Step 6: resolve — heal the p2-pair conflict, escalate the p4-pair conflict ---
	var escalatedBead, escalatedChange string
	for _, c := range integData.Conflicts {
		ref := refOf[c.Bead]
		switch ref {
		case "p2a", "p2b":
			open := r.runWeft(t, "", "conflict", "open", c.Bead)
			r.healAllConflicts(t, dataString(t, open.Data, "path"))
			fin := r.runWeft(t, "", "conflict", "finalize", c.Bead)
			if dataBool(t, fin.Data, "escalated") {
				t.Fatalf("p2 conflict %s unexpectedly escalated", c.Bead)
			}
		case "p4a", "p4b":
			open := r.runWeft(t, "", "conflict", "open", c.Bead)
			if dataString(t, open.Data, "path") == "" {
				t.Fatalf("conflict open (escalate) returned empty path")
			}
			fin := r.runWeft(t, "", "conflict", "finalize", c.Bead)
			if !dataBool(t, fin.Data, "escalated") {
				t.Fatalf("p4 conflict %s should escalate (markers left unresolved)", c.Bead)
			}
			escalatedBead, escalatedChange = c.Bead, c.Change
		default:
			t.Fatalf("unexpected conflict ref %q (bead %s) — cascade leaked across groups?", ref, c.Bead)
		}
	}
	if escalatedBead == "" {
		t.Fatal("no p4 conflict escalated")
	}

	// --- Step 7: land every non-escalated pick (the escalated tail fails the gate) ---
	for _, bead := range wave {
		if bead == escalatedBead {
			continue
		}
		r.runWeft(t, "", "pick", "land", bead)
	}

	// --- Step 8: cleanup the landed (non-escalated) workspaces ---
	toClean := make([]string, 0, len(wave)-1)
	for _, bead := range wave {
		if bead != escalatedBead {
			toClean = append(toClean, bead)
		}
	}
	r.runWeft(t, "", append([]string{"shed", "cleanup"}, toClean...)...)
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
	// The escalated change-id must be a member of resume.data.conflicts.
	// This catches regressions where resume reports a stale or different change-id.
	foundConflict := false
	for _, ch := range conflicts {
		if ch == escalatedChange {
			foundConflict = true
			break
		}
	}
	if !foundConflict {
		t.Fatalf("escalated change %s not found in resume.data.conflicts=%v; resume.Data=%s",
			escalatedChange, conflicts, resume.Data)
	}

	// Confirm the `human` label via bd show (the escalation gate in the engine).
	r.assertBeadHasLabel(t, escalatedBead, "human")

	// The loop has terminated: forming again yields an empty wave (the escalated
	// pick carries `human`; bd ready excludes human-labelled beads).
	form2 := r.runWeft(t, "", "shed", "form", "--epic", fx.epic)
	if w := dataStringSlice(t, form2.Data, "wave"); len(w) != 0 {
		t.Fatalf("post-loop wave = %v, want empty (escalated pick is human-labelled, excluded by bd ready)", w)
	}

	// --- Step 10: finish open dry-run ships the 5 landed picks, not the escalated one ---
	// finish open --dry-run runs finishOpenPreflight which calls `jj st` and requires
	// a clean working copy ("no changes"). The default workspace @ has accumulated
	// uncommitted changes from test-harness setup (.weft/config.toml, .beads/ mutations).
	// Commit them now so the preflight passes — this models the real workflow where the
	// orchestrator has a clean @ before shipping.
	{
		cmd := exec.Command("jj", "--no-pager", "commit", "-m", "chore: test harness state (weave E2E)")
		cmd.Dir = r.root
		cmd.Env = os.Environ()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("jj commit (pre-finish cleanup): %v\n%s", err, out)
		}
	}
	fo := r.runWeft(t, "", "finish", "open", fx.epic, "--dry-run")
	var foData struct {
		Picks []struct {
			Bead   string `json:"bead"`
			Change string `json:"change"`
		} `json:"picks"`
	}
	if err := json.Unmarshal(fo.Data, &foData); err != nil {
		t.Fatalf("parse finish.open data: %v", err)
	}
	if len(foData.Picks) != 5 {
		t.Fatalf("finish open picks = %d, want 5 (closed only): %s", len(foData.Picks), fo.Data)
	}
	for _, p := range foData.Picks {
		if p.Bead == escalatedBead {
			t.Fatalf("escalated bead %s must not be in finish picks", escalatedBead)
		}
	}
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
	// jj resolve --list exits non-zero (exit 2) when there are no conflicts
	// ("No conflicts found at this revision"). That is the desired clean state.
	// Any other non-nil error is a genuine jj failure and must fail loudly.
	// Exit 0 with non-empty output means conflicts remain — also fail loudly.
	if listErr != nil {
		// Treat "no conflicts" (exit 2) as the expected clean state; fail on anything else.
		if exitErr, ok := listErr.(*exec.ExitError); !ok || exitErr.ExitCode() != 2 {
			t.Fatalf("healAllConflicts: jj resolve --list unexpected error in %s: %v\n%s", resolveDir, listErr, listOut)
		}
		// Exit 2 means "No conflicts found" — workspace is clean, proceed.
	} else if strings.TrimSpace(string(listOut)) != "" {
		// Exit 0 with output means conflicts are still present.
		t.Fatalf("healAllConflicts: jj resolve --list still reports conflicts after write+snapshot in %s:\n%s", resolveDir, listOut)
	}
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
