// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// machine_schema.go — Orca machine-preset JSON schema, frozen on a REAL
// Orca 2.1.0.16 user export (references/orca/printers/Bambu Lab P2S 0.6 nozzle.json).
//
// Cardinality per key was READ from that file, never assumed (mapping.md,
// pitfalls #1–#3):
//   - scalars: printable_height, nozzle_height, firmware bools, gcode blobs
//   - per-variant arrays ([v] in v0): nozzle_diameter, nozzle_type,
//     extruder_type, min/max_layer_height, retraction_*
//   - kinetics pairs ([normal, silent] × variants → [v,v] in v0):
//     machine_max_speed_*, machine_max_acceleration_*, machine_max_jerk_*
//   - array[1] outliers: machine_min_travel_rate, machine_min_extruding_rate
//
// The two point mini-formats (XxY vs X,Y, pitfall #2) are NOT unified here:
// only the XxY form is on the render path (printable_area).
//
// Fields not rendered in v0 (overrides.orca in mapping.md) are absent from
// the struct — a closed struct never emits zero-valued keys.
type MachineSettings struct {
	// identity — the import gate is name/version/from. No "type" key:
	// user exports (machine AND filament) don't carry one; only system
	// profiles do. setting_id/instantiation likewise absent in 2.1.0.16.
	Name    string `json:"name"`
	Version string `json:"version"`
	From    string `json:"from"` // "User" for rendered presets
	// SCALAR, unlike the filament's array form — read off the real export.
	// Every other kind of user preset that Orca accepts carries its own
	// <kind>_settings_id; the machine was the only one missing it.
	PrinterSettingsID string `json:"printer_settings_id"`
	Inherits          string `json:"inherits"`
	PrinterModel      string `json:"printer_model"`
	PrinterVariant    string `json:"printer_variant"` // derived from nozzle diameter

	// volume
	PrintableArea   []string `json:"printable_area,omitempty"` // 4 corners, XxY mini-format
	PrintableHeight string   `json:"printable_height,omitempty"`

	// nozzle
	NozzleDiameter []string `json:"nozzle_diameter,omitempty"`
	NozzleType     []string `json:"nozzle_type,omitempty"`
	NozzleHeight   string   `json:"nozzle_height,omitempty"`
	NozzleHRC      string   `json:"nozzle_hrc,omitempty"`
	ExtruderType   []string `json:"extruder_type,omitempty"` // "Direct Drive", WITH space
	MinLayerHeight []string `json:"min_layer_height,omitempty"`
	MaxLayerHeight []string `json:"max_layer_height,omitempty"`

	// kinetics — [normal, silent] for the single v0 variant
	MachineMaxSpeedX        []string `json:"machine_max_speed_x,omitempty"`
	MachineMaxSpeedY        []string `json:"machine_max_speed_y,omitempty"`
	MachineMaxSpeedZ        []string `json:"machine_max_speed_z,omitempty"`
	MachineMaxSpeedE        []string `json:"machine_max_speed_e,omitempty"`
	MachineMaxAccelerationX []string `json:"machine_max_acceleration_x,omitempty"`
	MachineMaxAccelerationY []string `json:"machine_max_acceleration_y,omitempty"`
	MachineMaxAccelerationZ []string `json:"machine_max_acceleration_z,omitempty"`
	MachineMaxAccelerationE []string `json:"machine_max_acceleration_e,omitempty"`
	MachineMaxJerkX         []string `json:"machine_max_jerk_x,omitempty"`
	MachineMaxJerkY         []string `json:"machine_max_jerk_y,omitempty"`
	MachineMaxJerkZ         []string `json:"machine_max_jerk_z,omitempty"`
	MachineMaxJerkE         []string `json:"machine_max_jerk_e,omitempty"`
	MachineMinTravelRate    []string `json:"machine_min_travel_rate,omitempty"`
	MachineMinExtrudingRate []string `json:"machine_min_extruding_rate,omitempty"`

	// retraction — per variant; nil canonical → key omitted (the "nil"
	// string inheritance trick is filament-side only)
	RetractionLength         []string `json:"retraction_length,omitempty"`
	RetractionSpeed          []string `json:"retraction_speed,omitempty"`
	DeretractionSpeed        []string `json:"deretraction_speed,omitempty"`
	RetractionMinimumTravel  []string `json:"retraction_minimum_travel,omitempty"`
	ZHop                     []string `json:"z_hop,omitempty"`
	RetractWhenChangingLayer []string `json:"retract_when_changing_layer,omitempty"`
	Wipe                     []string `json:"wipe,omitempty"`

	// firmware — scalar "1"/"0"
	UseRelativeEDistances string `json:"use_relative_e_distances,omitempty"`
	UseFirmwareRetraction string `json:"use_firmware_retraction,omitempty"`

	// features — scalar "1"/"0". support_air_filtration is the machine key
	// (capability); air_filtration is a filament/process toggle, absent
	// from real machine exports.
	AuxiliaryFan         string `json:"auxiliary_fan,omitempty"`
	SupportAirFiltration string `json:"support_air_filtration,omitempty"`

	// gcode — opaque blobs, carried never interpreted
	GcodeFlavor            string `json:"gcode_flavor,omitempty"`
	MachineStartGcode      string `json:"machine_start_gcode,omitempty"`
	MachineEndGcode        string `json:"machine_end_gcode,omitempty"`
	BeforeLayerChangeGcode string `json:"before_layer_change_gcode,omitempty"`
	LayerChangeGcode       string `json:"layer_change_gcode,omitempty"`
	MachinePauseGcode      string `json:"machine_pause_gcode,omitempty"`
	ChangeFilamentGcode    string `json:"change_filament_gcode,omitempty"`

	// defaults
	DefaultFilamentProfile []string `json:"default_filament_profile,omitempty"` // array
	DefaultPrintProfile    string   `json:"default_print_profile,omitempty"`    // scalar

	// notes — the field exists in Orca too (empty on the P2S export);
	// capability TAGS stay Prusa-side (mapping.md, linkage irregularity)
	PrinterNotes string `json:"printer_notes,omitempty"`
}
