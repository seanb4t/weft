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
