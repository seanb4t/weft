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
