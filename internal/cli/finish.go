// internal/cli/finish.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

// finishPick is one closed pick: its bead id, conventional-commit subject
// (bead title), and woven jj change-id. The PR body is assembled from these
// (spec §5, design.md §5.1 audit — no SUMMARY.md).
type finishPick struct {
	Bead   string `json:"bead"`
	Title  string `json:"title"`
	Change string `json:"change"`
}

// closedPicks reads the epic's closed children in ONE bd list call, yielding
// bead id + title + jj-change label together. (beadIDsByStatus returns only
// ids; epicChanges returns only change-ids — neither alone carries the title,
// so finish open needs this combined reader; see spec §5.) Returns a non-nil
// empty slice for an epic with no closed children.
func closedPicks(r run.Runner, epic string) ([]finishPick, error) {
	res, err := run.BD(r, "list", "--parent", epic, "--status", "closed", "--json")
	if err != nil {
		return nil, exit.Hardf("bd list could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("bd list failed: %s", strings.TrimSpace(res.Stderr))
	}
	var arr []struct {
		ID     string   `json:"id"`
		Title  string   `json:"title"`
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil {
		return nil, exit.Hardf("parse bd json: %v", err)
	}
	picks := make([]finishPick, 0, len(arr))
	for _, b := range arr {
		picks = append(picks, finishPick{Bead: b.ID, Title: b.Title, Change: changeFromLabels(b.Labels)})
	}
	return picks, nil
}

// assemblePRBody renders the PR body from the epic's closed picks (spec §5):
// a one-line summary, one bullet per pick tying its subject to its change-id,
// and the generated-by trailer. Deterministic — no LLM call.
func assemblePRBody(epic, title string, picks []finishPick) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Summary\n\n%d picks woven for %s — %s.\n\n## Picks\n\n", len(picks), epic, title)
	for _, p := range picks {
		fmt.Fprintf(&b, "- `%s` %s (`%s`)\n", p.Bead, p.Title, p.Change)
	}
	b.WriteString("\n🤖 Generated with [Claude Code](https://claude.com/claude-code)\n")
	return b.String()
}

// (newFinishCmd and the subcommands are added in Tasks 3–6, which add the
// cobra import at that point.)
