// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cli implements the aldina command line: flag parsing, canonical
// loading and provider rendering. cmd/aldina is wiring only.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// Run executes one command and returns its error; args excludes the program name.
func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return errors.New("command required")
	}

	var err error
	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
	case "render":
		err = runRender(args[1:], stdout, stderr)
	case "validate":
		err = runValidate(args[1:], stdout, stderr)
	case "ingest":
		err = runIngest(args[1:], stdout, stderr)
	default:
		printUsage(stderr)
		err = fmt.Errorf("unknown command %q", args[0])
	}
	// A subcommand's -h returns flag.ErrHelp after its FlagSet printed the
	// usage: a success, caught once here rather than in every subcommand.
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  aldina render --provider orca --kind filament --in FILAMENT.yaml --out DIR")
	fmt.Fprintln(w, "  aldina render --provider orca --kind machine --in MACHINE.yaml --out DIR [--inherits PARENT]")
	fmt.Fprintln(w, "  aldina render --provider prusa --kind filament --in FILAMENT.yaml --out DIR")
	fmt.Fprintln(w, "  aldina render --provider prusa --kind machine --in MACHINE.yaml --out DIR [--inherits PARENT]")
	fmt.Fprintln(w, "  aldina validate --machine MACHINE.yaml --filament FILAMENT.yaml")
	fmt.Fprintln(w, "  aldina ingest --provider orca --kind filament --in EXPORT.json --out DIR")
	fmt.Fprintln(w, "  aldina ingest --provider prusa --kind filament --in BUNDLE.ini --out DIR")
}
