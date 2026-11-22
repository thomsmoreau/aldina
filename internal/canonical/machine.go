// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package canonical

// Machine is the canonical, slicer-agnostic printer profile: ONE printer
// variant (what Orca calls `machine`, e.g. "Bambu Lab P2S 0.6 nozzle").
// The parent machine_model file and the vendor meta file are parked.
//
// Conventions mirror Filament:
//   - strong types, imposed units: mm, °C, mm/s, mm/s², mm³/s
//   - a nil pointer = "absent": the key is omitted at render and the slicer
//     default applies; Validate warns, never blocks
//   - gcode blobs are opaque strings, never interpreted
//
// Canonical-only data (Limits.*, Features.Enclosed, Features.Plates,
// Nozzle.FilamentDiameterMM) is absent from BOTH slicer formats — verified
// on real exports (docs/mapping.md "Machine resource"). Providers drop it
// WITHOUT warning: it feeds CheckCompat, not slicer files.
type Machine struct {
	Kind          string `yaml:"kind"`           // always "machine"
	SchemaVersion int    `yaml:"schema_version"` // bump on breaking change

	Meta     MachineMeta     `yaml:"meta"`
	Volume   BuildVolume     `yaml:"volume"`
	Nozzle   NozzleSpec      `yaml:"nozzle"`
	Limits   MachineLimits   `yaml:"limits"`
	Kinetics Kinetics        `yaml:"kinetics"`
	Retract  MachineRetract  `yaml:"retraction"`
	Firmware MachineFirmware `yaml:"firmware"`
	Features MachineFeatures `yaml:"features"`
	Gcode    MachineGcode    `yaml:"gcode"`
	Defaults MachineDefaults `yaml:"defaults"`

	// Overrides: per-provider pass-through, same pattern as Filament.
	// Not wired yet — injection lands with the first field that needs it.
	Overrides map[string]map[string]any `yaml:"overrides,omitempty"`
}

// MachineMeta carries the identity. The LINKAGE key is (Model, Nozzle.
// DiameterMM) — what a filament's compat.machines names, resolved per slicer
// by internal/catalog.
//
// Name is a human LABEL: the title the rendered preset gets, nothing more.
// Writing Orca's catalogue title there ("Bambu Lab P2S 0.6 nozzle") restates
// Model and the nozzle in one slicer's convention — the very violation
// compat.printers used to carry (docs/mapping.md, machine pitfall #1).
//
// Vendor/Model also feed the Prusa linkage mechanism: the render derives
// PRINTER_VENDOR_*/PRINTER_MODEL_* keywords for printer_notes.
type MachineMeta struct {
	Name   *string  `yaml:"name"`
	Vendor *string  `yaml:"vendor"`
	Model  *string  `yaml:"model"`
	Tags   []string `yaml:"tags,omitempty"` // capability keywords for printer_notes ("INPUT_SHAPER", ...)
	Notes  *string  `yaml:"notes,omitempty"`
}

// BuildVolume is a simple rectangular box. Both slicers want 4 corner points
// in the "XxY" mini-format (Orca JSON array, Prusa CSV string): providers
// build them from Width/Depth. Non-rectangular beds → overrides.
type BuildVolume struct {
	WidthMM  *float64 `yaml:"width_mm"`
	DepthMM  *float64 `yaml:"depth_mm"`
	HeightMM *float64 `yaml:"height_mm"`
}

type NozzleType string

const (
	NozzleBrass     NozzleType = "brass"
	NozzleStainless NozzleType = "stainless_steel"
	NozzleHardened  NozzleType = "hardened_steel"
	NozzleTungsten  NozzleType = "tungsten_carbide"
)

type ExtruderType string

const (
	DirectDrive ExtruderType = "direct_drive"
	Bowden      ExtruderType = "bowden"
)

type NozzleSpec struct {
	DiameterMM         *float64      `yaml:"diameter_mm"`
	Type               *NozzleType   `yaml:"type"`                 // Orca-mapped; drives the abrasive cross-check
	Extruder           *ExtruderType `yaml:"extruder"`             // Orca-mapped ("Direct Drive" wire value)
	FilamentDiameterMM *float64      `yaml:"filament_diameter_mm"` // canonical-only: cross-check vs filament
	HeightMM           *float64      `yaml:"height_mm,omitempty"`
	// HRC is the nozzle hardness (Orca-only wire). 0 = unknown on real
	// profiles (the P2S exports hardened_steel with hrc "0") → skip.
	// Secondary to Type for the abrasive cross-check: the filament carries
	// no numeric required-HRC yet, so a naive numeric comparison would
	// false-positive on every abrasive filament.
	HRC *float64 `yaml:"hrc,omitempty"`
	// 1:1 in both formats (M0-verified): machine-level layer height bounds.
	MinLayerHeightMM *float64 `yaml:"min_layer_height_mm,omitempty"`
	MaxLayerHeightMM *float64 `yaml:"max_layer_height_mm,omitempty"`
}

// MachineLimits is the canonical-only section: none of these exist in either
// slicer format (they live in firmware / vendor specs). nil = unknown → the
// corresponding cross-check is SKIPPED, not failed. See crosscheck.go.
type MachineLimits struct {
	MaxNozzleTempC    *float64 `yaml:"max_nozzle_temp_c,omitempty"`
	MaxBedTempC       *float64 `yaml:"max_bed_temp_c,omitempty"`
	MaxChamberTempC   *float64 `yaml:"max_chamber_temp_c,omitempty"`
	MaxVolumetricMM3S *float64 `yaml:"max_volumetric_mm3_s,omitempty"`
}

// Kinetics holds firmware motion limits, NORMAL mode only. Both slicers store
// [normal, silent] value pairs; providers duplicate the normal mode and
// silent-mode tuning goes to overrides.
//
// Each axis group is a POINTER: motion limits are declared as a block, so
// nil = "the machine does not declare them" and the render omits the whole
// group. A zero axis stays a legal declared value.
type Kinetics struct {
	MaxSpeedMMS   *Axes    `yaml:"max_speed_mm_s"` // Prusa wire name: machine_max_feedrate_*
	MaxAccelMMS2  *Axes    `yaml:"max_accel_mm_s2"`
	MaxJerkMMS    *Axes    `yaml:"max_jerk_mm_s,omitempty"`
	MinTravelMMS  *float64 `yaml:"min_travel_mm_s,omitempty"`
	MinExtrudeMMS *float64 `yaml:"min_extrude_mm_s,omitempty"`
}

type Axes struct {
	X float64 `yaml:"x"`
	Y float64 `yaml:"y"`
	Z float64 `yaml:"z"`
	E float64 `yaml:"e"`
}

// MachineRetract holds machine-level DEFAULTS. A filament profile overrides
// them (filament retraction set) or inherits (nil → rendered "nil").
// nil HERE means "machine doesn't define it": the key is omitted at render.
type MachineRetract struct {
	LengthMM          *float64 `yaml:"length_mm,omitempty"`
	SpeedMMS          *float64 `yaml:"speed_mm_s,omitempty"`
	DeretractSpeedMMS *float64 `yaml:"deretract_speed_mm_s,omitempty"`
	MinTravelMM       *float64 `yaml:"min_travel_mm,omitempty"`
	ZHopMM            *float64 `yaml:"z_hop_mm,omitempty"`
	OnLayerChange     *bool    `yaml:"on_layer_change,omitempty"`
	Wipe              *bool    `yaml:"wipe,omitempty"`
	WipeDistanceMM    *float64 `yaml:"wipe_distance_mm,omitempty"` // Orca-only
}

// MachineFirmware holds gcode-generation behavior switches that exist in
// BOTH formats (M0-verified). use_volumetric_e is Prusa-only → overrides.
type MachineFirmware struct {
	RelativeE          *bool `yaml:"relative_e,omitempty"`
	FirmwareRetraction *bool `yaml:"firmware_retraction,omitempty"`
}

// MachineFeatures mixes two natures (docs/mapping.md): Enclosed and Plates
// are canonical-only (cross-check / future plate fan-out); the rest is
// Orca-mapped hardware description.
type MachineFeatures struct {
	Enclosed      *bool    `yaml:"enclosed"` // canonical-only: ABS/ASA chamber cross-check
	Plates        []string `yaml:"plates,omitempty"`
	AuxFan        *bool    `yaml:"aux_fan,omitempty"`
	AirFiltration *bool    `yaml:"air_filtration,omitempty"`
	HeatedChamber *bool    `yaml:"heated_chamber,omitempty"`
}

type GcodeFlavor string

const (
	FlavorMarlin  GcodeFlavor = "marlin"
	FlavorMarlin2 GcodeFlavor = "marlin2"
	FlavorKlipper GcodeFlavor = "klipper"
)

// MachineGcode: opaque blobs, never interpreted (slicer-specific templating
// on both sides). Only Flavor is structured.
type MachineGcode struct {
	Flavor      *GcodeFlavor `yaml:"flavor"`
	Start       *string      `yaml:"start,omitempty"`
	End         *string      `yaml:"end,omitempty"`
	BeforeLayer *string      `yaml:"before_layer_change,omitempty"`
	LayerChange *string      `yaml:"layer_change,omitempty"`
	Pause       *string      `yaml:"pause,omitempty"`
	FilamentChg *string      `yaml:"filament_change,omitempty"`
}

// MachineDefaults: Orca wraps default_filament_profile in an array, Prusa
// keeps it scalar — providers handle the asymmetry.
type MachineDefaults struct {
	PrintProfile    *string `yaml:"print_profile,omitempty"`
	FilamentProfile *string `yaml:"filament_profile,omitempty"`
}
