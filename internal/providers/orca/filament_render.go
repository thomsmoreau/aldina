// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// filament_render.go — canonical Filament → Orca filament preset JSON.
// Pedagogical selection: the COMPLEX cases (1:N fan-outs, broadcast,
// ratio conversion, nil inheritance, enum vocab). Straight 1:1 renames
// are left as obvious one-liners.

import (
	"fmt"
	"hash/fnv"
	"math/big"
	"slices"
	"strconv"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/providers"
)

// nExtruders is fixed to 1 variant in v0 (single value, no High Flow variant).
// If the target machine declares variants, bump this and broadcast.
const nExtruders = 1

// orcaVersion is the Semver written into every rendered preset. Any value
// parsing as x.y.z[.w] passes Orca's import gate; we pin the oldest Orca
// version the render is tested against.
const orcaVersion = "2.4.0.0"

// ---------------------------------------------------------------------------
// Small pure helpers (the composable bricks — no engine, no DSL)
// The value-free conversions (FormatFloat, RatioToPercent) live in
// internal/providers/convert.go; this file keeps only the Orca ENCODING.
// ---------------------------------------------------------------------------

// broadcast broadcasts one value into an Orca per-extruder array: "220" → ["220"].
// (Irregularity #2: 1 element = broadcast to all variants.)
//
// Prusa's `one` twin is NOT merged on purpose: a detached Prusa export always
// carries one element, Orca fans out per extruder — different encodings, same
// value (the value-free conversions live in providers/convert.go).
func broadcast(v string) []string {
	out := make([]string, nExtruders)
	for i := range out {
		out[i] = v
	}
	return out
}

// broadcastFloat is the everyday combo: broadcast(float-to-string).
func broadcastFloat(v float64) []string {
	return broadcast(providers.FormatFloat(v))
}

// broadcastFloatPtr renders a canonical *float64 with omit-on-absent policy:
// nil → nil slice → omitempty drops the key → Orca falls back to its default
// or parent preset. (Different mechanism from renderFloatPtr's explicit "nil"
// string, kept for retraction where inheritance is a VALUE, not an absence.)
func broadcastFloatPtr(p *float64) []string {
	if p == nil {
		return nil
	}
	return broadcastFloat(*p)
}

// broadcastStringPtr renders an optional string: nil → nil slice → omitempty
// drops the key.
func broadcastStringPtr(p *string) []string {
	if p == nil {
		return nil
	}
	return broadcast(*p)
}

// broadcastColour is broadcastStringPtr for the one field with a wire prefix:
// the canonical stores a bare hex ("B3B4B2", unprefixed because a leading '#'
// is an INI comment on the Prusa side), Orca wants "#B3B4B2".
func broadcastColour(p *string) []string {
	if p == nil {
		return nil
	}
	return broadcast("#" + *p)
}

// broadcastIntPtr is broadcastFloatPtr for the layer-count fields.
func broadcastIntPtr(p *int) []string {
	if p == nil {
		return nil
	}
	return broadcast(strconv.Itoa(*p))
}

// broadcastRatioPtr is RatioToPercent with the omit-on-absent policy.
func broadcastRatioPtr(p *float64) []string {
	if p == nil {
		return nil
	}
	return broadcast(providers.RatioToPercent(*p))
}

// renderShrink inverts the encoding: canonical carries a shrink RATE (0.0 =
// no shrink), Orca a size RATIO ("100%" = no shrink). Prusa spells the same
// zero "0%" — the trap this brick exists to contain.
//
// The '%' is part of the wire form: every real export carries "100%", never
// "100" (references/orca/filaments/*.json). The bare number shipped until the
// value round-trip test compared the two providers side by side.
func renderShrink(rate *float64) []string {
	if rate == nil {
		return nil
	}
	return broadcast(invertPercent(providers.RatioToExactPercent(*rate)) + "%")
}

// invertPercent turns a shrink percent into Orca's size percent, 100 − p, on
// the exact decimal: in float64, 1 − 0.07 is 0.9299999999999999. A
// non-decimal p (±Inf, NaN) passes through the float path.
func invertPercent(p string) string {
	d, ok := new(big.Rat).SetString(p)
	if !ok {
		v, _ := strconv.ParseFloat(p, 64)
		return providers.FormatFloat(100 - v)
	}
	f, _ := d.Sub(big.NewRat(100, 1), d).Float64()
	return providers.FormatFloat(f)
}

// renderFloatPtr renders a *float64: nil → "nil" (inherit machine), else the value.
// This is the shared Slic3r inheritance convention (irregularity #7).
func renderFloatPtr(p *float64) []string {
	if p == nil {
		return broadcast("nil")
	}
	return broadcastFloat(*p)
}

func renderBoolPtr(p *bool) []string {
	if p == nil {
		return broadcast("nil")
	}
	if *p {
		return broadcast("1")
	}
	return broadcast("0")
}

// ---------------------------------------------------------------------------
// CASE 1 — Identity: ONE canonical name → THREE consistent Orca fields (1:N)
// ---------------------------------------------------------------------------
// Orca carries redundant identity (irregularity #8): `name`,
// `filament_settings_id` and `filament_id` must all be generated
// COHERENTLY or the import misbehaves (pain point P2).
// Convention: presetName = "<Name> @<MachinePresetName>" like vendor profiles.
// renderFilamentIdentity returns the identity plus loss warnings: bubbling
// warnings through return values keeps every helper pure.
// Callers MUST guarantee f.Meta.Name != nil (RenderFilament's guard): it is
// the one canonical field without which no importable Orca file exists.
func renderFilamentIdentity(f canonical.Filament, target canonical.MachineRef) (FilamentIdentity, []providers.Warning) {
	var warns []providers.Warning
	// The suffix is a LABEL, the canonical reference verbatim: it no longer
	// has to match a preset in Orca's catalogue, which is the coupling this
	// slice removed.
	presetName := fmt.Sprintf("%s @%s", *f.Meta.Name, target)

	var filamentType []string
	if f.Meta.Material == nil {
		warns = append(warns, providers.Warning{Path: "meta.material", Message: "absent, filament_type omitted (slicer default applies)"})
	} else {
		material, loss := mapMaterialToOrca(*f.Meta.Material)
		if loss != "" {
			warns = append(warns, providers.Warning{Path: "meta.material", Message: loss})
		}
		filamentType = broadcast(material)
	}

	return FilamentIdentity{
		Name:                  new(presetName),
		Version:               new(orcaVersion),
		From:                  new("User"),
		FilamentSettingsID:    []string{presetName},   // MUST equal Name
		FilamentID:            makeFilamentID(f.Meta), // vendor prefix + hash, cf. makeFilamentID
		FilamentType:          filamentType,
		FilamentVendor:        broadcastStringPtr(f.Meta.Vendor),
		FilamentNotes:         broadcastStringPtr(f.Meta.Notes),
		DefaultFilamentColour: broadcastColour(f.Meta.ColorHex),
		Inherits:              "", // standalone preset; or a known system base
	}, warns
}

// mapMaterialToOrca maps the canonical enum to Orca vocabulary. The second
// return is a loss message ("" when the mapping is exact) — the caller, which
// knows the path, collects it into RenderFilament's warns, never a silent drop.
// (Prusa's version of this maps PETG → "PET": same concept, two vocabularies.)
func mapMaterialToOrca(m canonical.Material) (string, string) {
	// Orca happens to match the canonical names for common materials,
	// but the mapping stays EXPLICIT: never assume identity between
	// canonical vocabulary and a provider's.
	switch m {
	case canonical.PETG:
		return "PETG", ""
	case canonical.PHA:
		return "PLA", "PHA unsupported by orca, rendered as PLA"
	default:
		return string(m), ""
	}
}

// ---------------------------------------------------------------------------
// CASE 2 — Bed temperature: ONE canonical value → 12 Orca fields (worst 1:N)
// ---------------------------------------------------------------------------
// Orca declines bed temp per PLATE TYPE (6 types × normal/initial-layer).
// Policy (documented in canonical v0.2 §3.4):
//   - hot_plate + textured_plate  ← canonical bed temp (the common plates)
//   - eng_plate + supertack        ← same value (safe default, override if needed)
//   - cool_plate*                  ← "0" = plate INCOMPATIBLE (hidden semantics:
//                                    PETG at 80°C bed has no business on a cool plate)
// Anything finer goes through overrides.orca.

func renderTemperatures(t canonical.Temps) FilamentTemp {
	bed, bedFL := broadcastFloatPtr(t.Bed), broadcastFloatPtr(t.BedFirstLayer)
	incompatible := broadcast("0")

	// nozzle_range is declared as a block: absent → both bounds omitted.
	var rangeLow, rangeHigh []string
	if t.NozzleRange != nil {
		rangeLow, rangeHigh = broadcastFloat(t.NozzleRange[0]), broadcastFloat(t.NozzleRange[1])
	}

	return FilamentTemp{
		HotPlateTemp:                      bed,
		HotPlateTempInitialLayer:          bedFL,
		TexturedPlateTemp:                 bed,
		TexturedPlateTempInitialLayer:     bedFL,
		EngPlateTemp:                      bed,
		EngPlateTempInitialLayer:          bedFL,
		SupertackPlateTemp:                bed,
		SupertackPlateTempInitialLayer:    bedFL,
		CoolPlateTemp:                     incompatible,
		CoolPlateTempInitialLayer:         incompatible,
		TexturedCoolPlateTemp:             incompatible,
		TexturedCoolPlateTempInitialLayer: incompatible,

		// nozzle: plain rename + broadcast (the easy part of the same group)
		NozzleTemperature:             broadcastFloatPtr(t.Nozzle),
		NozzleTemperatureInitialLayer: broadcastFloatPtr(t.NozzleFirstLayer),
		NozzleTemperatureRangeLow:     rangeLow,
		NozzleTemperatureRangeHigh:    rangeHigh,
	}
}

// ---------------------------------------------------------------------------
// CASE 3 — Cooling: unit conversion (ratio 0–1 → "0–100" strings)
// ---------------------------------------------------------------------------

func renderFanCooling(c canonical.Cooling) FilamentFan {
	return FilamentFan{
		FanMinSpeed:             broadcastRatioPtr(c.FanMin), // 0.30 → "30"
		FanMaxSpeed:             broadcastRatioPtr(c.FanMax),
		CloseFanTheFirstXLayers: broadcastIntPtr(c.FanOffFirstLayers),
		FullFanSpeedLayer:       broadcastIntPtr(c.FullFanAtLayer),
		SlowDownLayerTime:       broadcastFloatPtr(c.SlowDownLayerTimeS),
		SlowDownMinSpeed:        broadcastFloatPtr(c.SlowDownMinSpeedMMS),
		SlowDownForLayerCooling: broadcast("1"),
		OverhangFanSpeed:        broadcastRatioPtr(c.BridgesFan),
		EnableOverhangBridgeFan: broadcast("1"),
	}
}

// ---------------------------------------------------------------------------
// CASE 4 — Retraction: nil-pointer → "nil" (inheritance pass-through)
// ---------------------------------------------------------------------------
// canonical nil = "inherit from machine". Orca spells it "nil" (string).
// One helper, applied uniformly — the whole quirk is contained here.

func renderRetractionOverrides(r canonical.Retraction) FilamentRetraction {
	return FilamentRetraction{
		FilamentRetractionLength:  renderFloatPtr(r.LengthMM),
		FilamentRetractionSpeed:   renderFloatPtr(r.SpeedMMS),
		FilamentDeretractionSpeed: renderFloatPtr(r.DeretractSpeedMMS),
		FilamentZHop:              renderFloatPtr(r.ZHopMM),
		FilamentWipe:              renderBoolPtr(r.Wipe),
		FilamentWipeDistance:      renderFloatPtr(r.WipeDistanceMM),
	}
}

// ---------------------------------------------------------------------------
// Assembly + overrides injection
// ---------------------------------------------------------------------------

// RenderFilament assembles the full Orca preset for ONE target machine.
// target is resolved against Orca's catalogue (compat.go): the preset names it
// yields drive compatible_printers, and the reference itself labels the preset.
// Loss policy: fields the canon carries but Orca can't express → collect
// warnings (returned to the CLI), never drop silently.
func RenderFilament(canonicalFilament canonical.Filament, target canonical.MachineRef) (FilamentSettings, []providers.Warning, error) {
	// Canonical fields Orca cannot import without (see required.go). Checked
	// here so a library caller cannot reach the renderers with a nil name.
	if err := providers.MissingError(filamentKind, requiredFilament(canonicalFilament)); err != nil {
		return FilamentSettings{}, nil, err
	}
	// target is a RENDER ARGUMENT, not a canonical field: it stays out of the
	// required list and is guarded by the catalogue lookup itself.
	compatiblePrinters, err := resolveCompat(target)
	if err != nil {
		return FilamentSettings{}, nil, err
	}
	// Self-link: also declare the machine's own title, so an imported aldina
	// machine pairs with its filament (mapping.md, machine pitfall #1).
	if self := target.String(); !slices.Contains(compatiblePrinters, self) {
		compatiblePrinters = append(compatiblePrinters, self)
	}

	identity, warns := renderFilamentIdentity(canonicalFilament, target)

	out := FilamentSettings{
		FilamentIdentity:      identity,
		FilamentTemp:          renderTemperatures(canonicalFilament.Temps),
		FilamentFan:           renderFanCooling(canonicalFilament.Cooling),
		FilamentRetraction:    renderRetractionOverrides(canonicalFilament.Retraction),
		FilamentCompatibility: FilamentCompatibility{CompatiblePrinters: compatiblePrinters},
		FilamentPhysical: FilamentPhysical{
			FilamentDiameter:         broadcastFloatPtr(canonicalFilament.Physical.DiameterMM),
			FilamentDensity:          broadcastFloatPtr(canonicalFilament.Physical.DensityGCm3),
			FilamentCost:             broadcastFloatPtr(canonicalFilament.Physical.CostPerKg),
			TemperatureVitrification: broadcastFloatPtr(canonicalFilament.Temps.Softening),
			// shrink: canonical RATE 0.0 → Orca "100%" (size ratio — inverted encodings!)
			FilamentShrink: renderShrink(canonicalFilament.Physical.ShrinkageXY),
			// same key name as Prusa's, inverted meaning (pitfall #14)
			FilamentShrinkageCompensationZ: renderShrink(canonicalFilament.Physical.ShrinkageZ),
		},
		FilamentFlow: FilamentFlow{
			FilamentMaxVolumetricSpeed: broadcastFloatPtr(canonicalFilament.Flow.MaxVolumetricMM3S),
			FilamentFlowRatio:          broadcastFloatPtr(canonicalFilament.Flow.FlowRatio),
		},
		FilamentGcode: FilamentGcode{
			FilamentStartGcode: gcodePtr(canonicalFilament.Gcode.Start),
			FilamentEndGcode:   gcodePtr(canonicalFilament.Gcode.End),
		},
	}

	if abrasive := canonicalFilament.Physical.Abrasive; abrasive != nil {
		if *abrasive {
			// canonical bool → Orca hardness scale. "40" is the value Orca's own
			// abrasive filaments carry (measured across 7053 system presets);
			// "55" was invented and matched nothing in the real data.
			out.RequiredNozzleHRC = broadcast("40")
		} else {
			// Orca's library carries HRC "0" on abrasive filaments too, so ingest
			// reads a low value as unknown (parseOrcaAbrasive): false cannot round-trip.
			warns = append(warns, providers.Warning{Path: "physical.abrasive", Message: "false has no orca encoding, required_nozzle_HRC omitted (reads back as unknown)"})
		}
	}
	if len(canonicalFilament.Overrides["prusa"]) > 0 {
		warns = append(warns, providers.Warning{Path: "overrides.prusa", Message: "prusa-only pass-through, not rendered"})
	}

	// TODO overrides: inject f.Overrides["orca"] as raw JSON fields on top
	// of the rendered struct (marshal struct → map, merge, marshal). This is
	// where the 20% irreducible lands untouched.

	return out, warns, nil
}

// gcodePtr renders an optional gcode blob: nil canonical → key omitted (Orca
// keeps its default), never an empty string blob.
func gcodePtr(p *string) []string {
	if p == nil {
		return nil
	}
	return []string{*p}
}

// makeFilamentID builds a short stable id: readable vendor prefix + 6 hex
// chars of FNV-1a(vendor+"/"+name). Name must feed the hash: the old
// vendor+material prefix collided ("eSUN PLA+" vs "eSUN PLA Matte"), and
// Orca uses filament_id to group presets of one filament — a collision
// merges two distinct filaments.
func makeFilamentID(m canonical.Meta) string {
	// Vendor is dereferenced through providers.DerefString: an absent vendor
	// must hash like an empty one, never like a pointer address.
	vendor := providers.DerefString(m.Vendor)
	h := fnv.New32a()
	fmt.Fprintf(h, "%s/%s", vendor, *m.Name) // Name guaranteed by RenderFilament's guard
	return "P" + truncate6(vendor) + fmt.Sprintf("%06x", h.Sum32()&0xFFFFFF)
}

func truncate6(s string) string {
	if len(s) > 6 {
		return s[:6]
	}
	return s
}
