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
