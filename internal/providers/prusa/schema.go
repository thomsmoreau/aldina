// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// FilamentSettings holds all fields of a PrusaSlicer filament preset
// (reference: PrusaSlicer 2.9.4 export, 85 keys).
//
// We SHIP it as a config bundle — `[filament:NAME]` then the keys — never as
// the flat headerless form. A headerless INI is a whole configuration, and
// importing one makes PrusaSlicer adopt it as the print AND the filament AND
// the printer simultaneously. See MarshalBundle in ini.go.
//
// Wire format notes (handled by the INI writer/parser, not by these structs):
//   - everything is a string on the wire; "nil" (unquoted) = inherit from machine
//   - some values are quoted ("..."), with \n escaped INSIDE the quotes (gcode, notes)
//   - some are unquoted even with spaces (filament_vendor = Forward AM)
//   - filament_colour = #F7F7F7 is unquoted and starts with '#': a naive INI
//     parser treats it as a comment — do NOT use a stock INI library blindly.
//
// There is no std-lib INI support and no library fits the dialect: the writer
// lives in ini.go, with the escaping rules taken from PrusaSlicer's own
// Config.cpp. Read that file's header before touching a type here — the Go
// type IS the encoding: `string` = never quoted, `[]string` = ';'-joined with
// per-item quoting.
//
// Struct is split into embedded groups for readability; the wire form is flat
// and Marshal flattens them.
type FilamentSettings struct {
	FilamentIdentity
	FilamentPhysical
	FilamentCompatibility
	FilamentTemp
	FilamentFan
	FilamentFlow
	FilamentRetraction
	FilamentToolchange
	FilamentGcode
}

// FilamentIdentity — preset metadata.
// LIST vs SCALAR is not a style choice, it is the encoding — see ini.go.
// Read off the real export: filament_settings_id renders as "" (a quoted EMPTY
// item, only a list does that) and filament_notes renders quoted (it holds a
// \n). filament_vendor stays scalar precisely because "Forward AM" appears
// unquoted despite its space, which a list would never do.
type FilamentIdentity struct {
	FilamentSettingsID []string `ini:"filament_settings_id"` // renders as "" when empty
	Inherits           string   `ini:"inherits"`             // empty on detached exports
	FilamentVendor     string   `ini:"filament_vendor"`      // unquoted, may contain spaces
	FilamentType       string   `ini:"filament_type"`        // "PET" = PETG family (vocab!)
	FilamentColour     string   `ini:"filament_colour"`      // "#RRGGBB" unquoted — '#' trap
	FilamentNotes      []string `ini:"filament_notes"`       // quoted, \n-escaped
	CustomParameters   string   `ini:"custom_parameters_filament"`
}

// FilamentPhysical — physical properties of the material.
type FilamentPhysical struct {
	FilamentDiameter                []string `ini:"filament_diameter"` // per-extruder in bundles; single in detached
	FilamentDensity                 []string `ini:"filament_density"`
	FilamentCost                    []string `ini:"filament_cost"`
	FilamentSpoolWeight             []string `ini:"filament_spool_weight"`
	FilamentSoluble                 []string `ini:"filament_soluble"`                   // "0"/"1"
	FilamentAbrasive                []string `ini:"filament_abrasive"`                  // 2.9+ ; Orca ~ required_nozzle_HRC
	FilamentShrinkageCompensationXY string   `ini:"filament_shrinkage_compensation_xy"` // "0%" = none (≠ Orca "100%"!)
	FilamentShrinkageCompensationZ  string   `ini:"filament_shrinkage_compensation_z"`
}

// FilamentCompatibility — which printers/prints this preset applies to.
type FilamentCompatibility struct {
	CompatiblePrinters          []string `ini:"compatible_printers"`           // list of exact preset names
	CompatiblePrintersCondition string   `ini:"compatible_printers_condition"` // regex-ish expr, opaque in v0
	CompatiblePrints            []string `ini:"compatible_prints"`
	CompatiblePrintsCondition   string   `ini:"compatible_prints_condition"`
}

// FilamentTemp — temperatures (flat: ONE bed temp, vs Orca's 6 plate types).
type FilamentTemp struct {
	Temperature               []string `ini:"temperature"`
	FirstLayerTemperature     []string `ini:"first_layer_temperature"`
	BedTemperature            []string `ini:"bed_temperature"`
	FirstLayerBedTemperature  []string `ini:"first_layer_bed_temperature"`
	IdleTemperature           []string `ini:"idle_temperature"` // often "nil"
	ChamberTemperature        []string `ini:"chamber_temperature"`
	ChamberMinimalTemperature []string `ini:"chamber_minimal_temperature"`
}

// FilamentFan — part-cooling fan and slowdown logic.
type FilamentFan struct {
	Cooling                            []string `ini:"cooling"` // master switch "0"/"1"
	FanAlwaysOn                        []string `ini:"fan_always_on"`
	MinFanSpeed                        []string `ini:"min_fan_speed"` // 0–100 (word order ≠ Orca fan_min_speed)
	MaxFanSpeed                        []string `ini:"max_fan_speed"`
	BridgeFanSpeed                     []string `ini:"bridge_fan_speed"`
	DisableFanFirstLayers              []string `ini:"disable_fan_first_layers"`
	FullFanSpeedLayer                  []string `ini:"full_fan_speed_layer"`
	FanBelowLayerTime                  []string `ini:"fan_below_layer_time"`
	SlowdownBelowLayerTime             []string `ini:"slowdown_below_layer_time"`
	MinPrintSpeed                      []string `ini:"min_print_speed"`
	EnableDynamicFanSpeeds             []string `ini:"enable_dynamic_fan_speeds"`
	OverhangFanSpeed0                  []string `ini:"overhang_fan_speed_0"` // 4 fixed tiers (≠ Orca threshold model)
	OverhangFanSpeed1                  []string `ini:"overhang_fan_speed_1"`
	OverhangFanSpeed2                  []string `ini:"overhang_fan_speed_2"`
	OverhangFanSpeed3                  []string `ini:"overhang_fan_speed_3"`
	CoolingSlowdownLogic               string   `ini:"cooling_slowdown_logic"` // 2.9+: "uniform_cooling"
	CoolingPerimeterTransitionDistance []string `ini:"cooling_perimeter_transition_distance"`
}

// FilamentFlow — extrusion/flow limits.
type FilamentFlow struct {
	ExtrusionMultiplier            []string `ini:"extrusion_multiplier"` // = Orca filament_flow_ratio
	FilamentMaxVolumetricSpeed     []string `ini:"filament_max_volumetric_speed"`
	FilamentInfillMaxSpeed         []string `ini:"filament_infill_max_speed"` // 2.9+
	FilamentInfillMaxCrossingSpeed []string `ini:"filament_infill_max_crossing_speed"`
	FilamentPurgeMultiplier        string   `ini:"filament_purge_multiplier"` // "100%"
}

// FilamentRetraction — filament-level retraction overrides ("nil" = inherit machine).
type FilamentRetraction struct {
	FilamentRetractLength            []string `ini:"filament_retract_length"` // ≠ Orca filament_retraction_length
	FilamentRetractSpeed             []string `ini:"filament_retract_speed"`
	FilamentDeretractSpeed           []string `ini:"filament_deretract_speed"` // ≠ Orca filament_deretraction_speed
	FilamentRetractBeforeTravel      []string `ini:"filament_retract_before_travel"`
	FilamentRetractBeforeWipe        []string `ini:"filament_retract_before_wipe"`
	FilamentRetractLayerChange       []string `ini:"filament_retract_layer_change"`
	FilamentRetractLift              []string `ini:"filament_retract_lift"` // = Orca filament_z_hop
	FilamentRetractLiftAbove         []string `ini:"filament_retract_lift_above"`
	FilamentRetractLiftBelow         []string `ini:"filament_retract_lift_below"`
	FilamentRetractRestartExtra      []string `ini:"filament_retract_restart_extra"`
	FilamentWipe                     []string `ini:"filament_wipe"`
	FilamentSeamGapDistance          []string `ini:"filament_seam_gap_distance"`           // 2.9+
	FilamentTravelLiftBeforeObstacle []string `ini:"filament_travel_lift_before_obstacle"` // 2.9+ travel family
	FilamentTravelMaxLift            []string `ini:"filament_travel_max_lift"`
	FilamentTravelRampingLift        []string `ini:"filament_travel_ramping_lift"`
	FilamentTravelSlope              []string `ini:"filament_travel_slope"`
}

// FilamentToolchange — MMU / multi-tool: load, unload, ramming, purge.
type FilamentToolchange struct {
	FilamentLoadingSpeed                  []string `ini:"filament_loading_speed"`
	FilamentLoadingSpeedStart             []string `ini:"filament_loading_speed_start"`
	FilamentUnloadingSpeed                []string `ini:"filament_unloading_speed"`
	FilamentUnloadingSpeedStart           []string `ini:"filament_unloading_speed_start"`
	FilamentLoadTime                      []string `ini:"filament_load_time"`
	FilamentUnloadTime                    []string `ini:"filament_unload_time"`
	FilamentToolchangeDelay               []string `ini:"filament_toolchange_delay"`
	FilamentCoolingMoves                  []string `ini:"filament_cooling_moves"`
	FilamentCoolingInitialSpeed           []string `ini:"filament_cooling_initial_speed"`
	FilamentCoolingFinalSpeed             []string `ini:"filament_cooling_final_speed"`
	FilamentMinimalPurgeOnWipeTower       []string `ini:"filament_minimal_purge_on_wipe_tower"`
	FilamentRammingParameters             string   `ini:"filament_ramming_parameters"` // opaque quoted blob
	FilamentMultitoolRamming              []string `ini:"filament_multitool_ramming"`
	FilamentMultitoolRammingFlow          []string `ini:"filament_multitool_ramming_flow"`
	FilamentMultitoolRammingVolume        []string `ini:"filament_multitool_ramming_volume"`
	FilamentRetractLengthToolchange       []string `ini:"filament_retract_length_toolchange"`
	FilamentRetractRestartExtraToolchange []string `ini:"filament_retract_restart_extra_toolchange"`
	FilamentStampingDistance              []string `ini:"filament_stamping_distance"`
	FilamentStampingLoadingSpeed          []string `ini:"filament_stamping_loading_speed"`
}

// FilamentGcode — opaque per-provider blobs (Prusa templating {if ...} inside).
// Lists, not scalars: both render quoted in real exports, which only the list
// encoding produces (a scalar carrying ';' would stay bare and be mis-parsed).
type FilamentGcode struct {
	StartFilamentGcode []string `ini:"start_filament_gcode"` // quoted, \n-escaped, {if} templating
	EndFilamentGcode   []string `ini:"end_filament_gcode"`
}
