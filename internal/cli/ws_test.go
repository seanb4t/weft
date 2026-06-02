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

func TestWsListEmptyEmitsJSONArrayNotNull(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Stdout: "", Code: 0}}
	out, err := newTestCmd(fake, "ws", "list", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if s := out.String(); !strings.Contains(s, `"workspaces": []`) {
		t.Errorf("empty workspace list must serialize as [], not null: %q", s)
	}
}
