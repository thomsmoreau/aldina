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
	"github.com/thomsmoreau/Aldina/internal/providers/orca"
	"github.com/thomsmoreau/Aldina/internal/providers/prusa"
)

func runRender(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("render", flag.ContinueOnError)
	flags.SetOutput(stderr)
	provider := flags.String("provider", "orca", "target provider: orca or prusa")
	kind := flags.String("kind", "", "resource kind: filament or machine")
	input := flags.String("in", "", "canonical YAML input")
	outDir := flags.String("out", "", "output directory")
	inherits := flags.String("inherits", "", "exact parent machine preset name, as installed in the target slicer")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("render: unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *input == "" || *outDir == "" || *kind == "" {
		return errors.New("render: --kind, --in and --out are required")
	}
	if *kind != "filament" && *kind != "machine" {
		return fmt.Errorf("render: unsupported kind %q", *kind)
	}
	if *inherits != "" && *kind != "machine" {
		return errors.New("render: --inherits is only valid with --kind machine")
	}

	switch {
	case *provider == "orca" && *kind == "filament":
		return renderFilament(*input, *outDir, stdout, stderr)
	case *provider == "orca" && *kind == "machine":
		return renderMachine(*input, *outDir, *inherits, stdout, stderr)
	case *provider == "prusa" && *kind == "filament":
		return renderPrusaFilament(*input, *outDir, stdout, stderr)
	case *provider == "prusa" && *kind == "machine":
		return renderPrusaMachine(*input, *outDir, *inherits, stdout, stderr)
	default:
		return fmt.Errorf("render: unsupported provider %q", *provider)
	}
}

func renderFilament(input, outDir string, stdout, stderr io.Writer) error {
	filament, err := canonical.LoadFilament(input)
	if err != nil {
		return fmt.Errorf("load filament: %w", err)
	}
	printWarnings(stderr, filament.Validate())
	// Orca identifies a filament preset by the machine it targets, so a target
	// is structural here, not merely declarative: one file per machine.
	if len(filament.Compat.Machines) == 0 {
		return errors.New("compat.machines: at least one target machine is required for orca")
	}

	// Render every target before writing any: a duplicate or a failed target
	// must leave outDir untouched, or a re-run after the fix mixes presets
	// from two inputs.
	seen := make(map[canonical.MachineRef]struct{}, len(filament.Compat.Machines))
	rendered := make([]orca.FilamentSettings, 0, len(filament.Compat.Machines))
	for i, target := range filament.Compat.Machines {
		if _, ok := seen[target]; ok {
			// target.String() and nothing hand-rolled: MachineRef formats
			// itself in exactly one place so no two messages can disagree on
			// the unit (see its doc comment).
			return fmt.Errorf("compat.machines[%d]: duplicate machine %s", i, target)
		}
		seen[target] = struct{}{}

		settings, warnings, err := orca.RenderFilament(*filament, target)
		if err != nil {
			return fmt.Errorf("render filament for %q: %w", target.Model, err)
		}
		printWarnings(stderr, warnings)
		rendered = append(rendered, settings)
	}
	for i, settings := range rendered {
		if err := writeJSON(outDir, *settings.Name, settings, stdout); err != nil {
			return fmt.Errorf("write filament for %q: %w", filament.Compat.Machines[i].Model, err)
		}
	}
	return nil
}

// renderPrusaFilament writes ONE file, where the Orca path writes one per
// target machine: Prusa binds compatibility inside the preset
// (compatible_printers) rather than through the preset name, so compat.printers
// never multiplies the output. An empty list is legal and means "every
// printer" (docs/mapping.md pitfall #13).
func renderPrusaFilament(input, outDir string, stdout, stderr io.Writer) error {
	filament, err := canonical.LoadFilament(input)
	if err != nil {
		return fmt.Errorf("load filament: %w", err)
	}
	printWarnings(stderr, filament.Validate())

	settings, warnings, err := prusa.RenderFilament(*filament)
	if err != nil {
		return fmt.Errorf("render filament: %w", err)
	}
	printWarnings(stderr, warnings)
	if err := writeINI(outDir, *filament.Meta.Name, "filament", settings, stdout); err != nil {
		return fmt.Errorf("write filament: %w", err)
	}
	return nil
}

// renderPrusaMachine writes the printer half of the pair. The [printer:...]
// section is what makes PrusaSlicer install it as a printer rather than adopt
// the file as a whole configuration (prusa.MarshalBundle).
func renderPrusaMachine(input, outDir, inherits string, stdout, stderr io.Writer) error {
	machine, err := canonical.LoadMachine(input)
	if err != nil {
		return fmt.Errorf("load machine: %w", err)
	}
	printWarnings(stderr, machine.Validate())

	settings, warnings, err := prusa.RenderMachine(*machine, inherits)
	if err != nil {
		return fmt.Errorf("render machine: %w", err)
	}
	printWarnings(stderr, warnings)
	if err := writeINI(outDir, *machine.Meta.Name, "printer", settings, stdout); err != nil {
		return fmt.Errorf("write machine: %w", err)
	}
	return nil
}

func renderMachine(input, outDir, inherits string, stdout, stderr io.Writer) error {
	machine, err := canonical.LoadMachine(input)
	if err != nil {
		return fmt.Errorf("load machine: %w", err)
	}
	printWarnings(stderr, machine.Validate())

	settings, warnings, err := orca.RenderMachine(*machine, inherits)
	if err != nil {
		return fmt.Errorf("render machine: %w", err)
	}
	printWarnings(stderr, warnings)
	if err := writeJSON(outDir, settings.Name, settings, stdout); err != nil {
		return fmt.Errorf("write machine: %w", err)
	}
	return nil
}
