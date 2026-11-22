// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/thomsmoreau/Aldina/internal/canonical"
)

func runValidate(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	machinePath := flags.String("machine", "", "canonical machine YAML")
	filamentPath := flags.String("filament", "", "canonical filament YAML")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("validate: unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *machinePath == "" || *filamentPath == "" {
		return errors.New("validate: --machine and --filament are required")
	}

	machine, err := canonical.LoadMachine(*machinePath)
	if err != nil {
		return fmt.Errorf("load machine: %w", err)
	}
	filament, err := canonical.LoadFilament(*filamentPath)
	if err != nil {
		return fmt.Errorf("load filament: %w", err)
	}
	printWarnings(stderr, machine.Validate())
	printWarnings(stderr, filament.Validate())

	warnings, err := canonical.CheckCompat(*machine, *filament)
	printWarnings(stderr, warnings)
	if err != nil {
		return err
	}
	// A compat warning is degraded compatibility (flow capped, wrong target):
	// a bare OK next to it would say the opposite.
	switch len(warnings) {
	case 0:
		fmt.Fprintln(stdout, "compatibility: OK")
	case 1:
		fmt.Fprintln(stdout, "compatibility: OK (1 warning)")
	default:
		fmt.Fprintf(stdout, "compatibility: OK (%d warnings)\n", len(warnings))
	}
	return nil
}
