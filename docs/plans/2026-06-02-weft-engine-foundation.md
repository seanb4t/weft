<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft Engine Foundation & Output Contract — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the `weft` Go binary with its full output contract (text default, `--json` envelope, `--pick` extraction, engine-success exit codes) and two thin read verbs (`shed form`, `ws list`) that exercise the bd and jj subprocess layers end-to-end.

**Architecture:** A cobra-based CLI (`cmd/weft` → `internal/cli`) whose verbs return typed errors mapped to exit codes (`internal/exit`), render results through a uniform `Envelope` (`internal/envelope`), and shell out to `bd`/`jj` through an injectable `Runner` (`internal/run`) so verbs are unit-testable with a fake. This is seam 1's *foundation only* — the coarse orchestration verbs (`shed integrate/isolate/cleanup/abandon`, `pick *`, `finish`, `conflict`) depend on seams 3/4 and land in follow-on plans.

**Tech Stack:** Go 1.26, `github.com/spf13/cobra` v1.9.x, stdlib `os/exec` + `encoding/json`. Subprocesses: `bd` (beads), `jj` (jujutsu).

**Spec:** `docs/seams/01-command-surface.md` (design-reviewer READY). Output contract = spec §3; verbs = spec §4.1 (`shed form`), §4.3 (`ws list`).

---

## File Structure

| File | Responsibility |
|---|---|
| `cmd/weft/main.go` | Entry point: build the root command, `Execute()`, map error → exit code. |
| `internal/exit/exit.go` | Exit-code taxonomy: `*Error{Code}`, `Invocation`/`Hard` constructors, `Code(err)`. |
| `internal/envelope/envelope.go` | The uniform `Envelope` struct + `JSON()` rendering. |
| `internal/envelope/pick.go` | `Pick(env, path)` — dot/bracket field extraction (the `--pick` engine). |
| `internal/run/run.go` | `Runner` interface, `Exec` real impl, `JJ`/`BD` helpers (auto `--no-pager`). |
| `internal/cli/root.go` | `App` (holds `Runner`), `NewRootCmd`, persistent `--json`/`--pick`, flag-error→exit wiring. |
| `internal/cli/emit.go` | `Emit(cmd, verb, data, text)` — the text/json/pick output switch. |
| `internal/cli/version.go` | `weft version` verb (first end-to-end Emit consumer). |
| `internal/cli/ws.go` | `weft ws list` verb (jj-backed). |
| `internal/cli/shed.go` | `weft shed form` verb (bd-backed). |

Tests live beside each file (`*_test.go`). `internal/exit`, `internal/envelope`, `internal/run` are pure/unit-testable foundations built first; the CLI bootstrap and verbs build on them.

---

### Task 1: Exit-code taxonomy

**Files:**
- Create: `internal/exit/exit.go`
- Test: `internal/exit/exit_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/exit/exit_test.go
package exit

import (
	"errors"
	"testing"
)

func TestCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil is ok", nil, 0},
		{"invocation is 1", Invocation(errors.New("bad flag")), 1},
		{"invocationf is 1", Invocationf("missing %s", "--epic"), 1},
		{"hard is 2", Hard(errors.New("jj blew up")), 2},
		{"untyped error defaults to hard", errors.New("surprise"), 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Code(c.err); got != c.want {
				t.Fatalf("Code(%v) = %d, want %d", c.err, got, c.want)
			}
		})
	}
}

func TestUnwrap(t *testing.T) {
	base := errors.New("root cause")
	if !errors.Is(Hard(base), base) {
		t.Fatal("Hard should unwrap to its cause")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/exit/`
Expected: FAIL — `undefined: Invocation` (package has no implementation yet).

- [ ] **Step 3: Write minimal implementation**

```go
// internal/exit/exit.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package exit defines the weft engine's exit-code taxonomy: the code reflects
// whether the engine did its job, never the verdict of the work (spec §3).
package exit

import (
	"errors"
	"fmt"
)

// Error carries an engine exit code alongside its cause.
//
//	1 = invocation error (bad args, missing workspace, unknown bead)
//	2 = hard failure (an underlying bd/jj/gh command failed)
type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

// Invocation wraps err as exit code 1.
func Invocation(err error) *Error { return &Error{Code: 1, Err: err} }

// Invocationf formats an exit-code-1 error.
func Invocationf(format string, a ...any) *Error {
	return &Error{Code: 1, Err: fmt.Errorf(format, a...)}
}

// Hard wraps err as exit code 2.
func Hard(err error) *Error { return &Error{Code: 2, Err: err} }

// Hardf formats an exit-code-2 error.
func Hardf(format string, a ...any) *Error {
	return &Error{Code: 2, Err: fmt.Errorf(format, a...)}
}

// Code returns the engine exit code for err: 0 for nil, the typed code for an
// *Error, and 2 (hard) for any other error.
func Code(err error) int {
	if err == nil {
		return 0
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return 2
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/exit/`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(exit): exit-code taxonomy for the engine contract"`

---

### Task 2: Envelope + JSON rendering

**Files:**
- Create: `internal/envelope/envelope.go`
- Test: `internal/envelope/envelope_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/envelope/envelope_test.go
package envelope

import (
	"encoding/json"
	"testing"
)

func TestJSONShape(t *testing.T) {
	e := Envelope{OK: true, Verb: "version", Data: map[string]string{"version": "0.0.0-dev"}}
	b, err := e.JSON()
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if got["ok"] != true {
		t.Errorf("ok = %v, want true", got["ok"])
	}
	if got["verb"] != "version" {
		t.Errorf("verb = %v, want version", got["verb"])
	}
	if _, present := got["next"]; present {
		t.Error("empty next must be omitted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/envelope/`
Expected: FAIL — `undefined: Envelope`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/envelope/envelope.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package envelope is the uniform JSON shape every weft verb emits under
// --json (spec §3.1). Optional fields (Next, and later Conflicts) are omitted
// when empty. The struct is intentionally extensible: later seams add fields.
package envelope

import "encoding/json"

// Envelope is the uniform result shape. Data is verb-specific.
type Envelope struct {
	OK   bool   `json:"ok"`
	Verb string `json:"verb"`
	Data any    `json:"data"`
	Next string `json:"next,omitempty"` // advisory hint; never authoritative
}

// JSON renders the envelope as indented JSON.
func (e Envelope) JSON() ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/envelope/`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(envelope): uniform result envelope + JSON rendering"`

---

### Task 3: `Pick` field extractor (the `--pick` engine)

**Files:**
- Create: `internal/envelope/pick.go`
- Test: `internal/envelope/pick_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/envelope/pick_test.go
package envelope

import "testing"

func TestPick(t *testing.T) {
	e := Envelope{
		OK:   true,
		Verb: "shed.form",
		Data: map[string]any{
			"epic": "weft-hjx",
			"wave": []any{"weft-a1", "weft-a2"},
		},
	}
	t.Run("top-level field", func(t *testing.T) {
		got, err := Pick(e, "verb")
		if err != nil || got != "shed.form" {
			t.Fatalf("Pick(verb) = %v, %v", got, err)
		}
	})
	t.Run("nested field", func(t *testing.T) {
		got, err := Pick(e, "data.epic")
		if err != nil || got != "weft-hjx" {
			t.Fatalf("Pick(data.epic) = %v, %v", got, err)
		}
	})
	t.Run("array index", func(t *testing.T) {
		got, err := Pick(e, "data.wave[1]")
		if err != nil || got != "weft-a2" {
			t.Fatalf("Pick(data.wave[1]) = %v, %v", got, err)
		}
	})
	t.Run("missing path errors", func(t *testing.T) {
		if _, err := Pick(e, "data.nope"); err == nil {
			t.Fatal("expected error for missing path")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/envelope/ -run TestPick`
Expected: FAIL — `undefined: Pick`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/envelope/pick.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package envelope

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Pick extracts a single value from the envelope by dot/bracket path, e.g.
// "data.wave[0]" (spec §3). It walks the marshaled JSON so the path matches
// exactly what --json would emit. Returns an error if any segment is missing.
func Pick(e Envelope, path string) (any, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	var cur any
	if err := json.Unmarshal(b, &cur); err != nil {
		return nil, err
	}
	for _, seg := range splitPath(path) {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg]
			if !ok {
				return nil, fmt.Errorf("pick: no field %q in path %q", seg, path)
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(node) {
				return nil, fmt.Errorf("pick: bad array index %q in path %q", seg, path)
			}
			cur = node[i]
		default:
			return nil, fmt.Errorf("pick: cannot descend into %q at %q", seg, path)
		}
	}
	return cur, nil
}

// splitPath turns "data.wave[0]" into ["data", "wave", "0"].
func splitPath(path string) []string {
	norm := strings.NewReplacer("[", ".", "]", "").Replace(path)
	var out []string
	for _, s := range strings.Split(norm, ".") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/envelope/`
Expected: PASS (both envelope and pick tests).

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(envelope): --pick dot/bracket path extraction"`

---

### Task 4: `Runner` abstraction + `Exec` + bd/jj helpers

**Files:**
- Create: `internal/run/run.go`
- Test: `internal/run/run_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/run/run_test.go
package run

import "testing"

func TestExecCapturesStdoutAndZeroExit(t *testing.T) {
	res, err := Exec{}.Run("echo", "hello")
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if res.Code != 0 {
		t.Errorf("Code = %d, want 0", res.Code)
	}
	if res.Stdout != "hello\n" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "hello\n")
	}
}

func TestExecNonZeroExitIsNotAnError(t *testing.T) {
	res, err := Exec{}.Run("false")
	if err != nil {
		t.Fatalf("non-zero exit must not be a Go error, got %v", err)
	}
	if res.Code == 0 {
		t.Error("Code should be non-zero for `false`")
	}
}

func TestExecMissingBinaryIsAnError(t *testing.T) {
	if _, err := Exec{}.Run("definitely-not-a-real-binary-xyz"); err == nil {
		t.Fatal("expected an error when the binary cannot start")
	}
}

func TestJJPrependsNoPager(t *testing.T) {
	var fake fakeRunner
	_, _ = JJ(&fake, "status")
	want := []string{"--no-pager", "status"}
	if fake.name != "jj" || !equal(fake.args, want) {
		t.Errorf("JJ ran %s %v, want jj %v", fake.name, fake.args, want)
	}
}

type fakeRunner struct {
	name string
	args []string
}

func (f *fakeRunner) Run(name string, args ...string) (Result, error) {
	f.name, f.args = name, args
	return Result{}, nil
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/run/`
Expected: FAIL — `undefined: Exec` / `undefined: JJ`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/run/run.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package run is the engine's subprocess layer. Verbs depend on the Runner
// interface so they can be unit-tested with a fake; Exec is the real impl.
package run

import (
	"bytes"
	"errors"
	"os/exec"
)

// Result is the captured outcome of a subprocess.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// Runner runs an external command and captures its output. A non-zero exit is
// reported in Result.Code, NOT as a Go error; err is non-nil only when the
// command could not start.
type Runner interface {
	Run(name string, args ...string) (Result, error)
}

// Exec is the real os/exec-backed Runner.
type Exec struct{}

func (Exec) Run(name string, args ...string) (Result, error) {
	cmd := exec.Command(name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	res := Result{Stdout: out.String(), Stderr: errb.String()}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		res.Code = ee.ExitCode()
		return res, nil // ran to completion with a non-zero exit
	}
	if err != nil {
		return res, err // could not start (e.g. binary not found)
	}
	return res, nil
}

// JJ runs jj with --no-pager always prepended (agent-safety profile).
func JJ(r Runner, args ...string) (Result, error) {
	return r.Run("jj", append([]string{"--no-pager"}, args...)...)
}

// BD runs the beads CLI.
func BD(r Runner, args ...string) (Result, error) {
	return r.Run("bd", args...)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/run/`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(run): subprocess Runner with jj/bd helpers"`

---

### Task 5: CLI bootstrap — root command, `Emit`, and `weft version`

**Files:**
- Create: `cmd/weft/main.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/emit.go`
- Create: `internal/cli/version.go`
- Test: `internal/cli/version_test.go`
- Delete: `cmd/weft/.gitkeep`, `internal/.gitkeep` (real files now exist)

- [ ] **Step 1: Add the cobra dependency**

Run: `go get github.com/spf13/cobra@v1.9.1 && go mod tidy`
Expected: `go.mod` gains `require github.com/spf13/cobra v1.9.1`.

- [ ] **Step 2: Write the failing test**

```go
// internal/cli/version_test.go
package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/run"
)

// newTestCmd builds a root command wired to a fake runner with captured output.
func newTestCmd(fake run.Runner, args ...string) (*bytes.Buffer, error) {
	out := &bytes.Buffer{}
	root := NewRootCmd(&App{Runner: fake})
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs(args)
	return out, root.Execute()
}

// scriptedRunner is the shared fake Runner for verb tests: it returns a fixed
// Result and records the call. Defined here (Task 5) so Tasks 6–7 can reuse it
// without a cross-task ordering dependency. (Unused in version_test itself —
// that is fine; an unused test type does not fail compilation.)
type scriptedRunner struct {
	res     run.Result
	gotName string
	gotArgs []string
}

func (s *scriptedRunner) Run(name string, args ...string) (run.Result, error) {
	s.gotName, s.gotArgs = name, args
	return s.res, nil
}

func TestVersionText(t *testing.T) {
	out, err := newTestCmd(nil, "version")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "weft "+Version) {
		t.Errorf("text output = %q, want it to contain %q", out.String(), "weft "+Version)
	}
}

func TestVersionJSON(t *testing.T) {
	out, err := newTestCmd(nil, "version", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"verb": "version"`) || !strings.Contains(s, `"ok": true`) {
		t.Errorf("json output missing envelope fields: %q", s)
	}
}

func TestVersionPick(t *testing.T) {
	out, err := newTestCmd(nil, "version", "--pick", "data.version")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.TrimSpace(out.String()) != Version {
		t.Errorf("pick output = %q, want %q", strings.TrimSpace(out.String()), Version)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/cli/`
Expected: FAIL — `undefined: NewRootCmd` / `undefined: App`.

- [ ] **Step 4: Write the root command**

```go
// internal/cli/root.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package cli wires the weft verb surface (spec §4) onto cobra. Verbs reach
// bd/jj through App.Runner so they are unit-testable with a fake.
package cli

import (
	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/spf13/cobra"
)

// App holds the engine's injectable dependencies.
type App struct {
	Runner run.Runner
}

// NewRootCmd builds the weft root command and its verb tree.
func NewRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:           "weft",
		Short:         "Weft — spec-driven AI dev orchestration on beads + jj",
		SilenceUsage:  true, // don't dump usage on every RunE error
		SilenceErrors: true, // main prints errors and sets the exit code
	}
	// Output contract (spec §3): default text, --json envelope, --pick field.
	root.PersistentFlags().Bool("json", false, "emit the uniform JSON envelope")
	root.PersistentFlags().String("pick", "", "extract one field by path (e.g. data.wave[0])")
	// Map cobra flag-parse errors to exit code 1 (invocation error).
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return exit.Invocation(err)
	})

	root.AddCommand(newVersionCmd())
	root.AddCommand(app.newShedCmd())
	root.AddCommand(app.newWsCmd())
	return root
}
```

- [ ] **Step 5: Write the `Emit` helper**

```go
// internal/cli/emit.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/seanb4t/weft/internal/envelope"
	"github.com/seanb4t/weft/internal/exit"
	"github.com/spf13/cobra"
)

// Emit renders a verb's result per the output contract (spec §3): --pick wins,
// then --json, else the human text. It writes to the command's out stream.
func Emit(cmd *cobra.Command, verb string, data any, text string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	pick, _ := cmd.Flags().GetString("pick")
	env := envelope.Envelope{OK: true, Verb: verb, Data: data}
	out := cmd.OutOrStdout()

	switch {
	case pick != "":
		v, err := envelope.Pick(env, pick)
		if err != nil {
			return exit.Invocation(err)
		}
		fmt.Fprintln(out, pickString(v))
	case jsonOut:
		b, err := env.JSON()
		if err != nil {
			return exit.Hard(err)
		}
		fmt.Fprintln(out, string(b))
	default:
		fmt.Fprintln(out, text)
	}
	return nil
}

// pickString prints scalars raw and structures as compact JSON.
func pickString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
```

- [ ] **Step 6: Write the `version` verb**

```go
// internal/cli/version.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import "github.com/spf13/cobra"

// Version is the engine version (overridable via -ldflags at build time later).
const Version = "0.0.0-dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the weft version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Emit(cmd, "version", map[string]string{"version": Version}, "weft "+Version)
		},
	}
}
```

- [ ] **Step 7: Write `main.go`**

```go
// cmd/weft/main.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package main

import (
	"fmt"
	"os"

	"github.com/seanb4t/weft/internal/cli"
	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

func main() {
	root := cli.NewRootCmd(&cli.App{Runner: run.Exec{}})
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exit.Code(err))
	}
}
```

- [ ] **Step 8: Remove the placeholder .gitkeep files**

Run: `rm cmd/weft/.gitkeep internal/.gitkeep`

- [ ] **Step 9: Run tests to verify they pass**

Run: `go test ./internal/cli/`
Expected: PASS (TestVersionText, TestVersionJSON, TestVersionPick).
(Note: `newTestCmd(nil, ...)` passes a nil Runner — fine, because `version` never touches it. `newShedCmd`/`newWsCmd` are referenced by `NewRootCmd`; stub them in Tasks 6–7. To compile this task standalone, add temporary stubs returning `&cobra.Command{Use: "shed"}` and `&cobra.Command{Use: "ws"}` as App methods, replaced in Tasks 6–7.)

- [ ] **Step 10: Build the binary**

Run: `go build ./cmd/weft && ./weft version`
Expected: prints `weft 0.0.0-dev`.

- [ ] **Step 11: Commit**

Run: `jj commit -m "feat(cli): cobra root, output contract (--json/--pick), version verb"`

---

### Task 6: `weft ws list` (jj-backed thin verb)

**Files:**
- Create: `internal/cli/ws.go` (replaces the Task-5 stub)
- Test: `internal/cli/ws_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/ws_test.go
package cli

import (
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/run"
)

// scriptedRunner is defined in version_test.go (Task 5) — shared across verb tests.

func TestWsListParsesWorkspaceNames(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Stdout: "default\nweft-a1\nweft-a2\n", Code: 0}}
	out, err := newTestCmd(fake, "ws", "list", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"weft-a1"`) || !strings.Contains(s, `"default"`) {
		t.Errorf("expected workspace names in output: %q", s)
	}
	// Verify it shelled out to jj with --no-pager and a name template.
	if fake.gotName != "jj" || fake.gotArgs[0] != "--no-pager" {
		t.Errorf("ran %s %v, want jj --no-pager ...", fake.gotName, fake.gotArgs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestWsList`
Expected: FAIL — the Task-5 `ws` stub has no `list` subcommand, so execution errors on unknown command.

- [ ] **Step 3: Write the implementation**

```go
// internal/cli/ws.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/spf13/cobra"
)

func (a *App) newWsCmd() *cobra.Command {
	ws := &cobra.Command{Use: "ws", Short: "Workspace escape hatches (spec §4.3)"}
	ws.AddCommand(a.newWsListCmd())
	return ws
}

func (a *App) newWsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List jj workspaces",
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := run.JJ(a.Runner, "workspace", "list", "-T", `name ++ "\n"`)
			if err != nil {
				return exit.Hardf("jj workspace list could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj workspace list failed: %s", strings.TrimSpace(res.Stderr))
			}
			var names []string
			for _, ln := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
				if ln = strings.TrimSpace(ln); ln != "" {
					names = append(names, ln)
				}
			}
			data := map[string]any{"workspaces": names}
			return Emit(cmd, "ws.list", data, fmt.Sprintf("workspaces: %s", strings.Join(names, " ")))
		},
	}
}
```

(Delete the temporary `newWsCmd` stub added in Task 5.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestWsList`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(cli): ws list verb (jj workspace list wrapper)"`

---

### Task 7: `weft shed form` (bd-backed thin verb)

**Files:**
- Create: `internal/cli/shed.go` (replaces the Task-5 stub)
- Test: `internal/cli/shed_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/shed_test.go
package cli

import (
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

func TestShedFormBuildsWaveFromBdReady(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{
		Stdout: `[{"id":"weft-a1","title":"x"},{"id":"weft-a2","title":"y"}]`,
		Code:   0,
	}}
	out, err := newTestCmd(fake, "shed", "form", "--epic", "weft-hjx", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"weft-a1"`) || !strings.Contains(s, `"weft-a2"`) {
		t.Errorf("wave missing expected picks: %q", s)
	}
	// Verify it scoped bd ready to the epic.
	joined := strings.Join(fake.gotArgs, " ")
	if fake.gotName != "bd" || !strings.Contains(joined, "ready") || !strings.Contains(joined, "--parent weft-hjx") {
		t.Errorf("ran %s %v, want bd ready --parent weft-hjx ...", fake.gotName, fake.gotArgs)
	}
}

func TestShedFormRequiresEpic(t *testing.T) {
	_, err := newTestCmd(&scriptedRunner{}, "shed", "form")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("missing --epic should be exit code 1, got %d (err=%v)", got, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestShedForm`
Expected: FAIL — the Task-5 `shed` stub has no `form` subcommand.

- [ ] **Step 3: Write the implementation**

```go
// internal/cli/shed.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/spf13/cobra"
)

func (a *App) newShedCmd() *cobra.Command {
	shed := &cobra.Command{Use: "shed", Short: "Wave-level orchestration (spec §4.1)"}
	shed.AddCommand(a.newShedFormCmd())
	return shed
}

func (a *App) newShedFormCmd() *cobra.Command {
	var epic string
	var max int
	c := &cobra.Command{
		Use:   "form",
		Short: "Form a shed: the ready wave for an epic (bd ready ∩ epic, capped)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if epic == "" {
				return exit.Invocationf("--epic is required")
			}
			res, err := run.BD(a.Runner, "ready", "--parent", epic, "--limit", strconv.Itoa(max), "--json")
			if err != nil {
				return exit.Hardf("bd ready could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("bd ready failed: %s", strings.TrimSpace(res.Stderr))
			}
			var issues []struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal([]byte(res.Stdout), &issues); err != nil {
				return exit.Hardf("parse bd ready json: %v", err)
			}
			wave := make([]string, 0, len(issues))
			for _, i := range issues {
				wave = append(wave, i.ID)
			}
			data := map[string]any{"epic": epic, "wave": wave}
			text := fmt.Sprintf("shed for %s: %s (%d picks)", epic, strings.Join(wave, " "), len(wave))
			return Emit(cmd, "shed.form", data, text)
		},
	}
	c.Flags().StringVar(&epic, "epic", "", "epic bead-id scoping the ready set (required)")
	// --max is the parallelism dial; its config-file default is seam 3. Plan-1
	// uses a fixed default of 5.
	c.Flags().IntVar(&max, "max", 5, "max wave size (parallelism dial)")
	return c
}
```

(Delete the temporary `newShedCmd` stub added in Task 5.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/`
Expected: PASS (all cli tests: version, ws list, shed form).

- [ ] **Step 5: Run the full suite + vet**

Run: `go vet ./... && go test ./...`
Expected: PASS, no vet complaints.

- [ ] **Step 6: Commit**

Run: `jj commit -m "feat(cli): shed form verb (bd ready wrapper, epic-scoped wave)"`

---

## Done criteria

- `go test ./...` passes; `go vet ./...` clean.
- `go build ./cmd/weft` produces a `weft` binary.
- `weft version`, `weft version --json`, `weft version --pick data.version` exercise all three output modes.
- `weft shed form --epic <id>` and `weft ws list` run against real `bd`/`jj` in a beads+jj repo.
- Exit codes honor the contract: `0` success, `1` invocation error (e.g. missing `--epic`), `2` hard failure (bd/jj failed).

## Out of scope (follow-on plans)

- Coarse verbs `shed isolate/integrate/cleanup/abandon`, `pick seal/verify/land/redo`, `finish open/reconcile`, `conflict open/finalize`, `resume` — they need the workspace lifecycle (seam 3), conflict resolution (seam 4), and jj rebase/squash choreography.
- `.weft/config.toml` (the `--max` default home) — seam 3.
- The `Conflicts` envelope field — added with `shed integrate`.
- `--json-errors` (structured error objects) — errors here go to stderr as text
  (the spec §3 default); the structured form is a spec §6 open sub-seam.
- Build tooling (Taskfile, golangci, goreleaser) — deferred per design.md §8.
<!-- adr-capture: sha256=d0573e3814b6efb3; session=cli; ts=2026-06-02T14:44:28Z; adrs=weft-re2 -->
