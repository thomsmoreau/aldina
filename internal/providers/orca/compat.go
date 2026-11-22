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
	"strconv"
	"strings"

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

// parseCompatMachines reverses resolveCompat: the wire preset names back to the
// canonical machine references, one per machine, the self-link's spelling
// first (docs/mapping.md "Orca ingest of compat"). A name that resolves through
// neither the catalogue nor the self-link form is a loss (the snapshot is
// older than the preset) and is surfaced, never dropped silently.
func parseCompatMachines(names []string) ([]canonical.MachineRef, []IngestIssue) {
	var refs []canonical.MachineRef
	var issues []IngestIssue
	type slot struct {
		index    int
		selfLink bool
	}
	seen := make(map[canonical.MachineRef]slot)
	for _, name := range names {
		ref, selfLink, ok := resolvePresetName(name)
		if !ok {
			issues = append(issues, IngestIssue{
				Bucket: BucketLostIdentity,
				Key:    "compatible_printers",
				Value:  name,
				Note:   "preset not in the catalogue snapshot",
			})
			continue
		}
		// Keyed on the catalogue machine, not the spelling: the preset and
		// the self-link name one machine in two vocabularies.
		id := orcaIdentity(ref)
		if s, dup := seen[id]; dup {
			if selfLink && !s.selfLink {
				refs[s.index] = ref
				seen[id] = slot{s.index, true}
			}
			continue
		}
		seen[id] = slot{len(refs), selfLink}
		refs = append(refs, ref)
	}
	return refs, issues
}

// resolvePresetName resolves one wire name back to a machine reference:
// catalogue preset first, then the self-link form render appends. selfLink
// reports the second case, whose spelling is the author's.
func resolvePresetName(name string) (ref canonical.MachineRef, selfLink, ok bool) {
	if m, ok := catalog.LookupPreset(catalog.Orca, name); ok {
		return canonical.MachineRef{Model: m.Model, NozzleMM: m.Nozzle}, false, true
	}
	ref, ok = parseSelfLink(name)
	return ref, ok, ok
}

// orcaIdentity is the catalogue machine a reference designates, aliases
// resolved. A reference the catalogue cannot resolve is its own identity.
func orcaIdentity(ref canonical.MachineRef) canonical.MachineRef {
	m, err := catalog.Lookup(catalog.Orca, ref.Model, ref.NozzleMM)
	if err != nil {
		return ref
	}
	return canonical.MachineRef{Model: m.Model, NozzleMM: m.Nozzle}
}

// parseSelfLink parses MachineRef.String()'s "<model> <nozzle>" form. Render
// appends it as a self-link; it is not a catalogue preset, so LookupPreset
// cannot see it.
func parseSelfLink(name string) (canonical.MachineRef, bool) {
	i := strings.LastIndex(name, " ")
	if i < 0 {
		return canonical.MachineRef{}, false
	}
	nozzle, err := strconv.ParseFloat(name[i+1:], 64)
	if err != nil {
		return canonical.MachineRef{}, false
	}
	return canonical.MachineRef{Model: name[:i], NozzleMM: nozzle}, true
}
