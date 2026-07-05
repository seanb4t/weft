// internal/config/config_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/seanb4t/weft/internal/exit"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("missing file must not error, got %v", err)
	}
	if cfg.ShedMax() != DefaultShedMax {
		t.Errorf("ShedMax() = %d, want default %d", cfg.ShedMax(), DefaultShedMax)
	}
	if cfg.Workspace.Root != "" {
		t.Errorf("Workspace.Root = %q, want empty", cfg.Workspace.Root)
	}
}

func TestLoadParsesValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := "[shed]\nmax = 7\n\n[workspace]\nroot = \"../wt\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.ShedMax() != 7 {
		t.Errorf("ShedMax() = %d, want 7", cfg.ShedMax())
	}
	if cfg.Workspace.Root != "../wt" {
		t.Errorf("Workspace.Root = %q, want ../wt", cfg.Workspace.Root)
	}
}

func TestShedMaxFallsBackWhenUnsetOrInvalid(t *testing.T) {
	var c Config // zero value
	if c.ShedMax() != DefaultShedMax {
		t.Errorf("zero ShedMax() = %d, want %d", c.ShedMax(), DefaultShedMax)
	}
	c.Shed.Max = -1
	if c.ShedMax() != DefaultShedMax {
		t.Errorf("negative ShedMax() = %d, want %d", c.ShedMax(), DefaultShedMax)
	}
}

func TestLoadParsesVerifyCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[verify]\ncommand = \"go test ./...\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Verify.Command != "go test ./..." {
		t.Errorf("Verify.Command = %q", cfg.Verify.Command)
	}
}

// weft-bcq: the repo must ship a committed .weft/config.toml carrying a verify
// gate, so a fresh clone can `weft pick verify` (self-host the weave) without
// hand-authoring one. Guards against the file being dropped or emptied.
func TestShippedConfigHasVerifyGate(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", ".weft", "config.toml"))
	if err != nil {
		t.Fatalf("shipped .weft/config.toml must parse: %v", err)
	}
	if cfg.Verify.Command == "" {
		t.Fatal("shipped .weft/config.toml must define [verify].command so a fresh clone has a verify gate (weft-bcq)")
	}
}

func TestPlanConfigDefaults(t *testing.T) {
	var c Config
	if c.PlanOverlapMax() != DefaultOverlapMax {
		t.Errorf("default overlap_max = %d, want %d", c.PlanOverlapMax(), DefaultOverlapMax)
	}
	if len(c.PlanStructural()) == 0 {
		t.Errorf("default structural must be non-empty")
	}
}

func TestLoadParsesPlanBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[plan]\nstructural = [\"schema.sql\"]\noverlap_max = 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(cfg.PlanStructural()) != 1 || cfg.PlanStructural()[0] != "schema.sql" {
		t.Errorf("structural = %v", cfg.PlanStructural())
	}
	if cfg.PlanOverlapMax() != 0 {
		t.Errorf("overlap_max = %d, want 0 (explicitly configured)", cfg.PlanOverlapMax())
	}
}

func TestPlanOverlapMaxNegativeClampsToZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[plan]\noverlap_max = -1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.PlanOverlapMax() != 0 {
		t.Errorf("negative overlap_max must clamp to 0 (serialize on any non-structural overlap), got %d", cfg.PlanOverlapMax())
	}
}

// TestMaxResolveAttemptsConfig verifies the [conflict] max_resolve_attempts cap
// (spec I4): default 3 when unset, an explicit value honored, and a configured
// value < 1 rejected as an invocation error — a cap must never silently invert
// to no-cap (the bd-ready-limit-0 gotcha class).
func TestMaxResolveAttemptsConfig(t *testing.T) {
	// Unset -> default.
	var c Config
	got, err := c.MaxResolveAttempts()
	if err != nil {
		t.Fatalf("unset must not error: %v", err)
	}
	if got != DefaultMaxResolveAttempts {
		t.Errorf("unset MaxResolveAttempts() = %d, want default %d", got, DefaultMaxResolveAttempts)
	}

	// Explicit value honored.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[conflict]\nmax_resolve_attempts = 5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	got, err = cfg.MaxResolveAttempts()
	if err != nil {
		t.Fatalf("explicit 5 must not error: %v", err)
	}
	if got != 5 {
		t.Errorf("MaxResolveAttempts() = %d, want 5", got)
	}

	// < 1 rejected as an invocation error (exit 1): a cap must never invert to no-cap.
	zero := 0
	var c2 Config
	c2.Conflict.MaxResolveAttempts = &zero
	if _, err := c2.MaxResolveAttempts(); err == nil {
		t.Fatal("max_resolve_attempts = 0 must be rejected")
	} else if code := exit.Code(err); code != 1 {
		t.Errorf("max_resolve_attempts = 0 must be an invocation error (exit 1), got exit %d", code)
	}
}

func TestLivenessThresholdDefaultAndParse(t *testing.T) {
	var c Config
	d, err := c.LivenessThreshold()
	if err != nil || d != 45*time.Minute {
		t.Errorf("unset threshold: got %v, %v; want 45m, nil", d, err)
	}
	c.Liveness.Threshold = "90m"
	d, err = c.LivenessThreshold()
	if err != nil || d != 90*time.Minute {
		t.Errorf("90m: got %v, %v", d, err)
	}
	c.Liveness.Threshold = "not-a-duration"
	if _, err = c.LivenessThreshold(); err == nil {
		t.Error("malformed threshold must error")
	}
}
