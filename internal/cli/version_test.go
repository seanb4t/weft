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
