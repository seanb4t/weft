// internal/run/run.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package run is the engine's subprocess layer. Verbs depend on the Runner
// interface so they can be unit-tested with a fake; Exec is the real impl.
package run

import (
	"bytes"
	"errors"
	"os/exec"
)

// Result is the captured outcome of a subprocess.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// Runner runs an external command and captures its output. A non-zero exit is
// reported in Result.Code, NOT as a Go error; err is non-nil only when the
// command could not start.
type Runner interface {
	Run(name string, args ...string) (Result, error)
}

// Exec is the real os/exec-backed Runner.
type Exec struct{}

func (Exec) Run(name string, args ...string) (Result, error) {
	cmd := exec.Command(name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	res := Result{Stdout: out.String(), Stderr: errb.String()}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		res.Code = ee.ExitCode()
		return res, nil // ran to completion with a non-zero exit
	}
	if err != nil {
		return res, err // could not start (e.g. binary not found)
	}
	return res, nil
}

// JJ runs jj with --no-pager always prepended (agent-safety profile).
func JJ(r Runner, args ...string) (Result, error) {
	return r.Run("jj", append([]string{"--no-pager"}, args...)...)
}

// BD runs the beads CLI.
func BD(r Runner, args ...string) (Result, error) {
	return r.Run("bd", args...)
}

// GH runs the GitHub CLI (introduced for the finish verbs: push/PR/merge-state).
// Like bd/jj it is a deterministic CLI wrapper, not agent dispatch.
func GH(r Runner, args ...string) (Result, error) {
	return r.Run("gh", args...)
}

// Claude runs the Claude Code CLI (introduced for `weft install`: it drives
// `claude plugin marketplace add` / `install`). Like bd/jj/gh it is a
// deterministic CLI wrapper, not agent dispatch.
func Claude(r Runner, args ...string) (Result, error) {
	return r.Run("claude", args...)
}
