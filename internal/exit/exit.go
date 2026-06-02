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

// Error carries an engine exit code alongside its cause.
//
//	1 = invocation error (bad args, missing workspace, unknown bead)
//	2 = hard failure (an underlying bd/jj/gh command failed)
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

// Invocation wraps err as exit code 1.
func Invocation(err error) *Error { return &Error{Code: 1, Err: err} }

// Invocationf formats an exit-code-1 error.
func Invocationf(format string, a ...any) *Error {
	return &Error{Code: 1, Err: fmt.Errorf(format, a...)}
}

// Hard wraps err as exit code 2.
func Hard(err error) *Error { return &Error{Code: 2, Err: err} }

// Hardf formats an exit-code-2 error.
func Hardf(format string, a ...any) *Error {
	return &Error{Code: 2, Err: fmt.Errorf(format, a...)}
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
