// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package canonical

import (
	"fmt"
	"slices"
)

// enumWarn reports an unknown enum spelling, checked only when present:
// nil = "not declared", which is legal and silent.
func enumWarn[T ~string](path string, p *T, allowed ...T) string {
	if p == nil || slices.Contains(allowed, *p) {
		return ""
	}
	return fmt.Sprintf("%s: unknown value %q", path, *p)
}

// Validate audits a canonical Filament and returns WARNINGS, never an error:
// the canonical format does not block creativity (a PLA at 190 °C is weird,
// not invalid — we signal it and render anyway). Two warning families:
//   - presence: a nil pointer field means "absent" — the slicer default or
//     parent preset will apply, which may not be what the author intended
//   - bounds: ratios outside [0,1], non-positive physical values, inverted
//     ranges — all checked ONLY when the field is present, because a zero is
//     a legitimate authored value, never a stand-in for absence
//
// BLOCKING validation lives in the providers, not here: "required" means
// "the target slicer cannot import the file without it", which is a property
// of the target format. See internal/providers/orca/required.go.
//
// Messages follow the "chemin.champ: message" convention. Callers print them
// all (no fail-fast) and decide whether to proceed — the CLI proceeds.
func (f *Filament) Validate() []string {
	var warns []string

	// Presence: fields expected in a complete profile. nil is LEGAL (the key
	// is omitted at render, the slicer default applies) but worth signalling.
	// physical.diameter_mm is deliberately absent from this list: 1.75 mm is
	// near-universal, so warning about it would be noise.
	presence := []struct {
		path string
		set  bool
		msg  string
	}{
		{"meta.name", f.Meta.Name != nil && *f.Meta.Name != "", "providers that need a preset name will refuse to render"},
		{"meta.material", f.Meta.Material != nil, "slicer default will apply"},
		{"temps.nozzle", f.Temps.Nozzle != nil, "slicer default will apply"},
		{"flow.max_volumetric_mm3_s", f.Flow.MaxVolumetricMM3S != nil, "slicer default will apply"},
		{"flow.flow_ratio", f.Flow.FlowRatio != nil, "slicer default will apply"},
	}
	for _, p := range presence {
		if !p.set {
			warns = append(warns, fmt.Sprintf("%s: absent, %s", p.path, p.msg))
		}
	}

	ratios := []struct {
		path  string
		value *float64
	}{
		{"cooling.fan_min", f.Cooling.FanMin},
		{"cooling.fan_max", f.Cooling.FanMax},
		{"cooling.bridges_fan", f.Cooling.BridgesFan},
		{"physical.shrinkage_xy", f.Physical.ShrinkageXY},
		{"physical.shrinkage_z", f.Physical.ShrinkageZ},
	}
	for _, r := range ratios {
		if r.value != nil && (*r.value < 0 || *r.value > 1) {
			warns = append(warns, fmt.Sprintf("%s: must be in [0,1]", r.path))
		}
	}
	if f.Cooling.FanMin != nil && f.Cooling.FanMax != nil && *f.Cooling.FanMin > *f.Cooling.FanMax {
		warns = append(warns, "cooling.fan_min: must be <= cooling.fan_max")
	}

	// Bounds checked only when present (nil already produced a presence
	// warning above, where one was due — no double jeopardy).
	positivePtrs := []struct {
		fieldPath string
		value     *float64
	}{
		{"physical.diameter_mm", f.Physical.DiameterMM},
		{"physical.density_g_cm3", f.Physical.DensityGCm3},
		{"temps.nozzle", f.Temps.Nozzle},
		{"flow.max_volumetric_mm3_s", f.Flow.MaxVolumetricMM3S},
		{"flow.flow_ratio", f.Flow.FlowRatio},
	}
	for _, p := range positivePtrs {
		if p.value != nil && *p.value <= 0 {
			warns = append(warns, fmt.Sprintf("%s: must be > 0", p.fieldPath))
		}
	}

	if f.Temps.NozzleRange != nil && f.Temps.NozzleRange[0] > f.Temps.NozzleRange[1] {
		warns = append(warns, "temps.nozzle_range: min must be <= max")
	}

	return warns
}

// Validate audits a canonical Machine under the same policy as
// Filament.Validate: warnings only, never errors. Limits and optional
// capabilities are NOT presence-checked: unknown (nil) simply means
// "skip the cross-check" — that is a feature, not a gap (crosscheck.go).
func (m *Machine) Validate() []string {
	var warns []string

	presence := []struct {
		path string
		set  bool
		msg  string
	}{
		{"meta.name", m.Meta.Name != nil && *m.Meta.Name != "", "providers that need a preset name will refuse to render"},
		{"meta.model", m.Meta.Model != nil && *m.Meta.Model != "", "slicer default will apply"},
	}
	for _, p := range presence {
		if !p.set {
			warns = append(warns, fmt.Sprintf("%s: absent, %s", p.path, p.msg))
		}
	}

	// Bounds on pointer values: checked only when present (nil = unknown,
	// the corresponding cross-check is skipped — no double jeopardy).
	positivePtrs := []struct {
		path  string
		value *float64
	}{
		{"volume.width_mm", m.Volume.WidthMM},
		{"volume.depth_mm", m.Volume.DepthMM},
		{"volume.height_mm", m.Volume.HeightMM},
		{"nozzle.diameter_mm", m.Nozzle.DiameterMM},
		{"nozzle.filament_diameter_mm", m.Nozzle.FilamentDiameterMM},
		{"nozzle.min_layer_height_mm", m.Nozzle.MinLayerHeightMM},
		{"nozzle.max_layer_height_mm", m.Nozzle.MaxLayerHeightMM},
		{"limits.max_nozzle_temp_c", m.Limits.MaxNozzleTempC},
		{"limits.max_bed_temp_c", m.Limits.MaxBedTempC},
		{"limits.max_chamber_temp_c", m.Limits.MaxChamberTempC},
		{"limits.max_volumetric_mm3_s", m.Limits.MaxVolumetricMM3S},
	}
	for _, p := range positivePtrs {
		if p.value != nil && *p.value <= 0 {
			warns = append(warns, fmt.Sprintf("%s: must be > 0", p.path))
		}
	}

	if m.Nozzle.MinLayerHeightMM != nil && m.Nozzle.MaxLayerHeightMM != nil &&
		*m.Nozzle.MinLayerHeightMM > *m.Nozzle.MaxLayerHeightMM {
		warns = append(warns, "nozzle.min_layer_height_mm: must be <= nozzle.max_layer_height_mm")
	}

	for _, w := range []string{
		enumWarn("nozzle.type", m.Nozzle.Type, NozzleBrass, NozzleStainless, NozzleHardened, NozzleTungsten),
		enumWarn("nozzle.extruder", m.Nozzle.Extruder, DirectDrive, Bowden),
		enumWarn("gcode.flavor", m.Gcode.Flavor, FlavorMarlin, FlavorMarlin2, FlavorKlipper),
	} {
		if w != "" {
			warns = append(warns, w)
		}
	}

	return warns
}
