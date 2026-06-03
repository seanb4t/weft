// internal/plan/plan_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import (
	"fmt"
	"testing"
)

func TestValidateAcceptsWellFormedPlan(t *testing.T) {
	p := WarpPlan{
		Epic: Epic{Title: "E"},
		Picks: []Pick{
			{Ref: "p1", Title: "A", Description: "do A"},
			{Ref: "p2", Title: "B", Description: "do B", Needs: []string{"p1"}},
		},
	}
	if got := Validate(p); len(got) != 0 {
		t.Fatalf("want valid, got issues: %+v", got)
	}
}

func TestValidateFlagsMissingFieldsAndBadNeeds(t *testing.T) {
	p := WarpPlan{
		Epic: Epic{},
		Picks: []Pick{
			{Ref: "p1", Title: "A"},         // missing description
			{Ref: "p1", Description: "dup"}, // duplicate ref + missing title
			{Ref: "p3", Title: "C", Description: "c", Needs: []string{"nope", "p3"}}, // unknown + self need
		},
	}
	issues := Validate(p)
	want := map[string]bool{
		"epic.title is required": false,
		"pick.description is required (the bead description is the plan)": false,
		"duplicate pick.ref":                       false,
		`pick.needs references unknown ref "nope"`: false,
		"pick.needs references itself":             false,
	}
	for _, is := range issues {
		if _, ok := want[is.Message]; ok {
			want[is.Message] = true
		}
	}
	for msg, seen := range want {
		if !seen {
			t.Errorf("expected issue %q; issues=%+v", msg, issues)
		}
	}
}

func TestValidateRejectsEpicKeyInNeeds(t *testing.T) {
	p := WarpPlan{
		Epic:  Epic{Title: "E"},
		Picks: []Pick{{Ref: "p1", Title: "A", Description: "a", Needs: []string{EpicKey}}},
	}
	issues := Validate(p)
	found := false
	for _, is := range issues {
		if is.Message == `pick.needs references reserved ref "@epic"` {
			found = true
		}
	}
	if !found {
		t.Errorf("expected reserved-ref-in-needs issue; issues=%+v", issues)
	}
}

func TestValidateRejectsOutOfRangePriority(t *testing.T) {
	neg := -1
	hi := 5
	valid := 0
	cases := []struct {
		name    string
		pri     *int
		wantErr bool
	}{
		{"negative", &neg, true},
		{"above-max", &hi, true},
		{"zero-valid", &valid, false},
		{"nil-unset", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := WarpPlan{
				Epic:  Epic{Title: "E"},
				Picks: []Pick{{Ref: "p1", Title: "T", Description: "d", Priority: tc.pri}},
			}
			issues := Validate(p)
			found := false
			for _, is := range issues {
				if is.Message == "pick.priority must be between 0 and 4" {
					found = true
				}
			}
			if tc.wantErr && !found {
				t.Errorf("expected priority-bounds issue; issues=%+v", issues)
			}
			if !tc.wantErr && found {
				t.Errorf("unexpected priority-bounds issue; issues=%+v", issues)
			}
		})
	}
}

func TestValidateEpicKeyRefYieldsOnlyReservedIssue(t *testing.T) {
	// A pick with ref=@epic and no description must produce ONLY the reserved-ref
	// issue, not a spurious description-required issue.
	p := WarpPlan{
		Epic:  Epic{Title: "E"},
		Picks: []Pick{{Ref: EpicKey, Title: "T"}}, // description intentionally missing
	}
	issues := Validate(p)
	reservedFound := false
	for _, is := range issues {
		if is.Message == fmt.Sprintf("pick.ref %q is reserved", EpicKey) {
			reservedFound = true
			continue
		}
		if is.Ref == EpicKey {
			t.Errorf("unexpected extra issue for @epic pick: %+v", is)
		}
	}
	if !reservedFound {
		t.Errorf("expected reserved-ref issue; issues=%+v", issues)
	}
}

func TestParseReadsPickFields(t *testing.T) {
	src := []byte(`{"epic":{"title":"E","description":"d"},"picks":[{"ref":"p1","title":"A","description":"a","files":["x.go"],"priority":1}]}`)
	p, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Epic.Title != "E" || len(p.Picks) != 1 || p.Picks[0].Ref != "p1" {
		t.Fatalf("bad parse: %+v", p)
	}
	if p.Picks[0].Priority == nil || *p.Picks[0].Priority != 1 {
		t.Fatalf("priority not parsed: %+v", p.Picks[0])
	}
}
