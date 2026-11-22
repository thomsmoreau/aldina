// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cli implements the aldina command line: flag parsing, canonical
// loading and provider rendering. cmd/aldina is wiring only.
package cli

import (
	"errors"
	"fmt"
	"io"
)

// Run executes one command and returns its error; args excludes the program name.
func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return errors.New("command required")
	}

	switch args[0] {
	case "render":
		return runRender(args[1:], stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  aldina render --provider orca --kind filament --in FILAMENT.yaml --out DIR")
	fmt.Fprintln(w, "  aldina render --provider orca --kind machine --in MACHINE.yaml --out DIR [--inherits PARENT]")
	fmt.Fprintln(w, "  aldina render --provider prusa --kind filament --in FILAMENT.yaml --out DIR")
	fmt.Fprintln(w, "  aldina render --provider prusa --kind machine --in MACHINE.yaml --out DIR [--inherits PARENT]")
	fmt.Fprintln(w, "  aldina validate --machine MACHINE.yaml --filament FILAMENT.yaml")
}
