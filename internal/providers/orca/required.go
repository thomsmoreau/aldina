// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// required.go — the canonical inputs Orca cannot import without.
// One function per resource kind; see providers.MissingError for why the list
// belongs to the provider and why it is a function rather than a path list.

import "github.com/thomsmoreau/Aldina/internal/canonical"

const (
	filamentKind = "orca filament"
	machineKind  = "orca machine"
)

// requiredFilament lists the paths whose absence makes the rendered preset
// non-importable. Orca's import gates are `name`, `filament_settings_id`,
// `version` and `from` (docs/mapping.md §Identity) — a preset missing one is
// silently dropped. Of those, `version` and `from` are render constants and
// `filament_settings_id` derives from `name`, so meta.name is the only
// canonical input that can block. It must be present AND non-empty: the whole
// identity triplet and the output filename are built from it, and no name can
// be invented on the author's behalf.
func requiredFilament(f canonical.Filament) []string {
	var missing []string
	if f.Meta.Name == nil || *f.Meta.Name == "" {
		missing = append(missing, "meta.name")
	}
	return missing
}

// requiredMachine is deliberately as short as its filament counterpart.
// Orca REJECTS our machine imports outright today ("There are 0 configs
// imported"), so its real import gate is unknown: meta.name is kept because
// the preset and its filename derive from it, and nothing more is added by
// deduction until one machine import actually succeeds.
func requiredMachine(m canonical.Machine) []string {
	var missing []string
	if m.Meta.Name == nil || *m.Meta.Name == "" {
		missing = append(missing, "meta.name")
	}
	return missing
}
