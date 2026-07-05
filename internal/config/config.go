// internal/config/config.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package config loads .weft/config.toml (spec §8). A missing file yields a
// zero-value Config; callers apply their own defaults via the accessors.
package config

import (
	"errors"
	"io/fs"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/seanb4t/weft/internal/exit"
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
	// Verify.Command is executed via `sh -c` by `pick verify`. It is a
	// trusted-config boundary, NOT a sandbox: any shell command here will run
	// with the invoking user's privileges. Treat .weft/config.toml as a
	// security-sensitive file — restrict write access to trusted parties.
	Verify struct {
		Command string `toml:"command"`
	} `toml:"verify"`
	Plan struct {
		Structural []string `toml:"structural"`
		OverlapMax *int     `toml:"overlap_max"` // pointer: distinguishes unset from an explicit 0
	} `toml:"plan"`
	Liveness struct {
		Threshold string `toml:"threshold"`
	} `toml:"liveness"`
	Conflict struct {
		MaxResolveAttempts *int `toml:"max_resolve_attempts"` // pointer: distinguishes unset from an explicit 0
	} `toml:"conflict"`
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

// DefaultMaxResolveAttempts bounds conflict-resolution attempts before an agent
// thrashing on an unresolvable merge is forced into escalation (spec I4).
const DefaultMaxResolveAttempts = 3

// DefaultLivenessThreshold is the conservative default liveness window when
// [liveness] threshold is unset (spec §5.1).
const DefaultLivenessThreshold = 45 * time.Minute

// MaxResolveAttempts returns the [conflict] max_resolve_attempts cap, defaulting
// to DefaultMaxResolveAttempts when unset. A configured value < 1 is rejected as
// an invocation error rather than clamped: unlike PlanOverlapMax (where 0 is a
// meaningful "serialize on any overlap"), a resolve cap below 1 cannot be
// honored as a bound — silently treating it as no-cap would let an oscillating
// resolver loop forever (the bd-ready-limit-0 gotcha class).
func (c Config) MaxResolveAttempts() (int, error) {
	if c.Conflict.MaxResolveAttempts == nil {
		return DefaultMaxResolveAttempts, nil
	}
	if *c.Conflict.MaxResolveAttempts < 1 {
		return 0, exit.Invocationf("[conflict] max_resolve_attempts = %d is invalid: must be >= 1 (a cap cannot invert to no-cap)", *c.Conflict.MaxResolveAttempts)
	}
	return *c.Conflict.MaxResolveAttempts, nil
}

// LivenessThreshold returns the [liveness] threshold, defaulting to 45m when
// unset. Conservative by design: a thinking-but-quiet executor can look dead;
// the cost is bounded because reap runs at orchestrator startup/resume, not
// mid-wave (seam 3 §5.1). A configured value <= 0 is rejected as an invocation
// error rather than honored: a non-positive threshold marks every workspace
// dead immediately, and reap would destroy every live in-progress executor
// (inverts I3) — the bd-ready-limit-0 gotcha class.
func (c Config) LivenessThreshold() (time.Duration, error) {
	if c.Liveness.Threshold == "" {
		return DefaultLivenessThreshold, nil
	}
	d, err := time.ParseDuration(c.Liveness.Threshold)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, exit.Invocationf("[liveness] threshold = %q is invalid: must be > 0 (a non-positive threshold marks every workspace dead and reap would destroy live executors)", c.Liveness.Threshold)
	}
	return d, nil
}
