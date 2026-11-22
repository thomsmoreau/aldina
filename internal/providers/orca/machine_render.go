// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// machine_render.go — canonical Machine → Orca machine preset JSON.
// Reuses the shared conversions (providers/convert.go) and the filament
// ENCODING bricks (broadcast, broadcastFloat, broadcastFloatPtr), and adds
// only what the machine wire form needs.

import (
	"fmt"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/providers"
)

// ---------------------------------------------------------------------------
// Machine-only bricks
// ---------------------------------------------------------------------------

// broadcastPair renders kinetics: one canonical nominal value → the
// [normal, silent] pair of the single v0 variant. Array length = 2 ×
// variants, confirmed by observation (mapping.md pitfall #3: P2S export =
// 2 variants → 4 elements, A1 mini system profile = 1 variant → 2).
func broadcastPair(v string) []string { return []string{v, v} }

func pairFloat(v float64) []string { return broadcastPair(providers.FormatFloat(v)) }

// pairAxes renders one kinetics axis group, declared as a block: nil = the
// machine does not state its motion limits → all four keys omitted. A zero
// axis is a legal declared value, no longer indistinguishable from silence.
func pairAxes(a *canonical.Axes) (x, y, z, e []string) {
	if a == nil {
		return nil, nil, nil, nil
	}
	return pairFloat(a.X), pairFloat(a.Y), pairFloat(a.Z), pairFloat(a.E)
}

// rectToPrintableArea expands a rectangular bed to its 4 corners, in the
// real export's order: origin → +X → +X+Y → +Y. Non-rectangular beds
// (delta, the Prusa MINI's rounded corner) are a v2 topic.
// A corner needs BOTH dimensions, so a missing one omits the whole key
// rather than emitting a degenerate 0-wide bed.
func rectToPrintableArea(widthMM, depthMM *float64) []string {
	if widthMM == nil || depthMM == nil {
		return nil
	}
	return []string{
		providers.FormatPoint(0, 0),
		providers.FormatPoint(*widthMM, 0),
		providers.FormatPoint(*widthMM, *depthMM),
		providers.FormatPoint(0, *depthMM),
	}
}

// bool01 renders a canonical bool as Orca's "1"/"0". Machine bools come in
// TWO wire forms — scalar (use_relative_e_distances) and per-variant array
// (wipe) — so the brick gets two wrappers, never one generic.
func bool01(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func boolScalarPtr(p *bool) string {
	if p == nil {
		return ""
	}
	return bool01(*p)
}

func boolArrayPtr(p *bool) []string {
	if p == nil {
		return nil
	}
	return broadcast(bool01(*p))
}

// mapExtruderTypeToOrca maps the canonical enum to Orca's vocabulary.
// "Direct Drive" WITH space — the composite references had it wrong, the
// real export wins (mapping.md pitfall #9).
func mapExtruderTypeToOrca(t canonical.ExtruderType) (string, string) {
	switch t {
	case canonical.DirectDrive:
		return "Direct Drive", ""
	case canonical.Bowden:
		return "Bowden", ""
	default:
		return "", fmt.Sprintf("%q unknown to orca, extruder_type omitted", t)
	}
}

// ---------------------------------------------------------------------------
// Assembly
// ---------------------------------------------------------------------------

// RenderMachine assembles a full Orca machine preset. inherits is a provider
// option, not canonical data: resolving parent names depends on Orca's locally
// installed machine catalog. Empty emits a root; an exact machine name emits a
// derived child. Canonical-only fields (limits.*, features.enclosed/plates) are
// NOT warnings: they are the `aldina validate` payload, consumed by CheckCompat
// — not lost render data (mapping.md, Limits table). Truly inexpressible fields
// → warnings.
func RenderMachine(m canonical.Machine, inherits string) (MachineSettings, []providers.Warning, error) {
	// Canonical fields Orca cannot import without (see required.go).
	// Everything else degrades to warnings + omitted keys.
	if err := providers.MissingError(machineKind, requiredMachine(m)); err != nil {
		return MachineSettings{}, nil, err
	}
	// inherits is a RENDER ARGUMENT, not a canonical field: guarded on its own.
	if inherits == *m.Meta.Name {
		return MachineSettings{}, nil, fmt.Errorf("inherits: must differ from meta.name to avoid a self-referential orca preset")
	}

	var warns []providers.Warning

	var extruderType string
	if m.Nozzle.Extruder != nil { // absent → omit, not a warning
		var loss string
		extruderType, loss = mapExtruderTypeToOrca(*m.Nozzle.Extruder)
		if loss != "" {
			warns = append(warns, providers.Warning{Path: "nozzle.extruder", Message: loss})
		}
	}
	if m.Features.HeatedChamber != nil {
		warns = append(warns, providers.Warning{Path: "features.heated_chamber", Message: "no orca machine field, dropped"})
	}
	if len(m.Meta.Tags) > 0 {
		warns = append(warns, providers.Warning{Path: "meta.tags", Message: "no orca consumer, dropped (prusa renders them into printer_notes)"})
	}

	out := MachineSettings{
		Name:              *m.Meta.Name,
		Version:           orcaVersion,
		From:              "User",
		PrinterSettingsID: *m.Meta.Name, // IS the preset name, like filament_settings_id
		Inherits:          inherits,
		PrinterModel:      providers.DerefString(m.Meta.Model),
		PrinterVariant:    providers.ScalarFloatPtr(m.Nozzle.DiameterMM),

		PrintableArea:   rectToPrintableArea(m.Volume.WidthMM, m.Volume.DepthMM),
		PrintableHeight: providers.ScalarFloatPtr(m.Volume.HeightMM),

		NozzleDiameter: broadcastFloatPtr(m.Nozzle.DiameterMM),
		NozzleHeight:   providers.ScalarFloatPtr(m.Nozzle.HeightMM),
		NozzleHRC:      providers.ScalarFloatPtr(m.Nozzle.HRC),
		MinLayerHeight: broadcastFloatPtr(m.Nozzle.MinLayerHeightMM),
		MaxLayerHeight: broadcastFloatPtr(m.Nozzle.MaxLayerHeightMM),

		MachineMinTravelRate:    broadcastFloatPtr(m.Kinetics.MinTravelMMS),
		MachineMinExtrudingRate: broadcastFloatPtr(m.Kinetics.MinExtrudeMMS),

		RetractionLength:         broadcastFloatPtr(m.Retract.LengthMM),
		RetractionSpeed:          broadcastFloatPtr(m.Retract.SpeedMMS),
		DeretractionSpeed:        broadcastFloatPtr(m.Retract.DeretractSpeedMMS),
		RetractionMinimumTravel:  broadcastFloatPtr(m.Retract.MinTravelMM),
		ZHop:                     broadcastFloatPtr(m.Retract.ZHopMM),
		RetractWhenChangingLayer: boolArrayPtr(m.Retract.OnLayerChange),
		Wipe:                     boolArrayPtr(m.Retract.Wipe),

		UseRelativeEDistances: boolScalarPtr(m.Firmware.RelativeE),
		UseFirmwareRetraction: boolScalarPtr(m.Firmware.FirmwareRetraction),

		AuxiliaryFan: boolScalarPtr(m.Features.AuxFan),

		MachineStartGcode:      providers.DerefString(m.Gcode.Start),
		MachineEndGcode:        providers.DerefString(m.Gcode.End),
		BeforeLayerChangeGcode: providers.DerefString(m.Gcode.BeforeLayer),
		LayerChangeGcode:       providers.DerefString(m.Gcode.LayerChange),
		MachinePauseGcode:      providers.DerefString(m.Gcode.Pause),
		ChangeFilamentGcode:    providers.DerefString(m.Gcode.FilamentChg),

		DefaultPrintProfile: providers.DerefString(m.Defaults.PrintProfile),
		PrinterNotes:        providers.DerefString(m.Meta.Notes),
	}
	if m.Gcode.Flavor != nil {
		out.GcodeFlavor = string(*m.Gcode.Flavor)
	}

	out.MachineMaxSpeedX, out.MachineMaxSpeedY, out.MachineMaxSpeedZ, out.MachineMaxSpeedE = pairAxes(m.Kinetics.MaxSpeedMMS)
	out.MachineMaxAccelerationX, out.MachineMaxAccelerationY, out.MachineMaxAccelerationZ, out.MachineMaxAccelerationE = pairAxes(m.Kinetics.MaxAccelMMS2)
	out.MachineMaxJerkX, out.MachineMaxJerkY, out.MachineMaxJerkZ, out.MachineMaxJerkE = pairAxes(m.Kinetics.MaxJerkMMS)

	if extruderType != "" {
		out.ExtruderType = broadcast(extruderType)
	}
	if m.Nozzle.Type != nil {
		out.NozzleType = broadcast(string(*m.Nozzle.Type))
	}
	if m.Features.AirFiltration != nil {
		out.SupportAirFiltration = bool01(*m.Features.AirFiltration)
	}
	if m.Defaults.FilamentProfile != nil {
		out.DefaultFilamentProfile = broadcast(*m.Defaults.FilamentProfile)
	}

	return out, warns, nil
}
