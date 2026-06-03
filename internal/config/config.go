// internal/config/config.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package config loads .weft/config.toml (spec §8). A missing file yields a
// zero-value Config; callers apply their own defaults via the accessors.
package config

import (
	"errors"
	"io/fs"

	"github.com/BurntSushi/toml"
)

// DefaultShedMax is the conservative wave-size cap when none is configured
// (spec §7: "Conservative default (≈3)").
const DefaultShedMax = 3

// Config is the project-local engine config (spec §8).
type Config struct {
	Shed struct {
		Max int `toml:"max"`
	} `toml:"shed"`
	Workspace struct {
		Root string `toml:"root"`
	} `toml:"workspace"`
	Verify struct {
		Command string `toml:"command"`
	} `toml:"verify"`
	Plan struct {
		Structural []string `toml:"structural"`
		OverlapMax *int     `toml:"overlap_max"` // pointer: distinguishes unset from an explicit 0
	} `toml:"plan"`
}

// Load reads the TOML config at path. A missing file is not an error — it
// returns a zero-value Config. Malformed TOML returns the decode error.
func Load(path string) (Config, error) {
	var c Config
	_, err := toml.DecodeFile(path, &c)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	return c, nil
}

// ShedMax returns the configured wave cap, falling back to DefaultShedMax when
// unset or non-positive.
func (c Config) ShedMax() int {
	if c.Shed.Max < 1 {
		return DefaultShedMax
	}
	return c.Shed.Max
}

// DefaultOverlapMax tolerates a single shared incidental (non-structural) file
// between same-shed picks; 2+ shared files serialize (spec §4.2).
const DefaultOverlapMax = 1

// DefaultStructural is the language-agnostic starter set of files whose
// concurrent edit is almost always a real conflict (spec §4.2). Globs match the
// path or its basename via filepath.Match; ** is unsupported (a §8 refinement).
func DefaultStructural() []string {
	return []string{"go.mod", "go.sum", "package.json", "package-lock.json", "Cargo.toml", "Cargo.lock", "*.lock"}
}

// PlanStructural returns the configured structural globs, or the defaults when
// none are set.
func (c Config) PlanStructural() []string {
	if len(c.Plan.Structural) == 0 {
		return DefaultStructural()
	}
	return c.Plan.Structural
}

// PlanOverlapMax returns the configured incidental-overlap tolerance, or the
// default when unset. A negative configured value clamps to 0 (serialize on any
// non-structural overlap).
func (c Config) PlanOverlapMax() int {
	if c.Plan.OverlapMax == nil {
		return DefaultOverlapMax
	}
	if *c.Plan.OverlapMax < 0 {
		return 0
	}
	return *c.Plan.OverlapMax
}
