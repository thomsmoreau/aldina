// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package canonical

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// CheckCompat cross-checks a Filament against a Machine's capabilities.
// This is Aldina's differentiator: none of these limits exist in any slicer
// format, so no slicer can run these checks. nil on EITHER side means
// "unknown" → the check is skipped, never failed.
//
// The return types carry the severity: warnings are legal but degraded
// (capped, wear, risk) and the caller may proceed; err is the join of the
// physically impossible or damaging combinations and blocks. Each message is
// prefixed with the canonical path of the limiting field.
func CheckCompat(m Machine, f Filament) (warnings []string, err error) {
	var errs []error

	if f.Temps.Nozzle != nil && m.Limits.MaxNozzleTempC != nil &&
		*f.Temps.Nozzle > *m.Limits.MaxNozzleTempC {
		errs = append(errs, fmt.Errorf("limits.max_nozzle_temp_c: filament nozzle %.0f °C exceeds machine max %.0f °C",
			*f.Temps.Nozzle, *m.Limits.MaxNozzleTempC))
	}

	if f.Temps.Bed != nil && m.Limits.MaxBedTempC != nil &&
		*f.Temps.Bed > *m.Limits.MaxBedTempC {
		errs = append(errs, fmt.Errorf("limits.max_bed_temp_c: filament bed %.0f °C exceeds machine max %.0f °C",
			*f.Temps.Bed, *m.Limits.MaxBedTempC))
	}

	if f.Flow.MaxVolumetricMM3S != nil && m.Limits.MaxVolumetricMM3S != nil &&
		*f.Flow.MaxVolumetricMM3S > *m.Limits.MaxVolumetricMM3S {
		warnings = append(warnings, fmt.Sprintf("limits.max_volumetric_mm3_s: filament flow %.1f mm³/s exceeds hotend capacity %.1f mm³/s: it will be capped",
			*f.Flow.MaxVolumetricMM3S, *m.Limits.MaxVolumetricMM3S))
	}

	// Abrasive wear is derived from the nozzle TYPE (enum = hardness class).
	// Nozzle.HRC is only a secondary refinement: 0 = unknown on real
	// profiles (the P2S exports hardened_steel with hrc "0"), and the
	// filament carries no numeric required-HRC yet — a naive numeric
	// comparison would false-positive on every abrasive filament.
	if f.Physical.Abrasive != nil && *f.Physical.Abrasive && m.Nozzle.Type != nil &&
		(*m.Nozzle.Type == NozzleBrass || *m.Nozzle.Type == NozzleStainless) {
		warnings = append(warnings, fmt.Sprintf("nozzle.type: abrasive filament wears a %s nozzle", *m.Nozzle.Type))
	}

	if f.Physical.DiameterMM != nil && m.Nozzle.FilamentDiameterMM != nil &&
		*f.Physical.DiameterMM != *m.Nozzle.FilamentDiameterMM {
		errs = append(errs, fmt.Errorf("nozzle.filament_diameter_mm: filament diameter %.2f mm does not match machine %.2f mm",
			*f.Physical.DiameterMM, *m.Nozzle.FilamentDiameterMM))
	}

	// ABS/ASA warp badly without an enclosed chamber. An UNDECLARED chamber
	// (nil) is unknown, not "open": no claim is made either way.
	if f.Meta.Material != nil && (*f.Meta.Material == ABS || *f.Meta.Material == ASA) &&
		m.Features.Enclosed != nil && !*m.Features.Enclosed {
		warnings = append(warnings, fmt.Sprintf("features.enclosed: %s prints best in an enclosed chamber", *f.Meta.Material))
	}

	// The declared target, which no other check looks at. A filament tuned for
	// another machine may still print here, so this is a warning: the canonical
	// layer signals, the provider is the one that blocks (compat.go, both
	// sides). Absent compat means "no restriction" and claims nothing; a
	// machine that declares no model or no nozzle is UNKNOWN, not different —
	// same rule as every other check here.
	machineIsIdentified := m.Meta.Model != nil && m.Nozzle.DiameterMM != nil
	covers := func(r MachineRef) bool { return r.Matches(m) }
	if len(f.Compat.Machines) > 0 && machineIsIdentified && !slices.ContainsFunc(f.Compat.Machines, covers) {
		warnings = append(warnings, fmt.Sprintf("compat.machines: this filament targets %s, not this machine",
			describeRefs(f.Compat.Machines)))
	}

	return warnings, errors.Join(errs...)
}

// describeRefs lists the targets the way MachineRef.String does, so the warning
// and the preset label Orca writes never disagree.
func describeRefs(refs []MachineRef) string {
	labels := make([]string, len(refs))
	for i, ref := range refs {
		labels[i] = ref.String()
	}
	return strings.Join(labels, ", ")
}
