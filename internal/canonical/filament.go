// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package canonical

import (
	"fmt"
	"strconv"
)

// Filament is the canonical, slicer-agnostic filament profile.
// Design rules (cf. docs/aldina-guide-format-canonique.md):
//   - strong types, imposed units: °C, mm, mm/s, mm³/s, ratios 0.0–1.0
//   - a nil pointer = "absent": the key is OMITTED at render and the slicer
//     default (or parent preset) applies. Validate warns but never blocks —
//     the canonical format signals, providers decide what is unrenderable.
//     (Exception: retraction.*, where nil renders the explicit "nil" string
//     both slicers use for "inherit from machine".)
//   - EVERY tunable scalar is a pointer: a zero value is a legitimate author
//     intent (`bed: 0` = unheated bed) and must stay distinguishable from an
//     absent key. Three deliberate exceptions, none of them a tunable:
//     Kind/SchemaVersion describe the document (absence = load error, not
//     "unset"), and slices/maps already carry their own nil-vs-empty
//     distinction, so wrapping them in a pointer would buy nothing.
//   - gcode blobs are opaque strings, never interpreted
//   - anything slicer-specific lives in Overrides, never in the core
type Filament struct {
	Kind          string `yaml:"kind"`           // always "filament"
	SchemaVersion int    `yaml:"schema_version"` // bump on breaking change

	Meta       Meta       `yaml:"meta"`
	Physical   Physical   `yaml:"physical"`
	Temps      Temps      `yaml:"temps"`
	Flow       Flow       `yaml:"flow"`
	Cooling    Cooling    `yaml:"cooling"`
	Retraction Retraction `yaml:"retraction"`
	Gcode      Gcode      `yaml:"gcode"`
	Compat     Compat     `yaml:"compat"`

	// Overrides: per-provider pass-through for the irreducible 20%.
	// Keys are provider names ("orca", "prusa"), values are raw
	// target-format fields injected as-is at render time.
	Overrides map[string]map[string]any `yaml:"overrides,omitempty"`
}

// Material is a canonical enum: each provider maps it to its own vocabulary
// (e.g. PETG → Orca "PETG", Prusa "PET").
type Material string

const (
	PLA  Material = "PLA"
	PETG Material = "PETG"
	ABS  Material = "ABS"
	ASA  Material = "ASA"
	TPU  Material = "TPU"
	PC   Material = "PC"
	PA   Material = "PA"
	PHA  Material = "PHA"
)

type Meta struct {
	Name     *string   `yaml:"name"`   // human name: "Overture PETG Rock White"
	Vendor   *string   `yaml:"vendor"` // "Overture"
	Material *Material `yaml:"material"`
	ColorHex *string   `yaml:"color_hex,omitempty"` // "B3B4B2" — WITHOUT '#' (INI trap)
	Notes    *string   `yaml:"notes,omitempty"`
}

type Physical struct {
	DiameterMM  *float64 `yaml:"diameter_mm"`   // 1.75
	DensityGCm3 *float64 `yaml:"density_g_cm3"` // 1.27
	CostPerKg   *float64 `yaml:"cost_per_kg,omitempty"`
	Abrasive    *bool    `yaml:"abrasive,omitempty"`     // → Prusa bool / Orca HRC
	ShrinkageXY *float64 `yaml:"shrinkage_xy,omitempty"` // shrink RATE, 0.0 = none
	ShrinkageZ  *float64 `yaml:"shrinkage_z,omitempty"`  // (Orca "100%" / Prusa "0%" both = 0.0 here)
}

type Temps struct {
	Nozzle           *float64    `yaml:"nozzle"` // °C
	NozzleFirstLayer *float64    `yaml:"nozzle_first_layer"`
	NozzleRange      *[2]float64 `yaml:"nozzle_range,omitempty"` // [min, max]
	Bed              *float64    `yaml:"bed"`                    // ONE value; Orca fans it out per plate type
	BedFirstLayer    *float64    `yaml:"bed_first_layer"`
	Softening        *float64    `yaml:"softening,omitempty"` // vitrification; dropped+warned by Prusa
}

type Flow struct {
	MaxVolumetricMM3S *float64 `yaml:"max_volumetric_mm3_s"` // 25.0
	FlowRatio         *float64 `yaml:"flow_ratio"`           // pure ratio, 1.0 = neutral
}

type Cooling struct {
	FanMin              *float64 `yaml:"fan_min"` // ratio 0–1 (0.30, not "30")
	FanMax              *float64 `yaml:"fan_max"`
	FanOffFirstLayers   *int     `yaml:"fan_off_first_layers"`
	FullFanAtLayer      *int     `yaml:"full_fan_at_layer,omitempty"`
	SlowDownLayerTimeS  *float64 `yaml:"slow_down_layer_time_s"`
	SlowDownMinSpeedMMS *float64 `yaml:"slow_down_min_speed_mm_s"`
	BridgesFan          *float64 `yaml:"bridges_fan,omitempty"` // ratio 0–1
}

// Retraction: every field is a POINTER — nil renders the explicit "nil"
// string = "inherit from machine", mirroring the convention shared by both
// slicers. (Unlike other groups, inheritance here is a VALUE, not an absence.)
type Retraction struct {
	LengthMM          *float64 `yaml:"length_mm,omitempty"`
	SpeedMMS          *float64 `yaml:"speed_mm_s,omitempty"`
	DeretractSpeedMMS *float64 `yaml:"deretract_speed_mm_s,omitempty"`
	ZHopMM            *float64 `yaml:"z_hop_mm,omitempty"`
	Wipe              *bool    `yaml:"wipe,omitempty"`
	WipeDistanceMM    *float64 `yaml:"wipe_distance_mm,omitempty"`
}

// Gcode blobs are opaque and provider-specific by nature; the canonical file
// may carry a neutral default, providers may replace via Overrides.
type Gcode struct {
	Start *string `yaml:"start,omitempty"`
	End   *string `yaml:"end,omitempty"`
}

// Compat names the MACHINES a filament targets, never presets. A preset name
// belongs to a slicer's catalogue, not to the machine — the same printer is
// "Prusa MINI" in Orca and "Original Prusa MINI & MINI+" in PrusaSlicer, and
// the two formats do not even bind the same way (exact names vs an expression:
// docs/mapping.md pitfall #13). Each provider resolves the reference in its own
// catalogue through internal/catalog.
//
// An empty list means "no restriction", which both formats express by omission.
type Compat struct {
	Machines []MachineRef `yaml:"machines,omitempty"`
}

// MachineRef identifies one machine: a model and the nozzle fitted to it.
//
// Bare scalars, not pointers, and this is the fourth deliberate exception to
// the pointer rule: MachineRef is itself the unit of presence, like *Axes on
// the machine side. Its fields are an identity rather than a tunable, so ""
// and 0 are simply invalid — there is no authored zero to keep distinct from
// an absence.
//
// No vendor field: a model name is globally unique inside each catalogue
// (measured, 0 collision across vendors on both sides), so a vendor would
// discriminate nothing — and YAGNI forbids a field no consumer reads.
type MachineRef struct {
	Model    string  `yaml:"model"`
	NozzleMM float64 `yaml:"nozzle_mm"`
}

// String is how a reference reads back to its author. There is ONE of these on
// purpose: the label Orca appends to a preset name and the warning CheckCompat
// prints are the same fact, and they used to be formatted by two different
// pieces of code that disagreed on the unit.
//
// No "mm": the field is already called nozzle_mm in the file.
func (r MachineRef) String() string {
	return fmt.Sprintf("%s %s", r.Model, strconv.FormatFloat(r.NozzleMM, 'g', -1, 64))
}

// Matches reports whether m is the machine this reference designates. Model
// names are compared as authored: the alias table that bridges the two
// catalogues lives in internal/catalog, and a canonical machine is described
// by its author, not by a slicer's catalogue.
func (r MachineRef) Matches(m Machine) bool {
	if m.Meta.Model == nil || *m.Meta.Model != r.Model {
		return false
	}
	return m.Nozzle.DiameterMM != nil && *m.Nozzle.DiameterMM == r.NozzleMM
}
