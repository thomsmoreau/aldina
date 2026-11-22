// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// machine_render.go — canonical Machine → PrusaSlicer printer preset INI.
// Reuses the shared conversions (providers/convert.go) and the filament
// bricks that keep their own ENCODING (one, pair, bool01Ptr) — and adds
// only what the machine wire form needs. Field-by-field mapping: docs/mapping.md §Machine.

import (
	"fmt"
	"strings"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/catalog"
	"github.com/thomsmoreau/Aldina/internal/providers"
)

// ---------------------------------------------------------------------------
// Machine-only bricks
// ---------------------------------------------------------------------------

// printerTechnology is the only constant of the preset: Aldina models FFF
// machines. SLA printers have a disjoint key set, not a variant of this one.
const printerTechnology = "FFF"

// pair renders one canonical nominal value as Prusa's [normal, silent] pair.
// COMMA-joined: Slic3r serialises numeric vectors with ',' where it uses ';'
// for string lists — which is why every kinetics field is a `string` and not a
// []string (machine_schema.go).
func pair(v float64) string { return providers.FormatFloat(v) + "," + providers.FormatFloat(v) }

// pairAxes renders one kinetics axis group, declared as a block: nil = the
// machine does not state its motion limits → all four keys omitted. A zero
// axis is a legal declared value.
func pairAxes(a *canonical.Axes) (x, y, z, e string) {
	if a == nil {
		return "", "", "", ""
	}
	return pair(a.X), pair(a.Y), pair(a.Z), pair(a.E)
}

// rectToBedShape expands a rectangular bed to its 4 corners as ONE CSV string —
// where Orca writes a JSON array of the same corners (pitfall #2). Same corner
// order: origin → +X → +X+Y → +Y. A corner needs BOTH dimensions, so a missing
// one omits the whole key rather than emitting a degenerate 0-wide bed.
func rectToBedShape(widthMM, depthMM *float64) string {
	if widthMM == nil || depthMM == nil {
		return ""
	}
	return strings.Join([]string{
		providers.FormatPoint(0, 0),
		providers.FormatPoint(*widthMM, 0),
		providers.FormatPoint(*widthMM, *depthMM),
		providers.FormatPoint(0, *depthMM),
	}, ",")
}

// bool01Ptr renders a machine bool. Prusa writes bare 1/0 where Orca writes the
// quoted strings "1"/"0" (pitfall #4); the ini writer never quotes a `string`,
// so the same characters land bare here.
func bool01Ptr(p *bool) string {
	if p == nil {
		return ""
	}
	if *p {
		return "1"
	}
	return "0"
}

// vendorKeyword derives the PRINTER_VENDOR_ suffix from a free-text vendor.
//
// It is a BEST EFFORT and cannot be otherwise: the real Prusa exports say
// PRINTER_VENDOR_PRUSA3D, which no mechanical rule produces from "Prusa
// Research". The consequence is documented as machine pitfall #11 — stock
// vendor filaments that regex on the canonical keyword will not match a printer
// we render. Our own filaments bind by printer_model and are unaffected.
func vendorKeyword(vendor string) string {
	var b strings.Builder
	b.Grow(len(vendor))
	for _, r := range strings.ToUpper(vendor) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// printerNotes assembles the field PrusaSlicer filaments match against: free
// text first, then the derived identity keywords, then the author's capability
// tags — one per line, the shape of the real XL export.
//
// PRINTER_MODEL_ takes the catalogue id VERBATIM (it may hold '+', '-', '.' or
// a space) because stock presets regex for that exact string; sanitising it
// would break the very match the keyword exists for. This is also where
// meta.tags land — orca.RenderMachine warns that it drops them here.
func printerNotes(m canonical.Machine, modelID string) string {
	lines := make([]string, 0, 3+len(m.Meta.Tags))
	if notes := providers.DerefString(m.Meta.Notes); notes != "" {
		lines = append(lines, notes)
	}
	if vendor := providers.DerefString(m.Meta.Vendor); vendor != "" {
		lines = append(lines, "PRINTER_VENDOR_"+vendorKeyword(vendor))
	}
	lines = append(lines, "PRINTER_MODEL_"+modelID)
	lines = append(lines, m.Meta.Tags...)
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Assembly
// ---------------------------------------------------------------------------

// RenderMachine assembles a PrusaSlicer printer preset.
//
// Unlike its Orca counterpart it CONSULTS THE CATALOGUE, and that is the point
// of the whole slice: PrusaSlicer binds a filament to a printer by expression
// (printer_model=="MINI"), comparing against a model ID the author never
// writes. Emitting meta.model verbatim would produce a preset that imports
// cleanly and binds to nothing — accepted but invisible. So the same
// catalog.Lookup that builds the filament's compatible_printers_condition
// builds the printer's identity, and an unknown machine is a hard error rather
// than a silently broken pair.
//
// inherits is a provider option, not canonical data: the parent must be a
// system preset installed in the reader's PrusaSlicer. Empty emits a standalone
// preset (mapping.md pitfall #7).
//
// Canonical-only fields (limits.*, features.enclosed/plates,
// nozzle.filament_diameter_mm) are NOT warnings: they feed `aldina validate`
// and CheckCompat, not slicer files. Fields Prusa genuinely cannot express are.
func RenderMachine(m canonical.Machine, inherits string) (MachineSettings, []string, error) {
	if err := providers.MissingError(machineKind, requiredMachine(m)); err != nil {
		return MachineSettings{}, nil, err
	}
	// inherits is a RENDER ARGUMENT, not a canonical field: guarded on its own.
	if inherits == *m.Meta.Name {
		return MachineSettings{}, nil, fmt.Errorf("inherits: must differ from meta.name to avoid a self-referential prusa preset")
	}

	entry, err := catalog.Lookup(catalog.Prusa, *m.Meta.Model, *m.Nozzle.DiameterMM)
	if err != nil {
		return MachineSettings{}, nil, fmt.Errorf("meta.model: %w", err)
	}

	var warns []string
	// Orca-only machine properties (mapping.md §Nozzle, §Retraction,
	// §Features). Real hardware facts the canonical is right to carry — Prusa
	// simply has no key for them, so they are announced, never dropped mute.
	for _, drop := range []struct {
		present bool
		field   string
	}{
		{m.Nozzle.Type != nil, "nozzle.type"},
		{m.Nozzle.Extruder != nil, "nozzle.extruder"},
		{m.Nozzle.HeightMM != nil, "nozzle.height_mm"},
		{m.Nozzle.HRC != nil, "nozzle.hrc"},
		{m.Retract.WipeDistanceMM != nil, "retraction.wipe_distance_mm"},
		{m.Features.AuxFan != nil, "features.aux_fan"},
		{m.Features.AirFiltration != nil, "features.air_filtration"},
		{m.Features.HeatedChamber != nil, "features.heated_chamber"},
	} {
		if drop.present {
			warns = append(warns, drop.field+": no prusa machine field, dropped")
		}
	}

	out := MachineSettings{
		PrinterModel: entry.ModelID,
		// The catalogue's own spelling, not providers.FormatFloat(nozzle): PrusaSlicer
		// matches variants as STRINGS and the catalogue holds "0.40"/"0.50"
		// for some models. Lookup only ever answers with a plain variant.
		PrinterVariant:    entry.Variant,
		PrinterVendor:     providers.DerefString(m.Meta.Vendor),
		PrinterTechnology: printerTechnology,
		PrinterSettingsID: *m.Meta.Name, // IS the preset name, like the filament's
		Inherits:          inherits,
		PrinterNotes:      printerNotes(m, entry.ModelID),

		BedShape:       rectToBedShape(m.Volume.WidthMM, m.Volume.DepthMM),
		MaxPrintHeight: providers.ScalarFloatPtr(m.Volume.HeightMM),

		NozzleDiameter: providers.FormatFloat(*m.Nozzle.DiameterMM),
		MinLayerHeight: providers.ScalarFloatPtr(m.Nozzle.MinLayerHeightMM),
		MaxLayerHeight: providers.ScalarFloatPtr(m.Nozzle.MaxLayerHeightMM),

		RetractLength:       providers.ScalarFloatPtr(m.Retract.LengthMM),
		RetractSpeed:        providers.ScalarFloatPtr(m.Retract.SpeedMMS),
		DeretractSpeed:      providers.ScalarFloatPtr(m.Retract.DeretractSpeedMMS),
		RetractBeforeTravel: providers.ScalarFloatPtr(m.Retract.MinTravelMM),
		RetractLift:         providers.ScalarFloatPtr(m.Retract.ZHopMM),
		RetractLayerChange:  bool01Ptr(m.Retract.OnLayerChange),
		Wipe:                bool01Ptr(m.Retract.Wipe),

		UseRelativeEDistance: bool01Ptr(m.Firmware.RelativeE),
		UseFirmwareRetract:   bool01Ptr(m.Firmware.FirmwareRetraction),

		StartGcode:       providers.DerefString(m.Gcode.Start),
		EndGcode:         providers.DerefString(m.Gcode.End),
		BeforeLayerGcode: providers.DerefString(m.Gcode.BeforeLayer),
		LayerGcode:       providers.DerefString(m.Gcode.LayerChange),
		PausePrintGcode:  providers.DerefString(m.Gcode.Pause),
		ColorChangeGcode: providers.DerefString(m.Gcode.FilamentChg),

		DefaultPrintProfile: providers.DerefString(m.Defaults.PrintProfile),
	}
	if m.Gcode.Flavor != nil {
		out.GcodeFlavor = string(*m.Gcode.Flavor)
	}
	// Scalar on the wire but a ConfigOptionStrings, hence the list encoding:
	// the real export quotes it, which only a list does (pitfall #5).
	if m.Defaults.FilamentProfile != nil {
		out.DefaultFilamentProfile = one(*m.Defaults.FilamentProfile)
	}

	out.MachineMaxFeedrateX, out.MachineMaxFeedrateY, out.MachineMaxFeedrateZ, out.MachineMaxFeedrateE = pairAxes(m.Kinetics.MaxSpeedMMS)
	out.MachineMaxAccelerationX, out.MachineMaxAccelerationY, out.MachineMaxAccelerationZ, out.MachineMaxAccelerationE = pairAxes(m.Kinetics.MaxAccelMMS2)
	out.MachineMaxJerkX, out.MachineMaxJerkY, out.MachineMaxJerkZ, out.MachineMaxJerkE = pairAxes(m.Kinetics.MaxJerkMMS)
	if p := m.Kinetics.MinTravelMMS; p != nil {
		out.MachineMinTravelRate = pair(*p)
	}
	if p := m.Kinetics.MinExtrudeMMS; p != nil {
		out.MachineMinExtrudingRate = pair(*p)
	}

	return out, warns, nil
}
