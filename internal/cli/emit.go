// internal/cli/emit.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/seanb4t/weft/internal/envelope"
	"github.com/seanb4t/weft/internal/exit"
	"github.com/spf13/cobra"
)

// Emit renders a verb's result per the output contract (spec §3): --pick wins,
// then --json, else the human text. It writes to the command's out stream.
func Emit(cmd *cobra.Command, verb string, data any, text string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	pick, _ := cmd.Flags().GetString("pick")
	env := envelope.Envelope{OK: true, Verb: verb, Data: data}
	out := cmd.OutOrStdout()

	switch {
	case pick != "":
		v, err := envelope.Pick(env, pick)
		if err != nil {
			return exit.Invocation(err)
		}
		fmt.Fprintln(out, pickString(v))
	case jsonOut:
		b, err := env.JSON()
		if err != nil {
			return exit.Hard(err)
		}
		fmt.Fprintln(out, string(b))
	default:
		fmt.Fprintln(out, text)
	}
	return nil
}

// pickString prints scalars raw and structures as compact JSON.
func pickString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
