// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// required.go — the canonical inputs PrusaSlicer cannot import without.
// See providers.MissingError for why the list belongs to the provider and why
// it is a function rather than a path list.
//
// This is the second implementation of that contract, and the first evidence
// that it holds: the rule differs from Orca's. Orca needs meta.name because a
// nameless preset is silently dropped on import; Prusa needs it because the
// preset IS its file name and its filament_settings_id. Same path, different
// reason, and neither provider had to know about the other.

import "github.com/thomsmoreau/Aldina/internal/canonical"

const (
	filamentKind = "prusa filament"
	machineKind  = "prusa machine"
)

// requiredFilament: meta.name must be present AND non-empty. A detached export
// carries no section header, so the preset's identity lives entirely in the
// file name and filament_settings_id — both built from the name, and neither
// inventable on the author's behalf.
//
// Notably NOT here: compat.printers. Orca binds a filament to a machine by
// exact preset name and cannot import without it; Prusa also accepts a
// compatible_printers_condition, and an empty list simply means "compatible
// with everything" — which is legal (docs/mapping.md pitfall #13).
func requiredFilament(f canonical.Filament) []string {
	var missing []string
	if f.Meta.Name == nil || *f.Meta.Name == "" {
		missing = append(missing, "meta.name")
	}
	return missing
}

// requiredMachine is LONGER than its Orca counterpart, and the extra entries
// are not deduction — they are what the catalogue lookup consumes.
//
//   - meta.name: the [printer:NAME] header, the file name and
//     printer_settings_id all derive from it, and none is inventable.
//   - meta.model + nozzle.diameter_mm: the linkage key. RenderMachine resolves
//     them into printer_model / printer_variant, because PrusaSlicer binds
//     filaments by an expression over the model ID rather than by preset name
//     (docs/mapping.md pitfall #13). Without them the preset would import and
//     bind to nothing — the failure mode this provider exists to prevent.
//
// Everything else degrades to an omitted key and the slicer's own default.
func requiredMachine(m canonical.Machine) []string {
	var missing []string
	if m.Meta.Name == nil || *m.Meta.Name == "" {
		missing = append(missing, "meta.name")
	}
	if m.Meta.Model == nil || *m.Meta.Model == "" {
		missing = append(missing, "meta.model")
	}
	if m.Nozzle.DiameterMM == nil {
		missing = append(missing, "nozzle.diameter_mm")
	}
	return missing
}
