// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// ingest_report.go — the loss classifier and the summary line.
//
// The report is the feature (docs/adr/0001, Q3): a real export is ~130 keys of
// which ~24 map. Every present key is either mapped (translated, silent) or
// classified by schema nature into a bucket, never dropped without a trace.
// The classifier walks the RAW key map, not the typed struct, because the loss
// is in the keys the struct does not model.

import (
	"encoding/json"
	"fmt"

	"github.com/thomsmoreau/Aldina/internal/providers"
)

// mappedOrcaKeys — wire keys mapped to canonical WITHOUT approximation.
// name and filament_type are also mapped (meta.name / meta.material) but their
// mapping can change shape (suffix strip, vocabulary), so their issues are
// emitted by ingestFilament, not by this classifier.
var mappedOrcaKeys = map[string]bool{
	"name": true, "filament_type": true,
	"filament_vendor": true, "filament_notes": true, "default_filament_colour": true,
	"filament_diameter": true, "filament_density": true, "filament_cost": true,
	"filament_shrink": true, "filament_shrinkage_compensation_z": true,
	"required_nozzle_HRC": true,
	"compatible_printers": true,
	"nozzle_temperature":  true, "nozzle_temperature_initial_layer": true,
	"nozzle_temperature_range_low": true, "nozzle_temperature_range_high": true,
	"hot_plate_temp": true, "hot_plate_temp_initial_layer": true,
	"temperature_vitrification":     true,
	"filament_max_volumetric_speed": true, "filament_flow_ratio": true,
	"fan_min_speed": true, "fan_max_speed": true, "close_fan_the_first_x_layers": true,
	"full_fan_speed_layer": true, "slow_down_layer_time": true, "slow_down_min_speed": true,
	"overhang_fan_speed":         true,
	"filament_retraction_length": true, "filament_retraction_speed": true,
	"filament_deretraction_speed": true, "filament_z_hop": true,
	"filament_wipe": true, "filament_wipe_distance": true,
	"filament_start_gcode": true, "filament_end_gcode": true,
}

// identityOrcaKeys — import-gate / file-description keys: they describe the
// preset, not a print intent, so they have no canonical home.
var identityOrcaKeys = map[string]bool{
	"filament_id": true, "filament_settings_id": true, "version": true, "from": true,
}

// gcodeOrcaKeys — opaque gcode blobs canonical never interprets. The start/end
// blobs ARE carried (mappedOrcaKeys); the toolchange blob is not.
var gcodeOrcaKeys = map[string]bool{
	"filament_change_extrusion_role_gcode": true,
}

// fanoutOrcaKeys — per-plate bed temperatures. Canonical carries ONE bed value
// (hot_plate_temp); the other plate types are the fan-out render spreads.
var fanoutOrcaKeys = map[string]bool{
	"cool_plate_temp": true, "cool_plate_temp_initial_layer": true,
	"textured_plate_temp": true, "textured_plate_temp_initial_layer": true,
	"eng_plate_temp": true, "eng_plate_temp_initial_layer": true,
	"supertack_plate_temp": true, "supertack_plate_temp_initial_layer": true,
	"textured_cool_plate_temp": true, "textured_cool_plate_temp_initial_layer": true,
}

// listOrcaKeys — fields that are lists by nature, not per-extruder fan-out:
// multiple entries are the point, so the fan-out collapse check must not fire.
var listOrcaKeys = map[string]bool{
	"compatible_printers": true,
	"compatible_prints":   true,
}

// arrayLen reports the element count of a JSON array, 0 for a scalar.
func arrayLen(raw json.RawMessage) int {
	var a []json.RawMessage
	if err := json.Unmarshal(raw, &a); err != nil {
		return 0
	}
	return len(a)
}

// classify assigns one loss bucket to a present wire key. A mapped key is
// translated (silent) unless its array carries more than one extruder variant,
// in which case the collapse to the first element is a fan-out loss.
func classify(key string, raw json.RawMessage) (LossBucket, string) {
	if listOrcaKeys[key] {
		if mappedOrcaKeys[key] {
			return BucketTranslated, ""
		}
		return BucketLostEngine, ""
	}
	if mappedOrcaKeys[key] {
		if n := arrayLen(raw); n > 1 {
			return BucketLostFanout, fmt.Sprintf("%d extruder variants collapsed to the first", n)
		}
		return BucketTranslated, ""
	}
	switch {
	case fanoutOrcaKeys[key]:
		return BucketLostFanout, "per-plate bed temperature; canonical carries one bed value"
	case identityOrcaKeys[key]:
		return BucketLostIdentity, "import-gate key, no print intent"
	case gcodeOrcaKeys[key]:
		return BucketLostGcode, "opaque gcode blob, never interpreted"
	}
	if n := arrayLen(raw); n > 1 {
		return BucketLostFanout, fmt.Sprintf("per-extruder array of %d", n)
	}
	return BucketLostEngine, ""
}

// buildSummary counts the issues per bucket for the shared headline
// (providers.LossCounts).
func buildSummary(issues []IngestIssue) string {
	var c providers.LossCounts
	for _, is := range issues {
		switch is.Bucket {
		case BucketLostEngine:
			c.Engine++
		case BucketLostFanout:
			c.Fanout++
		case BucketLostIdentity:
			c.Identity++
		case BucketLostGcode:
			c.Gcode++
		case BucketApproximated:
			c.Approximated++
		}
	}
	return c.Summary()
}
