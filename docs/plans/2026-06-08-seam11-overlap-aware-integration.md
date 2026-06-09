# Seam 11 — Overlap-aware integration topology: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `shed integrate`'s single lexicographic jj stack with a file-overlap forest so an escalated (permanently-conflicted) pick cannot cascade-poison independent picks, and add a `finish open` collapse step so the landed picks ship as one line while the escalated pick stays parked for a human.

**Architecture:** Two picks can conflict only if they touch a common file (jj conflicts are per-file). `shed integrate` groups the wave by file-overlap connected components, stacks linearly only within a group, and emits a forest rooted on `trunk()`. Conflicts are still detected up-front via the existing scoped `conflicts()` revset, but cross-group cascade is now structurally impossible. `finish open` collapses the epic's closed picks into one `@`-tipped line via per-change `jj rebase -r`, leaving any escalated/open pick parked on `trunk()`.

**Tech Stack:** Go 1.26; `cobra` commands under `internal/cli/`; `run.Runner` shells `jj`/`bd`/`gh`; envelopes via `Emit`; jj 0.42 (colocated); `//go:build integration` tests drive the real binary in `internal/weave/`.

**Spec:** `docs/seams/11-overlap-aware-integration.md`. **Design bead:** `weft-eoe`.

---

## File structure

| File | Responsibility | Action |
|---|---|---|
| `internal/cli/overlap.go` | Pure `overlapGroups` connected-components partitioner | Create |
| `internal/cli/overlap_test.go` | Table tests for `overlapGroups` | Create |
| `internal/cli/shed.go` | `newShedIntegrateCmd` — forest build + `groups` envelope; `changeFiles` helper | Modify (`158-268`) |
| `internal/cli/shed_test.go` | Integrate forest + envelope + drift tests | Modify |
| `internal/cli/finish.go` | `newFinishOpenCmd` collapse step; `collapseClosedPicks` helper; reconcile guard | Modify (`160-258`, `361-458`) |
| `internal/cli/finish_test.go` | Collapse unit tests; reconcile forest-safety | Modify |
| `weft/workflows/execute.md` | Steps 5–6 loop + `shed.integrate` envelope prose | Modify |
| `plugin/skills/execute/SKILL.md` | Lockstep copy of the above | Modify |
| `internal/weave/weave_integration_test.go` | Single-wave E2E (one `integrate`) | Modify |

Tasks 1–3 deliver the forest; Task 4–5 the ship-time collapse + reconcile safety; Task 6 the prose; Task 7 the end-to-end proof.

---

### Task 1: `overlapGroups` connected-components partitioner (pure)

**Files:**
- Create: `internal/cli/overlap.go`
- Test: `internal/cli/overlap_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/overlap_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"reflect"
	"testing"
)

func TestOverlapGroups(t *testing.T) {
	cases := []struct {
		name    string
		changes []string
		files   map[string][]string
		want    [][]string
	}{
		{"empty", nil, map[string][]string{}, [][]string{}},
		{"singleton", []string{"a"}, map[string][]string{"a": {"f"}}, [][]string{{"a"}}},
		{
			"all-independent",
			[]string{"a", "b", "c"},
			map[string][]string{"a": {"x"}, "b": {"y"}, "c": {"z"}},
			[][]string{{"a"}, {"b"}, {"c"}},
		},
		{
			"one-shared-pair",
			[]string{"a", "b"},
			map[string][]string{"a": {"x"}, "b": {"x"}},
			[][]string{{"a", "b"}},
		},
		{
			"two-disjoint-pairs",
			[]string{"a", "b", "c", "d"},
			map[string][]string{"a": {"p"}, "b": {"p"}, "c": {"q"}, "d": {"q"}},
			[][]string{{"a", "b"}, {"c", "d"}},
		},
		{
			"transitive-chain",
			[]string{"a", "b", "c"},
			map[string][]string{"a": {"x"}, "b": {"x", "y"}, "c": {"y"}},
			[][]string{{"a", "b", "c"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := overlapGroups(tc.changes, tc.files)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("overlapGroups = %v, want %v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestOverlapGroups`
Expected: FAIL — `undefined: overlapGroups`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/cli/overlap.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

// overlapGroups partitions changes into connected components over the
// "share at least one file" relation. Two changes that touch no common file
// cannot conflict (jj conflicts are per-file), so they need never be stacked
// together. `changes` is the deterministic (lex-sorted) change order the caller
// established; `files[ch]` is that change's touched file set. Returns groups,
// each a slice of change-ids preserving the input (lex) order, with groups
// ordered by their lexicographically-smallest member. Pure — no jj.
func overlapGroups(changes []string, files map[string][]string) [][]string {
	parent := make([]int, len(changes))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]] // path-halving
			i = parent[i]
		}
		return i
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}
	// First change index that claimed each file; union on every later collision.
	owner := map[string]int{}
	for i, ch := range changes {
		for _, f := range files[ch] {
			if j, ok := owner[f]; ok {
				union(i, j)
			} else {
				owner[f] = i
			}
		}
	}
	// Bucket indices by representative root, preserving first-appearance order.
	// Because `changes` is lex-sorted, first-appearance order == lex-smallest-member
	// order for the groups, and each bucket stays in lex order internally.
	buckets := map[int][]string{}
	order := []int{}
	for i, ch := range changes {
		r := find(i)
		if _, seen := buckets[r]; !seen {
			order = append(order, r)
		}
		buckets[r] = append(buckets[r], ch)
	}
	groups := make([][]string, 0, len(order))
	for _, r := range order {
		groups = append(groups, buckets[r])
	}
	return groups
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestOverlapGroups -v`
Expected: PASS (all sub-tests).

- [ ] **Step 5: Commit**

Commit using VCS-appropriate commands per `references/vcs-preamble.md`. Suggested: `feat(weft-eoe): overlapGroups connected-components partitioner (seam 11)`.

---

### Task 2: `changeFiles` helper — read a change's touched files

**Files:**
- Modify: `internal/cli/shed.go` (add helper near `newShedIntegrateCmd`)
- Test: `internal/cli/shed_test.go` (add `TestChangeFiles*`)

`jj diff --name-only` is verified present in jj 0.42 (spec §2): it prints one path per changed file. The helper wraps it with the same hard-failure idiom as the rest of `shed.go`.

- [ ] **Step 1: Write the failing test**

```go
// in internal/cli/shed_test.go
func TestChangeFilesParsesNameOnly(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "diff --name-only -r cha") {
			return run.Result{Stdout: "a.txt\ndir/b.txt\n", Code: 0}
		}
		return run.Result{Code: 0}
	}}
	got, err := changeFiles(r, "cha")
	if err != nil {
		t.Fatalf("changeFiles: %v", err)
	}
	want := []string{"a.txt", "dir/b.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changeFiles = %v, want %v", got, want)
	}
}

func TestChangeFilesNonZeroIsHardFailure(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		return run.Result{Code: 1, Stderr: "jj: no such revision"}
	}}
	_, err := changeFiles(r, "chx")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("jj diff failure must be hard (exit 2), got %d (err=%v)", got, err)
	}
}
```

Add `"reflect"` to the `shed_test.go` import block if not already present.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestChangeFiles`
Expected: FAIL — `undefined: changeFiles`.

- [ ] **Step 3: Write minimal implementation**

Add to `internal/cli/shed.go` (package-level, above `newShedIntegrateCmd`):

```go
// changeFiles returns the set of files a change touches, via
// `jj diff --name-only -r <change>` (verified present in jj 0.42). The caller
// MUST have validated the change-id against changeIDPattern before calling, as
// it is interpolated into the -r revset position.
func changeFiles(r run.Runner, change string) ([]string, error) {
	res, err := run.JJ(r, "diff", "--name-only", "-r", change)
	if err != nil {
		return nil, exit.Hardf("jj diff --name-only could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("jj diff --name-only %s failed: %s", change, strings.TrimSpace(res.Stderr))
	}
	return splitTrimLines(res.Stdout), nil
}
```

(`splitTrimLines` already exists in this package — `shed.go` uses it for the conflicts revset output.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestChangeFiles -v`
Expected: PASS.

- [ ] **Step 5: Commit**

Suggested: `feat(weft-eoe): changeFiles helper (jj diff --name-only) (seam 11)`.

---

### Task 3: `shed integrate` builds a forest and emits `groups`

Rewrite the rebase loop in `newShedIntegrateCmd` (`internal/cli/shed.go:158-268`) to: build a `change→bead` map, read each change's files, partition via `overlapGroups`, rebase each group as its own sub-stack rooted on `trunk()` (cursor resets to `trunk()` at each group boundary), rebuild `changeToBead` from all groups, and emit `data.groups` (replacing `data.stack`). `data.conflicts` shape and the scoped `conflicts()` detection are unchanged.

**Files:**
- Modify: `internal/cli/shed.go:158-268`
- Test: `internal/cli/shed_test.go`

- [ ] **Step 1: Write the failing tests**

Replace `TestShedIntegrateBuildsLinearStack` and `TestShedIntegrateSurfacesConflicts` with forest-aware versions, and add an envelope-drift test. The mock must now answer `jj diff --name-only`.

```go
// in internal/cli/shed_test.go — replaces TestShedIntegrateBuildsLinearStack

func TestShedIntegrateBuildsForestByFileOverlap(t *testing.T) {
	// Four picks, two disjoint overlap pairs:
	//   cha,chb touch shared.txt  -> one group
	//   chc,chd touch other.txt   -> another group
	// integrate must rebase each group rooted on trunk() (cursor resets per group),
	// never chaining group 2 onto group 1.
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-e.1"):
			return run.Result{Stdout: `[{"title":"a","status":"in_progress","labels":["jj-change:cha"]}]`, Code: 0}
		case strings.Contains(j, "bd show weft-e.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chb"]}]`, Code: 0}
		case strings.Contains(j, "bd show weft-e.3"):
			return run.Result{Stdout: `[{"title":"c","status":"in_progress","labels":["jj-change:chc"]}]`, Code: 0}
		case strings.Contains(j, "bd show weft-e.4"):
			return run.Result{Stdout: `[{"title":"d","status":"in_progress","labels":["jj-change:chd"]}]`, Code: 0}
		case strings.Contains(j, "diff --name-only -r cha"), strings.Contains(j, "diff --name-only -r chb"):
			return run.Result{Stdout: "shared.txt\n", Code: 0}
		case strings.Contains(j, "diff --name-only -r chc"), strings.Contains(j, "diff --name-only -r chd"):
			return run.Result{Stdout: "other.txt\n", Code: 0}
		case strings.Contains(j, "log -r conflicts()"):
			return run.Result{Stdout: "", Code: 0}
		default: // jj rebase
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "shed", "integrate", "weft-e.4", "weft-e.3", "weft-e.2", "weft-e.1", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var rebases [][]string
	for _, c := range r.calls {
		if len(c) >= 2 && c[0] == "jj" && contains(c, "rebase") {
			rebases = append(rebases, c)
		}
	}
	if len(rebases) != 4 {
		t.Fatalf("want 4 rebases, got %d: %v", len(rebases), rebases)
	}
	// Group 1 (cha,chb): cha onto trunk(), chb onto cha.
	if !contains(rebases[0], "cha") || !contains(rebases[0], "trunk()") {
		t.Errorf("rebase[0] should be cha onto trunk(): %v", rebases[0])
	}
	if !contains(rebases[1], "chb") || !contains(rebases[1], "cha") {
		t.Errorf("rebase[1] should be chb onto cha: %v", rebases[1])
	}
	// Group 2 (chc,chd): chc onto trunk() (NOT onto chb — cursor reset), chd onto chc.
	if !contains(rebases[2], "chc") || !contains(rebases[2], "trunk()") {
		t.Errorf("rebase[2] should be chc onto trunk() (group boundary reset): %v", rebases[2])
	}
	if contains(rebases[2], "chb") {
		t.Errorf("group 2 must not chain onto group 1's tip (chb): %v", rebases[2])
	}
	if !contains(rebases[3], "chd") || !contains(rebases[3], "chc") {
		t.Errorf("rebase[3] should be chd onto chc: %v", rebases[3])
	}
	// Envelope: groups present with {bead,change} pairs; no flat stack field.
	s := out.String()
	if !strings.Contains(s, `"groups"`) {
		t.Errorf("envelope must carry data.groups: %q", s)
	}
	if strings.Contains(s, `"stack"`) {
		t.Errorf("data.stack must be gone (replaced by groups): %q", s)
	}
	for _, want := range []string{`"change": "cha"`, `"change": "chb"`, `"change": "chc"`, `"change": "chd"`} {
		if !strings.Contains(s, want) {
			t.Errorf("groups missing %s: %q", want, s)
		}
	}
}

func TestShedIntegrateConflictMapsToBeadAcrossGroups(t *testing.T) {
	// chd (group 2 tail) comes back conflicted; integrate must still map it to
	// its bead via the rebuilt changeToBead and exit 0.
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-e.1"):
			return run.Result{Stdout: `[{"title":"a","status":"in_progress","labels":["jj-change:cha"]}]`, Code: 0}
		case strings.Contains(j, "bd show weft-e.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chb"]}]`, Code: 0}
		case strings.Contains(j, "bd show weft-e.3"):
			return run.Result{Stdout: `[{"title":"c","status":"in_progress","labels":["jj-change:chc"]}]`, Code: 0}
		case strings.Contains(j, "bd show weft-e.4"):
			return run.Result{Stdout: `[{"title":"d","status":"in_progress","labels":["jj-change:chd"]}]`, Code: 0}
		case strings.Contains(j, "diff --name-only -r cha"), strings.Contains(j, "diff --name-only -r chb"):
			return run.Result{Stdout: "shared.txt\n", Code: 0}
		case strings.Contains(j, "diff --name-only -r chc"), strings.Contains(j, "diff --name-only -r chd"):
			return run.Result{Stdout: "other.txt\n", Code: 0}
		case strings.Contains(j, "log -r conflicts()"):
			return run.Result{Stdout: "chd\n", Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "shed", "integrate", "weft-e.1", "weft-e.2", "weft-e.3", "weft-e.4", "--json")
	if err != nil {
		t.Fatalf("conflicts must not cause non-zero exit: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"bead": "weft-e.4"`) || !strings.Contains(s, `"change": "chd"`) {
		t.Errorf("conflict chd must map to bead weft-e.4 in conflicts[]: %q", s)
	}
}

func TestShedIntegrateEnvelopeAlwaysHasGroupsAndConflicts(t *testing.T) {
	// Single clean pick: groups and conflicts must both be present arrays (never null).
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-e.1"):
			return run.Result{Stdout: `[{"title":"a","status":"in_progress","labels":["jj-change:cha"]}]`, Code: 0}
		case strings.Contains(j, "diff --name-only -r cha"):
			return run.Result{Stdout: "a.txt\n", Code: 0}
		case strings.Contains(j, "log -r conflicts()"):
			return run.Result{Stdout: "", Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "shed", "integrate", "weft-e.1", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"groups"`) || !strings.Contains(s, `"conflicts"`) {
		t.Errorf("envelope must always carry groups + conflicts: %q", s)
	}
	if strings.Contains(s, `"groups": null`) || strings.Contains(s, `"conflicts": null`) {
		t.Errorf("groups/conflicts must be [] not null: %q", s)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run TestShedIntegrate`
Expected: FAIL — old code chains all four into one stack (group-2 rebase targets chb, not trunk()), emits `stack` not `groups`, and never calls `jj diff --name-only`.

- [ ] **Step 3: Rewrite the integrate body**

In `internal/cli/shed.go`, replace the block from the `// Resolve each pick's sealed change-id` comment through the `data := map[string]any{"stack": stack, "conflicts": conflicts}` line with:

```go
			// Resolve each pick's sealed change-id (the spine) and remember its bead.
			beadOf := map[string]string{}
			changes := make([]string, 0, len(beads))
			for _, b := range beads {
				ch, err := changeOf(a.Runner, b)
				if err != nil {
					return err
				}
				if ch == "" {
					return exit.Invocationf("bead %s has no jj-change label (not sealed)", b)
				}
				changes = append(changes, ch)
				beadOf[ch] = b
			}

			// Allowlist-validate every change-id before any revset interpolation
			// (the standing guard; conflict.go/resume.go apply the same).
			for _, ch := range changes {
				if !changeIDPattern.MatchString(ch) {
					return exit.Hardf("refusing to interpolate unsafe change-id %q into a revset", ch)
				}
			}

			// Sort change-ids lexicographically: overlapGroups orders groups by
			// lex-smallest member and keeps each group lex-internally, so its input
			// must be change-lex (not bead-lex) order. beadOf still maps back.
			sort.Strings(changes)

			// Partition by file-overlap: two changes can conflict only if they
			// share a file, so independent groups need never be stacked together.
			files := map[string][]string{}
			for _, ch := range changes {
				fs, err := changeFiles(a.Runner, ch)
				if err != nil {
					return err
				}
				files[ch] = fs
			}
			grouped := overlapGroups(changes, files)

			// Rebase each group as its own linear sub-stack rooted on trunk();
			// the cursor resets to trunk() at every group boundary, so no group
			// becomes an ancestor of another. --skip-emptied stays omitted within
			// a group so the cursor never points at an abandoned change (ADR weft-hjx.7).
			groups := make([][]map[string]string, 0, len(grouped))
			for _, g := range grouped {
				prev := "trunk()"
				grp := make([]map[string]string, 0, len(g))
				for _, ch := range g {
					if res, err := run.JJ(a.Runner, "rebase", "-s", ch, "-o", prev); err != nil {
						return exit.Hardf("jj rebase could not run: %v", err)
					} else if res.Code != 0 {
						return exit.Hardf("jj rebase %s failed: %s", ch, strings.TrimSpace(res.Stderr))
					}
					prev = ch
					grp = append(grp, map[string]string{"bead": beadOf[ch], "change": ch})
				}
				groups = append(groups, grp)
			}

			// Stack-scoped conflict detection (unchanged): only conflicts among
			// this wave's members. Cross-group cascade is now impossible.
			scopedRevset := "conflicts() & (" + strings.Join(changes, " | ") + ")"
			res, err := run.JJ(a.Runner, "log", "-r", scopedRevset, "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
			if err != nil {
				return exit.Hardf("jj log conflicts() could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj log conflicts() failed: %s", strings.TrimSpace(res.Stderr))
			}

			// Rebuild the change→bead map from ALL group members.
			changeToBead := map[string]string{}
			for _, g := range groups {
				for _, e := range g {
					changeToBead[e["change"]] = e["bead"]
				}
			}
			conflicts := []map[string]string{}
			for _, ln := range splitTrimLines(res.Stdout) {
				b, ok := changeToBead[ln]
				if !ok {
					return exit.Hardf("conflicted change %s is not in the integration forest — cannot map it to a bead", ln)
				}
				conflicts = append(conflicts, map[string]string{"bead": b, "change": ln})
			}

			data := map[string]any{"groups": groups, "conflicts": conflicts}
```

Then update the human-text summary block that follows. Replace the `text := fmt.Sprintf("integrated %d picks: %s", ...)` line (and the `changes[i] == stack[i]["change"]` comment above it) with:

```go
			// changes already holds the woven change-ids in lex order.
			text := fmt.Sprintf("integrated %d picks in %d group(s): %s", len(changes), len(groups), strings.Join(changes, " "))
```

Leave the existing `if len(conflicts) > 0 { … }` text-augmentation block and the final `return Emit(cmd, "shed.integrate", data, text)` unchanged.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestShedIntegrate|TestChangeFiles|TestOverlapGroups' -v`
Expected: PASS. Also run the whole package: `go test ./internal/cli/` — expect PASS (no other test referenced `data.stack`).

- [ ] **Step 5: Commit**

Suggested: `feat(weft-eoe): shed integrate builds file-overlap forest + groups envelope (seam 11)`.

---

### Task 4: `finish open` collapses closed picks into one line

`finish open` ships everything reachable from `@`. With a forest, the closed picks sit across group sub-stacks. Before `jj bookmark set <epic> -r @`, collapse the closed picks into one `@`-tipped line, parking any escalated/open pick on `trunk()`.

**Files:**
- Modify: `internal/cli/finish.go` (add `collapseClosedPicks`; call it in `newFinishOpenCmd` after the dry-run return, before `bookmark set`)
- Test: `internal/cli/finish_test.go`

- [ ] **Step 1: Empirically verify the collapse mechanism in a real jj repo**

Before writing code, confirm jj 0.42's behavior. In a scratch dir:

```bash
cd "$(mktemp -d)"
jj git init >/dev/null 2>&1
jj config set --repo user.name t >/dev/null 2>&1; jj config set --repo user.email t@e >/dev/null 2>&1
# group A: a0 <- a1 (both clean/closed-equivalent); group B (escalate): b0 <- b1(conflict)
echo base > base.txt; jj describe -m base >/dev/null 2>&1; jj bookmark create main -r @ >/dev/null 2>&1; jj new >/dev/null 2>&1
# Build two independent groups on trunk(); leave b1 conflicted.
# (Use add/add to force a conflict in b1, mirror the fixture.)
# ...drive: rebase a-group and b-group each rooted on trunk(); confirm:
#   1) jj rebase -r <a1> -o <a0-on-line> moves only a1 (a-group members land);
#   2) lifting a landed parent leaves a conflicted descendant re-parented onto trunk(), OFF the @-line;
#   3) the resulting @-line has NO conflicts (jj log -r 'conflicts() & ::@' is empty) so it is pushable.
jj --no-pager log
```

Record the exact ancestor-first ordering query that works (the collapse must place a group's lower member before its upper member). **Expected:** per-change `jj rebase -r <ch> -o <tip>` preserves correctness when changes are applied lowest-ancestor-first within each group; an escalated change not in the closed set is never moved and ends as a `trunk()` child outside `::@`. If jj's behavior differs, adjust Step 3's ordering source accordingly and note it on `weft-eoe`.

- [ ] **Step 2: Write the failing test**

```go
// in internal/cli/finish_test.go

func TestCollapseClosedPicksLinearizesInAncestorOrderExcludingEscalated(t *testing.T) {
	// Closed picks cha (group A base), chb (group A tail, healed), chc (group B base).
	// Ancestor-first order is provided by the topo query; collapse must rebase -r
	// each closed change onto the advancing tip and must NOT touch the escalated
	// change chd (not in the closed set).
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		// Topo-order query returns the closed changes ancestors-first.
		if strings.Contains(j, "log -r") && strings.Contains(j, "cha") && strings.Contains(j, "reverse") {
			return run.Result{Stdout: "cha\nchb\nchc\n", Code: 0}
		}
		return run.Result{Code: 0}
	}}
	picks := []finishPick{
		{Bead: "weft-e.1", Title: "a", Change: "cha"},
		{Bead: "weft-e.2", Title: "b", Change: "chb"},
		{Bead: "weft-e.3", Title: "c", Change: "chc"},
	}
	if err := collapseClosedPicks(r, picks); err != nil {
		t.Fatalf("collapseClosedPicks: %v", err)
	}
	var rebases [][]string
	for _, c := range r.calls {
		if len(c) >= 2 && c[0] == "jj" && contains(c, "rebase") && contains(c, "-r") {
			rebases = append(rebases, c)
		}
	}
	if len(rebases) != 3 {
		t.Fatalf("want 3 per-change rebases (cha,chb,chc), got %d: %v", len(rebases), rebases)
	}
	if !contains(rebases[0], "cha") || !contains(rebases[0], "trunk()") {
		t.Errorf("first collapse rebase should be cha onto trunk(): %v", rebases[0])
	}
	if !contains(rebases[1], "chb") || !contains(rebases[1], "cha") {
		t.Errorf("second should be chb onto cha: %v", rebases[1])
	}
	if !contains(rebases[2], "chc") || !contains(rebases[2], "chb") {
		t.Errorf("third should be chc onto chb: %v", rebases[2])
	}
	for _, c := range r.calls {
		if contains(c, "chd") {
			t.Errorf("escalated chd must never be rebased: %v", c)
		}
	}
}

func TestCollapseClosedPicksEmptyIsNoop(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result { return run.Result{Code: 0} }}
	if err := collapseClosedPicks(r, nil); err != nil {
		t.Fatalf("empty collapse must be a no-op: %v", err)
	}
	if len(r.calls) != 0 {
		t.Errorf("empty collapse must issue no jj calls: %v", r.calls)
	}
}
```

- [ ] **Step 3: Implement `collapseClosedPicks` and wire it in**

Add to `internal/cli/finish.go` (the ordering source is confirmed in Step 1; the query below returns the closed changes ancestors-first):

```go
// collapseClosedPicks rebases the epic's closed picks into a single linear line
// rooted on trunk(), so `finish open`'s `bookmark set -r @` + push ship exactly
// the landed work. Picks are placed ancestors-first (a healed upper member's
// content is parent-relative, so its lower member must remain its ancestor).
// An escalated/open pick is not in `picks`, so it is never moved and is left
// parked as a trunk() child, off the @-line.
func collapseClosedPicks(r run.Runner, picks []finishPick) error {
	if len(picks) == 0 {
		return nil
	}
	revs := make([]string, 0, len(picks))
	for _, p := range picks {
		if !changeIDPattern.MatchString(p.Change) {
			return exit.Hardf("refusing to interpolate unsafe change-id %q into a revset", p.Change)
		}
		revs = append(revs, p.Change)
	}
	// Ancestors-first order over the closed changes (verified jj 0.42 idiom, Step 1):
	// reverse(...) lists a chain root-first; independent groups interleave but each
	// group's internal order is preserved.
	res, err := run.JJ(r, "log", "-r", "reverse("+strings.Join(revs, " | ")+")", "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
	if err != nil {
		return exit.Hardf("jj log (collapse order) could not run: %v", err)
	}
	if res.Code != 0 {
		return exit.Hardf("jj log (collapse order) failed: %s", strings.TrimSpace(res.Stderr))
	}
	prev := "trunk()"
	var top string
	for _, ch := range splitTrimLines(res.Stdout) {
		if res, err := run.JJ(r, "rebase", "-r", ch, "-o", prev); err != nil {
			return exit.Hardf("jj rebase could not run: %v", err)
		} else if res.Code != 0 {
			return exit.Hardf("jj rebase -r %s failed: %s", ch, strings.TrimSpace(res.Stderr))
		}
		prev = ch
		top = ch
	}
	if top != "" {
		if res, err := run.JJ(r, "new", top); err != nil {
			return exit.Hardf("jj new could not run: %v", err)
		} else if res.Code != 0 {
			return exit.Hardf("jj new %s failed: %s", top, strings.TrimSpace(res.Stderr))
		}
	}
	return nil
}
```

In `newFinishOpenCmd`, insert the collapse call **after** the dry-run early-return (so `--dry-run` stays a no-op) and **before** `jj bookmark set <epic> -r @`:

```go
			// Collapse the closed picks into one @-tipped line (forest → line),
			// parking any escalated pick on trunk(), so the push ships only landed work.
			if err := collapseClosedPicks(a.Runner, picks); err != nil {
				return err
			}

			// Set the bookmark at the working-copy tip and push.
			if res, err := run.JJ(a.Runner, "bookmark", "set", epic, "-r", "@"); err != nil {
```

`splitTrimLines` is already in-package (`shed.go`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestCollapseClosedPicks|TestFinishOpen' -v`
Expected: PASS. Confirm `TestFinishOpenDryRunMutatesNothing` still passes (collapse is after the dry-run return, so dry-run issues no rebase).

- [ ] **Step 5: Commit**

Suggested: `feat(weft-eoe): finish open collapses closed picks into one line (seam 11)`.

---

### Task 5: `finish reconcile` survives the forest + parked escalated pick

Reconcile has two branches (`finish.go:405-430`). The squash branch (`jj new main` + abandon `roots(trunk()..@)`) excludes the parked escalated sibling for free. The merge-commit branch (`jj rebase -b @ -o main --skip-emptied`) must not drag the parked sibling.

**Files:**
- Modify: `internal/cli/finish.go` (`newFinishReconcileCmd`)
- Test: `internal/cli/finish_test.go`

- [ ] **Step 1: Empirically verify both reconcile branches against a parked sibling**

In a scratch repo, build `@`-line (landed) + a separate conflicted `trunk()` child (parked escalated). Then:

```bash
# squash branch: jj new main; jj log -r 'roots(trunk()..@)'  -> must NOT list the parked change
# merge branch:  jj rebase -b @ -o main --skip-emptied        -> inspect whether the parked sibling moved
jj --no-pager log
```

**Expected:** `roots(trunk()..@)` lists only the collapsed line's root (parked sibling excluded — squash branch is safe unchanged). For the merge branch, confirm whether `-b @` includes the parked sibling. Record the result on `weft-eoe`.

- [ ] **Step 2: Write the failing test (guard the merge branch)**

If Step 1 shows `-b @` drags the parked sibling, the merge branch must scope the rebase to `@`'s line. Encode the invariant as a test asserting reconcile does not rebase a non-`@`-ancestor change:

```go
// in internal/cli/finish_test.go
func TestFinishReconcileMergeBranchLeavesParkedEscalatedAlone(t *testing.T) {
	// Merge-commit style; a parked escalated change (chPark) is a trunk() sibling,
	// not an ancestor of @. Reconcile must not move it.
	r := mergedReconcileRunner(true, func(j string) (run.Result, bool) {
		if strings.Contains(j, "rebase") && strings.Contains(j, "chPark") {
			t.Errorf("reconcile must not rebase the parked escalated change: %s", j)
		}
		return run.Result{}, false
	})
	if _, err := newTestCmd(r, "finish", "reconcile", "weft-e", "--json"); err != nil {
		t.Fatalf("execute: %v", err)
	}
}
```

- [ ] **Step 3: Implement the guard (only if Step 1 requires it)**

If `-b @` is safe (does not reach the parked sibling), **no code change** is needed — keep the test as a regression guard and proceed. If `-b @` drags the sibling, change the merge-commit branch in `newFinishReconcileCmd` to scope the rebase to the collapsed line root instead of the whole `-b @` branch:

```go
			case mergeStyleMergeCommit:
				// Scope to @'s line so a parked escalated sibling under trunk() is
				// not dragged into the rebase.
				rootRes, err := run.JJ(a.Runner, "log", "-r", "roots(trunk()..@)", "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
				if err != nil {
					return exit.Hardf("jj log roots could not run: %v", err)
				}
				root := strings.TrimSpace(splitTrimLines(rootRes.Stdout)[0])
				if res, err := run.JJ(a.Runner, "rebase", "-s", root, "-o", "main", "--skip-emptied"); err != nil {
					return exit.Hardf("jj rebase could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj rebase failed: %s", strings.TrimSpace(res.Stderr))
				}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/cli/ -run TestFinishReconcile -v`
Expected: PASS (existing reconcile tests + the new guard).

- [ ] **Step 5: Commit**

Suggested: `fix(weft-eoe): finish reconcile leaves parked escalated pick off the merged line (seam 11)`.

---

### Task 6: Update `execute.md` + plugin `SKILL.md` (loop + envelope)

The `shed.integrate` envelope is now `groups` not `stack`, and the orchestrator no longer does "escalate-last ordering" — grouping makes it unnecessary. Update the Steps 5–6 prose and the envelope shape in BOTH copies (seam-7 lockstep).

**Files:**
- Modify: `weft/workflows/execute.md`
- Modify: `plugin/skills/execute/SKILL.md`

This is a docs change (TDD-exempt; no runtime behavior).

- [ ] **Step 1: Locate the affected prose in both files**

Run: `rg -n '"stack"|escalate-last|escalat|Steps 5|integrate' weft/workflows/execute.md plugin/skills/execute/SKILL.md`
Identify: (a) the `shed.integrate` envelope example showing `"stack"`, and (b) the Steps 5–6 escalate-last-ordering guidance added in `weft-w1y.6`.

- [ ] **Step 2: Rewrite the envelope + loop prose (both files, byte-identical bodies)**

Change the envelope example from `"stack": [{bead, change}]` to `"groups": [[{bead, change}], …]`. Replace the escalate-last-ordering paragraph with: one `integrate` yields a forest with conflicts confined to file-overlap groups; resolve each group's conflict (heal un-cascades within the group; escalate leaves the group tail for a human); land the conflict-free picks; the fixpoint is per-group and needs no global ordering. The escalated pick is parked by `finish open`, not reordered.

- [ ] **Step 3: Verify lockstep + no stale citations**

Run: `rg -n '"stack"|escalate-last' weft/workflows/execute.md plugin/skills/execute/SKILL.md`
Expected: no matches (both updated). Run the plugin grep-discipline used in CI:
`grep -RnE 'weft/(agents|references|workflows)/' plugin/skills/execute/SKILL.md` — expect no output.

- [ ] **Step 4: Commit**

Suggested: `docs(weft-eoe): execute.md loop + integrate envelope reflect overlap forest (seam 11)`.

---

### Task 7: Single-wave E2E — one `integrate` over the full wave

Rewrite `TestWeaveLoopEndToEnd` to drive **one** `shed integrate` over all 6 fixture picks and prove the grouped forest yields a deterministic 5-land + 1-escalate, then `finish open` collapses the 5 (excluding the escalated tail). This is the `weft-78k` follow-up the whole seam exists to enable.

**Files:**
- Modify: `internal/weave/weave_integration_test.go`

- [ ] **Step 1: Replace the two-call structure with one integrate**

Delete the `wave1`/`wave2` split (lines ~45-69), Steps 5a-8b, the `integ1Data`/`integ2Data` anonymous structs (which decode the removed `"stack"` field), and the now-unused `sortConflictsByStackPos` helper (~line 323). After Step 3+4 (dispatch + verify + seal, unchanged), integrate the whole wave:

```go
	// --- Step 5: integrate the WHOLE wave (one call) ---
	integ := r.runWeft(t, "", append([]string{"shed", "integrate"}, wave...)...)
	var integData struct {
		Groups    [][]struct{ Bead, Change string } `json:"groups"`
		Conflicts []struct{ Bead, Change string }   `json:"conflicts"`
	}
	if err := json.Unmarshal(integ.Data, &integData); err != nil {
		t.Fatalf("parse integrate data: %v", err)
	}
	// The forest confines conflicts to the two overlap pairs — exactly 2, no cascade.
	if len(integData.Conflicts) != 2 {
		t.Fatalf("conflicts = %d, want exactly 2 (p2 + p4 colliders, no cross-group cascade): %s",
			len(integData.Conflicts), integ.Data)
	}
```

- [ ] **Step 2: Resolve each conflict by ref (heal p2, escalate p4)**

```go
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
```

- [ ] **Step 3: Land the 5 conflict-free picks; escalated one refuses**

```go
	// --- Step 7: land every non-escalated pick (the escalated tail fails the gate) ---
	for _, bead := range wave {
		if bead == escalatedBead {
			continue
		}
		r.runWeft(t, "", "pick", "land", bead)
	}
```

- [ ] **Step 4: Keep Steps 8–9 (cleanup/reap/resume) and assertions**

Cleanup the landed picks' workspaces and reap (as today, but over all non-escalated beads). The Step 9 `resume` assertions are unchanged and already correct: `len(landed) == 5`, `len(blocked) == 0`, escalated bead in `in_flight`, `escalatedChange` ∈ `resume.data.conflicts`, `human` label present, post-loop `form` empty. Update any `c2.Change`/`c2.Bead` references to `escalatedChange`/`escalatedBead`.

- [ ] **Step 5: Add the finish-collapse assertion**

After resume, prove `finish open --dry-run` would ship the 5 landed picks and not the escalated one:

```go
	// --- Step 10: finish open dry-run ships the 5 landed picks, not the escalated one ---
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
```

Update the comment block at the top of `TestWeaveLoopEndToEnd` (lines 20-33) to describe the single-integrate forest strategy, removing the two-call rationale.

- [ ] **Step 6: Run the E2E under simulated-CI no-identity**

Run: `JJ_CONFIG=/tmp/empty.toml go test -tags integration ./internal/weave/ -run TestWeaveLoopEndToEnd -count=1 -v` (create `/tmp/empty.toml` empty first).
Expected: PASS — deterministic regardless of bd-assigned id order (the original `weft-78k` blocker is gone). Then run the full package: `go test -tags integration ./internal/weave/ -count=1`.

- [ ] **Step 7: Commit**

Suggested: `test(weft-eoe): single-wave weave E2E over one integrate (closes weft-78k follow-up) (seam 11)`.

---

## Verification (whole plan)

- `go test ./internal/cli/ -count=1` — all unit tests green (overlap, integrate forest, finish collapse, reconcile guard).
- `JJ_CONFIG=/tmp/empty.toml go test -tags integration ./internal/weave/ -count=1` — the weave package green under CI-like no-identity.
- `go vet ./...` and `go build ./...` clean.
- `rg -n '"stack"' weft/workflows/execute.md plugin/skills/execute/SKILL.md internal/cli/shed.go` — no stale `stack` envelope references remain.

## Open items carried from design review (verify during implementation)

- **`jj rebase -r` re-parenting** (Task 4 Step 1): confirm lifting a landed parent leaves a conflicted descendant parked on `trunk()`, off `::@`.
- **`finish reconcile` merge-commit branch** (Task 5 Step 1): confirm `-b @` excludes the parked sibling; add the scoping guard only if it does not.
- **Collapse ancestor-order query** (Task 4 Step 1): confirm `reverse(<changes>)` yields a usable ancestors-first order for the per-change `-r` collapse; adjust the query if jj 0.42 orders differently.
<!-- adr-capture: sha256=f64b04a2a059339f; session=cli; ts=2026-06-09T01:19:45Z; adrs=weft-90x -->
