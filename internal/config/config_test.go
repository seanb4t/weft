// internal/config/config_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package config

import (
	"os"
	"path/filepath"
	"testing"
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
