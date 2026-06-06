// internal/install/install_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package install

import (
	"testing"

	"github.com/seanb4t/weft/internal/exit"
)

func TestValidateScope(t *testing.T) {
	for _, ok := range []string{"user", "project", "local"} {
		if err := validateScope(ok); err != nil {
			t.Errorf("scope %q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"global", "", "User", "-x"} {
		if err := validateScope(bad); exit.Code(err) != 1 {
			t.Errorf("scope %q must be invocation error (exit 1), got %v", bad, err)
		}
	}
}

func TestValidateRefAllowlist(t *testing.T) {
	for _, ok := range []string{"main", "weft--v1.2.3", "v0.4.0", "0123456789abcdef0123456789abcdef01234567"} {
		if err := validateRef(ok); err != nil {
			t.Errorf("ref %q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"-rf", "a b", "a&b", "a|b", "a;b", "a$(x)", ".."} {
		if err := validateRef(bad); exit.Code(err) != 1 {
			t.Errorf("ref %q must be rejected (exit 1), got %v", bad, err)
		}
	}
}

func TestResolveSourceDefaultPinsPluginTag(t *testing.T) {
	src, ref, err := resolveSource("1.4.0", "", "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if src != "seanb4t/weft" || ref != "weft--v1.4.0" {
		t.Errorf("default must pin seanb4t/weft@weft--v1.4.0, got %q@%q", src, ref)
	}
}

func TestResolveSourceDevVersionRefuses(t *testing.T) {
	if _, _, err := resolveSource("0.0.0-dev", "", ""); exit.Code(err) != 1 {
		t.Errorf("dev/untagged version with no --ref/--local must refuse (exit 1), got %v", err)
	}
}

func TestResolveSourceRefOverride(t *testing.T) {
	src, ref, err := resolveSource("0.0.0-dev", "main", "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if src != "seanb4t/weft" || ref != "main" {
		t.Errorf("--ref must override to seanb4t/weft@main, got %q@%q", src, ref)
	}
}

func TestResolveSourceLocalUsesPathNoRef(t *testing.T) {
	src, ref, err := resolveSource("0.0.0-dev", "", "/tmp/weft-clone")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if src != "/tmp/weft-clone" || ref != "" {
		t.Errorf("--local must use the path with no ref, got %q@%q", src, ref)
	}
}
