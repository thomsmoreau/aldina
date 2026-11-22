// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// compat.go — turning canonical machine references into what PrusaSlicer binds
// by.
//
// PrusaSlicer binds by EXPRESSION, not by name: of the 7528 filament presets it
// ships, 1149 carry a compatible_printers_condition and only 17 an explicit
// compatible_printers list (docs/mapping.md pitfall #13). We follow its own
// convention rather than Orca's, which is the whole point of the canonical
// naming a machine instead of a preset.
//
// Of the variables its conditions actually use — nozzle_diameter (1321),
// printer_model (1264), printer_notes (498), nozzle_high_flow (240),
// num_extruders (6) — we generate the first two. They are exact identity
// comparisons; the printer_notes regex form is a keyword convention that only
// pays off once we render machines too.

import (
	"fmt"
	"strings"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/catalog"
	"github.com/thomsmoreau/Aldina/internal/providers"
)

// renderCompatCondition builds the compatible_printers_condition expression.
// Empty list → empty string → the key is omitted → compatible with everything,
// which is what an absent compat means canonically.
func renderCompatCondition(refs []canonical.MachineRef) (string, error) {
	clauses := make([]string, 0, len(refs))
	for i, ref := range refs {
		clause, err := compatClause(ref)
		if err != nil {
			// The index matters here: Prusa folds every machine into ONE file,
			// so without it the author cannot tell which entry of the list
			// failed.
			return "", fmt.Errorf("compat.machines[%d]: %w", i, err)
		}
		clauses = append(clauses, clause)
	}
	switch len(clauses) {
	case 0:
		return "", nil
	case 1:
		// A lone clause needs no parentheses, and the real exports do not add
		// any: `printer_model=="MEGA0"` is written exactly like that.
		return clauses[0], nil
	default:
		// Several machines are alternatives. Each is parenthesised because
		// `and` binds tighter than `or`, so an unparenthesised join would mean
		// something else entirely.
		for i, c := range clauses {
			clauses[i] = "(" + c + ")"
		}
		return strings.Join(clauses, " or "), nil
	}
}

// compatClause matches ONE machine: its model id and its nozzle.
//
// printer_model compares against the ID (`"MINI"`), never the human name
// ("Original Prusa MINI & MINI+") the author writes — the catalogue holds both
// precisely so the author never has to know the id exists.
func compatClause(ref canonical.MachineRef) (string, error) {
	machine, err := catalog.Lookup(catalog.Prusa, ref.Model, ref.NozzleMM)
	if err != nil {
		return "", err
	}
	// nozzle_diameter is per-extruder; every real condition tests [0].
	return fmt.Sprintf("printer_model==%q and nozzle_diameter[0]==%s",
		machine.ModelID, providers.FormatFloat(machine.Nozzle)), nil
}

// parseCompatMachines reads the plain compatible_printers list back to
// canonical machine references, deduplicated, through the prusa catalogue.
// Render never writes this list (it binds by condition), so there is no
// self-link form to parse — the orca side's second resolver has no twin here.
// A name the snapshot does not know is a loss, surfaced, never dropped.
func parseCompatMachines(names []string) ([]canonical.MachineRef, []IngestIssue) {
	var refs []canonical.MachineRef
	var issues []IngestIssue
	seen := make(map[canonical.MachineRef]bool)
	for _, name := range names {
		m, ok := catalog.LookupPreset(catalog.Prusa, name)
		if !ok {
			issues = append(issues, IngestIssue{
				Bucket: BucketLostIdentity,
				Key:    "compatible_printers",
				Value:  name,
				Note:   "preset not in the catalogue snapshot",
			})
			continue
		}
		ref := canonical.MachineRef{Model: m.Model, NozzleMM: m.Nozzle}
		if seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	return refs, issues
}
