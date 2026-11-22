// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// compat.go — turning a canonical machine reference into what Orca binds by.
//
// Orca binds a filament to a machine by EXACT PRESET NAME: of the 4455 filament
// presets it ships, 3927 carry a compatible_printers list and not one carries a
// compatible_printers_condition (docs/mapping.md pitfall #13). So we resolve the
// reference against Orca's catalogue rather than guess the name — 222 of its
// presets do not follow the "<model> <nozzle> nozzle" convention, and a filament
// naming a preset that does not exist is silently dropped on import.

import (
	"fmt"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/catalog"
)

// resolveCompat returns the Orca preset names a filament must declare to be
// offered for the referenced machine.
//
// A model/nozzle pair can map to SEVERAL presets (Raise3D Left/Right/Dual,
// Volumic COPY/MIRROR MODE): all of them are returned, because the filament
// holds for every variant of the same hardware.
func resolveCompat(ref canonical.MachineRef) ([]string, error) {
	machine, err := catalog.Lookup(catalog.Orca, ref.Model, ref.NozzleMM)
	if err != nil {
		// The catalogue reports a catalogue failure; naming the canonical field
		// it came from is ours. No index: Orca renders one file per machine, so
		// the caller already knows which one it asked for.
		return nil, fmt.Errorf("compat.machines: %w", err)
	}
	return machine.Presets, nil
}
