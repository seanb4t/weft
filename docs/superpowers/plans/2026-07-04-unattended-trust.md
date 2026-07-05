<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Unattended-Trust Milestone Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make weft trustworthy unattended — a crashed executor, a stranded workspace, or a half-finished wave is detected and reported by the engine, not discovered by archaeology (roadmap §3).

**Architecture:** Seven picks under one epic (promoted from `weft-x38`). A new `internal/liveness` package infers executor liveness from existing jj/filesystem state; a new read-only `weft doctor` verb joins bead × workspace × change × PR into categorized findings with suggested recovery verbs; `reap` gains the seam-3 §5 liveness decision table plus a foreign-workspace guard; three hardening picks close the remaining seam-§8 "unattended goes wrong" edges (finish-reconcile topology re-verification, resolver oscillation bound, replan removed-pick enactment with the I2 never-drop-woven-work invariant).

**Tech Stack:** Go 1.26, cobra, injectable `run.Runner` (all subprocess calls mocked in unit tests), build-tag-`integration` E2E in `internal/weave/` against pinned `bd`+`jj` (CI job `integration`, `.github/workflows/ci.yml:139`).

**Spec:** `docs/superpowers/specs/2026-07-04-unattended-trust-design.md` (design-review READY round 2). Invariants I1–I4 and all decisions live there; this plan implements them.

---

## Task table (plan-to-beads source of truth)

| Task | Title | Priority | Blocks on | Model |
|---|---|---|---|---|
| 1 | `internal/liveness` — inferred executor recency signal | P2 | — | sonnet |
| 2 | `weft doctor` — whole-warp health join (report + propose) | P2 | Task 1 | opus |
| 3 | `reap` executor_live wiring + foreign-workspace guard + `--dry-run` | P2 | Task 1 | sonnet |
| 4 | `finish reconcile` topology re-verification vs forest + parked-escalated | P2 | — | opus |
| 5 | Resolver oscillation guard (`resolve-attempts:<n>` cap) | P3 | — | sonnet |
| 6 | Replan removed-pick enactment + I2 guard | P3 | — | sonnet |
| 7 | Milestone exit test — doctor + reap E2E (roadmap §7.4) | P2 | Tasks 2, 3 | opus |

Wave shape: {1, 4, 5, 6} → {2, 3} → {7}.

## File Structure

```
internal/liveness/liveness.go        (new)  LastActivity + Live — the one liveness question
internal/liveness/liveness_test.go   (new)
internal/cli/doctor.go               (new)  weft doctor verb — join + classify, no mutation
internal/cli/doctor_test.go          (new)
internal/cli/reap.go                 (mod)  liveness decision table + foreign guard + --dry-run
internal/cli/reap_test.go            (mod)
internal/cli/conflict.go             (mod)  attempt counter read/increment/clear + cap escalation
internal/cli/conflict_test.go        (mod)
internal/cli/plan.go                 (mod)  enact removals; hard-fail removed_blocked (I2)
internal/cli/plan_test.go            (mod)
internal/cli/finish.go               (mod)  only if Task 4's tests force a rebase-scoping fix
internal/cli/root.go                 (mod)  register newDoctorCmd (line ~51, after newReapCmd)
internal/config/config.go            (mod)  [liveness] threshold, [conflict] max_resolve_attempts
internal/config/config_test.go       (mod)
internal/plan/emit.go                (mod)  BuildReplan: RemovedBlocked classification
internal/plan/emit_test.go           (mod)
internal/weave/finish_topology_test.go (mod) integration: forest + parked-escalated reconcile
internal/weave/doctor_reap_e2e_test.go (new) integration: the milestone exit test
```

Existing symbols reused (verified signatures): `workspace.Resolve(name) (beadID string, kind Kind)` (`internal/workspace/workspace.go:44`), `workspace.Root(jjRoot, cfgRoot)` (`:54`), `workspace.ContainsResolved(parent, child)` (`:100`), `beadStatus(r, bead)` (`internal/cli/reap.go:100`), `changeFromLabels` (`internal/cli/bead.go:57`), `changeIDPattern` (`internal/cli/conflict.go:23`), `Emit(cmd, verb, data, text)`, `run.JJ/BD/GH`, `exit.Hardf/Invocationf`, test fakes `routeRunner`/`errRunner`/`newTestCmd` (package-scope test helpers defined in `internal/cli/version_test.go` and `internal/cli/shed_test.go`, usable from any `cli` test file).

Verified CLI contracts: `bd list --status in_progress --json` (global, no `--parent`); `bd close <id> -r "<reason>"`; `bd update <id> --add-label X --remove-label Y`; `bd supersede` REQUIRES `--with <new>` (hence close-with-reason for successor-less removals). Verified jj facts (2026-07-04, jj 0.43): per-workspace snapshot refreshes `<name>@`'s committer timestamp (idle workspace showed 12-day-old timestamp; active showed minutes); `jj workspace list -T` has NO `path` keyword (workspace paths must come from the `wtRoot/name` join); op templates expose no workspace attribution.

---

### Task 1: `internal/liveness` — inferred executor recency signal

**Files:**

- Create: `internal/liveness/liveness.go`
- Create: `internal/liveness/liveness_test.go`
- Modify: `internal/config/config.go` (add `[liveness]` block)
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/liveness/liveness_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package liveness

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/seanb4t/weft/internal/run"
)

// fakeRunner routes jj calls; mirrors internal/cli's routeRunner shape
// (that type is package-private to cli, so liveness has its own).
type fakeRunner struct{ fn func(name string, args []string) run.Result }

func (f *fakeRunner) Run(name string, args ...string) (run.Result, error) {
	return f.fn(name, args), nil
}

const tsLayout = "2006-01-02T15:04:05-0700"

func jjTimestampRunner(ts string) *fakeRunner {
	return &fakeRunner{fn: func(name string, args []string) run.Result {
		return run.Result{Stdout: ts + "\n", Code: 0}
	}}
}

func TestLastActivityUsesWorkspaceCommitTimestamp(t *testing.T) {
	// No directory on disk: the jj signal alone carries the answer.
	r := jjTimestampRunner("2026-07-01T10:00:00-0400")
	got, err := LastActivity(r, "weft-abc__1", filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := time.Parse(tsLayout, "2026-07-01T10:00:00-0400")
	if !got.Equal(want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestLastActivityNewerMtimeWins(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "edited.go")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	newer := time.Now().Add(-1 * time.Minute)
	if err := os.Chtimes(f, newer, newer); err != nil {
		t.Fatal(err)
	}
	r := jjTimestampRunner("2026-01-01T00:00:00+0000") // stale jj signal
	got, err := LastActivity(r, "weft-abc__1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sub(newer).Abs() > 2*time.Second {
		t.Errorf("mtime should win: got %v want ~%v", got, newer)
	}
}

func TestLastActivityIgnoresDotJJ(t *testing.T) {
	dir := t.TempDir()
	jjdir := filepath.Join(dir, ".jj", "working_copy")
	if err := os.MkdirAll(jjdir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(jjdir, "checkout")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// .jj mtime is NOW; the jj signal is old — .jj must not count as activity.
	r := jjTimestampRunner("2026-01-01T00:00:00+0000")
	got, err := LastActivity(r, "weft-abc__1", dir)
	if err != nil {
		t.Fatal(err)
	}
	old, _ := time.Parse(tsLayout, "2026-01-01T00:00:00+0000")
	if !got.Equal(old) {
		t.Errorf(".jj contents must be ignored: got %v want %v", got, old)
	}
}

func TestLastActivityRejectsUnsafeWorkspaceName(t *testing.T) {
	r := jjTimestampRunner("2026-01-01T00:00:00+0000")
	if _, err := LastActivity(r, "bad name & rev", t.TempDir()); err == nil {
		t.Fatal("unsafe workspace name must not reach a revset")
	}
}

func TestLastActivityJJFailureIsError(t *testing.T) {
	r := &fakeRunner{fn: func(string, []string) run.Result {
		return run.Result{Stderr: "boom", Code: 1}
	}}
	if _, err := LastActivity(r, "weft-abc__1", t.TempDir()); err == nil {
		t.Fatal("jj failure is an infrastructure anomaly, not silence")
	}
}

func TestLive(t *testing.T) {
	now := time.Now()
	if !Live(now.Add(-10*time.Minute), now, 45*time.Minute) {
		t.Error("10m ago within 45m threshold must be live")
	}
	if Live(now.Add(-2*time.Hour), now, 45*time.Minute) {
		t.Error("2h ago beyond 45m threshold must be dead")
	}
}
```

Config test (append to `internal/config/config_test.go`, matching its existing table style — read the file first and follow its conventions):

```go
func TestLivenessThresholdDefaultAndParse(t *testing.T) {
	var c Config
	d, err := c.LivenessThreshold()
	if err != nil || d != 45*time.Minute {
		t.Errorf("unset threshold: got %v, %v; want 45m, nil", d, err)
	}
	c.Liveness.Threshold = "90m"
	d, err = c.LivenessThreshold()
	if err != nil || d != 90*time.Minute {
		t.Errorf("90m: got %v, %v", d, err)
	}
	c.Liveness.Threshold = "not-a-duration"
	if _, err = c.LivenessThreshold(); err == nil {
		t.Error("malformed threshold must error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/liveness/ ./internal/config/`
Expected: FAIL — package `liveness` does not exist; `LivenessThreshold` undefined.

- [ ] **Step 3: Implement**

```go
// internal/liveness/liveness.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package liveness answers one question from existing state only: when was
// this workspace last worked? No PID files, no heartbeats — the engine never
// spawns agents, so it trusts only signals it can observe (spec: decision 3).
package liveness

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/seanb4t/weft/internal/run"
)

// wsNamePattern matches a Sanitize()d workspace name ([a-z0-9_-]); it excludes
// every revset metacharacter, so a name cannot alter revset evaluation when
// interpolated as <name>@ (same rationale as cli's workspaceRevPattern).
var wsNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// tsLayout parses the explicit strftime format requested in the template
// below — locale-independent, unlike jj's default timestamp rendering.
const tsLayout = "2006-01-02T15:04:05-0700"

// LastActivity returns the most recent evidence of executor work in a
// workspace: the committer timestamp of the workspace's working-copy commit
// (jj refreshes it on every per-workspace snapshot, i.e. every jj command run
// there) joined with the newest file mtime under the workspace directory
// (guards the edited-files-but-ran-no-jj-command window). The .jj directory
// is excluded from the walk — jj's own bookkeeping is not executor activity.
// A missing/unwalkable directory contributes nothing (the jj signal stands);
// a failing jj call is an infrastructure anomaly and errors — callers decide
// policy (reap hard-fails; doctor may degrade).
func LastActivity(r run.Runner, wsName, wsDir string) (time.Time, error) {
	if !wsNamePattern.MatchString(wsName) {
		return time.Time{}, fmt.Errorf("refusing to interpolate unsafe workspace name %q into a revset", wsName)
	}
	res, err := run.JJ(r, "log", "--no-graph", "-r", wsName+"@",
		"-T", `committer.timestamp().format("%Y-%m-%dT%H:%M:%S%z") ++ "\n"`)
	if err != nil {
		return time.Time{}, fmt.Errorf("jj log %s@ could not run: %w", wsName, err)
	}
	if res.Code != 0 {
		return time.Time{}, fmt.Errorf("jj log %s@ failed: %s", wsName, strings.TrimSpace(res.Stderr))
	}
	last, err := time.Parse(tsLayout, strings.TrimSpace(res.Stdout))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse jj timestamp %q: %w", strings.TrimSpace(res.Stdout), err)
	}
	// Join with the newest mtime under the workspace dir (best-effort walk).
	_ = filepath.WalkDir(wsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries contribute nothing
		}
		if d.IsDir() && d.Name() == ".jj" {
			return filepath.SkipDir
		}
		if info, ierr := d.Info(); ierr == nil && info.ModTime().After(last) {
			last = info.ModTime()
		}
		return nil
	})
	return last, nil
}

// Live reports whether activity at t is within threshold of now.
func Live(t, now time.Time, threshold time.Duration) bool {
	return now.Sub(t) <= threshold
}
```

Config additions (`internal/config/config.go` — add to the `Config` struct after the `Plan` block, plus the accessor; import `time`):

```go
	Liveness struct {
		Threshold string `toml:"threshold"`
	} `toml:"liveness"`
```

```go
// LivenessThreshold returns the [liveness] threshold, defaulting to 45m when
// unset. Conservative by design: a thinking-but-quiet executor can look dead;
// the cost is bounded because reap runs at orchestrator startup/resume, not
// mid-wave (seam 3 §5.1).
// (Value receiver — matches the file's existing accessor convention.)
func (c Config) LivenessThreshold() (time.Duration, error) {
	if c.Liveness.Threshold == "" {
		return 45 * time.Minute, nil
	}
	return time.ParseDuration(c.Liveness.Threshold)
}
```

Check how `run.Runner`'s interface is declared (`internal/run`) before writing the fake — the test's `fakeRunner` must satisfy it exactly (if the real interface method differs from `Run(name string, args ...string)`, mirror the real one).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/liveness/ ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(liveness): inferred executor recency signal (workspace @ timestamp + mtime join)"`

**Acceptance:** package exists with the two functions; unsafe names rejected; `.jj` excluded; jj failure errors; config default 45m; all tests green.

---

### Task 2: `weft doctor` — whole-warp health join

**Files:**

- Create: `internal/cli/doctor.go`
- Create: `internal/cli/doctor_test.go`
- Modify: `internal/cli/root.go` (register after `newReapCmd()`, line ~51)

**Contract (spec Component 2, invariant I1):** read-only; exit 0 even with findings; gh best-effort (warning, never abort); every finding `{category, reason, bead, workspace, change, evidence, suggest}`; envelope keys `findings` and `warnings` always `[]`-initialized.

- [ ] **Step 1: Write the failing tests**

Follow `reap_test.go`'s `routeRunner`/`newTestCmd` conventions. One test per category plus envelope/degradation tests:

```go
// internal/cli/doctor_test.go — representative cases (write all of these)

// orphan: workspace whose bead is closed → finding{category:orphan,
// reason:bead-not-in-progress, suggest:weft reap}. Mock: workspace list
// returns "weft-hjx__1__1"; bd show → [{"status":"closed"}]. Dir exists
// under wtRoot (create with os.MkdirAll as reap_test.go does).

// foreign: workspace "worktree-agent-abc" → bd show says no issue AND no dir
// under wtRoot → finding{category:foreign, reason:no-bead, suggest:manual
// sweep}. MUST NOT be classified orphan.

// orphan(bead-missing): bd show says no issue BUT dir EXISTS under wtRoot →
// finding{category:orphan, reason:bead-missing}.

// stray(stale-activity): bead in_progress, workspace present, liveness stale
// (mock jj log -r 'name@' returning an old timestamp; threshold from config
// default). finding{category:stray, reason:stale-activity}.

// healthy: in_progress + fresh timestamp → NO finding (findings==[]).

// stray(landed-unclosed): in_progress bead carrying jj-change:abc123 whose
// change is in ::trunk() (mock the revset call non-empty) → finding{category:
// stray, reason:landed-unclosed, suggest:bd close}.

// lost: in_progress bead with NO workspace and change NOT in trunk →
// finding{category:lost, reason:workspace-missing}.

// conflicted: sealed change in conflicts() (mock conflicts() & <ch> revset
// non-empty) → finding{category:conflicted, suggest:weft conflict open <bead>}.

// unreconciled: open epic (bd epic status --json) whose bookmark exists and
// gh pr view says MERGED → finding{category:unreconciled, suggest:weft finish
// reconcile <epic>}.

// gh degradation: gh pr view exits 1 → warnings[] carries one entry; command
// still exits 0 and other findings still emitted.

// --epic scoping: findings restricted to beads with the epic prefix
// (same dotted-prefix rule as reap.go:46).

// envelope shape: zero-finding run emits findings:[] and warnings:[] (both
// present, non-null) — seam 9 discipline.

// jj/bd infrastructure failure (workspace list exits 1, or bd list exits 1)
// → exit 2 hard error (doctor's bd/jj side keeps the fail-safe posture).
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run TestDoctor -v`
Expected: FAIL — `newDoctorCmd` undefined.

- [ ] **Step 3: Implement `internal/cli/doctor.go`**

Skeleton (the finding struct and classification order are the load-bearing parts; helpers `beadStatus`, `workspace.Resolve`, `changeIDPattern` already exist):

```go
// internal/cli/doctor.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

// finding is one doctor diagnosis. reason is machine-readable and
// category-scoped (spec Component 2): stray → stale-activity|landed-unclosed;
// orphan → bead-not-in-progress|bead-missing; lost → workspace-missing;
// conflicted → change-conflicted; unreconciled → pr-merged-local-remains;
// foreign → no-bead.
type finding struct {
	Category  string `json:"category"`
	Reason    string `json:"reason"`
	Bead      string `json:"bead,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Change    string `json:"change,omitempty"`
	Evidence  string `json:"evidence"`
	Suggest   string `json:"suggest"`
}

func (a *App) newDoctorCmd() *cobra.Command {
	var epic string
	c := &cobra.Command{
		Use:   "doctor",
		Short: "Whole-warp health: join beads × workspaces × changes × PRs; report, never mutate (spec I1)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			findings := []finding{}
			warnings := []string{}
			threshold, err := a.Config.LivenessThreshold()
			if err != nil {
				return exit.Invocationf("[liveness] threshold: %v", err)
			}
			// Pass 1 — workspace-side (reap's join, report-only):
			//   for each non-default workspace: Resolve → beadStatus →
			//   missing + no dir under wtRoot → foreign
			//   missing + dir exists       → orphan/bead-missing
			//   not in_progress            → orphan/bead-not-in-progress
			//   in_progress                → liveness.LastActivity; stale → stray/stale-activity
			// Pass 2 — bead-side (global bd list --status in_progress --json,
			// labels hydrated): for each in_progress bead (dedup pass-1 strays):
			//   sealed change in ::trunk()   → stray/landed-unclosed
			//   sealed change in conflicts() → conflicted/change-conflicted
			//   no workspace + not landed    → lost/workspace-missing
			// Pass 3 — epic-side (bd epic status --json → open epics; for each,
			// jj bookmark list <epic> present + gh pr view <epic> MERGED →
			// unreconciled/pr-merged-local-remains). EVERY gh error → append
			// warnings, continue (deleteRemoteBranch posture, finish.go:403).
			// --epic: same dotted-prefix scope rule as reap.go:46.
			data := map[string]any{"findings": findings, "warnings": warnings}
			return Emit(cmd, "doctor", data, doctorSummary(findings, warnings))
		},
	}
	c.Flags().StringVar(&epic, "epic", "", "scope the join to descendants of this epic")
	return c
}
```

Implementation notes (all verified against the codebase):

- Change-in-trunk check: `jj log -r '<change> & ::trunk()'` non-empty, with
  `changeIDPattern` validation before interpolation (the `resume.go:119`
  pattern). Conflict check: reuse `changeConflicted(a.Runner, change)`.
- bd side: `bd list --status in_progress --json` parses into the same
  `{id,title,labels,status}` shape as `closedPicks` (`finish.go:62`); DO NOT
  use `--skip-labels` — the `jj-change:` labels are the join key.
- Liveness: `liveness.LastActivity(a.Runner, name, filepath.Join(wtRoot, name))`
  + `liveness.Live(t, time.Now(), threshold)`. A liveness *error* on a
  workspace is a hard error (jj is local infrastructure, not best-effort).
- `doctorSummary`: one line per finding — `"<category>(<reason>) <bead|workspace>: <evidence> → <suggest>"` —
  plus a header count line; "warp healthy — no findings" when empty.
- Register in `root.go`: `root.AddCommand(app.newDoctorCmd())` after line 51.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestDoctor -v` then `go test ./...`
Expected: PASS, no regressions.

- [ ] **Step 5: Commit**

`jj commit -m "feat(doctor): whole-warp health join — report + propose, never mutate (seam 7 / roadmap §3)"`

**Acceptance:** all category tests green; exit 0 with findings; gh failure degrades to warning; bd/jj failure hard-fails; envelope always carries `findings`/`warnings`.

---

### Task 3: `reap` executor_live wiring + foreign-workspace guard + `--dry-run`

**Files:**

- Modify: `internal/cli/reap.go` (the decision block at lines 53-61 and the loop around it)
- Modify: `internal/cli/reap_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `reap_test.go`. Existing tests must keep passing: `:17` (`TestReapCollectsNonInProgressWorkspaces`) has an in_progress-kept workspace, so its mock gains a fresh-timestamp answer for the `jj log -r '<name>@'` liveness call; `:77` (`TestReapMalformedBeadJSONIsHardFailureAndPreservesWorkspace`) hard-fails on malformed `bd show` JSON before the liveness branch is ever reached and needs no mock change.

```go
// TestReapCollectsCrashedExecutor: in_progress bead, workspace dir exists,
// jj log -r 'name@' mocked to a timestamp older than the 45m default →
// workspace reaped (forget called, dir removed, bead in reaped[]).

// TestReapKeepsBusyExecutor: in_progress + fresh timestamp → kept (this is
// the current TestReapCollectsNonInProgressWorkspaces live-dir assertion,
// now passing through the liveness gate — update that test's mock to answer
// the jj log 'name@' call with a fresh timestamp).

// TestReapSkipsForeignWorkspace: workspace "worktree-agent-abc" (no such
// bead, per bd show "no issues found" payload) AND no dir under wtRoot →
// NOT forgotten (assert no `workspace forget worktree-agent-abc` call),
// reported in envelope foreign[].

// TestReapStillReapsBeadlessDirUnderRoot: bd show says no issue BUT the dir
// exists under wtRoot → reaped (a genuine weft orphan whose bead was
// deleted) — the existing missing-bead semantic, now dir-gated.

// TestReapDryRunMutatesNothing: --dry-run with one reapable orphan → envelope
// {would_reap:[...], reaped:[], foreign:[], dry_run:true}; no forget call,
// dir still on disk.
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run TestReap -v`
Expected: new tests FAIL (no liveness gate, no foreign guard, no --dry-run).

- [ ] **Step 3: Implement**

Replace `reap.go:53-61` (the `if status == "in_progress" { continue }` block) with the decision table; thread `--dry-run`; initialize `foreign := []string{}` and `wouldReap := []string{}`. NOTE: `reap.go:65` already declares `dir := filepath.Join(wtRoot, name)` later in the same loop body — **move that declaration up** to feed the new checks and delete the later `:=` (a second `:=` of `dir` in the same block is a compile error):

```go
			dir := filepath.Join(wtRoot, name) // moved up from reap.go:65 — sole declaration
			_, statErr := os.Stat(dir)
			dirExists := statErr == nil
			if status == "" && !dirExists {
				// Foreign: resolves to no bead AND lives outside the worktrees
				// root (weft creates every workspace it owns under wtRoot).
				// Forgetting it would break whoever owns it (e.g. a Claude Code
				// worktree-agent-* session). Doctor reports these; reap skips.
				foreign = append(foreign, name)
				continue
			}
			if status == "in_progress" {
				last, err := liveness.LastActivity(a.Runner, name, dir)
				if err != nil {
					return exit.Hardf("liveness probe for %s: %v", name, err)
				}
				if liveness.Live(last, time.Now(), threshold) {
					continue // busy — the seam 3 §5 authoritative guard
				}
				// crashed: in_progress but dead past threshold → fall through to reap
			}
```

(`threshold` from `a.Config.LivenessThreshold()` once before the loop, invocation-error on malformed — same as doctor.) The existing path-safety `ContainsResolved` guard and forget+RemoveAll mechanics stay untouched; on `--dry-run`, append to `wouldReap` and skip both mutations. Envelope: `{"reaped": reaped, "would_reap": wouldReap, "foreign": foreign, "dry_run": dryRun}` — all `[]`-initialized on every path (seam 9).

Update the stale v1 comment block (`reap.go:53-58`) — it documents the deferral this task removes.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestReap -v` then `go test ./...`
Expected: PASS, including the two pre-existing reap tests.

- [ ] **Step 5: Commit**

`jj commit -m "feat(reap): executor_live decision table + foreign-workspace guard + --dry-run (seam 3 §5/§10)"`

**Acceptance:** crashed reaped / busy kept / foreign skipped / beadless-dir-under-root reaped / dry-run inert; I3 holds; envelope keys stable.

---

### Task 4: `finish reconcile` topology re-verification

**Files:**

- Modify: `internal/weave/finish_topology_test.go` (integration tests; build tag `integration` — match the file's existing tag line and harness usage)
- Modify: `internal/cli/finish.go:488` (ONLY if the tests prove the drag)

This is a verification-first pick (spec Component 4): the tests are the deliverable; the code change is conditional on what they reveal.

**Scope — the delta only.** `internal/weave/finish_topology_test.go` ALREADY proves (landed 2026-06-09, bead `weft-8ou.7`): the merge-commit `jj rebase -b @ -o main --skip-emptied` leaves a **clean** parked sibling untouched (`TestReconcileMergeBranchLeavesParkedSiblingUntouched`, `:535` — which per its own `:531` comment "replays the literal jj rebase command rather than driving `weft finish`"), and real `weft finish open`'s `-r` collapse leaves an escalated trunk-sibling unmoved (`TestFinishOpenCollapseTopology`, `:266` — picks touch disjoint files, deliberately non-conflicting). Do NOT re-derive those. This task owns exactly two untested dimensions: (a) the parked escalated sibling being **actually conflicted**, and (b) the **real `weft finish reconcile` verb** driven end-to-end.

- [ ] **Step 1: Write the two integration tests (expected to characterize, possibly fail)**

Extend `finish_topology_test.go`, reusing its existing fixture/parser helpers (read `:13-100` first — it already has jj-log parsers tolerant of snapshot warnings and a gh-routing runner pattern for `weft finish open`):

```go
// TestReconcileVerbExcludesConflictedParkedSibling (tag: integration)
// Same base shape as TestReconcileMergeBranchLeavesParkedSiblingUntouched
// (:535) BUT: (a) the parked sibling E is made CONFLICTED (rebase a change
// with a colliding edit onto E's line, the conflict_proof_test.go recipe —
// jj records the conflict in-commit and proceeds), and (b) instead of
// replaying the raw jj command, run the real verb: `weft finish reconcile
// <epic>` with a runner that passes jj through and intercepts only
// `gh pr view` → {"state":"MERGED"} and `gh repo view`/`gh api` (the
// deleteRemoteBranch calls) → benign results.
// ASSERT: E's change-id unchanged, parent still the base commit, E still in
// conflicts(); no live change has E as parent; landed line emptied/absorbed;
// reconcile's envelope reports merge_style=merge_commit.

// TestFinishOpenCollapseWithConflictedEscalatedSibling (tag: integration)
// TestFinishOpenCollapseTopology (:266) with one change: the escalated pick's
// change is CONFLICTED (collide its file with a landed pick's edit before
// integrate). Drive real `weft finish open` and ASSERT the `-r` collapse
// (collapseClosedPicks, finish.go:90) re-parents/leaves the conflicted ESC on
// trunk() — still conflicted, excluded from the pushed line, NOT dragged
// (the untested half of seam 11 §7's second bullet).
```

- [ ] **Step 2: Run them**

Run: `go test -tags integration ./internal/weave/ -run 'TestReconcileVerbExcludes|TestFinishOpenCollapseWithConflicted' -v`
Expected: either PASS (the conflicted-sibling delta behaves like the proven clean-sibling case) or FAIL with the conflicted E dragged/moved.

- [ ] **Step 3 (conditional — only on FAIL): scope the merge-commit rebase**

Replace `finish.go:488`'s `jj rebase -b @ -o main --skip-emptied` with a rebase scoped to the collapsed line's root so a trunk-parked sibling can never be selected:

```go
			case mergeStyleMergeCommit:
				// Scope to @'s own line: -b @ selects every branch reachable
				// from the destination's merge base, which can drag a parked
				// escalated sibling (seam 11 §7). roots(trunk()..@) is exactly
				// the collapsed line's root(s) containing @.
				rootRes, err := run.JJ(a.Runner, "log", "-r", "roots(trunk()..@)", "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
				// ... hard-fail plumbing as the squash branch does (finish.go:499-505),
				// then for each root: run.JJ(a.Runner, "rebase", "-s", root, "-o", "main", "--skip-emptied")
```

CAUTION: `roots(trunk()..@)` in the default workspace can also select the parked sibling if it is a root of `trunk()..@`'s set — verify in the test whether E ∈ `trunk()..@` (it is, if E is a descendant of trunk and not of main). If so, scope tighter: `roots(trunk()..@) & ::@` (roots on @'s ancestry line only). The test, not this plan, decides the final revset — encode the passing revset with a comment citing the test.

- [ ] **Step 4: Re-run + full suite**

Run: `go test -tags integration ./internal/plan/ ./internal/weave/` and `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "test(finish): reconcile topology re-verification vs forest + parked-escalated (seam 11 §7)"` (add `fix(finish): scope merge-commit reconcile rebase to the collapsed line` as a separate commit if Step 3 fired).

**Acceptance:** both topology tests exist and pass; the seam 11 §7 risk is either disproven-by-test or fixed-and-proven; no unreviewed conflicted work can ship via reconcile.

---

### Task 5: Resolver oscillation guard

**Files:**

- Modify: `internal/cli/conflict.go` (open: counter read/increment + cap gate; finalize: clear on heal)
- Modify: `internal/cli/conflict_test.go`
- Modify: `internal/config/config.go` (`[conflict] max_resolve_attempts`, default 3)

- [ ] **Step 1: Write the failing tests**

```go
// TestConflictOpenIncrementsAttemptCounter: bead with no resolve-attempts:*
// label → open succeeds AND issues `bd update <bead> --add-label
// resolve-attempts:1`. With resolve-attempts:1 → increment issues
// --remove-label resolve-attempts:1 --add-label resolve-attempts:2.

// TestConflictOpenEscalatesAtCap: bead labelled resolve-attempts:3 (cap 3)
// → NO workspace add call; `bd update --add-label human` issued; envelope
// {escalated:true, attempts:3}; exit 0 (escalation is an outcome, not an
// error — same shape as finalize's gate, conflict.go:162-174).

// TestConflictOpenEscalatedEnvelopeOnNormalPath: non-escalated open emits
// escalated:false and attempts:<n> — both keys on both paths (seam 9).

// TestConflictFinalizeClearsCounterOnHeal: healed finalize issues
// --remove-label resolve-attempts:<n>. Escalated finalize does NOT clear.

// TestMaxResolveAttemptsConfig: default 3; [conflict] max_resolve_attempts=5
// honored; <1 rejected as invocation error (the bd-ready-limit-0 gotcha
// class: never let a cap silently invert to no-cap).
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run TestConflict -v`
Expected: new tests FAIL.

- [ ] **Step 3: Implement**

In `newConflictOpenCmd` (before the `changeConflicted` gate at `conflict.go:86`): read the bead's labels (a `labelsOf(r, bead)` helper mirroring `changeOf` — both parse `bd show <bead> --json`; extract to one shared reader if `changeOf` already fetches labels), parse `resolve-attempts:<n>` (strict `^resolve-attempts:([0-9]+)$`; unparseable value → treat as 0 and overwrite — a tampered label must not wedge resolution), then:

```go
			maxAttempts := a.Config.MaxResolveAttempts() // default 3; <1 → invocation error at config read
			if attempts >= maxAttempts {
				// Forced escalation (spec I4): same human-label mechanics as
				// finalize's still-conflicted gate.
				if res, err := run.BD(a.Runner, "update", bead, "--add-label", "human"); err != nil || res.Code != 0 {
					// hard-fail plumbing identical to conflict.go:163-167
				}
				data := map[string]any{"bead": bead, "change": change, "escalated": true,
					"attempts": attempts, "workspace": "", "path": ""}
				return Emit(cmd, "conflict.open", data, fmt.Sprintf(
					"escalated %s: %d resolve attempts exhausted (cap %d) — flagged `human`, change %s left for a person",
					bead, attempts, maxAttempts, change))
			}
			// increment BEFORE opening the workspace: a crash between open and
			// finalize must still count the attempt (crash-durable, spec Component 5)
```

Config field is `MaxResolveAttempts *int` under `[conflict]` — the pointer follows the `OverlapMax *int` precedent (`config.go:37`: "pointer: distinguishes unset from an explicit 0"), because a plain int cannot tell unset (Go zero value) from an explicit `max_resolve_attempts = 0`. Accessor: nil → default 3; explicit value < 1 → invocation error (the bd-ready-limit-0 gotcha class — a cap must never silently invert to no-cap). Finalize's healed path (after the squash + reap, before Emit) removes the counter label; best-effort is NOT acceptable here — hard-fail on bd error like every other bd write in the file.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestConflict -v` then `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(conflict): resolve-attempt cap with forced escalation (seam 4 §8, I4)"`

**Acceptance:** counter increments on open, clears on heal, survives crashes (label-durable), cap escalates with `human` + `escalated:true`, cap<1 rejected.

---

### Task 6: Replan removed-pick enactment + I2 guard

**Files:**

- Modify: `internal/plan/emit.go` (`BuildReplan`, `Replan` struct at :196-215)
- Modify: `internal/plan/emit_test.go` (extend `TestBuildReplanRemovedRefs`, :130)
- Modify: `internal/cli/plan.go` (`planReplan`, :237-345)
- Modify: `internal/cli/plan_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/plan/emit_test.go
// TestBuildReplanClassifiesRemovedByStatus: refToID carries ExistingBead
// statuses; a removed ref with status "open" lands in rp.Removed; statuses
// "in_progress" and "closed" land in rp.RemovedBlocked (both sorted).

// internal/cli/plan_test.go
// TestReplanEnactsRemovals: live replan with one removed open pick issues
// `bd close <id> -r "removed by replan of <epic> (was weft-ref:<ref>)"`
// after edge wiring; envelope carries removed:[ref] and removed_blocked:[].

// TestReplanHardFailsOnBlockedRemoval (I2): live replan where the plan drops
// an in_progress pick → exit 2 BEFORE any bd import call (assert no import
// in the runner's call log); error names the pick and its status.

// TestReplanDryRunReportsBlockedWithoutMutating: dry-run emits
// {removed:[...], removed_blocked:[...], dry_run:true}; no bd calls.

// TestReplanCloseFailureIsHard: bd close non-zero → exit 2 "warp is
// incomplete; investigate" (matches the surrounding planReplan posture).
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/plan/ ./internal/cli/ -run 'TestBuildReplan|TestReplan' -v`
Expected: FAIL — `RemovedBlocked` undefined; no close calls issued.

- [ ] **Step 3: Implement**

`internal/plan/emit.go`: add `RemovedBlocked []string` to `Replan` (`:196` block, `[]`-initialized at `:215`); in the removal loop (`:276`), branch on `refToID[ref].Status`:

```go
		switch existing.Status {
		case "in_progress", "closed":
			rp.RemovedBlocked = append(rp.RemovedBlocked, ref)
		default: // open (and any future pre-work status): removable
			rp.Removed = append(rp.Removed, ref)
		}
```

`internal/cli/plan.go` `planReplan`: immediately after `BuildReplan` (`:242`):

```go
	if !dryRun && len(rp.RemovedBlocked) > 0 {
		return exit.Hardf("re-plan drops %d pick(s) with woven or landed work (%s) — a plan cannot remove in_progress or closed picks (I2); supersede intent must be expressed against open picks only",
			len(rp.RemovedBlocked), strings.Join(rp.RemovedBlocked, ", "))
	}
```

Enactment after the DeferredEdges loop (`:336`), before the final envelope: for each `ref` in `rp.Removed`, resolve its id from the pre-import `existing` map and run `bd close <id> -r "removed by replan of <epic> (was weft-ref:<ref>)"`, hard-failing on error with the file's "warp is incomplete; investigate" posture. Add `"removed_blocked": rp.RemovedBlocked` to BOTH the dry-run (`:248`) and live (`:337`) envelopes. Update the stale doc comment at `plan.go:234-236` ("Removed-pick supersede remains a §8 open sub-seam") and `emit.go:196`'s "(supersede is §8)".

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/plan/ ./internal/cli/` then `go test ./...` and `go test -tags integration ./internal/plan/`
Expected: PASS (the integration replan test exercises the live path against pinned bd — confirm it still passes with the new close calls).

- [ ] **Step 5: Commit**

`jj commit -m "feat(plan): enact removed-pick closure on replan; hard-fail dropping woven work (seam 2 §8, I2)"`

**Acceptance:** open removals closed with audit reason; in_progress/closed removals abort live replans pre-import (I2); dry-run classifies both without mutating; envelopes carry `removed_blocked` on all paths.

---

### Task 7: Milestone exit test — doctor + reap E2E

**Files:**

- Create: `internal/weave/doctor_reap_e2e_test.go` (build tag `integration`, harness-based like `weave_integration_test.go`)

The roadmap §7.4 exit test: *kill an executor mid-pick and strand a workspace — one command surfaces both.*

- [ ] **Step 1: Write the E2E (expected to pass — components landed in Tasks 1-3)**

```go
// TestDoctorSurfacesCrashAndStrand (tag: integration)
// Fixture (harness helpers): temp colocated repo + bd db; epic with two
// picks; `weft shed isolate` both → two workspaces, both beads in_progress.
// - "Crash" pick A: leave its workspace untouched. Backdating jj state is
//   impractical, so liveness staleness is driven by config: write
//   .weft/config.toml with [liveness] threshold = "1ms" and sleep ~50ms —
//   both workspaces are now past threshold; then for the "busy" control,
//   assert the SAME warp under threshold = "24h" yields zero stray findings.
// - "Strand" pick B: bd close its bead, leave its workspace (the incomplete
//   shed-cleanup shape, seam 3 §5).
// Assertions:
// 1. ONE `weft doctor --json` run (threshold 1ms) reports BOTH:
//    stray/stale-activity for A's bead AND orphan/bead-not-in-progress for
//    B's workspace, each with a non-empty suggest. Exit code 0.
// 2. `weft reap --dry-run --json` (1ms) lists both in would_reap; nothing
//    forgotten (jj workspace list unchanged).
// 3. `weft reap --json` (1ms) reaps both; with threshold 24h A is kept
//    (busy) — run the 24h case FIRST, then the 1ms destructive case.
// 4. A foreign workspace (jj workspace add outside wtRoot with a non-bead
//    name) is reported by doctor as foreign/no-bead, skipped by reap, and
//    still registered afterward.
```

- [ ] **Step 2: Run it**

Run: `go test -tags integration ./internal/weave/ -run TestDoctorSurfacesCrashAndStrand -v`
Expected: PASS. Any failure here is a Task 1-3 bug — fix there, not by weakening this test.

- [ ] **Step 3: Wire into CI**

No workflow change needed — `.github/workflows/ci.yml:139` already runs `go test -tags integration ./internal/plan/ ./internal/weave/`. Verify the new test appears in the CI run's output on the PR.

- [ ] **Step 4: Commit**

`jj commit -m "test(weave): unattended-trust milestone exit test — doctor+reap E2E (roadmap §7.4)"`

**Acceptance:** the milestone's definition of done, executable: one doctor run surfaces a crashed executor and a stranded workspace with correct categories/suggestions; reap collects exactly the dead, keeps the busy, skips the foreign.

---

## Done criteria (milestone)

1. `go test ./...` and `go test -tags integration ./internal/plan/ ./internal/weave/` green.
2. The roadmap §7.4 exit test (Task 7) passes in CI.
3. Invariants hold by test: I1 (doctor mutates nothing — no bd/jj write calls in any doctor test's runner log), I2 (blocked removal aborts), I3 (dead reaped, busy kept), I4 (attempt cap escalates).
4. `reap.go`'s v1-deferral comment is gone; seam 3 §10's open question is closed by `internal/liveness`.

## Out of scope (per spec)

Doctor `--fix`; self-dogfood meta-loop; fovea onboard; golangci-lint / multi-arch; any change to spine / overlap forest / conflict-as-data vocabulary; non-Claude hosts.
<!-- adr-capture: sha256=353b41b4960bf0d2; session=38459612; ts=2026-07-04T20:16:46Z; adrs=weft-jcg,weft-qc0,weft-0pq -->
