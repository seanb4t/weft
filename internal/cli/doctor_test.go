// internal/cli/doctor_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

// doctorCfg parameterizes a doctor mock: each field maps to one subprocess the
// three passes make. Zero values yield a healthy, empty warp (no workspaces, no
// in_progress beads, no open epics), so a test sets only the pieces it exercises.
type doctorCfg struct {
	root       string
	workspaces string                // jj workspace list stdout (default "default\n")
	beadShow   map[string]run.Result // bd show <bead> --json, keyed by bead-id
	stale      map[string]bool       // workspace name → stale liveness timestamp
	inProgress string                // bd list --status in_progress --json (default "[]")
	landed     map[string]bool       // change-id present in ::trunk()
	conflicted map[string]bool       // change-id present in conflicts()
	epicStatus string                // bd epic status --json (default "[]")
	bookmarks  map[string]string     // epic → jj bookmark list stdout ("" = absent)
	prView     map[string]run.Result // epic → gh pr view --json state result
}

// argAfter returns the argument following flag, or "".
func argAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// doctorFake builds a routeRunner covering every subprocess doctor dispatches.
// jj/bd calls succeed (Code 0); bd show defaults to the not-found payload (as
// reapFake does) so an unlisted workspace resolves to a missing bead.
func doctorFake(cfg doctorCfg) *routeRunner {
	if cfg.workspaces == "" {
		cfg.workspaces = "default\n"
	}
	if cfg.inProgress == "" {
		cfg.inProgress = "[]"
	}
	if cfg.epicStatus == "" {
		cfg.epicStatus = "[]"
	}
	tsLayout := "2006-01-02T15:04:05-0700"
	tsFresh := time.Now().Format(tsLayout)
	tsStale := time.Now().Add(-72 * time.Hour).Format(tsLayout)
	return &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case name == "jj" && len(args) >= 2 && args[1] == "root":
			return run.Result{Stdout: cfg.root, Code: 0}
		case name == "jj" && strings.Contains(j, "workspace list"):
			return run.Result{Stdout: cfg.workspaces, Code: 0}
		case name == "jj" && strings.Contains(j, "committer.timestamp"): // liveness probe
			ws := strings.TrimSuffix(argAfter(args, "-r"), "@")
			if cfg.stale[ws] {
				return run.Result{Stdout: tsStale + "\n", Code: 0}
			}
			return run.Result{Stdout: tsFresh + "\n", Code: 0}
		case name == "jj" && strings.Contains(j, "::trunk()"): // change-in-trunk
			for ch := range cfg.landed {
				if strings.Contains(j, ch) {
					return run.Result{Stdout: ch + "\n", Code: 0}
				}
			}
			return run.Result{Code: 0}
		case name == "jj" && strings.Contains(j, "conflicts()"): // change-conflicted
			for ch := range cfg.conflicted {
				if strings.Contains(j, ch) {
					return run.Result{Stdout: ch + "\n", Code: 0}
				}
			}
			return run.Result{Code: 0}
		case name == "jj" && strings.Contains(j, "bookmark list"):
			return run.Result{Stdout: cfg.bookmarks[args[len(args)-1]], Code: 0}
		case name == "bd" && len(args) >= 2 && args[0] == "show":
			if r, ok := cfg.beadShow[args[1]]; ok {
				return r
			}
			return run.Result{Code: 1, Stdout: `{"error":"no issues found matching the provided IDs"}`}
		case name == "bd" && len(args) >= 2 && args[0] == "epic" && args[1] == "status":
			return run.Result{Stdout: cfg.epicStatus, Code: 0}
		case name == "bd" && len(args) >= 1 && args[0] == "list":
			return run.Result{Stdout: cfg.inProgress, Code: 0}
		case name == "gh" && len(args) >= 3 && args[0] == "pr" && args[1] == "view":
			if r, ok := cfg.prView[args[2]]; ok {
				return r
			}
			return run.Result{Stdout: `{"state":"OPEN"}`, Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
}

// mkdirs creates each workspace directory under root+"_worktrees".
func mkdirs(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(root+"_worktrees", n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// --- Pass 1: workspace-side ------------------------------------------------

// orphan/bead-not-in-progress: workspace whose bead is closed, dir present.
func TestDoctorOrphanBeadNotInProgress(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "weft-hjx__1__1")
	fake := doctorFake(doctorCfg{
		root:       root,
		workspaces: "default\nweft-hjx__1__1\n",
		beadShow:   map[string]run.Result{"weft-hjx.1.1": {Stdout: `[{"status":"closed"}]`, Code: 0}},
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0 even with findings, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"category": "orphan"`) || !strings.Contains(s, `"reason": "bead-not-in-progress"`) {
		t.Errorf("want orphan/bead-not-in-progress finding, got: %s", s)
	}
	if !strings.Contains(s, `"suggest": "weft reap"`) {
		t.Errorf("orphan should suggest weft reap, got: %s", s)
	}
}

// foreign/no-bead: workspace resolving to no bead AND no dir under wtRoot.
// MUST NOT be classified orphan.
func TestDoctorForeignNoBead(t *testing.T) {
	root := t.TempDir()
	// deliberately do NOT create a dir for the foreign workspace
	fake := doctorFake(doctorCfg{
		root:       root,
		workspaces: "default\nworktree-agent-abc\n",
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"category": "foreign"`) || !strings.Contains(s, `"reason": "no-bead"`) {
		t.Errorf("want foreign/no-bead finding, got: %s", s)
	}
	if strings.Contains(s, `"category": "orphan"`) {
		t.Errorf("a bead-less workspace with no dir must be foreign, never orphan: %s", s)
	}
}

// orphan/bead-missing: bead missing BUT its dir EXISTS under wtRoot.
func TestDoctorOrphanBeadMissing(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "weft-hjx__1__1")
	fake := doctorFake(doctorCfg{
		root:       root,
		workspaces: "default\nweft-hjx__1__1\n",
		// bead show falls through to the default not-found payload → missing
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"category": "orphan"`) || !strings.Contains(s, `"reason": "bead-missing"`) {
		t.Errorf("want orphan/bead-missing finding, got: %s", s)
	}
}

// stray/stale-activity: in_progress bead, workspace present, liveness stale.
func TestDoctorStrayStaleActivity(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "weft-hjx__1__2")
	fake := doctorFake(doctorCfg{
		root:       root,
		workspaces: "default\nweft-hjx__1__2\n",
		beadShow:   map[string]run.Result{"weft-hjx.1.2": {Stdout: `[{"status":"in_progress"}]`, Code: 0}},
		stale:      map[string]bool{"weft-hjx__1__2": true},
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"category": "stray"`) || !strings.Contains(s, `"reason": "stale-activity"`) {
		t.Errorf("want stray/stale-activity finding, got: %s", s)
	}
}

// healthy: in_progress + fresh timestamp → NO finding.
func TestDoctorHealthyNoFindings(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "weft-hjx__1__2")
	fake := doctorFake(doctorCfg{
		root:       root,
		workspaces: "default\nweft-hjx__1__2\n",
		beadShow:   map[string]run.Result{"weft-hjx.1.2": {Stdout: `[{"status":"in_progress"}]`, Code: 0}},
		// not stale → fresh liveness → healthy
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"findings": []`) {
		t.Errorf("healthy warp must emit an empty findings array, got: %s", s)
	}
}

// --- Pass 1 × Pass 2 composites: a live workspace must NOT mask a bead whose
// sealed change already landed / conflicts (the dedup-vs-workspace bug). -----

// landed-unclosed must be reported even when the bead still has a live/fresh
// workspace — Pass 1 leaves it healthy (no finding) but Pass 2 must still catch
// the sealed change already in trunk().
func TestDoctorLandedUnclosedWithLiveWorkspace(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "weft-hjx__1__6")
	fake := doctorFake(doctorCfg{
		root:       root,
		workspaces: "default\nweft-hjx__1__6\n",
		beadShow:   map[string]run.Result{"weft-hjx.1.6": {Stdout: `[{"status":"in_progress"}]`, Code: 0}},
		// fresh liveness (not stale) → Pass 1 emits nothing for this workspace
		inProgress: `[{"id":"weft-hjx.1.6","labels":["jj-change:xyz999"]}]`,
		landed:     map[string]bool{"xyz999": true},
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"category": "stray"`) || !strings.Contains(s, `"reason": "landed-unclosed"`) {
		t.Errorf("a live workspace must not mask a landed-unclosed bead, got: %s", s)
	}
	if n := strings.Count(s, `"category"`); n != 1 {
		t.Errorf("want exactly one finding, got %d: %s", n, s)
	}
}

// conflicted must likewise be reported despite a live/fresh workspace.
func TestDoctorConflictedWithLiveWorkspace(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "weft-hjx__1__6")
	fake := doctorFake(doctorCfg{
		root:       root,
		workspaces: "default\nweft-hjx__1__6\n",
		beadShow:   map[string]run.Result{"weft-hjx.1.6": {Stdout: `[{"status":"in_progress"}]`, Code: 0}},
		inProgress: `[{"id":"weft-hjx.1.6","labels":["jj-change:xyz999"]}]`,
		conflicted: map[string]bool{"xyz999": true},
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"category": "conflicted"`) || !strings.Contains(s, `"reason": "change-conflicted"`) {
		t.Errorf("a live workspace must not mask a conflicted bead, got: %s", s)
	}
	if n := strings.Count(s, `"category"`); n != 1 {
		t.Errorf("want exactly one finding, got %d: %s", n, s)
	}
}

// A live workspace whose sealed change is neither landed nor conflicted stays
// healthy through BOTH passes — no finding (the negative that guards against
// over-reporting after the dedup split).
func TestDoctorLiveWorkspaceBenignChangeNoFinding(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "weft-hjx__1__8")
	fake := doctorFake(doctorCfg{
		root:       root,
		workspaces: "default\nweft-hjx__1__8\n",
		beadShow:   map[string]run.Result{"weft-hjx.1.8": {Stdout: `[{"status":"in_progress"}]`, Code: 0}},
		inProgress: `[{"id":"weft-hjx.1.8","labels":["jj-change:aaa000"]}]`,
		// aaa000 neither landed nor conflicted, workspace present + fresh → healthy
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"findings": []`) {
		t.Errorf("a live workspace with a benign change must yield no finding, got: %s", s)
	}
}

// A Pass-1 stray/stale-activity bead must NOT be reported a second time by Pass
// 2, even when its sealed change is also in trunk() — seenStray dedups it.
func TestDoctorStrayNotDoubleReported(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "weft-hjx__1__7")
	fake := doctorFake(doctorCfg{
		root:       root,
		workspaces: "default\nweft-hjx__1__7\n",
		beadShow:   map[string]run.Result{"weft-hjx.1.7": {Stdout: `[{"status":"in_progress"}]`, Code: 0}},
		stale:      map[string]bool{"weft-hjx__1__7": true},
		inProgress: `[{"id":"weft-hjx.1.7","labels":["jj-change:www777"]}]`,
		landed:     map[string]bool{"www777": true},
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"reason": "stale-activity"`) {
		t.Errorf("want the Pass-1 stray/stale-activity finding, got: %s", s)
	}
	if strings.Contains(s, `"reason": "landed-unclosed"`) {
		t.Errorf("a Pass-1 stray must not be re-reported by Pass 2, got: %s", s)
	}
	if n := strings.Count(s, `"category"`); n != 1 {
		t.Errorf("want exactly one finding (no double-report), got %d: %s", n, s)
	}
}

// --- Pass 2: bead-side -----------------------------------------------------

// stray/landed-unclosed: in_progress bead carrying a jj-change already in trunk.
func TestDoctorStrayLandedUnclosed(t *testing.T) {
	root := t.TempDir()
	fake := doctorFake(doctorCfg{
		root:       root,
		inProgress: `[{"id":"weft-hjx.1.3","labels":["jj-change:abc123"]}]`,
		landed:     map[string]bool{"abc123": true},
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"category": "stray"`) || !strings.Contains(s, `"reason": "landed-unclosed"`) {
		t.Errorf("want stray/landed-unclosed finding, got: %s", s)
	}
	if !strings.Contains(s, `"suggest": "bd close weft-hjx.1.3"`) {
		t.Errorf("landed-unclosed should suggest bd close, got: %s", s)
	}
}

// lost/workspace-missing: in_progress bead, no workspace, change not in trunk.
func TestDoctorLostWorkspaceMissing(t *testing.T) {
	root := t.TempDir()
	fake := doctorFake(doctorCfg{
		root:       root,
		inProgress: `[{"id":"weft-hjx.1.4","labels":["jj-change:def456"]}]`,
		// def456 neither landed nor conflicted, and no workspace → lost
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"category": "lost"`) || !strings.Contains(s, `"reason": "workspace-missing"`) {
		t.Errorf("want lost/workspace-missing finding, got: %s", s)
	}
}

// conflicted/change-conflicted: sealed change in conflicts().
func TestDoctorConflictedChange(t *testing.T) {
	root := t.TempDir()
	fake := doctorFake(doctorCfg{
		root:       root,
		inProgress: `[{"id":"weft-hjx.1.5","labels":["jj-change:ghi789"]}]`,
		conflicted: map[string]bool{"ghi789": true},
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"category": "conflicted"`) || !strings.Contains(s, `"reason": "change-conflicted"`) {
		t.Errorf("want conflicted/change-conflicted finding, got: %s", s)
	}
	if !strings.Contains(s, `"suggest": "weft conflict open weft-hjx.1.5"`) {
		t.Errorf("conflicted should suggest weft conflict open, got: %s", s)
	}
}

// --- Pass 3: epic-side -----------------------------------------------------

// unreconciled/pr-merged-local-remains: open epic, bookmark present, PR MERGED.
func TestDoctorUnreconciledPRMerged(t *testing.T) {
	root := t.TempDir()
	fake := doctorFake(doctorCfg{
		root:       root,
		epicStatus: `[{"epic":{"id":"weft-hjx","status":"open"}}]`,
		bookmarks:  map[string]string{"weft-hjx": "weft-hjx: qp 12ab (empty) description\n"},
		prView:     map[string]run.Result{"weft-hjx": {Stdout: `{"state":"MERGED"}`, Code: 0}},
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"category": "unreconciled"`) || !strings.Contains(s, `"reason": "pr-merged-local-remains"`) {
		t.Errorf("want unreconciled/pr-merged-local-remains finding, got: %s", s)
	}
	if !strings.Contains(s, `"suggest": "weft finish reconcile weft-hjx"`) {
		t.Errorf("unreconciled should suggest weft finish reconcile, got: %s", s)
	}
}

// No unreconciled finding when the epic's bookmark is absent (nothing local
// remains), even if a PR would report MERGED — bookmark presence gates the gh call.
func TestDoctorNoUnreconciledWhenBookmarkAbsent(t *testing.T) {
	root := t.TempDir()
	fake := doctorFake(doctorCfg{
		root:       root,
		epicStatus: `[{"epic":{"id":"weft-hjx","status":"open"}}]`,
		// no bookmark entry → absent
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if strings.Contains(s, `"category": "unreconciled"`) {
		t.Errorf("no local bookmark → no unreconciled finding, got: %s", s)
	}
	if !strings.Contains(s, `"findings": []`) {
		t.Errorf("want empty findings, got: %s", s)
	}
}

// --- Envelope / degradation / scoping --------------------------------------

// gh degradation: gh pr view exits non-zero → a warning is recorded, doctor
// still exits 0, and unrelated findings are still emitted.
func TestDoctorGHDegradesToWarning(t *testing.T) {
	root := t.TempDir()
	fake := doctorFake(doctorCfg{
		root:       root,
		epicStatus: `[{"epic":{"id":"weft-hjx","status":"open"}}]`,
		bookmarks:  map[string]string{"weft-hjx": "weft-hjx: qp 12ab\n"},
		prView:     map[string]run.Result{"weft-hjx": {Stderr: "gh: could not connect", Code: 1}},
		// an independent pass-2 finding must still surface
		inProgress: `[{"id":"weft-hjx.1.4","labels":[]}]`,
	})
	out, err := newTestCmd(fake, "doctor", "--json")
	if err != nil {
		t.Fatalf("gh failure must not abort doctor (exit 0), got err=%v", err)
	}
	s := out.String()
	if strings.Contains(s, `"warnings": []`) {
		t.Errorf("gh failure should populate warnings, got: %s", s)
	}
	if !strings.Contains(s, `"category": "lost"`) {
		t.Errorf("independent findings must still be emitted after gh degrades, got: %s", s)
	}
	if strings.Contains(s, `"category": "unreconciled"`) {
		t.Errorf("an unknown PR state must not yield an unreconciled finding, got: %s", s)
	}
}

// --epic scoping: only beads under the epic's dotted prefix are joined.
func TestDoctorEpicScopeFilter(t *testing.T) {
	root := t.TempDir()
	fake := doctorFake(doctorCfg{
		root: root,
		inProgress: `[{"id":"weft-hjx.1","labels":[]},` +
			`{"id":"weft-zzz.9","labels":[]}]`,
	})
	out, err := newTestCmd(fake, "doctor", "--epic", "weft-hjx", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, "weft-hjx.1") {
		t.Errorf("in-scope bead should be joined, got: %s", s)
	}
	if strings.Contains(s, "weft-zzz.9") {
		t.Errorf("out-of-scope bead must be excluded by --epic, got: %s", s)
	}
}

// envelope shape: a zero-finding run emits findings:[] and warnings:[] — both
// present and non-null (seam 9 discipline).
func TestDoctorEnvelopeAlwaysInitialized(t *testing.T) {
	root := t.TempDir()
	out, err := newTestCmd(doctorFake(doctorCfg{root: root}), "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"findings": []`) {
		t.Errorf("findings must be an initialized [] , got: %s", s)
	}
	if !strings.Contains(s, `"warnings": []`) {
		t.Errorf("warnings must be an initialized [] , got: %s", s)
	}
	if strings.Contains(s, "null") {
		t.Errorf("envelope must never carry null, got: %s", s)
	}
}

// bd/jj infrastructure failures keep the fail-safe exit-2 posture.
func TestDoctorWorkspaceListFailureIsHardExit2(t *testing.T) {
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "jj" && len(args) >= 2 && args[1] == "root" {
			return run.Result{Stdout: "/repo/weft", Code: 0}
		}
		if name == "jj" && len(args) >= 2 && args[1] == "workspace" {
			return run.Result{Code: 1, Stderr: "jj: not a workspace"}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(fake, "doctor")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("jj workspace list failure must hard-fail (exit 2), got %d (err=%v)", got, err)
	}
}

func TestDoctorBDListFailureIsHardExit2(t *testing.T) {
	root := t.TempDir()
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case name == "jj" && len(args) >= 2 && args[1] == "root":
			return run.Result{Stdout: root, Code: 0}
		case name == "jj" && strings.Contains(j, "workspace list"):
			return run.Result{Stdout: "default\n", Code: 0}
		case name == "bd" && len(args) >= 1 && args[0] == "list":
			return run.Result{Code: 1, Stderr: "bd: dial tcp: connection refused"}
		default:
			return run.Result{Code: 0}
		}
	}}
	_, err := newTestCmd(fake, "doctor")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("bd list failure must hard-fail (exit 2), got %d (err=%v)", got, err)
	}
}

// Text (non-JSON) output stays terse and healthy when the warp is clean.
func TestDoctorTextHealthy(t *testing.T) {
	root := t.TempDir()
	out, err := newTestCmd(doctorFake(doctorCfg{root: root}), "doctor")
	if err != nil {
		t.Fatalf("doctor must exit 0, got err=%v", err)
	}
	if !strings.Contains(out.String(), "warp healthy") {
		t.Errorf("clean warp should report healthy, got: %s", out.String())
	}
}
