// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// material.go — the filament_type vocabulary, one table for both directions.
//
// PrusaSlicer 2.9.4 defines filament_type (PrintConfig.cpp) as coStrings with
// an open select: PLA, PET, ABS, ASA, FLEX, HIPS, EDGE, NGEN, PA, NYLON, PVA,
// PC, PP, PEI, PEEK, PEKK, POM, PSU, PVDF, SCAFF — and the user may type any
// other value.

import (
	"fmt"

	"github.com/thomsmoreau/Aldina/internal/canonical"
)

// prusaMaterials is the lossless pairs: the name render writes for each
// canonical material. A material missing here renders with a warning.
var prusaMaterials = map[canonical.Material]string{
	canonical.PLA:  "PLA",
	canonical.PETG: "PET", // same family, different name — the vocabulary trap
	canonical.ABS:  "ABS",
	canonical.ASA:  "ASA",
	canonical.TPU:  "FLEX",
	canonical.PC:   "PC",
	canonical.PA:   "PA",
}

// mapMaterialToPrusa maps the canonical enum to Prusa's vocabulary. The second
// return is a loss message ("" when exact), collected by the caller under its
// path — never a silent substitution.
func mapMaterialToPrusa(m canonical.Material) (string, string) {
	if name, ok := prusaMaterials[m]; ok {
		return name, ""
	}
	if m == canonical.PHA {
		return "PLA", "PHA unknown to prusa, rendered as PLA"
	}
	return string(m), fmt.Sprintf("%q not in prusa's filament_type list, passed through", m)
}

// parsePrusaMaterial reads filament_type. The wire is coStrings, one type
// per extruder, and canonical holds one material: the first wins and the
// collapse is noted. A name with no canonical home returns nil and a note;
// the caller turns every note into an approximation issue.
func parsePrusaMaterial(a []string) (*canonical.Material, string) {
	first, ok := unbroadcast(a)
	if !ok || first == "" {
		return nil, ""
	}
	m, ok := materialFromPrusa(first)
	switch {
	case !ok:
		return nil, fmt.Sprintf("filament_type %q has no canonical material, raw kept in overrides.prusa", first)
	case len(a) > 1:
		return &m, fmt.Sprintf("filament_type holds one type per extruder (%d), kept the first (%s)", len(a), first)
	}
	return &m, ""
}

// materialFromPrusa is the ingest half of the table: the table read
// backwards, then NYLON (Prusa's second name for PA), then a canonical name
// typed into the open select. A name with a canonical home that lands false
// here is a table bug, not a loss.
func materialFromPrusa(name string) (canonical.Material, bool) {
	for m, n := range prusaMaterials {
		if n == name {
			return m, true
		}
	}
	if name == "NYLON" {
		return canonical.PA, true
	}
	switch m := canonical.Material(name); m {
	case canonical.PLA, canonical.PETG, canonical.ABS, canonical.ASA,
		canonical.TPU, canonical.PC, canonical.PA, canonical.PHA:
		return m, true
	}
	return "", false
}
