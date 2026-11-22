// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// FilamentSettings holds all fields from an OrcaSlicer filament JSON settings file
// (reference: user export "Overture PETG @Bambu Lab P2S 0.6 nozzle", Orca 2.6.x).
//
// Wire format notes (handled at the adapter boundary, not in these structs):
//   - everything is a string: "220", "0.945", "1" (bool), "70%", "-1" (sentinel: auto/off)
//   - "nil" = inherit from machine profile
//   - most values are ARRAYS with inconsistent cardinality in the same file:
//     1 element = broadcast to all extruder variants, N elements = one per variant
//     declared in filament_extruder_variant (e.g. ["Direct Drive Standard", "... High Flow"])
//   - some strings hide structures (adaptive_pressure_advance_model = a serialized matrix)
//
// Struct is split into embedded groups for readability; json.Unmarshal/Marshal
// work identically because embedded struct fields are flattened at the JSON level.
// Group partition is aligned with the Prusa schema (Identity / Physical / Compat /
// Temp / Fan / Flow / Retraction / Toolchange / Gcode) + Orca-only groups.
//
// NOTE: this struct is CLOSED — unknown fields are silently dropped on a
// parse→rewrite round-trip. Fine for MVP (render-only, built from scratch).
// Before implementing import: add Extra map[string]json.RawMessage with a
// custom (un)marshaller, and keep a DisallowUnknownFields test against
// references/ to detect schema drift when Orca adds fields.
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
	// Orca-only domains (no Prusa counterpart):
	FilamentPressureAdvance
	FilamentTowerIroning
	FilamentExhaustAirChamber
}

// FilamentIdentity — preset metadata. All three IDs (Name, FilamentSettingsID,
// FilamentID) must be generated CONSISTENTLY at render time, or Orca import
// misbehaves (cf. pain point P2).
//
// Pointer fields = Orca IMPORT GATES: without them Orca silently drops the
// preset (verified in source: PresetBundle::import_json_presets returns false
// when `version` doesn't parse as Semver or `filament_settings_id` is absent;
// PresetCollection::load_presets skips the file the same way). nil before
// marshal = render bug upstream, never a legit state. FilamentSettingsID
// needs no pointer: a nil/empty slice is already detectable and omitempty
// drops it. Rules: Name non-empty (Orca rewrites filament_settings_id[0]
// from it on every save), Version must parse as Semver x.y.z[.w].
type FilamentIdentity struct {
	FilamentID            string   `json:"filament_id"`
	Name                  *string  `json:"name,omitempty"`
	FilamentType          []string `json:"filament_type,omitempty"`
	FilamentVendor        []string `json:"filament_vendor,omitempty"`
	FilamentNotes         []string `json:"filament_notes,omitempty"`
	FilamentSettingsID    []string `json:"filament_settings_id,omitempty"`
	DefaultFilamentColour []string `json:"default_filament_colour,omitempty"` // "#RRGGBB"
	Version               *string  `json:"version,omitempty"`
	From                  *string  `json:"from,omitempty"` // "User" for rendered presets
	Inherits              string   `json:"inherits"`
}

// FilamentPhysical — physical properties of the material.
type FilamentPhysical struct {
	FilamentDiameter               []string `json:"filament_diameter,omitempty"`
	FilamentDensity                []string `json:"filament_density,omitempty"`
	FilamentCost                   []string `json:"filament_cost,omitempty"`
	FilamentSoluble                []string `json:"filament_soluble,omitempty"`
	FilamentIsSupport              []string `json:"filament_is_support,omitempty"`
	FilamentAdhesivenessCategory   []string `json:"filament_adhesiveness_category,omitempty"`
	RequiredNozzleHRC              []string `json:"required_nozzle_HRC,omitempty"` // hardness ~ Prusa filament_abrasive
	FilamentShrink                 []string `json:"filament_shrink,omitempty"`     // "100%" = NO shrink (≠ Prusa 0% = no comp!)
	FilamentShrinkageCompensationZ []string `json:"filament_shrinkage_compensation_z,omitempty"`
	FilamentPrintable              []string `json:"filament_printable,omitempty"`
	FilamentExtruderVariant        []string `json:"filament_extruder_variant,omitempty"` // drives array cardinality
	TemperatureVitrification       []string `json:"temperature_vitrification,omitempty"` // material property (canonical temps.softening)
}

// FilamentCompatibility — which printers/prints this preset works with.
type FilamentCompatibility struct {
	CompatiblePrinters          []string `json:"compatible_printers,omitempty"`
	CompatiblePrintersCondition string   `json:"compatible_printers_condition"`
	CompatiblePrints            []string `json:"compatible_prints,omitempty"`
	CompatiblePrintsCondition   string   `json:"compatible_prints_condition"`
}

// FilamentTemp — temperatures. Bed temp is declined per PLATE TYPE (6 types × 2),
// vs a single bed_temperature at Prusa: the worst 1:N of the mapping.
// A plate temp of "0" means "plate incompatible", not 0°C (hidden semantics).
type FilamentTemp struct {
	NozzleTemperature                 []string `json:"nozzle_temperature,omitempty"`
	NozzleTemperatureInitialLayer     []string `json:"nozzle_temperature_initial_layer,omitempty"`
	NozzleTemperatureRangeLow         []string `json:"nozzle_temperature_range_low,omitempty"`
	NozzleTemperatureRangeHigh        []string `json:"nozzle_temperature_range_high,omitempty"`
	HotPlateTemp                      []string `json:"hot_plate_temp,omitempty"`
	HotPlateTempInitialLayer          []string `json:"hot_plate_temp_initial_layer,omitempty"`
	TexturedPlateTemp                 []string `json:"textured_plate_temp,omitempty"`
	TexturedPlateTempInitialLayer     []string `json:"textured_plate_temp_initial_layer,omitempty"`
	CoolPlateTemp                     []string `json:"cool_plate_temp,omitempty"`
	CoolPlateTempInitialLayer         []string `json:"cool_plate_temp_initial_layer,omitempty"`
	TexturedCoolPlateTemp             []string `json:"textured_cool_plate_temp,omitempty"`
	TexturedCoolPlateTempInitialLayer []string `json:"textured_cool_plate_temp_initial_layer,omitempty"`
	EngPlateTemp                      []string `json:"eng_plate_temp,omitempty"`
	EngPlateTempInitialLayer          []string `json:"eng_plate_temp_initial_layer,omitempty"`
	SupertackPlateTemp                []string `json:"supertack_plate_temp,omitempty"`
	SupertackPlateTempInitialLayer    []string `json:"supertack_plate_temp_initial_layer,omitempty"`
	IdleTemperature                   []string `json:"idle_temperature,omitempty"`
	ChamberTemperature                []string `json:"chamber_temperature,omitempty"`
}

// FilamentFan — PART-COOLING fan and layer-time slowdown only.
// (MMU toolchange cooling moves live in FilamentToolchange.)
type FilamentFan struct {
	FanMaxSpeed                      []string `json:"fan_max_speed,omitempty"` // 0–100 (word order ≠ Prusa min_fan_speed)
	FanMinSpeed                      []string `json:"fan_min_speed,omitempty"`
	FirstXLayerFanSpeed              []string `json:"first_x_layer_fan_speed,omitempty"`
	FullFanSpeedLayer                []string `json:"full_fan_speed_layer,omitempty"`
	CloseFanTheFirstXLayers          []string `json:"close_fan_the_first_x_layers,omitempty"`
	AdditionalCoolingFanSpeed        []string `json:"additional_cooling_fan_speed,omitempty"` // aux part fan (P/X series)
	CloseAdditionalFanFirstXLayers   []string `json:"close_additional_fan_first_x_layers,omitempty"`
	AdditionalFanFullSpeedLayer      []string `json:"additional_fan_full_speed_layer,omitempty"`
	OverhangFanSpeed                 []string `json:"overhang_fan_speed,omitempty"`     // threshold model (≠ Prusa 4 tiers)
	OverhangFanThreshold             []string `json:"overhang_fan_threshold,omitempty"` // "25%"
	EnableOverhangBridgeFan          []string `json:"enable_overhang_bridge_fan,omitempty"`
	InternalBridgeFanSpeed           []string `json:"internal_bridge_fan_speed,omitempty"` // "-1" = auto (sentinel)
	SupportMaterialInterfaceFanSpeed []string `json:"support_material_interface_fan_speed,omitempty"`
	IroningFanSpeed                  []string `json:"ironing_fan_speed,omitempty"`
	FanCoolingLayerTime              []string `json:"fan_cooling_layer_time,omitempty"`
	ReduceFanStopStartFreq           []string `json:"reduce_fan_stop_start_freq,omitempty"`
	SlowDownForLayerCooling          []string `json:"slow_down_for_layer_cooling,omitempty"`
	SlowDownLayerTime                []string `json:"slow_down_layer_time,omitempty"`
	SlowDownMinSpeed                 []string `json:"slow_down_min_speed,omitempty"`
	DontSlowDownOuterWall            []string `json:"dont_slow_down_outer_wall,omitempty"`
}

// FilamentFlow — extrusion/flow limits and modifiers.
// FIX: filament_max_volumetric_speed was MISSING from the previous version —
// it is a canonical-core field (flow.max_volumetric_mm3_s).
type FilamentFlow struct {
	FilamentFlowRatio               []string `json:"filament_flow_ratio,omitempty"` // = Prusa extrusion_multiplier
	FilamentMaxVolumetricSpeed      []string `json:"filament_max_volumetric_speed,omitempty"`
	FilamentAdaptiveVolumetricSpeed []string `json:"filament_adaptive_volumetric_speed,omitempty"`
	VolumetricSpeedCoefficients     []string `json:"volumetric_speed_coefficients,omitempty"`
	PelletFlowCoefficient           []string `json:"pellet_flow_coefficient,omitempty"` // pellet extruders
}

// FilamentRetraction — filament-level retraction/wipe/z-hop overrides
// ("nil" = inherit from machine).
type FilamentRetraction struct {
	FilamentRetractionLength           []string `json:"filament_retraction_length,omitempty"` // ≠ Prusa filament_retract_length
	FilamentRetractionSpeed            []string `json:"filament_retraction_speed,omitempty"`
	FilamentDeretractionSpeed          []string `json:"filament_deretraction_speed,omitempty"` // ≠ Prusa filament_deretract_speed
	FilamentRetractionMinimumTravel    []string `json:"filament_retraction_minimum_travel,omitempty"`
	FilamentRetractWhenChangingLayer   []string `json:"filament_retract_when_changing_layer,omitempty"`
	FilamentRetractRestartExtra        []string `json:"filament_retract_restart_extra,omitempty"`
	FilamentRetractLiftAbove           []string `json:"filament_retract_lift_above,omitempty"`
	FilamentRetractLiftBelow           []string `json:"filament_retract_lift_below,omitempty"`
	FilamentRetractLiftEnforce         []string `json:"filament_retract_lift_enforce,omitempty"`
	FilamentWipe                       []string `json:"filament_wipe,omitempty"`
	FilamentWipeDistance               []string `json:"filament_wipe_distance,omitempty"`
	FilamentRetractBeforeWipe          []string `json:"filament_retract_before_wipe,omitempty"` // "70%"
	FilamentZHop                       []string `json:"filament_z_hop,omitempty"`               // = Prusa filament_retract_lift
	FilamentZHopTypes                  []string `json:"filament_z_hop_types,omitempty"`         // "Spiral Lift"
	LongRetractionsWhenEc              []string `json:"long_retractions_when_ec,omitempty"`
	RetractionDistancesWhenEc          []string `json:"retraction_distances_when_ec,omitempty"`
	FilamentLongRetractionsWhenCut     []string `json:"filament_long_retractions_when_cut,omitempty"`
	FilamentRetractionDistancesWhenCut []string `json:"filament_retraction_distances_when_cut,omitempty"`
}

// FilamentToolchange — AMS/MMU & multi-tool: load/unload, toolchange cooling
// moves, ramming, flush. (Moved out of Fan/Speeds: this is toolchange logic,
// not part cooling — partition aligned with prusa.FilamentToolchange.)
type FilamentToolchange struct {
	FilamentLoadingSpeed             []string `json:"filament_loading_speed,omitempty"`
	FilamentLoadingSpeedStart        []string `json:"filament_loading_speed_start,omitempty"`
	FilamentUnloadingSpeed           []string `json:"filament_unloading_speed,omitempty"`
	FilamentUnloadingSpeedStart      []string `json:"filament_unloading_speed_start,omitempty"`
	FilamentToolchangeDelay          []string `json:"filament_toolchange_delay,omitempty"`
	FilamentCoolingMoves             []string `json:"filament_cooling_moves,omitempty"`
	FilamentCoolingInitialSpeed      []string `json:"filament_cooling_initial_speed,omitempty"`
	FilamentCoolingFinalSpeed        []string `json:"filament_cooling_final_speed,omitempty"`
	FilamentCoolingBeforeTower       []string `json:"filament_cooling_before_tower,omitempty"`
	FilamentRammingParameters        []string `json:"filament_ramming_parameters,omitempty"` // opaque blob
	FilamentMultitoolRamming         []string `json:"filament_multitool_ramming,omitempty"`
	FilamentMultitoolRammingFlow     []string `json:"filament_multitool_ramming_flow,omitempty"`
	FilamentMultitoolRammingVolume   []string `json:"filament_multitool_ramming_volume,omitempty"`
	FilamentChangeLength             []string `json:"filament_change_length,omitempty"`
	FilamentFlushTemp                []string `json:"filament_flush_temp,omitempty"`
	FilamentFlushVolumetricSpeed     []string `json:"filament_flush_volumetric_speed,omitempty"`
	FilamentStampingDistance         []string `json:"filament_stamping_distance,omitempty"`
	FilamentStampingLoadingSpeed     []string `json:"filament_stamping_loading_speed,omitempty"`
	FilamentChangeExtrusionRoleGcode []string `json:"filament_change_extrusion_role_gcode,omitempty"`
}

// FilamentGcode — opaque per-provider blobs.
type FilamentGcode struct {
	FilamentStartGcode []string `json:"filament_start_gcode,omitempty"`
	FilamentEndGcode   []string `json:"filament_end_gcode,omitempty"`
}

// ---- Orca-only domains (no Prusa counterpart — override territory) ----

// FilamentPressureAdvance — PA / adaptive PA (Orca-specific tuning).
type FilamentPressureAdvance struct {
	EnablePressureAdvance            []string `json:"enable_pressure_advance,omitempty"`
	PressureAdvance                  []string `json:"pressure_advance,omitempty"`
	AdaptivePressureAdvance          []string `json:"adaptive_pressure_advance,omitempty"`
	AdaptivePressureAdvanceBridges   []string `json:"adaptive_pressure_advance_bridges,omitempty"`
	AdaptivePressureAdvanceModel     []string `json:"adaptive_pressure_advance_model,omitempty"` // serialized matrix "0,0,0\n0,0,0"
	AdaptivePressureAdvanceOverhangs []string `json:"adaptive_pressure_advance_overhangs,omitempty"`
}

// FilamentTowerIroning — wipe tower interface and ironing (Orca-specific).
type FilamentTowerIroning struct {
	FilamentTowerInterfacePreExtrusionDist   []string `json:"filament_tower_interface_pre_extrusion_dist,omitempty"`
	FilamentTowerInterfacePreExtrusionLength []string `json:"filament_tower_interface_pre_extrusion_length,omitempty"`
	FilamentTowerInterfacePrintTemp          []string `json:"filament_tower_interface_print_temp,omitempty"` // "-1" sentinel
	FilamentTowerInterfacePurgeVolume        []string `json:"filament_tower_interface_purge_volume,omitempty"`
	FilamentMinimalPurgeOnWipeTower          []string `json:"filament_minimal_purge_on_wipe_tower,omitempty"`
	FilamentTowerIroningArea                 []string `json:"filament_tower_ironing_area,omitempty"`
	FilamentIroningFlow                      []string `json:"filament_ironing_flow,omitempty"`
	FilamentIroningInset                     []string `json:"filament_ironing_inset,omitempty"`
	FilamentIroningSpacing                   []string `json:"filament_ironing_spacing,omitempty"`
	FilamentIroningSpeed                     []string `json:"filament_ironing_speed,omitempty"`
}

// FilamentExhaustAirChamber — exhaust fan, air filtration, chamber control
// (enclosed Bambu printers; Orca-specific).
type FilamentExhaustAirChamber struct {
	DuringPrintExhaustFanSpeed        []string `json:"during_print_exhaust_fan_speed,omitempty"`
	CompletePrintExhaustFanSpeed      []string `json:"complete_print_exhaust_fan_speed,omitempty"`
	ActivateAirFiltration             []string `json:"activate_air_filtration,omitempty"`
	ActivateAirFiltrationDuringPrint  []string `json:"activate_air_filtration_during_print,omitempty"`
	ActivateAirFiltrationOnCompletion []string `json:"activate_air_filtration_on_completion,omitempty"`
	ActivateChamberTempControl        []string `json:"activate_chamber_temp_control,omitempty"`
}
