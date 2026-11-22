// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// filament_render.go — canonical Filament → PrusaSlicer filament INI.
//
// Mirror of the Orca render, NOT a shared engine: the two providers share
// only the value-free conversions (providers/convert.go) and deliberately
// keep their own ENCODINGS. What still looks duplicated here (broadcast vs
// one, the "nil" inheritance token, the shrink spelling) is where the
// formats actually disagree, and merging it would hide exactly the
// differences this provider exists to expose.
//
// Field-by-field mapping and its traps: docs/mapping.md §Filament resource.
// Wire encoding (quoting, escaping, list vs scalar): ini.go.

import (
	"strconv"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/providers"
)

// ---------------------------------------------------------------------------
// Small pure bricks
// ---------------------------------------------------------------------------

// one wraps a value in the single-element list a DETACHED export uses. Orca
// broadcasts to nExtruders here; a detached Prusa export always carries one.
// (Orca's broadcast twin is NOT merged on purpose: different encodings, same
// value — see providers/convert.go.)
func one(v string) []string { return []string{v} }

// oneFloatPtr applies the omit-on-absent policy: nil → nil slice → Marshal
// leaves the key out → the slicer keeps its own default.
func oneFloatPtr(p *float64) []string {
	if p == nil {
		return nil
	}
	return one(providers.FormatFloat(*p))
}

func oneIntPtr(p *int) []string {
	if p == nil {
		return nil
	}
	return one(strconv.Itoa(*p))
}

// oneRatioPtr is RatioToPercent with the omit-on-absent policy.
func oneRatioPtr(p *float64) []string {
	if p == nil {
		return nil
	}
	return one(providers.RatioToPercent(*p))
}

// renderShrink is where the two providers visibly diverge. Canonical carries a
// shrink RATE (0.0 = no shrink). Prusa asks the same question — "how much does
// it shrink" — so the rate maps straight through as a percentage. Orca asks the
// INVERSE ("how big should it end up") and spells that same zero "100%".
// docs/mapping.md pitfall #14; the round-trip test pins both sides.
func renderShrink(rate *float64) string {
	if rate == nil {
		return ""
	}
	return providers.RatioToExactPercent(*rate) + "%"
}

// inheritFloatPtr renders a filament-level retraction override: nil = "inherit
// from the machine", spelled "nil". Slic3r heritage, and the one convention
// Orca and Prusa spell identically.
func inheritFloatPtr(p *float64) []string {
	if p == nil {
		return one("nil")
	}
	return one(providers.FormatFloat(*p))
}

func inheritBoolPtr(p *bool) []string {
	if p == nil {
		return one("nil")
	}
	if *p {
		return one("1")
	}
	return one("0")
}

func boolPtr01(p *bool) []string {
	if p == nil {
		return nil
	}
	if *p {
		return one("1")
	}
	return one("0")
}

func gcodeBlob(p *string) []string {
	if p == nil {
		return nil
	}
	return one(*p)
}

// ---------------------------------------------------------------------------
// Assembly
// ---------------------------------------------------------------------------

// RenderFilament assembles a full PrusaSlicer filament preset.
//
// Unlike Orca, this takes no machine preset name: Prusa binds compatibility
// through compatible_printers (a list, possibly empty = compatible with
// everything), not through the preset name. One canonical filament renders to
// ONE Prusa file, where Orca needs one file per target machine.
//
// Loss policy: a canonical field Prusa cannot express becomes a warning,
// never a silent drop.
func RenderFilament(f canonical.Filament) (FilamentSettings, []providers.Warning, error) {
	if err := providers.MissingError(filamentKind, requiredFilament(f)); err != nil {
		return FilamentSettings{}, nil, err
	}
	// Resolved before anything else: an unknown machine is a hard stop, and
	// there is no point rendering 80 keys we are about to throw away.
	condition, err := renderCompatCondition(f.Compat.Machines)
	if err != nil {
		return FilamentSettings{}, nil, err
	}

	var warns []providers.Warning

	var filamentType []string
	if f.Meta.Material == nil {
		warns = append(warns, providers.Warning{Path: "meta.material", Message: "absent, filament_type omitted (slicer default applies)"})
	} else {
		mapped, loss := mapMaterialToPrusa(*f.Meta.Material)
		if loss != "" {
			warns = append(warns, providers.Warning{Path: "meta.material", Message: loss})
		}
		filamentType = one(mapped)
	}

	// Canonical concepts with no Prusa field at all. Collected here rather than
	// scattered so the loss list stays readable and provably complete.
	if f.Temps.NozzleRange != nil {
		warns = append(warns, providers.Warning{Path: "temps.nozzle_range", Message: "no prusa field, dropped (orca renders it)"})
	}
	if f.Temps.Softening != nil {
		warns = append(warns, providers.Warning{Path: "temps.softening", Message: "no prusa field, dropped (orca renders temperature_vitrification)"})
	}
	if f.Retraction.WipeDistanceMM != nil {
		warns = append(warns, providers.Warning{Path: "retraction.wipe_distance_mm", Message: "no prusa filament override, dropped"})
	}
	if len(f.Overrides["orca"]) > 0 {
		warns = append(warns, providers.Warning{Path: "overrides.orca", Message: "orca-only pass-through, not rendered"})
	}

	var colour string
	if f.Meta.ColorHex != nil {
		colour = "#" + *f.Meta.ColorHex // canonical stores it WITHOUT '#'
	}
	var notes []string
	if f.Meta.Notes != nil {
		notes = one(*f.Meta.Notes)
	}

	out := FilamentSettings{
		FilamentIdentity: FilamentIdentity{
			// EMPTY on purpose: in a config bundle the preset name lives in the
			// `[filament:NAME]` header, and every real bundle leaves this ""
			// (see MarshalBundle). meta.name is still required — it becomes
			// that header and the file name.
			FilamentSettingsID: one(""),
			FilamentVendor:     providers.DerefString(f.Meta.Vendor),
			FilamentType:       filamentType,
			FilamentColour:     colour,
			FilamentNotes:      notes,
		},
		FilamentCompatibility: FilamentCompatibility{
			// compatible_printers stays EMPTY, like 7511 of the 7528 system
			// presets: PrusaSlicer binds by condition (compat.go).
			CompatiblePrintersCondition: condition,
		},
		FilamentPhysical: FilamentPhysical{
			FilamentDiameter:                oneFloatPtr(f.Physical.DiameterMM),
			FilamentDensity:                 oneFloatPtr(f.Physical.DensityGCm3),
			FilamentCost:                    oneFloatPtr(f.Physical.CostPerKg),
			FilamentAbrasive:                boolPtr01(f.Physical.Abrasive),
			FilamentShrinkageCompensationXY: renderShrink(f.Physical.ShrinkageXY),
			FilamentShrinkageCompensationZ:  renderShrink(f.Physical.ShrinkageZ),
		},
		FilamentTemp: FilamentTemp{
			Temperature:              oneFloatPtr(f.Temps.Nozzle),
			FirstLayerTemperature:    oneFloatPtr(f.Temps.NozzleFirstLayer),
			BedTemperature:           oneFloatPtr(f.Temps.Bed),
			FirstLayerBedTemperature: oneFloatPtr(f.Temps.BedFirstLayer),
		},
		FilamentFan: FilamentFan{
			MinFanSpeed:            oneRatioPtr(f.Cooling.FanMin),
			MaxFanSpeed:            oneRatioPtr(f.Cooling.FanMax),
			BridgeFanSpeed:         oneRatioPtr(f.Cooling.BridgesFan),
			DisableFanFirstLayers:  oneIntPtr(f.Cooling.FanOffFirstLayers),
			FullFanSpeedLayer:      oneIntPtr(f.Cooling.FullFanAtLayer),
			SlowdownBelowLayerTime: oneFloatPtr(f.Cooling.SlowDownLayerTimeS),
			MinPrintSpeed:          oneFloatPtr(f.Cooling.SlowDownMinSpeedMMS),
		},
		FilamentFlow: FilamentFlow{
			ExtrusionMultiplier:        oneFloatPtr(f.Flow.FlowRatio),
			FilamentMaxVolumetricSpeed: oneFloatPtr(f.Flow.MaxVolumetricMM3S),
		},
		FilamentRetraction: FilamentRetraction{
			FilamentRetractLength:  inheritFloatPtr(f.Retraction.LengthMM),
			FilamentRetractSpeed:   inheritFloatPtr(f.Retraction.SpeedMMS),
			FilamentDeretractSpeed: inheritFloatPtr(f.Retraction.DeretractSpeedMMS),
			FilamentRetractLift:    inheritFloatPtr(f.Retraction.ZHopMM),
			FilamentWipe:           inheritBoolPtr(f.Retraction.Wipe),
		},
		FilamentGcode: FilamentGcode{
			StartFilamentGcode: gcodeBlob(f.Gcode.Start),
			EndFilamentGcode:   gcodeBlob(f.Gcode.End),
		},
	}

	return out, warns, nil
}
