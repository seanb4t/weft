// internal/envelope/envelope_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

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
