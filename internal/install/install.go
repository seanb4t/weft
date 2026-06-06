// internal/install/install.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package install drives Claude Code's `claude plugin` CLI to register the weft
// repo as a marketplace and install the weft plugin, pinned to the running
// binary's release (spec docs/seams/07-weft-install.md). It depends only on the
// run.Runner interface so it is unit-testable with the engine's fake runner.
package install

import (
	"os"
	"regexp"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
)

// validScopes are the Claude Code install scopes (claude plugin install --scope).
var validScopes = map[string]bool{"user": true, "project": true, "local": true}

// refPattern allowlists a git ref before it is interpolated into the
// `claude plugin marketplace add <source>@<ref>` argument. Leading char is
// alphanumeric (rejects a leading '-' that bd/claude could read as a flag); the
// rest is the git-ref-safe set [A-Za-z0-9._/-]. This excludes every shell/
// revset metacharacter and rejects ".." traversal. Mirrors the guard idiom on
// changeIDPattern/epicIDPattern (conflict.go/finish.go); see the engram memory
// weft-cli-validate-user-id-before-revset-or-gh-api.
var refPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

func validateScope(scope string) error {
	if !validScopes[scope] {
		return exit.Invocationf("invalid --scope %q — must be user, project, or local", scope)
	}
	return nil
}

func validateRef(ref string) error {
	if !refPattern.MatchString(ref) || strings.Contains(ref, "..") {
		return exit.Invocationf("invalid --ref %q", ref)
	}
	return nil
}

// validateLocal requires an existing directory carrying a marketplace manifest,
// and rejects a leading dash (flag confusion).
func validateLocal(path string) error {
	if path == "" || strings.HasPrefix(path, "-") {
		return exit.Invocationf("invalid --local path %q", path)
	}
	if fi, err := os.Stat(path + "/.claude-plugin/marketplace.json"); err != nil || fi.IsDir() {
		return exit.Invocationf("--local %q has no .claude-plugin/marketplace.json", path)
	}
	return nil
}

const repoSlug = "seanb4t/weft"

// semverPattern matches a clean release version X.Y.Z only (no pre-release or
// build suffix). The dev sentinel "0.0.0-dev" — and any other suffixed build —
// deliberately fails it, so the default pin path refuses to float; pre-release
// or dev builds must pass --ref (or --local).
var semverPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// resolveSource picks the marketplace source + ref. Precedence: --local (a clone
// path, no ref) > --ref (override the ref on the repo) > default (pin the plugin
// tag weft--v<version> for a released binary). A dev/untagged version with no
// --ref/--local refuses rather than silently floating to a branch (spec §4.2).
func resolveSource(version, ref, local string) (source, refArg string, err error) {
	if local != "" {
		return local, "", nil
	}
	if ref != "" {
		return repoSlug, ref, nil
	}
	if !semverPattern.MatchString(version) {
		return "", "", exit.Invocationf(
			"weft %s is not a released build — pass --ref <git-ref> or --local <path> to install", version)
	}
	return repoSlug, "weft--v" + version, nil
}
