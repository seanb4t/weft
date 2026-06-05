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

// New returns a success envelope (OK true) for verb carrying data. Construct
// success envelopes via New rather than a struct literal: the zero value has
// OK=false, which reads as a failure envelope. weft never emits a failure
// envelope (engine errors surface as exit codes, spec §3), so New is the only
// blessed constructor.
func New(verb string, data any) Envelope {
	return Envelope{OK: true, Verb: verb, Data: data}
}

// JSON renders the envelope as indented JSON.
func (e Envelope) JSON() ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}
