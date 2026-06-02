// internal/envelope/pick_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

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
