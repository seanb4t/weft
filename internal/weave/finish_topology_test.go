// internal/weave/finish_topology_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave_test

// This file holds real-jj integration tests (no mocked Runner) that close the
// coverage gaps from bead weft-8ou.7: TestFinishOpenCollapseTopology exercises
// collapseClosedPicks against real jj, and TestReconcileMergeBranchLeavesParkedSiblingUntouched
// proves the merge-commit reconcile path (seam-6 §6.1 step 3) does not drag a
// parked escalated sibling into the rebase. TestJJLogParsersTolerateSnapshotWarning
// additionally guards the jj-log parser helpers against jj's working-copy
// snapshot warnings under CI. Each test's own godoc documents its fixture,
// assertions, and rationale.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// changeIDLineRe matches a bare jj change-id token (the alphabet jj uses for
// short change-ids). It mirrors the production changeIDPattern in
// internal/cli/conflict.go and lets the jj-log parsers below distinguish a real
// data line from jj's working-copy snapshot warnings. In CI jj prints a
// multi-line "Warning: Refused to snapshot some files:" / "Hint:" block on
// stderr (which CombinedOutput folds in) when an over-size untracked file is
// present; none of those lines are change-id-shaped, so validating the token
// against this pattern skips them instead of erroring on them.
var changeIDLineRe = regexp.MustCompile(`^[a-z0-9]+$`)

// runWeftWithEnv is like runWeft but replaces specific env variables (e.g. PATH)
// with values from extraEnv. Keys present in extraEnv are stripped from
// os.Environ() before the extra entries are appended, so there is exactly one
// copy of each variable in the subprocess environment. dir=="" means repo root.
func (r *scratchRepo) runWeftWithEnv(t *testing.T, dir string, extraEnv []string, args ...string) envelope {
	t.Helper()
	if dir == "" {
		dir = r.root
	}
	full := append(append([]string{}, args...), "--json")
	cmd := exec.Command(weftBin, full...)
	cmd.Dir = dir

	// Build a set of key names that extraEnv overrides so we can strip them
	// from the inherited environment.
	overrideKeys := make(map[string]bool, len(extraEnv))
	for _, kv := range extraEnv {
		if idx := strings.Index(kv, "="); idx >= 0 {
			overrideKeys[kv[:idx]] = true
		}
	}
	// Start from the inherited env, drop any variable whose key is in extraEnv,
	// then append BEADS_DIR and the caller-supplied overrides.
	base := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if idx := strings.Index(kv, "="); idx >= 0 && overrideKeys[kv[:idx]] {
			continue // replaced by extraEnv
		}
		base = append(base, kv)
	}
	base = append(base, "BEADS_DIR="+r.beadsDir)
	base = append(base, extraEnv...)
	cmd.Env = base

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("weft %s (dir=%s) failed: %v\noutput:\n%s",
			strings.Join(args, " "), dir, err, out)
	}
	var env envelope
	if err := json.Unmarshal([]byte(lastJSONLine(out)), &env); err != nil {
		t.Fatalf("weft %s: parse envelope: %v\noutput:\n%s", strings.Join(args, " "), err, out)
	}
	if !env.OK {
		t.Fatalf("weft %s: envelope ok=false:\n%s", strings.Join(args, " "), out)
	}
	return env
}

// writeFakeGH writes a shell script that acts as a minimal fake `gh` binary for
// the collapse test. It responds to the commands `finish open` requires beyond
// the jj steps: `auth status` (exit 0), `pr view` (exit 1 / no PR), and
// `pr create` (exit 0, prints a fake URL). All other subcommands exit 0 silently.
func writeFakeGH(t *testing.T, dir string) string {
	t.Helper()
	script := `#!/bin/sh
# Fake gh for finish-open topology test.
# Routes only the commands that weft finish open calls beyond jj.
args="$*"
case "$args" in
  auth\ status*)   exit 0 ;;
  pr\ view*)       echo "no pull requests found" >&2; exit 1 ;;
  pr\ create*)     echo "https://github.com/fake/repo/pull/1"; exit 0 ;;
  repo\ view*)     echo '{"nameWithOwner":"fake/repo"}'; exit 0 ;;
  api\ -X\ DELETE*) exit 0 ;;
  *)               exit 0 ;;
esac
`
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	return ghPath
}

// jjLogParents runs `jj log -r <revset> --no-graph` with a template that emits
// `<change-id-12> <parent1-id>,<parent2-id>,...\n` per commit in repoDir.
// Returns a map from 12-char change-id → comma-joined parent change-ids (empty
// string for root nodes with no parents).
func jjLogParents(t *testing.T, repoDir, revset string) map[string]string {
	t.Helper()
	// Separator is a pipe "|" (not a space) so we can split unambiguously even
	// when the parents list is empty (root nodes). The trailing newline acts as
	// the record delimiter; the pipe separates the two fields.
	tmpl := `change_id.short(12) ++ "|" ++ parents.map(|p| p.change_id().short(12)).join(",") ++ "\n"`
	cmd := exec.Command("jj", "--no-pager", "log", "-r", revset, "--no-graph", "-T", tmpl)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jj log -r %q in %s: %v\n%s", revset, repoDir, err, out)
	}
	result := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// A data line is "<change-id>|<parents>". Skip anything that does not fit
		// that shape — in CI, CombinedOutput folds in jj's multi-line snapshot
		// "Warning:"/"Hint:" block, none of whose lines carry a change-id-shaped
		// token before a "|". Skipping (rather than erroring) mirrors the
		// production parser in collapseClosedPicks, which reads stdout and guards
		// each token with changeIDPattern. A genuinely missing data line still
		// fails loudly: callers assert the specific change-ids they expect to find
		// in the returned map.
		idx := strings.Index(line, "|")
		if idx < 0 {
			continue
		}
		changeID := line[:idx]
		if !changeIDLineRe.MatchString(changeID) {
			continue
		}
		parentList := line[idx+1:]
		result[changeID] = parentList
	}
	return result
}

// jjChangeIDAt returns the 12-char change-id of the commit at revset in repoDir.
func jjChangeIDAt(t *testing.T, repoDir, revset string) string {
	t.Helper()
	cmd := exec.Command("jj", "--no-pager", "log", "-r", revset, "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jj log -r %q: %v\n%s", revset, err, out)
	}
	// Keep only change-id-shaped lines: jj's snapshot "Warning:"/"Hint:" block
	// (folded in by CombinedOutput under CI) is dropped, leaving just the single
	// data line the template emits.
	lines := changeIDLines(strings.TrimSpace(string(out)))
	if len(lines) != 1 {
		t.Fatalf("jj log -r %q: expected 1 change-id line, got %d: %q", revset, len(lines), string(out))
	}
	return lines[0]
}

// changeIDLines splits s by newlines, trims each line, drops blanks, and keeps
// only lines that are a bare jj change-id (per changeIDLineRe). It mirrors
// production's splitTrimLines but additionally filters out jj's working-copy
// snapshot warning/hint lines so callers see only real data.
func changeIDLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if changeIDLineRe.MatchString(l) {
			out = append(out, l)
		}
	}
	return out
}

// TestJJLogParsersTolerateSnapshotWarning reproduces the CI-only failure that
// made TestFinishOpenCollapseTopology red on PR #43: in CI jj emits a multi-line
// "Warning: Refused to snapshot some files:" / "Hint:" block (on stderr, folded
// into CombinedOutput) when an over-size untracked file is present in the
// working copy, and the jj-log parsers errored on those non-data lines. Here we
// force that exact condition by setting a tiny snapshot.max-new-file-size and
// writing an over-size untracked file, then assert both jjChangeIDAt and
// jjLogParents skip the warning lines and return the real change-id.
func TestJJLogParsersTolerateSnapshotWarning(t *testing.T) {
	requireSubstrate(t)

	repoDir := t.TempDir()
	mustJJIn(t, repoDir, "git", "init", "--colocate")
	mustJJIn(t, repoDir, "config", "set", "--repo", "user.name", "Snapshot CI")
	mustJJIn(t, repoDir, "config", "set", "--repo", "user.email", "snapshot-ci@example.com")
	mustJJIn(t, repoDir, "describe", "-m", "base")

	// Capture the working-copy change-id BEFORE introducing the over-size file,
	// so we have a warning-free baseline to compare against.
	cleanChID := jjChangeIDAt(t, repoDir, "@")

	// Force jj to refuse snapshotting an untracked file: a 10-byte cap plus a
	// larger file makes every subsequent working-copy-reading jj command emit the
	// snapshot warning. The refused file is not added, so @ keeps its change-id.
	mustJJIn(t, repoDir, "config", "set", "--repo", "snapshot.max-new-file-size", "10")
	writeFileIn(t, repoDir, "big.txt", strings.Repeat("x", 64)+"\n")

	// Guard against a vacuous test: confirm jj actually prints the warning now, so
	// the parsers below are genuinely exercised against it. If a future jj stops
	// emitting it on stderr-into-CombinedOutput, fail loudly rather than pass for
	// the wrong reason.
	raw := exec.Command("jj", "--no-pager", "log", "-r", "@", "--no-graph",
		"-T", `change_id.short(12) ++ "\n"`)
	raw.Dir = repoDir
	rawOut, err := raw.CombinedOutput()
	if err != nil {
		t.Fatalf("raw jj log: %v\n%s", err, rawOut)
	}
	if !strings.Contains(string(rawOut), "Refused to snapshot") {
		t.Fatalf("snapshot warning did not reproduce; this test is no longer exercising the "+
			"parser skip path. Raw jj log output:\n%s", rawOut)
	}

	// jjChangeIDAt must skip the warning/hint lines and return the single data line.
	if got := jjChangeIDAt(t, repoDir, "@"); got != cleanChID {
		t.Fatalf("jjChangeIDAt under snapshot warning = %q, want %q (warning lines not skipped)", got, cleanChID)
	}

	// jjLogParents must skip the warning/hint lines and still return the data row.
	parents := jjLogParents(t, repoDir, "@")
	if _, ok := parents[cleanChID]; !ok {
		t.Fatalf("jjLogParents under snapshot warning dropped the data line for %q; got %v", cleanChID, parents)
	}
}

// TestFinishOpenCollapseTopology proves that collapseClosedPicks produces a real
// linear ancestor chain in jj and that the escalated (parked) pick is excluded.
//
// Topology built before finish open:
//
//	trunk() ← A ← B ← C   (three closed picks: p1, p2, p3 as a stack)
//	trunk() ← ESC           (escalated pick, trunk() sibling, parked off the stack)
//
// After finish open (non-dry-run), collapseClosedPicks linearises A, B, C onto
// trunk() via `jj rebase -r` and leaves ESC on trunk(). The test asserts:
//   - A's parent is trunk().
//   - B's parent is A.
//   - C's parent is B.
//   - ESC's parent is still trunk() (unchanged by collapse).
//   - `finish open` returns 3 picks (closed), not 4.
//
// A fake `gh` script is placed first on PATH so the real-gh gate is satisfied
// without network access. `jj git push` uses the bare local remote set up by
// newScratchRepo.
func TestFinishOpenCollapseTopology(t *testing.T) {
	requireSubstrate(t)

	r := newScratchRepo(t)

	// --- Build the epic in beads ---
	epic := r.mustCreateEpic(t, "Topology test epic")

	// Create 4 picks: three will be closed (the linear stack), one escalated (parked).
	pickA := r.mustCreateChild(t, epic, "feat: pick A", "tA")
	pickB := r.mustCreateChild(t, epic, "feat: pick B", "tB")
	pickC := r.mustCreateChild(t, epic, "feat: pick C", "tC")
	pickESC := r.mustCreateChild(t, epic, "feat: pick ESC (escalated)", "tE")

	// --- Build the jj topology manually (bypass the weave loop; we just need the
	// right jj shape to give collapseClosedPicks something real to linearize) ---
	//
	// Approach: use the standard weft flow (isolate + seal) for the 3 linear picks
	// to create the jj changes with their jj-change labels, then close them via bd.
	// For the escalated pick, isolate + seal it as a SIBLING of trunk() (which
	// happens naturally since all isolate calls fork from trunk()), then close
	// the 3 picks and leave ESC open.

	wave := []string{pickA, pickB, pickC, pickESC}

	// Isolate all four into separate jj workspaces.
	r.runWeft(t, "", append([]string{"shed", "isolate"}, wave...)...)

	// Seal each pick with a unique file (no collisions — we don't want conflict
	// storms; we just need real jj changes stamped with jj-change labels).
	r.sealWith(t, pickA, map[string]string{"topo_a.txt": "pick-A\n"})
	r.sealWith(t, pickB, map[string]string{"topo_b.txt": "pick-B\n"})
	r.sealWith(t, pickC, map[string]string{"topo_c.txt": "pick-C\n"})
	r.sealWith(t, pickESC, map[string]string{"topo_esc.txt": "pick-ESC\n"})

	// No conflicts among the 4 (they touch disjoint files). Integrate all four.
	r.runWeft(t, "", append([]string{"shed", "integrate"}, wave...)...)

	// Land the 3 non-escalated picks (closes them via bd + jj pick land marks done).
	r.runWeft(t, "", "pick", "land", pickA)
	r.runWeft(t, "", "pick", "land", pickB)
	r.runWeft(t, "", "pick", "land", pickC)

	// Escalate pickESC: add `human` label (matches what conflict finalize does
	// on escalation). The pick stays in_progress, jj change stays sealed but not
	// landed. Do NOT close it — finish open reads only CLOSED picks.
	r.mustBD(t, "update", pickESC, "--add-label", "human")

	// Cleanup the 3 landed workspaces (not the escalated one, it might still exist).
	r.runWeft(t, "", "shed", "cleanup", pickA, pickB, pickC)
	// Attempt cleanup of escalated workspace too — it won't error if absent.
	_ = exec.Command("jj", "--no-pager", "workspace", "forget", pickESC).Run()

	// --- Commit any test-harness state so `jj st` reports "no changes" ---
	{
		cmd := exec.Command("jj", "--no-pager", "commit", "-m", "chore: finish-topology test state")
		cmd.Dir = r.root
		cmd.Env = append(os.Environ(), "BEADS_DIR="+r.beadsDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("jj commit (pre-finish-open): %v\n%s", err, out)
		}
	}

	// Capture the parked ESC change-id BEFORE collapse (to compare after).
	// ESC was sealed in its workspace; find it via the jj-change label on the bead.
	escChange := r.changeIDFromLabel(t, pickESC)
	if escChange == "" {
		t.Fatal("escalated pick has no jj-change label — cannot assert topology")
	}

	// Capture A, B, C change-ids similarly (for ancestor-chain assertions).
	chA := r.changeIDFromLabel(t, pickA)
	chB := r.changeIDFromLabel(t, pickB)
	chC := r.changeIDFromLabel(t, pickC)
	if chA == "" || chB == "" || chC == "" {
		t.Fatalf("closed picks missing jj-change labels: A=%q B=%q C=%q", chA, chB, chC)
	}

	// Also capture trunk()'s change-id for the parent assertions.
	trunkChID := jjChangeIDAt(t, r.root, "trunk()")

	// --- Write a fake `gh` and prepend its directory to PATH ---
	fakeBinDir := t.TempDir()
	writeFakeGH(t, fakeBinDir)
	// Build PATH with the fake bin dir first.
	origPATH := os.Getenv("PATH")
	fakePATH := fakeBinDir + string(os.PathListSeparator) + origPATH

	// --- Run real `weft finish open` (non-dry-run) ---
	fo := r.runWeftWithEnv(t, "",
		[]string{"PATH=" + fakePATH},
		"finish", "open", epic,
	)

	// Decode the finish.open envelope.
	var foData struct {
		Picks []struct {
			Bead   string `json:"bead"`
			Change string `json:"change"`
		} `json:"picks"`
		DryRun bool `json:"dry_run"`
	}
	if err := json.Unmarshal(fo.Data, &foData); err != nil {
		t.Fatalf("parse finish.open data: %v\ndata: %s", err, fo.Data)
	}
	if foData.DryRun {
		t.Fatal("finish open --dry-run flag was NOT passed but envelope reports dry_run:true")
	}

	// Contract: only the 3 CLOSED picks appear; the escalated (human-labelled,
	// still open) pick is excluded.
	if len(foData.Picks) != 3 {
		t.Fatalf("finish open picks = %d, want 3 (closed only): %s", len(foData.Picks), fo.Data)
	}
	for _, p := range foData.Picks {
		if p.Bead == pickESC {
			t.Fatalf("escalated bead %s must not appear in finish open picks", pickESC)
		}
	}

	// --- Assert the resulting jj topology ---
	//
	// After collapseClosedPicks + `jj new <top>`, the working copy (@) is an empty
	// change above the collapsed tip. The collapsed line is @-^3 .. @- (C, B, A
	// ancestors-first). We query parents for each change-id directly.
	//
	// Use the revset `chA | chB | chC | escChange` to get all four at once.
	revset := fmt.Sprintf("%s | %s | %s | %s", chA, chB, chC, escChange)
	parents := jjLogParents(t, r.root, revset)

	// Every change must appear in the topology output.
	for _, ch := range []string{chA, chB, chC, escChange} {
		if _, ok := parents[ch]; !ok {
			t.Fatalf("change %q not found in jj topology query; got: %v", ch, parents)
		}
	}

	// The collapsed line must be linear: we verify the chain property (exactly one
	// of {A,B,C} has trunk() as parent, the other two each have a closed pick as
	// parent), but we do NOT assert a specific permutation order (e.g. A before B
	// before C). This is correct because A, B, and C are created with no dependency
	// edges between them (mustCreateChild only sets --parent epic; no bd dep edges).
	// collapseClosedPicks comments: "inter-group order is unspecified and irrelevant
	// because the loop linearizes all changes onto a single chain — correctness
	// requires only that each change's own in-set ancestors precede it."
	// Since none of A/B/C is an in-set ancestor of any other, the only invariant
	// that collapseClosedPicks guarantees (and that this test can assert) is that
	// all three end up in a linear chain, not their relative positions in it.
	closedParents := map[string]string{chA: parents[chA], chB: parents[chB], chC: parents[chC]}
	// Count how many closed changes have trunk() as parent.
	trunkChildren := 0
	for _, pList := range closedParents {
		if pList == trunkChID {
			trunkChildren++
		}
	}
	if trunkChildren != 1 {
		t.Fatalf("exactly 1 closed pick must be a direct child of trunk() after collapse; got %d. parents: %v trunkChID=%q",
			trunkChildren, closedParents, trunkChID)
	}

	// All three must form a linear chain: each pick (except the trunk child) has
	// exactly one parent that is also a closed pick.
	closedSet := map[string]bool{chA: true, chB: true, chC: true}
	inChainParents := 0
	for _, pList := range closedParents {
		if closedSet[pList] {
			inChainParents++
		}
	}
	if inChainParents != 2 {
		t.Fatalf("linear chain must have 2 picks whose parent is also a closed pick (root is excluded); got %d. parents: %v",
			inChainParents, closedParents)
	}

	// The escalated change must remain parked on trunk() — collapseClosedPicks
	// docs: "An escalated/open pick is not in `picks`, so it is never moved and
	// is left parked as a trunk() child, off the @-line." Assert:
	//   (a) ESC's parent is still trunk() (unchanged by collapse).
	//   (b) None of the 3 closed picks has ESC as its parent (not dragged in).
	//   (c) ESC is not a parent of any closed pick (not in the line).
	escParent := parents[escChange]
	if escParent != trunkChID {
		t.Fatalf("escalated pick %q parent = %q, want trunk() (%q) — collapseClosedPicks must leave escalated picks on trunk()",
			escChange, escParent, trunkChID)
	}
	// (b) No closed pick has ESC as its parent.
	for _, pList := range closedParents {
		if pList == escChange {
			t.Fatalf("a closed pick has escalated change %q as its parent — ESC was dragged into the collapsed line", escChange)
		}
	}
	// (c) ESC's parent is not any of the closed picks (redundant given (a) but
	// makes the bidirectional exclusion explicit).
	if closedSet[escParent] {
		t.Fatalf("escalated pick %q parent %q is one of the closed picks — collapse incorrectly moved ESC",
			escChange, escParent)
	}
}

// changeIDFromLabel reads the jj-change:<id> label off a bead's bd record and
// returns the 12-char change-id. Returns "" if no such label exists.
//
// `bd show --json` returns a JSON array `[{...}]`. We parse the array directly
// rather than using lastJSONLine (which finds the last top-level JSON object and
// would return only the inner object, not the wrapping array).
func (r *scratchRepo) changeIDFromLabel(t *testing.T, bead string) string {
	t.Helper()
	cmd := execBD(r, "show", bead, "--json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bd show %s: %v", bead, err)
	}
	var arr []struct {
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &arr); err != nil {
		t.Fatalf("bd show %s parse: %v\nout: %s", bead, err, string(out))
	}
	if len(arr) == 0 {
		return ""
	}
	for _, lbl := range arr[0].Labels {
		if strings.HasPrefix(lbl, "jj-change:") {
			return strings.TrimPrefix(lbl, "jj-change:")
		}
	}
	return ""
}

// TestReconcileMergeBranchLeavesParkedSiblingUntouched proves that
// `jj rebase -b @ -o main --skip-emptied` (the production merge-commit reconcile
// path, seam-6 §6.1 step 3) does NOT drag a trunk()-sibling parked change into
// the rebased branch.
//
// Fixture (true merge-commit scenario):
//
//	base ← S1 ← S2        (the PR stack; S1 and S2 are ancestors of main via M)
//	base ← PARKED          (a separate parked change; base sibling)
//	base ← M1              (a mainline advance commit on base)
//	M1, S2 ← M            (the merge commit; main bookmark points here)
//	S2 ← @                 (the working copy, a NON-EMPTY child of S2)
//
// The §6.1 ancestry condition holds: S1 and S2 are both in ::main (ancestors of
// M via M1 and the stack leg). main..@ = {@ alone} — a non-empty set — so the
// rebase actually has work to do (non-tautological). The parked change is a
// sibling of base, not reachable from main.
//
// IMPORTANT — why @ must be non-empty: if @ were an empty change, --skip-emptied
// would abandon it in BOTH the correct merge-commit fixture AND a regressed no-op
// fixture (main=S2). The post-rebase state would be identical in both cases (@
// gone), so the non-triviality assertion would accept either. Making @ non-empty
// (carries wc.txt) prevents skip-emptied from abandoning it; after the rebase its
// parent must be mChID. Against the old no-op fixture the parent stays s2ChID,
// which causes the assertion to fail — that is the regression detector.
//
// Then run `jj rebase -b @ -o main --skip-emptied`. Assert:
//   - NON-TRIVIALITY: @ still exists AND its parent is mChID (the merge tip). A
//     regression to main=S2 would leave the parent as s2ChID and fail here.
//   - PARKED's change-id is unchanged.
//   - PARKED's parent is still baseChID (it is still a base sibling).
//   - No live change has PARKED as its parent (it was not dragged in).
//   - S1 and S2 remain as ancestors of main with their original parents unchanged
//     (they were merged; the rebase must not move them).
//
// Approach: replays the literal jj rebase command rather than driving `weft finish
// reconcile` because reconcile requires a live GitHub remote for `gh pr view` and
// `jj git fetch`. The literal command is exactly what the production code executes
// (finish.go line ~488) and provides the same topological proof.
func TestReconcileMergeBranchLeavesParkedSiblingUntouched(t *testing.T) {
	// NOTE: The unit test TestFinishReconcileMergeBranchLeavesParkedEscalatedAlone
	// (internal/cli/finish_test.go) is mock-tautological: the production reconcile
	// code never literally mentions the parked change-id, so asserting no jj
	// invocation mentions "chPark" passes trivially regardless of behavior. This
	// integration test provides the actual topological proof against real jj.
	requireSubstrate(t)

	// --- Build a fresh minimal scratch jj repo (no bd needed) ---
	repoDir := t.TempDir()
	mustJJIn(t, repoDir, "git", "init", "--colocate")
	mustJJIn(t, repoDir, "config", "set", "--repo", "user.name", "Reconcile CI")
	mustJJIn(t, repoDir, "config", "set", "--repo", "user.email", "reconcile-ci@example.com")

	// --- Step 1: base commit — the shared fork point ---
	// Describe the initial working copy as "base". Create the `main` bookmark
	// here; we will move it to the merge commit M later.
	mustJJIn(t, repoDir, "describe", "-m", "root: reconcile test base")
	mustJJIn(t, repoDir, "bookmark", "create", "main", "-r", "@")
	baseChID := jjChangeIDAt(t, repoDir, "@")

	// --- Step 2: PARKED change — a base sibling (must not be touched by rebase) ---
	mustJJIn(t, repoDir, "new", baseChID)
	writeFileIn(t, repoDir, "parked.txt", "parked change\n")
	mustJJIn(t, repoDir, "describe", "-m", "parked: escalated pick")
	parkedChID := jjChangeIDAt(t, repoDir, "@")

	// --- Step 3: PR stack S1 ← S2, forking from base ---
	mustJJIn(t, repoDir, "new", baseChID)
	writeFileIn(t, repoDir, "stack1.txt", "stack commit 1\n")
	mustJJIn(t, repoDir, "describe", "-m", "feat: stack commit 1")
	s1ChID := jjChangeIDAt(t, repoDir, "@")

	mustJJIn(t, repoDir, "new", s1ChID)
	writeFileIn(t, repoDir, "stack2.txt", "stack commit 2\n")
	mustJJIn(t, repoDir, "describe", "-m", "feat: stack commit 2")
	s2ChID := jjChangeIDAt(t, repoDir, "@")

	// --- Step 4: mainline advance M1 — a child of base (not of the stack) ---
	mustJJIn(t, repoDir, "new", baseChID)
	writeFileIn(t, repoDir, "mainline.txt", "mainline advance\n")
	mustJJIn(t, repoDir, "describe", "-m", "chore: mainline advance M1")
	m1ChID := jjChangeIDAt(t, repoDir, "@")

	// --- Step 5: merge commit M with two parents (M1, S2) ---
	// `jj new <m1ChID> <s2ChID>` creates a commit whose parents are M1 and S2.
	// This is the true merge-commit that integrates the PR stack into mainline.
	mustJJIn(t, repoDir, "new", m1ChID, s2ChID)
	writeFileIn(t, repoDir, "merge_marker.txt", "merge commit M\n")
	mustJJIn(t, repoDir, "describe", "-m", "merge: PR stack into mainline (M)")
	mChID := jjChangeIDAt(t, repoDir, "@")

	// Advance `main` bookmark to the merge commit M (simulates the GitHub merge).
	mustJJIn(t, repoDir, "bookmark", "set", "main", "-r", mChID)

	// --- Step 6: working copy @ as a NON-EMPTY child of S2 ---
	// Production always has a working copy above the stack tip. `jj new <s2ChID>`
	// puts @ as a child of S2. We MUST write a file and describe it so @ is
	// non-empty: if @ were empty, `--skip-emptied` would abandon it in BOTH the
	// correct merge-commit fixture AND a no-op fixture (main=S2), making the
	// non-triviality assertion vacuous. A non-empty @ is NOT skip-emptied, so
	// after rebase its parent must be mChID (the merge tip) — which would NOT hold
	// against the old no-op fixture where main=S2 (parent would remain s2ChID).
	mustJJIn(t, repoDir, "new", s2ChID)
	writeFileIn(t, repoDir, "wc.txt", "working copy content\n")
	mustJJIn(t, repoDir, "describe", "-m", "wip: working copy above stack")
	// Capture wcChID AFTER describe so jj auto-snapshot includes the file write.
	wcChID := jjChangeIDAt(t, repoDir, "@")

	// --- Pre-rebase sanity checks ---
	before := jjLogParents(t, repoDir, parkedChID)
	if before[parkedChID] != baseChID {
		t.Fatalf("pre-rebase: parked %q parent = %q, want base %q", parkedChID, before[parkedChID], baseChID)
	}
	beforeWC := jjLogParents(t, repoDir, wcChID)
	if beforeWC[wcChID] != s2ChID {
		t.Fatalf("pre-rebase: working copy %q parent = %q, want s2ChID %q", wcChID, beforeWC[wcChID], s2ChID)
	}
	beforeS1 := jjLogParents(t, repoDir, s1ChID)
	if beforeS1[s1ChID] != baseChID {
		t.Fatalf("pre-rebase: S1 %q parent = %q, want base %q", s1ChID, beforeS1[s1ChID], baseChID)
	}
	beforeS2 := jjLogParents(t, repoDir, s2ChID)
	if beforeS2[s2ChID] != s1ChID {
		t.Fatalf("pre-rebase: S2 %q parent = %q, want S1 %q", s2ChID, beforeS2[s2ChID], s1ChID)
	}

	// --- Run the production reconcile command: jj rebase -b @ -o main --skip-emptied ---
	// This is exactly what finish.go ~488 does for the mergeStyleMergeCommit branch.
	cmd := exec.Command("jj", "--no-pager", "rebase", "-b", "@", "-o", "main", "--skip-emptied")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jj rebase -b @ -o main --skip-emptied: %v\n%s", err, out)
	}

	// --- NON-TRIVIALITY: the working copy must still exist AND be reparented to M ---
	//
	// Because @ is non-empty (carries wc.txt), --skip-emptied MUST NOT abandon it.
	// After a correct rebase onto main (the merge commit M), wcChID's parent must
	// be mChID. Against the old tautological fixture where main was set to s2ChID
	// (a no-op rebase), the parent would remain s2ChID and this assertion FAILS —
	// that is the regression catch this assertion provides.
	afterWC := jjLogParents(t, repoDir, wcChID)
	if _, exists := afterWC[wcChID]; !exists {
		t.Fatalf("NON-TRIVIALITY FAILED: working copy %q was abandoned by --skip-emptied "+
			"but it carried content (wc.txt) — fixture or rebase command is wrong",
			wcChID)
	}
	if afterWC[wcChID] != mChID {
		t.Fatalf("NON-TRIVIALITY FAILED: working copy %q parent after rebase = %q, want mChID %q. "+
			"The rebase did not relocate @ onto the merge tip. "+
			"(Against the old no-op fixture where main=S2, this parent would be s2ChID %q — "+
			"that regression is exactly what this assertion catches.)",
			wcChID, afterWC[wcChID], mChID, s2ChID)
	}

	// --- Assert parked sibling is untouched ---
	after := jjLogParents(t, repoDir, parkedChID)

	// Parked change-id must be the same (not reassigned).
	if _, exists := after[parkedChID]; !exists {
		t.Fatalf("parked change %q not found in jj after rebase — its change-id may have been altered or the change abandoned", parkedChID)
	}

	// Parked change's parent must still be the base commit (unchanged by rebase).
	if after[parkedChID] != baseChID {
		t.Fatalf("parked change %q parent after rebase = %q, want base %q — jj rebase -b dragged the parked sibling",
			parkedChID, after[parkedChID], baseChID)
	}

	// No live change may have PARKED as a parent. jjLogParents returns a
	// comma-joined parent list, so split before comparing: a merge commit that
	// dragged PARKED in as one of two parents ("X,PARKED") must still be caught,
	// not just the sole-parent case.
	allParents := jjLogParents(t, repoDir, "all()")
	for ch, pList := range allParents {
		for _, p := range strings.Split(pList, ",") {
			if p == parkedChID {
				t.Fatalf("change %q has parked %q as parent after rebase — parked was dragged into the rebased branch",
					ch, parkedChID)
			}
		}
	}

	// --- Assert S1 and S2 are ancestors of main and have unchanged parents ---
	// The §6.1 ancestry condition (S1,S2 ∈ ::main) must hold. After the rebase
	// with --skip-emptied, S1 and S2 may be abandoned (they were already reachable
	// via M); check their original parents via `all() | ancestors(main, 1)` revsets.
	//
	// Robust check: query S1 and S2 directly; if they are in all() their parents
	// must be unchanged; if they are abandoned (not in all()) the rebase
	// correctly skip-emptied them as expected.
	if s1Parents, ok := allParents[s1ChID]; ok {
		if s1Parents != baseChID {
			t.Fatalf("S1 %q survived rebase but parent changed: got %q, want base %q", s1ChID, s1Parents, baseChID)
		}
	}
	if s2Parents, ok := allParents[s2ChID]; ok {
		if s2Parents != s1ChID {
			t.Fatalf("S2 %q survived rebase but parent changed: got %q, want S1 %q", s2ChID, s2Parents, s1ChID)
		}
	}
}

// mustJJIn runs a jj command in dir, fataling on error.
func mustJJIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("jj", append([]string{"--no-pager"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("jj %s (dir=%s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

// writeFileIn writes content to filename in dir, fataling on error.
func writeFileIn(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s/%s: %v", dir, filename, err)
	}
}
