// internal/cli/bead.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

// jjChangeLabelPrefix pins a bead's jj change-id — the bead↔change spine
// (spec §5.1). `pick seal` writes it; integrate/land/redo/resume read it.
const jjChangeLabelPrefix = "jj-change:"

// beadInfo is the subset of `bd show --json` the weave verbs need.
type beadInfo struct {
	Title  string   `json:"title"`
	Status string   `json:"status"`
	Labels []string `json:"labels"`
}

// showBead reads one bead's facts. `bd show --json` returns a single-element
// array.
func showBead(r run.Runner, bead string) (beadInfo, error) {
	res, err := run.BD(r, "show", bead, "--json")
	if err != nil {
		return beadInfo{}, exit.Hardf("bd show could not run: %v", err)
	}
	if res.Code != 0 {
		return beadInfo{}, exit.Hardf("bd show %s failed: %s", bead, strings.TrimSpace(res.Stderr))
	}
	var arr []beadInfo
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil {
		return beadInfo{}, exit.Hardf("parse bd show json for %s: %v", bead, err)
	}
	if len(arr) == 0 {
		return beadInfo{}, exit.Hardf("bd show %s returned no issue", bead)
	}
	return arr[0], nil
}

// changeOf returns the bead's pinned jj change-id (from its jj-change:<id>
// label), or "" if the bead has not been sealed yet.
func changeOf(r run.Runner, bead string) (string, error) {
	info, err := showBead(r, bead)
	if err != nil {
		return "", err
	}
	return changeFromLabels(info.Labels), nil
}

// changeFromLabels extracts the jj-change:<id> value from a label set, or "".
func changeFromLabels(labels []string) string {
	for _, l := range labels {
		if strings.HasPrefix(l, jjChangeLabelPrefix) {
			return strings.TrimPrefix(l, jjChangeLabelPrefix)
		}
	}
	return ""
}
