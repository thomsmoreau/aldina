// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/providers/orca"
	"github.com/thomsmoreau/Aldina/internal/providers/prusa"
)

// runIngest is the provider → canonical direction: a slicer file becomes a
// canonical YAML plus a report. Two-step scaffold (docs/adr/0001): the author
// reads the report, edits the YAML, then renders — never a live
// provider→provider pipeline.
func runIngest(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("ingest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	provider := flags.String("provider", "orca", "source provider: orca or prusa")
	kind := flags.String("kind", "filament", "resource kind: filament")
	input := flags.String("in", "", "source slicer file")
	outDir := flags.String("out", "", "output directory for the canonical YAML")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("ingest: unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *input == "" || *outDir == "" {
		return errors.New("ingest: --in and --out are required")
	}
	if *kind != "filament" {
		return fmt.Errorf("ingest: unsupported kind %q (only filament)", *kind)
	}

	var ingest func(raw []byte, outDir string, stdout, stderr io.Writer) error
	switch *provider {
	case "orca":
		ingest = ingestOrca
	case "prusa":
		ingest = ingestPrusa
	default:
		return fmt.Errorf("ingest: unsupported provider %q (orca or prusa)", *provider)
	}
	raw, err := os.ReadFile(*input)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}
	return ingest(raw, *outDir, stdout, stderr)
}

func ingestOrca(raw []byte, outDir string, stdout, stderr io.Writer) error {
	filament, report, err := orca.IngestFilament(raw)
	if err != nil {
		return fmt.Errorf("ingest: %w", err)
	}
	// The report is the feature: the author must see what did not transfer.
	printReport(stderr, report)
	return writeCanonical(outDir, filament, stdout)
}

// ingestPrusa writes one YAML per preset of the bundle (prusa.Ingest). A
// preset that fails — refused or unwritable — does not stop the others: its
// reason is printed under its name, and the command fails at the end, so the
// exit status tells a script the migration is partial.
func ingestPrusa(raw []byte, outDir string, stdout, stderr io.Writer) error {
	results, err := prusa.Ingest(raw)
	if err != nil {
		return fmt.Errorf("ingest: %w", err)
	}
	var failed []string
	for _, r := range results {
		fmt.Fprintf(stderr, "preset %q:\n", r.Name)
		err := r.Err
		if err == nil {
			printPrusaReport(stderr, r.Report)
			err = writeCanonical(outDir, r.Filament, stdout)
		}
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			failed = append(failed, strconv.Quote(r.Name))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("ingest: %d of %d presets not written: %s", len(failed), len(results), strings.Join(failed, ", "))
	}
	return nil
}

func writeCanonical(outDir string, filament *canonical.Filament, stdout io.Writer) error {
	if filament.Meta.Name == nil || *filament.Meta.Name == "" {
		return errors.New("ingest: the source has no name to title the canonical file")
	}
	data, err := canonical.MarshalFilament(filament)
	if err != nil {
		return fmt.Errorf("marshal canonical: %w", err)
	}
	if err := writePreset(outDir, *filament.Meta.Name, ".yaml", data, stdout); err != nil {
		return fmt.Errorf("write canonical: %w", err)
	}
	return nil
}

// printReport writes the ingest report: the one-line headline, then one issue
// per line, bucketed. It goes to stderr like render warnings — diagnostics,
// not the primary artifact.
func printReport(w io.Writer, r *orca.IngestReport) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "summary: %s\n", r.Summary)
	for _, is := range r.Issues {
		printIssue(w, string(is.Bucket), is.Key, is.Value, is.Note)
	}
}

// printPrusaReport is printReport's prusa twin: the report types stay per
// provider (spec #6); only the line format is shared.
func printPrusaReport(w io.Writer, r *prusa.IngestReport) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "summary: %s\n", r.Summary)
	for _, is := range r.Issues {
		printIssue(w, string(is.Bucket), is.Key, is.Value, is.Note)
	}
}

func printIssue(w io.Writer, bucket, key, value, note string) {
	if note != "" {
		fmt.Fprintf(w, "  [%s] %s = %s — %s\n", bucket, key, value, note)
	} else {
		fmt.Fprintf(w, "  [%s] %s = %s\n", bucket, key, value)
	}
}
