<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 10 — Prove the weave loop end-to-end: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use dev-flow:subagent-driven-development (recommended) or dev-flow:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the weft weave loop closes end-to-end with a deterministic, CI-able Go integration test that drives the real `weft` binary against a real jj+bd scratch repo through every branch (clean land, conflict→heal, verify-fail→retry, unresolvable→human escalation), plus a one-time cmux-driven live dogfood.

**Architecture:** A new `internal/weave/` package containing only `//go:build integration` test code. The test plays the orchestrator role `weft/workflows/execute.md` plays at runtime — it shells out to the real built `weft` binary for each verb and branches on the JSON envelope — while a scripted executor/resolver (harness code) stands in for the dispatched agents. One committed `testdata/weave-fixture/warp-plan.json` is the substrate for both the scripted gate and the live run. The CI `integration` job is extended to install `jj` and run the new package; `execute.md` is corrected against the shipped verb surface.

**Tech Stack:** Go (`testing`, `os/exec`, `encoding/json`), `jj` (colocated git), `bd` (beads), the `weft` binary. No new production code and no new external Go dependencies (stdlib only).

**Spec:** `docs/seams/10-weave-loop-e2e.md`. **Design bead:** `weft-w1y`.

---

## File structure

| File | Responsibility | Created by |
|---|---|---|
| `internal/weave/harness_test.go` | Build-`weft`-once (`TestMain`), `runWeft` subprocess+envelope helper, scratch jj+bd repo setup, `exec.LookPath`/`t.Skip` guards | Task 1 |
| `internal/weave/conflict_proof_test.go` | The blocking spike: prove a deterministic add/add conflict surfaces in `data.conflicts` at `shed integrate` | Task 1 |
| `testdata/weave-fixture/warp-plan.json` | The single committed fixture warp (epic + 6 picks); consumed by both the scripted gate and the live run | Task 2 |
| `internal/weave/fixture_test.go` | `seedFixture` (run `weft plan emit`, map `weft-ref` → bead-id) + a non-integration unit test asserting the warp parses with the 6 expected refs | Task 2 |
| `internal/weave/agents_test.go` | `scriptedExecutor` (write a pick's content files; optional verify-fail marker) and `scriptedResolver` (heal: edit markers; escalate: leave them) | Task 3 |
| `internal/weave/weave_integration_test.go` | `TestWeaveLoopEndToEnd`: drive Steps 1–9, assert every branch closes and the epic terminates woven | Task 4 |
| `.github/workflows/ci.yml` | Extend the `integration` job: install `jj`, widen the `-tags integration` path to `./internal/weave/` | Task 5 |
| `weft/workflows/execute.md` | Drift fix: finish verbs exist; `data.conflicts` envelope shape | Task 6 |

All `internal/weave/*_test.go` files except `fixture_test.go`'s unit test carry `//go:build integration`. `fixture_test.go` splits: the `seedFixture` helper is integration-tagged; the warp-parse unit test is not (so it runs in the default `go test ./...`).

---

### Task 1: Scratch-repo harness + deterministic-conflict spike

This is the **§7 blocking task**: before any loop logic, prove the harness can force a first-class jj conflict through `weft shed integrate` and observe it in `data.conflicts`. If this cannot be made deterministic, that is a finding (file a bead, stop) — everything downstream depends on it.

**Files:**

- Create: `internal/weave/harness_test.go`
- Create: `internal/weave/conflict_proof_test.go`

- [ ] **Step 1: Write the harness scaffolding**

Create `internal/weave/harness_test.go`:

```go
// internal/weave/harness_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// weftBin is the path to the weft binary built once for the whole package.
var weftBin string

// TestMain builds weft once into a temp dir and shares it across tests. If
// jj or bd are not on PATH, every test in the package skips (the loop cannot
// run without the real substrate).
func TestMain(m *testing.M) {
	if _, err := exec.LookPath("jj"); err != nil {
		// No jj: skip the whole package by reporting success with nothing run.
		os.Stderr.WriteString("weave: jj not on PATH — skipping integration package\n")
		os.Exit(0)
	}
	if _, err := exec.LookPath("bd"); err != nil {
		os.Stderr.WriteString("weave: bd not on PATH — skipping integration package\n")
		os.Exit(0)
	}
	dir, err := os.MkdirTemp("", "weft-bin-")
	if err != nil {
		os.Stderr.WriteString("weave: mktemp: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	weftBin = filepath.Join(dir, "weft")
	// Build from the module root (two levels up from internal/weave).
	build := exec.Command("go", "build", "-o", weftBin, "../../cmd/weft")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		os.Stderr.WriteString("weave: go build weft: " + err.Error() + "\n")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// envelope is the subset of weft's JSON envelope the harness branches on.
type envelope struct {
	OK   bool            `json:"ok"`
	Verb string          `json:"verb"`
	Data json.RawMessage `json:"data"`
}

// scratchRepo is a colocated jj+bd repo with a weft verify gate configured.
type scratchRepo struct {
	root     string // jj root; also where .beads and .weft live
	beadsDir string // BEADS_DIR for all bd-backed verbs
}

// newScratchRepo creates a fresh colocated jj+bd repo with a .weft/config.toml
// whose verify gate passes unless a `.weft-verify-fail` marker exists in cwd.
func newScratchRepo(t *testing.T) *scratchRepo {
	t.Helper()
	root := t.TempDir()
	r := &scratchRepo{root: root, beadsDir: filepath.Join(root, ".beads")}

	// Colocated jj repo (jj + git in one dir).
	r.mustJJ(t, "git", "init", "--colocate")
	// An initial commit so trunk() resolves to a real base for workspaces.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/.weft-bin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.mustJJ(t, "describe", "-m", "root: scratch base")
	r.mustJJ(t, "new") // leave an empty working copy on top of the described base

	// Fresh beads DB.
	r.mustBD(t, "init", "--non-interactive", "-p", "wv")

	// weft config: shed cap big enough for the whole fixture wave; verify gate
	// fails only when a `.weft-verify-fail` marker is present (per-pick control).
	cfg := "[shed]\nmax = 10\n\n[verify]\ncommand = \"test ! -f .weft-verify-fail\"\n"
	if err := os.MkdirAll(filepath.Join(root, ".weft"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".weft", "config.toml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return r
}

// run invokes the weft binary with --json, in the given working directory,
// with BEADS_DIR pointed at the scratch repo. dir == "" means the repo root.
func (r *scratchRepo) runWeft(t *testing.T, dir string, args ...string) envelope {
	t.Helper()
	if dir == "" {
		dir = r.root
	}
	full := append(append([]string{}, args...), "--json")
	cmd := exec.Command(weftBin, full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BEADS_DIR="+r.beadsDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("weft %s (dir=%s) failed: %v\noutput:\n%s",
			strings.Join(args, " "), dir, err, out)
	}
	// The envelope is the LAST non-empty line (bd/jj chatter may precede it on
	// stderr, but --json prints the envelope to stdout as the final line).
	var env envelope
	if err := json.Unmarshal([]byte(lastJSONLine(out)), &env); err != nil {
		t.Fatalf("weft %s: parse envelope: %v\noutput:\n%s", strings.Join(args, " "), err, out)
	}
	if !env.OK {
		t.Fatalf("weft %s: envelope ok=false:\n%s", strings.Join(args, " "), out)
	}
	return env
}

// lastJSONLine returns the last line that looks like a JSON object.
func lastJSONLine(b []byte) string {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		ln := strings.TrimSpace(lines[i])
		if strings.HasPrefix(ln, "{") && strings.HasSuffix(ln, "}") {
			return ln
		}
	}
	return ""
}

func (r *scratchRepo) mustJJ(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("jj", append([]string{"--no-pager"}, args...)...)
	cmd.Dir = r.root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("jj %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func (r *scratchRepo) mustBD(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("bd", args...)
	cmd.Dir = r.root
	cmd.Env = append(os.Environ(), "BEADS_DIR="+r.beadsDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bd %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
```

- [ ] **Step 2: Write the conflict-proof spike**

Create `internal/weave/conflict_proof_test.go`. It seeds two beads that both *create the same new file with different content*, isolates them, has the harness write the colliding files and seal each, then integrates and asserts a conflict surfaces. (This validates the add/add topology that Task 2's fixture relies on for P2/P4.)

```go
// internal/weave/conflict_proof_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDeterministicConflictAtIntegrate is the §7 de-risking spike: two picks
// that add the SAME new path with different content must produce a first-class
// jj conflict at `weft shed integrate`, surfaced in data.conflicts.
func TestDeterministicConflictAtIntegrate(t *testing.T) {
	r := newScratchRepo(t)

	// Two sibling beads under one epic, no deps → both ready in one wave.
	epic := r.mustCreateEpic(t, "conflict-proof")
	a := r.mustCreateChild(t, epic, "Pick A", "ca")
	b := r.mustCreateChild(t, epic, "Pick B", "cb")

	// Form the wave; expect both beads.
	form := r.runWeft(t, "", "shed", "form", "--epic", epic)
	wave := dataStringSlice(t, form.Data, "wave")
	if len(wave) != 2 {
		t.Fatalf("wave = %v, want 2 picks", wave)
	}

	// Isolate, then for each pick write the SAME path with DIFFERENT content
	// in its own workspace and seal.
	r.runWeft(t, "", "shed", "isolate", a, b)
	r.sealWith(t, a, map[string]string{"collide.txt": "content-from-A\n"})
	r.sealWith(t, b, map[string]string{"collide.txt": "content-from-B\n"})

	// Integrate the wave; a conflict must surface in data.conflicts.
	integ := r.runWeft(t, "", "shed", "integrate", a, b)
	var d struct {
		Conflicts []struct {
			Bead   string `json:"bead"`
			Change string `json:"change"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal(integ.Data, &d); err != nil {
		t.Fatalf("parse integrate data: %v", err)
	}
	if len(d.Conflicts) == 0 {
		t.Fatalf("expected a conflict in data.conflicts, got none: %s", integ.Data)
	}
}

// --- helpers used here and reused by later tasks ---

// mustCreateEpic creates an open epic and returns its bead-id.
func (r *scratchRepo) mustCreateEpic(t *testing.T, title string) string {
	t.Helper()
	return r.bdCreateID(t, "--type", "epic", "--title", title, "--description", "d", "--priority", "2")
}

// mustCreateChild creates an open task child of epic, stamped with a
// weft-ref:<ref> label so the harness can identify it later.
func (r *scratchRepo) mustCreateChild(t *testing.T, epic, title, ref string) string {
	t.Helper()
	id := r.bdCreateID(t, "--type", "task", "--title", title, "--description", "d",
		"--priority", "2", "--labels", "weft-ref:"+ref)
	r.mustBD(t, "update", id, "--parent", epic)
	return id
}

// bdCreateID runs `bd create ... --json` and returns the new bead id.
func (r *scratchRepo) bdCreateID(t *testing.T, args ...string) string {
	t.Helper()
	cmd := execBD(r, append([]string{"create", "--json"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd create: %v\n%s", err, out)
	}
	var res struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(lastJSONLine(out)), &res); err != nil {
		t.Fatalf("parse bd create json: %v\n%s", err, out)
	}
	if res.ID == "" {
		t.Fatalf("bd create returned empty id:\n%s", out)
	}
	return res.ID
}

// sealWith writes files into bead's workspace then seals the pick there.
func (r *scratchRepo) sealWith(t *testing.T, bead string, files map[string]string) {
	t.Helper()
	ws := r.workspacePath(t, bead)
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r.runWeft(t, ws, "pick", "seal", bead)
}
```

- [ ] **Step 3: Add the remaining shared helpers**

Append to `internal/weave/harness_test.go` the helpers the spike references (`dataStringSlice`, `workspacePath`, `execBD`):

```go
// dataStringSlice extracts data.<key> as a []string from an envelope's Data.
func dataStringSlice(t *testing.T, data json.RawMessage, key string) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("data not an object: %v", err)
	}
	raw, ok := m[key]
	if !ok {
		t.Fatalf("data has no key %q: %s", key, data)
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("data.%s not []string: %v", key, err)
	}
	return out
}

// workspacePath returns the jj workspace directory weft created for a bead.
// weft names workspaces by bead id under <root>_worktrees/ (or [workspace].root);
// the scratch config leaves [workspace].root empty → the default sibling layout.
// Resolve it from `jj workspace list` so the harness does not hard-code the path.
func (r *scratchRepo) workspacePath(t *testing.T, bead string) string {
	t.Helper()
	// weft workspace.Name(bead) == bead id; the directory basename matches.
	// jj workspace list prints "<name>: <change> ...". Find <name> == bead, then
	// derive the path from the known sibling layout: <root>_worktrees/<bead>.
	// (Confirm the basename via jj; see internal/workspace.Path for the layout.)
	return filepath.Join(filepath.Dir(r.root), filepath.Base(r.root)+"_worktrees", bead)
}

func execBD(r *scratchRepo, args ...string) *exec.Cmd {
	cmd := exec.Command("bd", args...)
	cmd.Dir = r.root
	cmd.Env = append(os.Environ(), "BEADS_DIR="+r.beadsDir)
	return cmd
}
```

> **Note on `workspacePath`:** the exact on-disk layout is owned by
> `internal/workspace.Path` (`<root>_worktrees/<bead>` when `[workspace].root`
> is empty). The implementer MUST confirm this against `internal/workspace/workspace.go`
> before relying on it; if the layout differs, derive the path by parsing
> `jj --no-pager workspace list` instead of reconstructing it. This is the one
> place the harness reaches into engine-internal layout.

- [ ] **Step 4: Run the spike against real binaries**

Run: `go test -tags integration ./internal/weave/ -run TestDeterministicConflictAtIntegrate -v`
Expected: PASS — the integrate envelope's `data.conflicts` is non-empty. If it is empty, STOP: the add/add topology does not deterministically conflict in this engine; file a findings bead under `weft-w1y`'s epic per spec §7 and reassess before continuing.

- [ ] **Step 5: Commit**

Commit using VCS-appropriate commands per `references/vcs-preamble.md`. Suggested message: `test(weft-w1y): scratch-repo harness + deterministic-conflict spike (seam 10)`.

---

### Task 2: The committed fixture warp + seeding

**Files:**

- Create: `testdata/weave-fixture/warp-plan.json`
- Create: `internal/weave/fixture_test.go`

- [ ] **Step 1: Author the fixture warp**

Create `testdata/weave-fixture/warp-plan.json`. Six independent picks (no `needs` → one wave). `weft plan emit` auto-stamps `weft-ref:<ref>`, so authored labels carry only `phase:impl`:

```json
{
  "epic": {
    "title": "Weave-loop E2E fixture",
    "description": "Synthetic branch-coverage epic for the seam-10 weave-loop proof.",
    "acceptance": "All non-escalated picks land; the escalated pick is flagged human and blocked."
  },
  "picks": [
    {"ref": "p1",  "title": "Clean pick",          "description": "lands directly", "labels": ["phase:impl"]},
    {"ref": "p2a", "title": "Conflict-heal A",      "description": "collides on collide_h.txt", "labels": ["phase:impl"]},
    {"ref": "p2b", "title": "Conflict-heal B",      "description": "collides on collide_h.txt", "labels": ["phase:impl"]},
    {"ref": "p3",  "title": "Verify-fail then pass","description": "fails verify once, then passes", "labels": ["phase:impl"]},
    {"ref": "p4a", "title": "Escalate A",           "description": "collides on collide_e.txt", "labels": ["phase:impl"]},
    {"ref": "p4b", "title": "Escalate B",           "description": "collides on collide_e.txt", "labels": ["phase:impl"]}
  ]
}
```

- [ ] **Step 2: Write the non-integration parse unit test**

Create `internal/weave/fixture_test.go` starting with a unit test (NO build tag — runs in default `go test ./...`) that asserts the committed warp parses and carries the six expected refs. This catches a fixture regression without the full E2E run:

```go
// internal/weave/fixture_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package weave_test

import (
	"sort"
	"testing"

	"github.com/seanb4t/weft/internal/plan"
)

// TestFixtureWarpParses asserts the committed fixture warp loads and declares
// exactly the six refs the weave E2E harness drives. A non-integration guard
// so a broken fixture is caught by the default test run.
func TestFixtureWarpParses(t *testing.T) {
	wp, err := plan.Load("../../testdata/weave-fixture/warp-plan.json")
	if err != nil {
		t.Fatalf("load fixture warp: %v", err)
	}
	got := make([]string, 0, len(wp.Picks))
	for _, p := range wp.Picks {
		got = append(got, p.Ref)
	}
	sort.Strings(got)
	want := []string{"p1", "p2a", "p2b", "p3", "p4a", "p4b"}
	if len(got) != len(want) {
		t.Fatalf("refs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("refs = %v, want %v", got, want)
		}
	}
	if wp.Epic.Title == "" {
		t.Fatal("fixture epic has no title")
	}
}
```

- [ ] **Step 3: Run the unit test**

Run: `go test ./internal/weave/ -run TestFixtureWarpParses -v`
Expected: PASS (this runs without the integration tag).

- [ ] **Step 4: Add the integration seed helper**

Append to `internal/weave/fixture_test.go` the integration-tagged seeding helper. It must be in a separate build-tagged section; split the file so the unit test stays untagged. The simplest correct structure is a **second file** `internal/weave/fixture_seed_test.go` carrying `//go:build integration`:

```go
// internal/weave/fixture_seed_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave_test

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// seededFixture is the result of seeding: the epic id and a ref→bead-id map.
type seededFixture struct {
	epic   string
	byRef  map[string]string // "p1" → bead-id
}

// seedFixture emits the committed fixture warp into the scratch repo's beads DB
// via `weft plan emit`, then resolves the ref→bead-id map by reading the
// weft-ref:<ref> label off each child of the created epic.
func (r *scratchRepo) seedFixture(t *testing.T) seededFixture {
	t.Helper()
	// Locate the committed warp relative to this source file (robust to cwd).
	_, thisFile, _, _ := runtime.Caller(0)
	warp := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "weave-fixture", "warp-plan.json")

	emit := r.runWeft(t, "", "plan", "emit", warp)
	// plan emit creates the epic + children; recover the epic id from data.
	var ed map[string]json.RawMessage
	if err := json.Unmarshal(emit.Data, &ed); err != nil {
		t.Fatalf("parse plan emit data: %v", err)
	}
	epic := mustEpicID(t, ed) // see helper below

	// Map ref → bead-id via the weft-ref label on each child.
	byRef := map[string]string{}
	for _, child := range r.childBeads(t, epic) {
		for _, lbl := range child.Labels {
			if strings.HasPrefix(lbl, "weft-ref:") {
				byRef[strings.TrimPrefix(lbl, "weft-ref:")] = child.ID
			}
		}
	}
	for _, ref := range []string{"p1", "p2a", "p2b", "p3", "p4a", "p4b"} {
		if byRef[ref] == "" {
			t.Fatalf("seedFixture: ref %q not found among epic children", ref)
		}
	}
	return seededFixture{epic: epic, byRef: byRef}
}

type childBead struct {
	ID     string   `json:"id"`
	Labels []string `json:"labels"`
}

// childBeads returns the epic's children via `bd list --parent <epic> --json`.
func (r *scratchRepo) childBeads(t *testing.T, epic string) []childBead {
	t.Helper()
	cmd := execBD(r, "list", "--parent", epic, "--json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd list --parent: %v\n%s", err, out)
	}
	var beads []childBead
	if err := json.Unmarshal([]byte(lastJSONLine(out)), &beads); err != nil {
		t.Fatalf("parse bd list json: %v\n%s", err, out)
	}
	return beads
}
```

> **Implementer note — `mustEpicID`:** the exact JSON shape of `weft plan emit`'s
> `data` (and where the created epic id lives) MUST be confirmed against
> `internal/cli/plan.go`'s `planFirstEmit` Emit call before writing `mustEpicID`.
> If `plan emit` does not echo the epic id in a stable field, recover the epic
> instead by querying `bd list --type epic --json` in the freshly-seeded scratch
> DB (exactly one epic exists) and taking its id. Write `mustEpicID` to match
> whichever is true; do not guess the field name.

- [ ] **Step 5: Run the seed against real binaries**

Add a thin integration test `TestSeedFixture` in `fixture_seed_test.go` that calls `newScratchRepo` + `seedFixture` and asserts `len(byRef) == 6` and `epic != ""`.
Run: `go test -tags integration ./internal/weave/ -run TestSeedFixture -v`
Expected: PASS.

- [ ] **Step 6: Commit**

Commit. Suggested message: `test(weft-w1y): committed weave fixture warp + seeding helper (seam 10)`.

---

### Task 3: Scripted executor and resolver

**Files:**

- Create: `internal/weave/agents_test.go`

- [ ] **Step 1: Write the scripted executor/resolver**

Create `internal/weave/agents_test.go`. The executor writes a pick's deterministic content (keyed by ref) into its workspace; for `p3` the first call also drops the `.weft-verify-fail` marker so the verify gate fails. The resolver either edits the conflicted file to remove markers (heal) or leaves it untouched (escalate).

```go
// internal/weave/agents_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave_test

import (
	"os"
	"path/filepath"
	"testing"
)

// pickFiles returns the files a ref's executor writes into its workspace.
// p2a/p2b collide on collide_h.txt; p4a/p4b collide on collide_e.txt
// (add/add, different content → first-class conflict at integrate).
func pickFiles(ref string) map[string]string {
	switch ref {
	case "p1":
		return map[string]string{"p1.txt": "p1\n"}
	case "p2a":
		return map[string]string{"collide_h.txt": "heal-A\n"}
	case "p2b":
		return map[string]string{"collide_h.txt": "heal-B\n"}
	case "p3":
		return map[string]string{"p3.txt": "p3\n"}
	case "p4a":
		return map[string]string{"collide_e.txt": "esc-A\n"}
	case "p4b":
		return map[string]string{"collide_e.txt": "esc-B\n"}
	}
	return nil
}

// scriptedExecutor writes the ref's content into the bead's workspace. When
// failVerify is true it also drops the .weft-verify-fail marker so the verify
// gate (`test ! -f .weft-verify-fail`) reports data.pass:false.
func (r *scratchRepo) scriptedExecutor(t *testing.T, ws, ref string, failVerify bool) {
	t.Helper()
	for name, content := range pickFiles(ref) {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(ws, ".weft-verify-fail")
	if failVerify {
		if err := os.WriteFile(marker, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	} else {
		_ = os.Remove(marker) // ensure absent on the passing (re)try
	}
}

// scriptedResolver acts in the resolution workspace opened by `conflict open`.
// heal=true: rewrite the conflicted file with a clean merged body (markers
// removed) so finalize squashes+heals. heal=false: leave the workspace
// untouched so finalize sees a still-conflicted @ and escalates (human label).
func (r *scratchRepo) scriptedResolver(t *testing.T, resolveDir, conflictedFile string, heal bool) {
	t.Helper()
	if !heal {
		return // leave markers → escalation path
	}
	if err := os.WriteFile(filepath.Join(resolveDir, conflictedFile), []byte("resolved\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Verify it compiles under the integration tag**

Run: `go vet -tags integration ./internal/weave/`
Expected: no errors (the helpers are referenced by Task 4; vet confirms they compile and the package builds under the tag).

- [ ] **Step 3: Commit**

Commit. Suggested message: `test(weft-w1y): scripted executor + resolver stand-ins (seam 10)`.

---

### Task 4: The full weave-loop E2E test

**Files:**

- Create: `internal/weave/weave_integration_test.go`

- [ ] **Step 1: Write the end-to-end test**

Create `internal/weave/weave_integration_test.go`. It drives `execute.md` Steps 1–9 once over the fixture wave and asserts every branch. The resolution-workspace path comes from `conflict open`'s `data.path`; the conflicted filename per ref is known from `pickFiles`.

```go
// internal/weave/weave_integration_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave_test

import (
	"encoding/json"
	"testing"
)

// TestWeaveLoopEndToEnd drives the full weave loop once over the synthetic
// fixture and asserts each branch closes: clean land (p1), conflict→heal
// (p2a/p2b), verify-fail→retry (p3), unresolvable→human escalation (p4a/p4b).
func TestWeaveLoopEndToEnd(t *testing.T) {
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

	// --- Step 2: isolate ---
	r.runWeft(t, "", "shed", append([]string{"isolate"}, wave...)...)

	// --- Step 3+4: dispatch (scripted) + verify gate, with p3 retry ---
	for _, bead := range wave {
		ref := refOf[bead]
		ws := r.workspacePath(t, bead)

		// p3 fails verify on the first attempt, then passes.
		r.scriptedExecutor(t, ws, ref, ref == "p3")
		v := r.runWeft(t, ws, "pick", "verify", bead)
		pass := dataBool(t, v.Data, "pass")
		if ref == "p3" {
			if pass {
				t.Fatalf("p3 first verify should fail")
			}
			// retry: clear the fail marker and re-verify
			r.scriptedExecutor(t, ws, ref, false)
			v = r.runWeft(t, ws, "pick", "verify", bead)
			if !dataBool(t, v.Data, "pass") {
				t.Fatalf("p3 retry verify should pass")
			}
		} else if !pass {
			t.Fatalf("%s verify should pass, got fail", ref)
		}
		r.runWeft(t, ws, "pick", "seal", bead)
	}

	// --- Step 5: integrate ---
	integ := r.runWeft(t, "", "shed", append([]string{"integrate"}, wave...)...)
	var integData struct {
		Conflicts []struct {
			Bead   string `json:"bead"`
			Change string `json:"change"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal(integ.Data, &integData); err != nil {
		t.Fatalf("parse integrate data: %v", err)
	}
	// Exactly two conflicts expected: one in the heal pair, one in the escalate
	// pair (the lex-later bead of each colliding pair).
	if len(integData.Conflicts) != 2 {
		t.Fatalf("conflicts = %d, want 2: %s", len(integData.Conflicts), integ.Data)
	}

	// --- Step 6: resolve each conflict (heal vs escalate by ref) ---
	escalated := map[string]bool{}
	for _, c := range integData.Conflicts {
		ref := refOf[c.Bead]
		open := r.runWeft(t, "", "conflict", "open", c.Bead)
		resolveDir := dataString(t, open.Data, "path")

		heal := ref == "p2a" || ref == "p2b" // the heal pair
		var conflictedFile string
		if heal {
			conflictedFile = "collide_h.txt"
		} else {
			conflictedFile = "collide_e.txt"
		}
		r.scriptedResolver(t, resolveDir, conflictedFile, heal)

		fin := r.runWeft(t, "", "conflict", "finalize", c.Bead)
		if dataBool(t, fin.Data, "escalated") {
			escalated[c.Bead] = true
		}
	}
	if len(escalated) != 1 {
		t.Fatalf("escalated count = %d, want 1", len(escalated))
	}

	// --- Step 7: land every pick that is not escalated ---
	for _, bead := range wave {
		if escalated[bead] {
			continue
		}
		r.runWeft(t, "", "pick", "land", bead)
	}

	// --- Step 8: cleanup + reap (idempotent) ---
	r.runWeft(t, "", "shed", append([]string{"cleanup"}, wave...)...)
	r.runWeft(t, "", "reap", "--epic", fx.epic)
	r.runWeft(t, "", "reap", "--epic", fx.epic) // second call must also succeed

	// --- Step 9: resume shows terminal state ---
	resume := r.runWeft(t, "", "resume", "--epic", fx.epic)
	landed := dataStringSlice(t, resume.Data, "landed")
	blocked := dataStringSlice(t, resume.Data, "blocked")
	if len(landed) != 5 {
		t.Fatalf("landed = %v, want 5", landed)
	}
	if len(blocked) < 1 {
		t.Fatalf("blocked = %v, want the escalated pick", blocked)
	}

	// The loop has terminated: forming again yields an empty (or human-only) wave.
	form2 := r.runWeft(t, "", "shed", "form", "--epic", fx.epic)
	if w := dataStringSlice(t, form2.Data, "wave"); len(w) != 0 {
		t.Fatalf("post-loop wave = %v, want empty (escalated pick is human-blocked)", w)
	}
}
```

- [ ] **Step 2: Add the remaining envelope extractors**

Append `dataBool` and `dataString` to `internal/weave/harness_test.go`:

```go
func dataBool(t *testing.T, data json.RawMessage, key string) bool {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("data not an object: %v", err)
	}
	var b bool
	if err := json.Unmarshal(m[key], &b); err != nil {
		t.Fatalf("data.%s not bool: %v", key, err)
	}
	return b
}

func dataString(t *testing.T, data json.RawMessage, key string) string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("data not an object: %v", err)
	}
	var s string
	if err := json.Unmarshal(m[key], &s); err != nil {
		t.Fatalf("data.%s not string: %v", key, err)
	}
	return s
}
```

- [ ] **Step 3: Run the full E2E**

Run: `go test -tags integration ./internal/weave/ -run TestWeaveLoopEndToEnd -v`
Expected: PASS. Every branch is exercised: 6-pick wave formed, p3 retried, 2 conflicts at integrate, 1 healed + 1 escalated, 5 landed, 1 human-blocked, idempotent reap, terminal empty wave.

> **If the post-loop wave is non-empty (not just the escalated pick):** confirm
> the escalated pick carries the `human` label and that `shed form`'s `bd ready`
> excludes `human`-labelled beads. If `bd ready` does NOT exclude `human`, that
> is a finding (the escalation gate does not actually block re-formation) — file
> a bead under `weft-w1y`'s epic per spec §7 rather than weakening the assertion.

- [ ] **Step 4: Run the whole integration package**

Run: `go test -tags integration ./internal/weave/ -v`
Expected: all tests PASS (spike, seed, fixture-parse, full E2E).

- [ ] **Step 5: Commit**

Commit. Suggested message: `test(weft-w1y): full weave-loop end-to-end gate (seam 10)`.

---

### Task 5: Extend the CI integration job

**Files:**

- Modify: `.github/workflows/ci.yml` (the `integration:` job, currently ending with the `go test -tags integration ./internal/plan/` step)

- [ ] **Step 1: Inspect the current job**

Run: `sed -n '61,100p' .github/workflows/ci.yml`
Confirm: it installs only `bd` and runs `go test -tags integration ./internal/plan/`; there is no `jj` install step.

- [ ] **Step 2: Add a jj install step**

After the `Install bd (pinned, checksum-verified)` step and before the test step, add a pinned `jj` install. Mirror the bd step's pin+verify discipline (exact version, deliberate Renovate-tracked bump). Concrete step (adjust the version to the latest stable `jj` release at implementation time; pin it, do not float):

```yaml
      # jj is required by the weave end-to-end gate (internal/weave): it drives
      # the real `weft` binary against real jj workspaces. Pin deliberately;
      # Renovate tracks JJ_VERSION (github-releases customManager in renovate.json).
      - name: Install jj (pinned)
        run: |
          set -euo pipefail
          JJ_VERSION=0.42.0
          url="https://github.com/jj-vcs/jj/releases/download/v${JJ_VERSION}/jj-v${JJ_VERSION}-x86_64-unknown-linux-musl.tar.gz"
          curl -fsSL "$url" -o /tmp/jj.tar.gz
          tar -xzf /tmp/jj.tar.gz -C /usr/local/bin jj
          jj --version
```

- [ ] **Step 3: Widen the test path**

Change the integration test step to include the weave package. Update the existing command from `./internal/plan/` to cover both packages:

```yaml
      - name: Integration tests (live bd + jj)
        run: |
          set -euo pipefail
          BEADS_DIR="$WS/.beads" go test -tags integration ./internal/plan/ ./internal/weave/
```

(Keep whatever `$WS`/`BEADS_DIR` setup the existing step uses; only the test paths and the step name change. If the weave package needs no pre-seeded `BEADS_DIR`, the env is harmless — the harness creates its own scratch DBs via `t.TempDir`.)

- [ ] **Step 4: Add JJ_VERSION to Renovate tracking**

Mirror the existing `BD_VERSION` customManager in `renovate.json` so `JJ_VERSION` gets bump PRs. Show the addition:

Run: `rg -n 'BD_VERSION|customManager|customManagers' renovate.json`
Then add a sibling `customManagers` entry matching `JJ_VERSION=` in `.github/workflows/ci.yml` against the `jj-vcs/jj` github-releases datasource, modeled exactly on the `BD_VERSION` entry.

- [ ] **Step 5: Validate the workflow + Renovate config locally**

Run: `actionlint .github/workflows/ci.yml` (if available) and `npx --yes renovate-config-validator renovate.json`
Expected: both clean. (`renovate-config-validator` is already a CI gate per the seam-9 follow-ups.)

- [ ] **Step 6: Commit**

Commit. Suggested message: `ci(weft-w1y): run the weave E2E gate (install jj, widen integration path) (seam 10)`.

> **Acceptance signal for this task:** a CI run on the PR shows the `integration`
> job *executed* the weave test (a `TestWeaveLoopEndToEnd` PASS line in the job
> log), not a skip. A skipped test here means `jj` was not installed or the path
> was not widened — the deliverable is not met until the log proves execution.

---

### Task 6: Fix `execute.md` drift

**Files:**

- Modify: `weft/workflows/execute.md` (§4 step termination + §5 Termination + §4 Step 5 envelope example)

- [ ] **Step 1: Correct the conflicts envelope example (Step 5 block)**

In `weft/workflows/execute.md`, the §4 "Step 5 — Integrate the wave" JSON block shows `conflicts` as a top-level field carrying `paths`/`lowest_ancestor`. Replace it with the real shape: `conflicts` nested under `data`, entries `{bead,change}` only. Change the block to:

```json
{
  "ok": true,
  "verb": "shed.integrate",
  "data": {
    "stack": [{"bead": "...", "change": "..."}],
    "conflicts": [{"bead": "...", "change": "..."}]
  }
}
```

Update the surrounding prose that says "`conflicts` is DATA emitted at exit 0" to read "`data.conflicts` is DATA emitted at exit 0", and remove any reference to `paths`/`lowest_ancestor` (those fields are deferred per seam-4 §8 and do not exist; the resolver brief comes from `weft conflict open`, not from the integrate envelope).

- [ ] **Step 2: Correct the finish-verb staleness (§5 Termination)**

In §5 ("Termination") and the §4 termination note, replace the claim that epic finishing "is deferred engine work, not yet part of the stable verb surface this workflow restricts itself to." `weft finish open/reconcile` shipped (seam 6). Reword to: the execute loop still terminates when the ready set is empty and does **not** itself call finish, but finishing is now an available, separate operator step (`weft finish open` then `weft finish reconcile` after merge) — not a nonexistent verb. Do not add a finish call to the loop; only correct the "does not exist" language.

- [ ] **Step 3: Verify no other stale envelope references remain**

Run: `rg -n 'lowest_ancestor|"paths"|top-level conflicts|not yet part of the stable verb' weft/workflows/execute.md`
Expected: no matches (all corrected).

- [ ] **Step 4: Commit**

Commit. Suggested message: `docs(weft-w1y): fix execute.md drift — finish verbs exist, data.conflicts shape (seam 10)`.

---

## Live dogfood (post-merge, one-time — tracked, not a code task)

After the gate is green and merged, run the one-time cmux-driven live dogfood per spec §6: spawn a `claude` worker, point `/weft-execute` at the fixture epic (seeded from `testdata/weave-fixture/warp-plan.json` in a scratch repo), observe via cmux, and file every gap as a bead under `weft-w1y`'s epic. Append a "Live run 1" record (date, fixture, branches observed, findings) to `docs/seams/10-weave-loop-e2e.md`. The re-runnable-harness decision (spec §9) is made from that run's evidence. This is operator/orchestrator work, not part of the scripted-gate task chain above.

---

## Self-review notes

- **Spec coverage:** §3 scripted gate → Tasks 1,3,4; §4 fixture → Task 2; §3/§8 CI extension → Task 5; §5 drift fix → Task 6; §6 live dogfood → tracked post-merge section; §7 risk → Task 1 is the explicit de-risking spike with a STOP/findings-bead escape. All spec sections map to a task.
- **No placeholders:** every code step shows complete code; the two engine-internal unknowns (`weft plan emit` data shape for the epic id; `internal/workspace.Path` layout) are called out as explicit "confirm against source, here's the fallback" implementer notes rather than left vague.
- **Type consistency:** `envelope`, `scratchRepo`, `runWeft`, `dataStringSlice`/`dataBool`/`dataString`, `seededFixture.byRef`, `pickFiles`, `scriptedExecutor`/`scriptedResolver` are defined once and used with consistent signatures across tasks.
<!-- adr-capture: sha256=47b58678aef644ca; session=cli; ts=2026-06-08T16:12:49Z; adrs=weft-9w5 -->
