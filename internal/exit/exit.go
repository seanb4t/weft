// internal/exit/exit.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package exit defines the weft engine's exit-code taxonomy: the code reflects
// whether the engine did its job, never the verdict of the work (spec §3).
package exit

import (
	"errors"
	"fmt"
)

// Engine exit codes (spec §3). The code reflects whether the engine did its
// job, never the verdict of the work.
const (
	CodeInvocation = 1 // bad args, missing workspace, unknown bead
	CodeHard       = 2 // an underlying bd/jj/gh command failed
)

// Error carries an engine exit code alongside its cause. Error MUST be built
// via the Invocation*/Hard* constructors; a bare &Error{} has Code 0 and reads
// as success even though it is a non-nil error.
type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

// Invocation wraps err as exit code 1 (CodeInvocation).
func Invocation(err error) *Error { return &Error{Code: CodeInvocation, Err: err} }

// Invocationf formats a CodeInvocation (exit code 1) error.
func Invocationf(format string, a ...any) *Error {
	return &Error{Code: CodeInvocation, Err: fmt.Errorf(format, a...)}
}

// Hard wraps err as exit code 2 (CodeHard).
func Hard(err error) *Error { return &Error{Code: CodeHard, Err: err} }

// Hardf formats a CodeHard (exit code 2) error.
func Hardf(format string, a ...any) *Error {
	return &Error{Code: CodeHard, Err: fmt.Errorf(format, a...)}
}

// Code returns the engine exit code for err: 0 for nil, the typed code for an
// *Error, and 2 (hard) for any other error.
func Code(err error) int {
	if err == nil {
		return 0
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return 2
}
