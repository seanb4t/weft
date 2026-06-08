// internal/plan/verify_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import (
	"strings"
	"testing"
)

// helpers

func makeExpect(ref, title string, priority int, labels []string, hasDesc bool) ReplanExpect {
	return ReplanExpect{Ref: ref, Title: title, Priority: priority, Labels: labels, HasDesc: hasDesc}
}

func makeActual(title string, priority int, labels []string, description string) ReadbackBead {
	return ReadbackBead{Title: title, Priority: priority, Labels: labels, Description: description}
}

// TestVerifyReplanCleanRoundTrip — every field matches; must return empty (non-nil) slice.
func TestVerifyReplanCleanRoundTrip(t *testing.T) {
	expect := []ReplanExpect{
		makeExpect("a", "Task A", 2, []string{"weft-ref:a", "phase:alpha"}, true),
	}
	actual := map[string]ReadbackBead{
		"a": makeActual("Task A", 2, []string{"weft-ref:a", "phase:alpha"}, "some description"),
	}
	disc := VerifyReplan(expect, actual)
	if disc == nil {
		t.Fatal("VerifyReplan must return non-nil slice (never nil)")
	}
	if len(disc) != 0 {
		t.Errorf("clean round-trip must return empty discrepancies, got %v", disc)
	}
}

// TestVerifyReplanMissingRef — ref absent from actual.
func TestVerifyReplanMissingRef(t *testing.T) {
	expect := []ReplanExpect{
		makeExpect("a", "Task A", 2, []string{"weft-ref:a"}, false),
	}
	actual := map[string]ReadbackBead{} // empty: ref "a" not persisted
	disc := VerifyReplan(expect, actual)
	if len(disc) != 1 {
		t.Fatalf("missing ref must yield 1 discrepancy, got %v", disc)
	}
	if !strings.Contains(disc[0], "not found after import") {
		t.Errorf("discrepancy must mention 'not found after import', got %q", disc[0])
	}
	if !strings.Contains(disc[0], "pick a:") {
		t.Errorf("discrepancy must identify the ref, got %q", disc[0])
	}
}

// TestVerifyReplanTitleMismatch — title not persisted.
func TestVerifyReplanTitleMismatch(t *testing.T) {
	expect := []ReplanExpect{
		makeExpect("b", "Correct Title", 2, []string{"weft-ref:b"}, false),
	}
	actual := map[string]ReadbackBead{
		"b": makeActual("Wrong Title", 2, []string{"weft-ref:b"}, ""),
	}
	disc := VerifyReplan(expect, actual)
	if len(disc) != 1 {
		t.Fatalf("title mismatch must yield 1 discrepancy, got %v", disc)
	}
	if !strings.Contains(disc[0], "title not persisted") {
		t.Errorf("discrepancy must mention 'title not persisted', got %q", disc[0])
	}
	if !strings.Contains(disc[0], "Correct Title") {
		t.Errorf("discrepancy must include sent title, got %q", disc[0])
	}
	if !strings.Contains(disc[0], "Wrong Title") {
		t.Errorf("discrepancy must include received title, got %q", disc[0])
	}
}

// TestVerifyReplanPriorityMismatch — priority not persisted.
func TestVerifyReplanPriorityMismatch(t *testing.T) {
	expect := []ReplanExpect{
		makeExpect("c", "Task C", 1, []string{"weft-ref:c"}, false),
	}
	actual := map[string]ReadbackBead{
		"c": makeActual("Task C", 3, []string{"weft-ref:c"}, ""),
	}
	disc := VerifyReplan(expect, actual)
	if len(disc) != 1 {
		t.Fatalf("priority mismatch must yield 1 discrepancy, got %v", disc)
	}
	if !strings.Contains(disc[0], "priority not persisted") {
		t.Errorf("discrepancy must mention 'priority not persisted', got %q", disc[0])
	}
}

// TestVerifyReplanDroppedAuthoredLabel — one of the authored labels not in actual.
func TestVerifyReplanDroppedAuthoredLabel(t *testing.T) {
	expect := []ReplanExpect{
		makeExpect("d", "Task D", 2, []string{"weft-ref:d", "phase:beta"}, false),
	}
	actual := map[string]ReadbackBead{
		// "phase:beta" is missing; bd may add its own labels (e.g. "status:open") — OK
		"d": makeActual("Task D", 2, []string{"weft-ref:d", "bd-internal:something"}, ""),
	}
	disc := VerifyReplan(expect, actual)
	if len(disc) != 1 {
		t.Fatalf("dropped authored label must yield 1 discrepancy, got %v", disc)
	}
	if !strings.Contains(disc[0], "label") || !strings.Contains(disc[0], "phase:beta") {
		t.Errorf("discrepancy must identify the missing label, got %q", disc[0])
	}
}

// TestVerifyReplanExtraBdLabelOK — bd adds a label weft did not author; that is fine.
func TestVerifyReplanExtraBdLabelOK(t *testing.T) {
	expect := []ReplanExpect{
		makeExpect("e", "Task E", 2, []string{"weft-ref:e"}, false),
	}
	actual := map[string]ReadbackBead{
		// bd added "bd-added:foo" — subset check, not equality
		"e": makeActual("Task E", 2, []string{"weft-ref:e", "bd-added:foo"}, ""),
	}
	disc := VerifyReplan(expect, actual)
	if len(disc) != 0 {
		t.Errorf("extra bd label must not produce discrepancy, got %v", disc)
	}
}

// TestVerifyReplanDroppedDescription — HasDesc true but actual description is empty.
func TestVerifyReplanDroppedDescription(t *testing.T) {
	expect := []ReplanExpect{
		makeExpect("f", "Task F", 2, []string{"weft-ref:f"}, true), // HasDesc=true
	}
	actual := map[string]ReadbackBead{
		"f": makeActual("Task F", 2, []string{"weft-ref:f"}, ""), // description dropped
	}
	disc := VerifyReplan(expect, actual)
	if len(disc) != 1 {
		t.Fatalf("dropped description must yield 1 discrepancy, got %v", disc)
	}
	if !strings.Contains(disc[0], "description not persisted") {
		t.Errorf("discrepancy must mention 'description not persisted', got %q", disc[0])
	}
}

// TestVerifyReplanDescriptionWhitespaceOnly — only whitespace counts as empty.
func TestVerifyReplanDescriptionWhitespaceOnly(t *testing.T) {
	expect := []ReplanExpect{
		makeExpect("g", "Task G", 2, []string{"weft-ref:g"}, true),
	}
	actual := map[string]ReadbackBead{
		"g": makeActual("Task G", 2, []string{"weft-ref:g"}, "   \n\t  "),
	}
	disc := VerifyReplan(expect, actual)
	if len(disc) != 1 {
		t.Fatalf("whitespace-only description must be treated as dropped, got %v", disc)
	}
}

// TestVerifyReplanNoDescRequiredOK — HasDesc false, actual empty: not a problem.
func TestVerifyReplanNoDescRequiredOK(t *testing.T) {
	expect := []ReplanExpect{
		makeExpect("h", "Task H", 2, []string{"weft-ref:h"}, false),
	}
	actual := map[string]ReadbackBead{
		"h": makeActual("Task H", 2, []string{"weft-ref:h"}, ""),
	}
	disc := VerifyReplan(expect, actual)
	if len(disc) != 0 {
		t.Errorf("HasDesc=false with empty actual must not produce discrepancy, got %v", disc)
	}
}

// TestVerifyReplanDeterministicOrder — output is sorted by ref.
func TestVerifyReplanDeterministicOrder(t *testing.T) {
	expect := []ReplanExpect{
		makeExpect("z", "Z", 2, []string{"weft-ref:z"}, false),
		makeExpect("a", "A", 2, []string{"weft-ref:a"}, false),
		makeExpect("m", "M", 2, []string{"weft-ref:m"}, false),
	}
	actual := map[string]ReadbackBead{} // all missing
	disc := VerifyReplan(expect, actual)
	if len(disc) != 3 {
		t.Fatalf("all missing must yield 3 discrepancies, got %v", disc)
	}
	// refs must appear in sorted order: a, m, z
	if !strings.Contains(disc[0], "pick a:") {
		t.Errorf("first discrepancy must be ref 'a', got %q", disc[0])
	}
	if !strings.Contains(disc[1], "pick m:") {
		t.Errorf("second discrepancy must be ref 'm', got %q", disc[1])
	}
	if !strings.Contains(disc[2], "pick z:") {
		t.Errorf("third discrepancy must be ref 'z', got %q", disc[2])
	}
}

// TestVerifyReplanMultipleFieldsOnSameRef — multiple fields wrong on one ref.
func TestVerifyReplanMultipleFieldsOnSameRef(t *testing.T) {
	expect := []ReplanExpect{
		makeExpect("x", "Right Title", 1, []string{"weft-ref:x", "phase:gamma"}, true),
	}
	actual := map[string]ReadbackBead{
		"x": makeActual("Wrong Title", 3, []string{"weft-ref:x"}, ""),
	}
	disc := VerifyReplan(expect, actual)
	// title wrong, priority wrong, label dropped, description dropped => 4 discrepancies
	if len(disc) != 4 {
		t.Errorf("expected 4 discrepancies (title+priority+label+desc), got %v", disc)
	}
}
