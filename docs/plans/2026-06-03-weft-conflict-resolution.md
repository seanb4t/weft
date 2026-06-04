<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft Conflict-Resolution UX (Seam 4) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the seam-4 `weft conflict` verbs — `conflict open <bead>` (open a resolution workspace on a conflicted pick and emit the resolver brief) and `conflict finalize <bead>` (fold the resolver's marker edits into the conflicted change, heal descendants, reap the workspace, or escalate) — so the first-class jj conflicts that `shed integrate` tolerates (seam 2's deliberate path) get resolved post-hoc by a fresh-context resolver agent the engine brackets.

**Architecture:** Two coarse cobra verbs in a new `internal/cli/conflict.go` bracket the resolver agent's marker-editing — the engine owns the dangerous jj choreography (create resolution workspace at the conflicted change, squash the resolution in, reap), the agent owns the merge judgment. The engine reaches the resolution workspace's working copy by jj's `<workspace-name>@` revset addressing (no cwd plumbing). Reuses the already-built seam-1/3 substrate: `changeOf`/`showBead` (the jj-change spine), `conflictChanges` (resume.go), `jjRoot`, and the `workspace` package (whose `-resolve` kind + kind-aware `Resolve`/reaper already exist).

**Tech Stack:** Go 1.26, `github.com/spf13/cobra` v1.10.x, stdlib `os`/`strings`/`path/filepath`. Subprocesses: `jj`, `bd`.

**Spec:** `docs/seams/04-conflict-resolution.md` (design-reviewer READY round 2). Model §2; flow §3; the two verbs §4; resolution-workspace identity §4.1; marker style §5; escalation §6.

**What already exists (build on it, don't reinvent):**
- `internal/workspace`: `Sanitize`/`Desanitize`, `Name`, `Path`, `Root`, `Contains`, **`resolveSuffix` const + `Kind`/`KindResolve` + kind-aware `Resolve(name) → (bead, kind)`** (seam 4 §4.1's naming half is already landed; this plan adds only the `ResolveName`/`ResolvePath` constructors).
- `internal/cli/reap.go`: the reaper is **already kind-aware** (`workspace.Resolve(name)` strips `-resolve` → owning bead). Seam 4 §9's reaper reconciliation needs no further code.
- `internal/cli/pick.go`: `pick land` **already** asserts the change ∉ `conflicts()` before `bd close` (seam 4 §9's land-gate is landed).
- `internal/cli/bead.go`: `changeOf(r, bead)` (the bead's `jj-change:<id>` spine label, "" if unsealed), `showBead`, `jjChangeLabelPrefix`.
- `internal/cli/resume.go`: `conflictChanges(r) → ([]string, error)` (short change-ids in `conflicts()`, repo-wide).
- `internal/cli/ws.go`: `jjRoot(r) → (string, error)`.
- `internal/cli/shed.go`: `shed integrate` builds the linear stack and emits conflicts (enriched here in Task 2).
- Test fakes: `routeRunner{fn, calls}`, `newTestCmd(fake, args...)`, `runRoot(r, args...)` (in `internal/cli/*_test.go`).

**Grounded jj/bd contracts** (probed live this session; recorded as notes on `weft-hjx.4`):
- `jj workspace add <path> --name <name> -r <rev>` — `-r` is "a list of **parent** revisions for the working-copy commit of the newly created workspace", so `-r <conflicted-change>` makes the new workspace's `@` a **child** of the conflicted change (= "`jj new L` in a resolution workspace", §3).
- **`jj` addresses another workspace's working copy as `<workspace-name>@`** (verified: `jj log -r 'default@'` resolves). The engine runs from the default workspace and targets the resolution workspace's `@` as `<resolve-name>@` — no cwd support needed in `run.Runner`.
- `jj squash --from <rev> --into <rev>` moves changes from→into (default of each is `@`). `--from <resolve-name>@ --into <change>` folds the resolver's edits into the conflicted change (its parent), healing descendants via jj's auto-rebase + conflict-simplification (§2.1).
- `jj config set --repo ui.conflict-marker-style diff` is valid (`--user|--repo|--workspace` required; `ui.conflict-marker-style` is a known key). Repo-scoped here; per-workspace pinning is a §8 refinement.
- **Escalation = the `human` label, not a `bd human <id>` command.** `bd human` has only `list`/`respond`/`dismiss`/`stats`; a human-needed bead is one "with 'human' label". So §6's `bd human <bead>` is implemented as `bd update <bead> --add-label human`.

**Out of scope (§8 sub-seams / deferred):** the resolver agent's prompt/brief *format* and its dispatch (a host-runtime job like executor dispatch, not an engine verb); a resolve-attempt oscillation cap; re-running the pick's `verify` gate inside `finalize` (finalize only asserts `conflicts()` shrank); per-workspace vs repo-wide marker-style pinning; enriching `conflicts[]` with `paths`/`lowest_ancestor` (Task 2 enriches only `{bead, change}` — the resolver enumerates paths in-workspace via `jj resolve --list`).

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/workspace/workspace.go` (modify) | Add `ResolveName(beadID)` + `ResolvePath(jjRoot, cfgRoot, beadID)` constructors (mirror `Name`/`Path` for the `-resolve` kind). |
| `internal/cli/shed.go` (modify) | Enrich `shed integrate`'s `conflicts` from `[]change-id` to `[]{bead, change}` so the orchestrator can call `conflict open <bead>`. |
| `internal/cli/conflict.go` (create) | `weft conflict` group: `open`, `finalize`; the `changeConflicted` helper. |
| `internal/cli/root.go` (modify) | Register `conflict`. |

Tests live beside each file (`workspace_test.go`, `shed_test.go`, `conflict_test.go`). The verbs are thin wrappers over the existing seam-1/3 plumbing and the `routeRunner` fake.

**Project output contract (enforced):** every list-emitting field initializes as `[]T{}`, never nil (nil → JSON `null`, breaks `--json` consumers). `healed`/`remaining_conflicts`/`conflicts` all follow this.

---

### Task 1: `internal/workspace` — `ResolveName` + `ResolvePath` constructors

**Files:**
- Modify: `internal/workspace/workspace.go`
- Test: `internal/workspace/workspace_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/workspace/workspace_test.go`:

```go
func TestResolveNameAndPath(t *testing.T) {
	name := ResolveName("weft-hjx.4.2")
	if name != "weft-hjx__4__2-resolve" {
		t.Fatalf("ResolveName = %q, want weft-hjx__4__2-resolve", name)
	}
	// Round-trips through the existing kind-aware Resolve.
	bead, kind := Resolve(name)
	if bead != "weft-hjx.4.2" || kind != KindResolve {
		t.Fatalf("Resolve(%q) = %q,%v; want weft-hjx.4.2,resolve", name, bead, kind)
	}
	p := ResolvePath("/repo", "", "weft-hjx.4.2")
	if filepath.Base(p) != name {
		t.Fatalf("ResolvePath base = %q, want %q", filepath.Base(p), name)
	}
	// Same worktrees root as an executor workspace, different leaf.
	if filepath.Dir(p) != filepath.Dir(Path("/repo", "", "weft-hjx.4.2")) {
		t.Fatalf("ResolvePath root = %q, want same as Path root", filepath.Dir(p))
	}
}
```

(`workspace_test.go` already imports `path/filepath` and `testing` — confirm; if not, add `path/filepath`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/workspace/ -run TestResolveNameAndPath`
Expected: FAIL — `undefined: ResolveName` / `undefined: ResolvePath`.

- [ ] **Step 3: Add the constructors**

In `internal/workspace/workspace.go`, after `Path` (and reusing the existing `resolveSuffix` const):

```go
// ResolveName returns the resolution-workspace name for a bead (seam 4 §4.1):
// the executor name plus the -resolve suffix that marks the second kind. Resolve
// inverts it back to the owning bead-id + KindResolve.
func ResolveName(beadID string) string { return Name(beadID) + resolveSuffix }

// ResolvePath returns the absolute resolution-workspace directory for a bead —
// the same worktrees root as Path, with the -resolve leaf.
func ResolvePath(jjRoot, cfgRoot, beadID string) string {
	return filepath.Join(Root(jjRoot, cfgRoot), ResolveName(beadID))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/workspace/ -run TestResolveNameAndPath`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(workspace): ResolveName/ResolvePath for the -resolve workspace kind (seam 4 §4.1)"`

---

### Task 2: `shed integrate` — enrich `conflicts[]` with the owning bead

**Files:**
- Modify: `internal/cli/shed.go`
- Test: `internal/cli/shed_test.go`

**Why:** `conflict open` takes a `<bead>` (it derives the change + names the `<bead>-resolve` workspace). `shed integrate` already knows each conflicted change's owning bead (its `stack` is `[]{bead, change}`), but currently emits `conflicts` as bare change-id strings. Enriching to `[{bead, change}]` lets the orchestrator loop call `conflict open <bead>` directly (spec §3's `conflicts[]` carries `{bead, change, …}`; `paths`/`lowest_ancestor` stay deferred per §8).

- [ ] **Step 1: Update the failing test**

In `internal/cli/shed_test.go`, the existing `TestShedIntegrateBuildsLinearStack` asserts the clean path (empty conflicts) and is unaffected. Add a new test that exercises a conflicted member and asserts the enriched shape:

```go
func TestShedIntegrateConflictsCarryBead(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.4.1"):
			return run.Result{Stdout: `[{"title":"a","status":"in_progress","labels":["jj-change:chA"]}]`, Code: 0}
		case strings.Contains(j, "bd show weft-hjx.4.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chB"]}]`, Code: 0}
		case strings.Contains(j, "log -r conflicts()"):
			return run.Result{Stdout: "chB\n", Code: 0} // chB is conflicted
		default: // jj rebase
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "shed", "integrate", "weft-hjx.4.1", "weft-hjx.4.2", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// conflicts[] must carry the owning bead, not a bare change-id.
	if !strings.Contains(out.String(), `"bead": "weft-hjx.4.2"`) || !strings.Contains(out.String(), `"change": "chB"`) {
		t.Errorf("expected enriched conflict {bead,change}, got %q", out.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestShedIntegrateConflictsCarryBead`
Expected: FAIL — conflicts are emitted as bare change-id strings, so `"bead":` is absent.

- [ ] **Step 3: Enrich the conflicts construction**

In `internal/cli/shed.go`'s `newShedIntegrateCmd`, replace the conflict-collection block (the `conflicts := []string{}` loop over the scoped-revset output) with a `{bead, change}` mapping built from the `stack`:

```go
				// Map each conflicted change-id back to its owning bead via the
				// stack we just built, so the orchestrator can `conflict open <bead>`
				// (seam 4 §3). paths/lowest_ancestor enrichment is deferred (§8).
				changeToBead := map[string]string{}
				for _, e := range stack {
					changeToBead[e["change"]] = e["bead"]
				}
				conflicts := []map[string]string{}
				for _, ln := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
					if ln = strings.TrimSpace(ln); ln != "" {
						conflicts = append(conflicts, map[string]string{"bead": changeToBead[ln], "change": ln})
					}
				}
```

Then update the human-text summary to read the change off the map entries:

```go
				data := map[string]any{"stack": stack, "conflicts": conflicts}
				text := fmt.Sprintf("integrated %d picks: %s", len(stack), strings.Join(changeIDs, " -> "))
				if len(conflicts) > 0 {
					ids := make([]string, 0, len(conflicts))
					for _, c := range conflicts {
						ids = append(ids, c["change"])
					}
					text += fmt.Sprintf("  [%d conflicted: %s]", len(conflicts), strings.Join(ids, " "))
				}
				return Emit(cmd, "shed.integrate", data, text)
```

(The `changeIDs` slice for the happy-path summary is unchanged — it is still built from `stack` earlier in the function.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestShedIntegrate`
Expected: PASS (both `TestShedIntegrateBuildsLinearStack` and `TestShedIntegrateConflictsCarryBead`).

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(cli): shed integrate emits conflicts as {bead,change} (seam 4 §3)"`

---

### Task 3: `weft conflict open` — resolution workspace + resolver brief

**Files:**
- Create: `internal/cli/conflict.go`
- Modify: `internal/cli/root.go` (register `conflict`)
- Test: `internal/cli/conflict_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/conflict_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/run"
)

func TestConflictOpenCreatesResolveWorkspace(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.4.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chB"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: "/repo/weft", Code: 0}
		case strings.Contains(j, "conflicts() & chB"):
			return run.Result{Stdout: "chB\n", Code: 0} // chB IS conflicted -> proceed
		default: // workspace add, config set
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "conflict", "open", "weft-hjx.4.2", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var sawAdd, sawMarker bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "workspace add") && strings.Contains(j, "weft-hjx__4__2-resolve") && strings.Contains(j, "-r chB") {
			sawAdd = true
		}
		if strings.Contains(j, "config set --repo ui.conflict-marker-style diff") {
			sawMarker = true
		}
	}
	if !sawAdd {
		t.Errorf("expected workspace add of weft-hjx__4__2-resolve at -r chB; calls=%v", r.calls)
	}
	if !sawMarker {
		t.Errorf("expected ui.conflict-marker-style=diff; calls=%v", r.calls)
	}
	if !strings.Contains(out.String(), `"change": "chB"`) {
		t.Errorf("brief missing change: %q", out.String())
	}
}

func TestConflictOpenRefusesUnconflictedChange(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chB"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: "/repo/weft", Code: 0}
		case strings.Contains(j, "conflicts() & chB"):
			return run.Result{Stdout: "", Code: 0} // NOT conflicted
		default:
			return run.Result{Code: 0}
		}
	}}
	if got := exit.Code(runRoot(r, "conflict", "open", "weft-hjx.4.2")); got != 1 {
		t.Fatalf("opening a non-conflicted change must be exit 1, got %d", got)
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "workspace add") {
			t.Fatalf("must NOT create a workspace for a non-conflicted change: %v", r.calls)
		}
	}
}
```

(Add `"github.com/seanb4t/weft/internal/exit"` to `conflict_test.go`'s imports for the second test.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestConflictOpen`
Expected: FAIL — `conflict` command not registered.

- [ ] **Step 3: Write `conflict.go` with the `open` verb + `changeConflicted` helper**

```go
// internal/cli/conflict.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/seanb4t/weft/internal/workspace"
	"github.com/spf13/cobra"
)

func (a *App) newConflictCmd() *cobra.Command {
	c := &cobra.Command{Use: "conflict", Short: "Conflict-resolution choreography (spec seam 4)"}
	c.AddCommand(a.newConflictOpenCmd())
	return c
}

// changeConflicted reports whether a revision is in jj's conflicts() set. The
// revision may be a change-id or a <workspace-name>@ working-copy reference.
func changeConflicted(r run.Runner, rev string) (bool, error) {
	res, err := run.JJ(r, "log", "-r", "conflicts() & "+rev, "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
	if err != nil {
		return false, exit.Hardf("jj conflicts check could not run: %v", err)
	}
	if res.Code != 0 {
		return false, exit.Hardf("jj conflicts check failed: %s", strings.TrimSpace(res.Stderr))
	}
	return strings.TrimSpace(res.Stdout) != "", nil
}

func (a *App) newConflictOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open <bead>",
		Short: "Open a resolution workspace on a conflicted pick + emit the resolver brief",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			change, err := changeOf(a.Runner, bead)
			if err != nil {
				return err
			}
			if change == "" {
				return exit.Invocationf("bead %s has no jj-change label (not sealed)", bead)
			}
			// Only open a resolution workspace for an actually-conflicted change.
			conflicted, err := changeConflicted(a.Runner, change)
			if err != nil {
				return err
			}
			if !conflicted {
				return exit.Invocationf("change %s for bead %s is not conflicted — nothing to resolve", change, bead)
			}
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			name := workspace.ResolveName(bead)
			path := workspace.ResolvePath(root, a.Config.Workspace.Root, bead)
			// Resolution workspace: -r <change> makes its @ a CHILD of the conflicted
			// change, so the conflict materializes there for the resolver to edit
			// (spec §3 "jj new L in a resolution workspace").
			if res, err := run.JJ(a.Runner, "workspace", "add", path, "--name", name, "-r", change); err != nil {
				return exit.Hardf("jj workspace add could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj workspace add %s failed: %s", name, strings.TrimSpace(res.Stderr))
			}
			// Pin diff marker style — the only built-in style that represents 3+-sided
			// conflicts natively (§5). Repo-scoped; per-workspace pinning is a §8 refinement.
			if res, err := run.JJ(a.Runner, "config", "set", "--repo", "ui.conflict-marker-style", "diff"); err != nil {
				return exit.Hardf("jj config set could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj config set failed: %s", strings.TrimSpace(res.Stderr))
			}
			data := map[string]any{"bead": bead, "change": change, "workspace": name, "path": path}
			text := fmt.Sprintf(
				"opened resolution workspace %s for %s (change %s) at %s\n"+
					"resolver: edit the conflict markers in that workspace (jj resolve --list to find them), remove them, then `weft conflict finalize %s`",
				name, bead, change, path, bead)
			return Emit(cmd, "conflict.open", data, text)
		},
	}
}
```

- [ ] **Step 4: Register `conflict` in `root.go`**

In `internal/cli/root.go`'s `NewRootCmd`, add after `app.newPlanCmd()` (the seam-2 registration):

```go
	root.AddCommand(app.newPlanCmd())
	root.AddCommand(app.newConflictCmd())
	return root
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestConflictOpen`
Expected: PASS.

- [ ] **Step 6: Commit**

Run: `jj commit -m "feat(cli): conflict open — resolution workspace + resolver brief (seam 4 §4)"`

---

### Task 4: `weft conflict finalize` — squash the resolution / heal / reap / escalate

**Files:**
- Modify: `internal/cli/conflict.go`
- Test: `internal/cli/conflict_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/conflict_test.go`:

```go
func TestConflictFinalizeSquashesAndReaps(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.4.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chB"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: "/repo/weft", Code: 0}
		case strings.Contains(j, "conflicts() & weft-hjx__4__2-resolve@"):
			return run.Result{Stdout: "", Code: 0} // resolver cleared the markers
		case strings.Contains(j, "diff --git -r weft-hjx__4__2-resolve@"):
			return run.Result{Stdout: "diff --git a/x b/x\n+fixed\n", Code: 0} // non-empty resolution
		case strings.Contains(j, "log -r conflicts()"):
			return run.Result{Stdout: "", Code: 0} // post-squash: nothing conflicted -> healed
		default: // squash, workspace forget
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "conflict", "finalize", "weft-hjx.4.2", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var sawSquash, sawForget bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "squash --from weft-hjx__4__2-resolve@ --into chB") {
			sawSquash = true
		}
		if strings.Contains(j, "workspace forget weft-hjx__4__2-resolve") {
			sawForget = true
		}
	}
	if !sawSquash {
		t.Errorf("expected squash --from <resolve>@ --into chB; calls=%v", r.calls)
	}
	if !sawForget {
		t.Errorf("expected reap (workspace forget) of the resolution workspace; calls=%v", r.calls)
	}
	if !strings.Contains(out.String(), `"healed"`) || !strings.Contains(out.String(), "chB") {
		t.Errorf("expected healed:[chB] in output: %q", out.String())
	}
}

func TestConflictFinalizeEscalatesWhenStillConflicted(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chB"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: "/repo/weft", Code: 0}
		case strings.Contains(j, "conflicts() & weft-hjx__4__2-resolve@"):
			return run.Result{Stdout: "chB\n", Code: 0} // STILL conflicted -> escalate
		default:
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "conflict", "finalize", "weft-hjx.4.2", "--json")
	if err != nil {
		t.Fatalf("finalize must exit 0 even when escalating (verdict is data): %v", err)
	}
	var sawHuman, sawSquash bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "bd update weft-hjx.4.2 --add-label human") {
			sawHuman = true
		}
		if strings.Contains(j, "squash") {
			sawSquash = true
		}
	}
	if !sawHuman {
		t.Errorf("expected bd update --add-label human escalation; calls=%v", r.calls)
	}
	if sawSquash {
		t.Fatalf("must NOT squash a still-conflicted resolution: %v", r.calls)
	}
	if !strings.Contains(out.String(), `"escalated": true`) {
		t.Errorf("expected escalated:true: %q", out.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestConflictFinalize`
Expected: FAIL — `conflict` has no `finalize` subcommand.

- [ ] **Step 3: Add the `finalize` verb**

Register it in `newConflictCmd`:

```go
func (a *App) newConflictCmd() *cobra.Command {
	c := &cobra.Command{Use: "conflict", Short: "Conflict-resolution choreography (spec seam 4)"}
	c.AddCommand(a.newConflictOpenCmd(), a.newConflictFinalizeCmd())
	return c
}
```

Add the verb (and `"os"` to `conflict.go`'s import block):

```go
func (a *App) newConflictFinalizeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "finalize <bead>",
		Short: "Fold the resolver's edits into the conflicted change, heal descendants, reap (or escalate)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			change, err := changeOf(a.Runner, bead)
			if err != nil {
				return err
			}
			if change == "" {
				return exit.Invocationf("bead %s has no jj-change label (not sealed)", bead)
			}
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			name := workspace.ResolveName(bead)
			path := workspace.ResolvePath(root, a.Config.Workspace.Root, bead)
			wsRev := name + "@" // jj addresses a workspace's working copy as <name>@

			// Escalation gate (§6): the resolver must have removed the markers. If
			// the resolution workspace's @ is still conflicted, do NOT squash —
			// flag the bead with the `human` label and leave the change conflicted.
			// (`bd human` only lists/responds/dismisses; the flag IS the label.)
			stillConflicted, err := changeConflicted(a.Runner, wsRev)
			if err != nil {
				return err
			}
			if stillConflicted {
				if res, err := run.BD(a.Runner, "update", bead, "--add-label", "human"); err != nil {
					return exit.Hardf("bd update could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("bd update %s failed: %s", bead, strings.TrimSpace(res.Stderr))
				}
				data := map[string]any{
					"bead": bead, "change": change, "escalated": true,
					"healed": []string{}, "remaining_conflicts": []string{change},
				}
				return Emit(cmd, "conflict.finalize", data,
					fmt.Sprintf("escalated %s: resolution still conflicted — flagged `human`, change %s left for a person", bead, change))
			}

			// The resolution must be non-empty (the resolver actually edited the
			// markers) before we fold it in.
			res, err := run.JJ(a.Runner, "diff", "--git", "-r", wsRev)
			if err != nil {
				return exit.Hardf("jj diff could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj diff failed: %s", strings.TrimSpace(res.Stderr))
			}
			if strings.TrimSpace(res.Stdout) == "" {
				return exit.Invocationf("resolution workspace %s has no changes — resolver did not edit the markers", name)
			}

			// Fold the resolution into the conflicted change; jj auto-rebases and
			// conflict-simplifies descendants, so one resolution heals the stack (§2.1).
			if res, err := run.JJ(a.Runner, "squash", "--from", wsRev, "--into", change); err != nil {
				return exit.Hardf("jj squash could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj squash failed: %s", strings.TrimSpace(res.Stderr))
			}

			// Reap the resolution workspace (seam 3 mechanics + path-safety guard).
			wtRoot := workspace.Root(root, a.Config.Workspace.Root)
			if !workspace.Contains(wtRoot, path) {
				return exit.Hardf("refusing to reap %q: resolves outside worktrees root %s", name, wtRoot)
			}
			if res, err := run.JJ(a.Runner, "workspace", "forget", name); err != nil {
				return exit.Hardf("jj workspace forget could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj workspace forget %s failed: %s", name, strings.TrimSpace(res.Stderr))
			}
			if err := os.RemoveAll(path); err != nil {
				return exit.Hardf("rm resolution workspace %s: %v", path, err)
			}

			// Re-query conflicts() (jj ground truth): change is healed if it (and any
			// descendants the squash simplified) are no longer conflicted.
			remaining, err := conflictChanges(a.Runner)
			if err != nil {
				return err
			}
			remainingSet := map[string]bool{}
			for _, c := range remaining {
				remainingSet[c] = true
			}
			healed := []string{}
			if !remainingSet[change] {
				healed = append(healed, change)
			}
			data := map[string]any{"bead": bead, "change": change, "healed": healed, "remaining_conflicts": remaining}
			text := fmt.Sprintf("finalized %s (change %s): %d healed, %d conflict(s) remaining",
				bead, change, len(healed), len(remaining))
			return Emit(cmd, "conflict.finalize", data, text)
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestConflictFinalize`
Expected: PASS.

- [ ] **Step 5: Run the full suite + vet + build**

Run: `go vet ./... && go test ./... && go build ./cmd/weft`
Expected: PASS across all packages; no vet complaints; binary builds.

- [ ] **Step 6: Commit**

Run: `jj commit -m "feat(cli): conflict finalize — squash/heal/reap or escalate via human label (seam 4 §4/§6)"`

---

## Done criteria

- `go test ./...` passes; `go vet ./...` clean; `go build ./cmd/weft` builds.
- `weft conflict open <bead>` refuses a non-conflicted change (exit 1); otherwise creates a `<bead>-resolve` workspace whose `@` is a child of the conflicted change, pins `ui.conflict-marker-style=diff`, and emits the resolver brief `{bead, change, workspace, path}`.
- `weft conflict finalize <bead>` reads the resolution workspace's `@` via `<name>@`: if still conflicted → `bd update --add-label human` and leave the change (no squash, exit 0, `escalated:true`); else assert the resolution is non-empty, `jj squash --from <name>@ --into <change>`, reap the workspace (path-guarded), and report `{healed, remaining_conflicts}` from a re-queried `conflicts()`.
- `shed integrate` emits `conflicts` as `[{bead, change}]`.
- `internal/workspace` exposes `ResolveName`/`ResolvePath`, round-tripping through the existing kind-aware `Resolve`.
- All list fields serialize as `[]`, never `null`.

## Out of scope (follow-on / §8 sub-seams)

- The resolver agent's brief *format* and its dispatch (host-runtime orchestration, like executor dispatch — not an engine verb).
- Oscillation cap on resolve attempts before forced escalation; re-running the pick's `verify` gate inside `finalize`; per-workspace (vs repo-wide) `ui.conflict-marker-style` pinning; `paths`/`lowest_ancestor` enrichment of `conflicts[]`.
- Seam 5 (GSD markdown ports), including porting a GSD resolver agent.

<!-- adr-capture: sha256=2cf21a210ad58e2d; session=cli; ts=2026-06-04T00:44:10Z; adrs=weft-lc7 -->
