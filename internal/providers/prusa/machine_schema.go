// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// machine_schema.go — PrusaSlicer printer-preset schema, frozen on a REAL
// PrusaSlicer 2.9.x user export
// (references/prusa/printers/Prusa_XL.ini, 91 keys).
//
// CLOSED struct, like orca/machine_schema.go and unlike the filament schema:
// keys parked under `overrides.prusa` or "Excluded forever" (docs/mapping.md)
// are absent, so a zero-valued key can never leak into a printer bundle. The
// wire-schema exemption covers the filament side, where the extra
// keys document the format; here they would document nothing we do not already
// have in mapping.md.
//
// Encoding is chosen by the Go TYPE (ini.go) and only two exist. The trap this
// schema exists to avoid: `[]string` is ConfigOptionStrings — items joined by
// ';'. A NUMERIC vector is not that. PrusaSlicer serialises numeric vectors
// with ',' (machine_max_feedrate_x = 400,140), and points likewise
// (bed_shape = 0x0,360x0,360x360,0x360). Those are therefore `string` fields
// carrying a comma-joined value, never `[]string`:
//
//	string    → bare, never quoted: every scalar, every numeric vector, every
//	            point list, every gcode blob (start_gcode holds ';' AND '"' raw
//	            in the real export — the opposite of the filament's gcode, which
//	            is a quoted list).
//	[]string  → ConfigOptionStrings only. On the machine that is exactly ONE
//	            key: default_filament_profile, which the real export writes
//	            quoted ("Prusament PLA @XLIS") — proof of the list encoding.
type MachineSettings struct {
	// identity & linkage — printer_settings_id IS the preset name (pitfall
	// #10), and the [printer:NAME] header carries it too. printer_model is
	// the catalogue's MODEL ID ("MINI"), never the human name: it is what a
	// filament's compatible_printers_condition compares against.
	PrinterModel      string `ini:"printer_model"`
	PrinterVariant    string `ini:"printer_variant"` // catalogue verbatim ("0.40" is a real value)
	PrinterVendor     string `ini:"printer_vendor"`
	PrinterTechnology string `ini:"printer_technology"` // constant "FFF"
	PrinterSettingsID string `ini:"printer_settings_id"`
	Inherits          string `ini:"inherits"` // parent SYSTEM preset, empty = standalone
	PrinterNotes      string `ini:"printer_notes"`

	// volume — bed_shape is ONE CSV string of XxY corners, not a JSON array
	// like Orca's printable_area (pitfall #2).
	BedShape       string `ini:"bed_shape"`
	MaxPrintHeight string `ini:"max_print_height"` // ≠ Orca printable_height

	// nozzle — nozzle_diameter is per-extruder (comma-joined at N>1).
	// nozzle_type / extruder_type / nozzle_height / nozzle_hrc do not exist
	// on this side: Orca-only, warned at render.
	NozzleDiameter string `ini:"nozzle_diameter"`
	MinLayerHeight string `ini:"min_layer_height"`
	MaxLayerHeight string `ini:"max_layer_height"`

	// kinetics — every value is a "normal,silent" pair. FEEDRATE, not speed:
	// the one false friend of the whole machine mapping.
	MachineMaxFeedrateX     string `ini:"machine_max_feedrate_x"`
	MachineMaxFeedrateY     string `ini:"machine_max_feedrate_y"`
	MachineMaxFeedrateZ     string `ini:"machine_max_feedrate_z"`
	MachineMaxFeedrateE     string `ini:"machine_max_feedrate_e"`
	MachineMaxAccelerationX string `ini:"machine_max_acceleration_x"`
	MachineMaxAccelerationY string `ini:"machine_max_acceleration_y"`
	MachineMaxAccelerationZ string `ini:"machine_max_acceleration_z"`
	MachineMaxAccelerationE string `ini:"machine_max_acceleration_e"`
	MachineMaxJerkX         string `ini:"machine_max_jerk_x"`
	MachineMaxJerkY         string `ini:"machine_max_jerk_y"`
	MachineMaxJerkZ         string `ini:"machine_max_jerk_z"`
	MachineMaxJerkE         string `ini:"machine_max_jerk_e"`
	MachineMinTravelRate    string `ini:"machine_min_travel_rate"`
	MachineMinExtrudingRate string `ini:"machine_min_extruding_rate"`

	// retraction — retract_*, NOT retraction_* (Orca's spelling).
	RetractLength        string `ini:"retract_length"`
	RetractSpeed         string `ini:"retract_speed"`
	DeretractSpeed       string `ini:"deretract_speed"`
	RetractBeforeTravel  string `ini:"retract_before_travel"` // = Orca retraction_minimum_travel
	RetractLift          string `ini:"retract_lift"`          // = Orca z_hop
	RetractLayerChange   string `ini:"retract_layer_change"`
	Wipe                 string `ini:"wipe"`
	UseRelativeEDistance string `ini:"use_relative_e_distances"`
	UseFirmwareRetract   string `ini:"use_firmware_retraction"`

	// gcode — opaque blobs, carried never interpreted (pitfall #6).
	GcodeFlavor      string `ini:"gcode_flavor"`
	StartGcode       string `ini:"start_gcode"`
	EndGcode         string `ini:"end_gcode"`
	BeforeLayerGcode string `ini:"before_layer_gcode"`
	LayerGcode       string `ini:"layer_gcode"`
	PausePrintGcode  string `ini:"pause_print_gcode"`
	ColorChangeGcode string `ini:"color_change_gcode"` // = Orca change_filament_gcode

	// defaults — the asymmetry of pitfall #5, and the only []string here.
	DefaultPrintProfile    string   `ini:"default_print_profile"`
	DefaultFilamentProfile []string `ini:"default_filament_profile"`
}
