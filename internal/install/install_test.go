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
