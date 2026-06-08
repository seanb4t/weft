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

	"github.com/seanb4t/weft/internal/workspace"
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
	weftBin = filepath.Join(dir, "weft")
	// Build from the module root (two levels up from internal/weave).
	build := exec.Command("go", "build", "-o", weftBin, "../../cmd/weft")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		os.Stderr.WriteString("weave: go build weft: " + err.Error() + "\n")
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
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
	// Configure a jj identity at the repo level. CI runners have no user-level
	// jj identity, so commits would be authored " <>" and `jj git push` rejects
	// them ("no author and/or committer set"). Repo-level config is shared across
	// all workspaces of the repo, so it also covers weft's own jj operations. Set
	// before the base commit below: `describe` re-authors an empty-author commit
	// from the now-configured identity.
	r.mustJJ(t, "config", "set", "--repo", "user.name", "Weft CI")
	r.mustJJ(t, "config", "set", "--repo", "user.email", "weft-ci@example.com")
	// An initial commit so trunk() resolves to a real base for workspaces.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/.weft-bin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.mustJJ(t, "describe", "-m", "root: scratch base")

	// Push the base commit to a local bare remote so `jj git fetch` (called by
	// `shed isolate`) has at least one remote and does not error with
	// "No git remotes to fetch from". We use `jj git push` (not git push) so
	// jj's export layer is coherent.
	bareDir := t.TempDir()
	mustGit(t, bareDir, "init", "--bare")
	// Configure the remote via git (read-only config — jj colocated repos
	// share .git/config, so this is safe and does not trigger the mutating-git guard).
	mustGit(t, root, "remote", "add", "origin", bareDir)
	// Create a bookmark at the described base and push it so origin has ≥1 ref.
	r.mustJJ(t, "bookmark", "create", "main", "-r", "@")
	r.mustJJ(t, "git", "push", "--bookmark", "main")

	r.mustJJ(t, "new") // leave an empty working copy on top of the described base

	// Create the sibling worktrees directory that jj workspace add expects.
	// weft's workspace.Root returns filepath.Dir(root) + "/" + filepath.Base(root) + "_worktrees".
	worktreesDir := filepath.Join(filepath.Dir(root), filepath.Base(root)+"_worktrees")
	if err := os.MkdirAll(worktreesDir, 0o755); err != nil {
		t.Fatal(err)
	}

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

// lastJSONLine returns the last top-level JSON object in b. It handles both
// single-line ({"ok":true,...}) and pretty-printed multi-line output.
//
// The scanner is string-aware: it tracks whether the current position is inside
// a double-quoted string (honoring backslash escapes) so that braces inside
// string values (e.g. "text":"merged {a,b}") are not counted toward the
// brace-balance depth. Algorithm:
//
//  1. Forward-scan the whole input building a brace-depth array; entries inside
//     strings are marked as depth -1 (inString).
//  2. Find the last position where depth returns to 0 after opening at depth 1
//     — that is the end of the last top-level object.
//  3. Walk backwards from that end to find its matching '{' at depth 0→1.
func lastJSONLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return ""
	}

	// Forward pass: record the brace depth at every character, skipping string
	// contents so embedded braces don't affect the count.
	n := len(s)
	depth := make([]int, n) // depth[i] == depth of s[i] after processing it
	cur := 0
	inStr := false
	for i := 0; i < n; i++ {
		c := s[i]
		if inStr {
			if c == '\\' {
				// Escape sequence: mark both bytes as inside-string (depth 0 means
				// "irrelevant to brace matching"; use cur unchanged).
				depth[i] = cur
				i++
				if i < n {
					depth[i] = cur
				}
				continue
			}
			if c == '"' {
				inStr = false
			}
			depth[i] = cur
			continue
		}
		// Not in a string.
		switch c {
		case '"':
			inStr = true
			depth[i] = cur
		case '{':
			cur++
			depth[i] = cur
		case '}':
			depth[i] = cur
			cur--
		default:
			depth[i] = cur
		}
	}

	// Find the last index where depth returns to 0 from 1 — the closing '}'
	// of the last top-level object.
	end := -1
	for i := n - 1; i >= 0; i-- {
		if s[i] == '}' && depth[i] == 1 {
			end = i
			break
		}
	}
	if end < 0 {
		return ""
	}

	// Walk backwards from end to find the matching '{' (where depth goes 0→1).
	for i := end - 1; i >= 0; i-- {
		if s[i] == '{' && depth[i] == 1 {
			return s[i : end+1]
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

// mustGit runs a git command in the given directory, fataling on error.
// Used only for scratch-repo setup (bare remote) where jj doesn't apply.
func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s (dir=%s): %v\n%s", strings.Join(args, " "), dir, err, out)
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
// Uses workspace.Name (== workspace.Sanitize) so the harness stays in lockstep
// if the sanitization rule changes (e.g. "wv-abc.1" → "wv-abc__1"). The
// on-disk layout mirrors workspace.Root with empty cfgRoot:
// <jjRoot>_worktrees/<sanitized-bead-id>.
func (r *scratchRepo) workspacePath(t *testing.T, bead string) string {
	t.Helper()
	return filepath.Join(filepath.Dir(r.root), filepath.Base(r.root)+"_worktrees", workspace.Name(bead))
}

func execBD(r *scratchRepo, args ...string) *exec.Cmd {
	cmd := exec.Command("bd", args...)
	cmd.Dir = r.root
	cmd.Env = append(os.Environ(), "BEADS_DIR="+r.beadsDir)
	return cmd
}

// dataBool extracts data.<key> as a bool from an envelope's Data.
func dataBool(t *testing.T, data json.RawMessage, key string) bool {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("data not an object: %v", err)
	}
	raw, ok := m[key]
	if !ok {
		t.Fatalf("data has no key %q: %s", key, data)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("data.%s not bool: %v", key, err)
	}
	return b
}

// dataString extracts data.<key> as a string from an envelope's Data.
func dataString(t *testing.T, data json.RawMessage, key string) string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("data not an object: %v", err)
	}
	raw, ok := m[key]
	if !ok {
		t.Fatalf("data has no key %q: %s", key, data)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("data.%s not string: %v", key, err)
	}
	return s
}
