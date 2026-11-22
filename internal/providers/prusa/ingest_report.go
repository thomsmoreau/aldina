// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// ingest_report.go — the loss classifier and the summary line.
//
// Every key present in a [filament:] section is either mapped (translated,
// silent) or classified by nature into a bucket, never dropped without a
// trace. The classifier walks the RAW key map, not the typed struct: the real
// 2.9.6 bundle already carries two keys the schema does not model
// (filament_flush_speed, filament_flush_volume), and the loss is in those.
// Own copy of the orca classifier, not a shared one: the key sets and the
// list encoding differ (same shape, different semantics).

import (
	"fmt"
	"sort"

	"github.com/thomsmoreau/Aldina/internal/providers"
)

// mappedPrusaLists — list-typed wire keys mapped to canonical, each with the
// check that its first item parses. The check calls the very parser Ingest
// uses, so the report cannot disagree with the canonical: a value the parser
// drops to nil is reported, never counted as translated. Ingest keeps the
// first item, so more than one is a fan-out loss. filament_type and
// compatible_printers are mapped too, but their mapping can change shape
// (vocabulary, catalogue), so Ingest emits their issues and the classifier
// stays silent on them (ingestOwnedPrusaKeys).
var mappedPrusaLists = map[string]func([]string) bool{
	"filament_notes": parsesText, "start_filament_gcode": parsesText, "end_filament_gcode": parsesText,
	"filament_diameter": parsesFloat, "filament_density": parsesFloat, "filament_cost": parsesFloat,
	"temperature": parsesFloat, "first_layer_temperature": parsesFloat,
	"bed_temperature": parsesFloat, "first_layer_bed_temperature": parsesFloat,
	"extrusion_multiplier": parsesFloat, "filament_max_volumetric_speed": parsesFloat,
	"slowdown_below_layer_time": parsesFloat, "min_print_speed": parsesFloat,
	"filament_retract_length": parsesFloat, "filament_retract_speed": parsesFloat,
	"filament_deretract_speed": parsesFloat, "filament_retract_lift": parsesFloat,
	"min_fan_speed": parsesPercent, "max_fan_speed": parsesPercent, "bridge_fan_speed": parsesPercent,
	"disable_fan_first_layers": parsesInt, "full_fan_speed_layer": parsesInt,
	"filament_abrasive": parsesBool, "filament_wipe": parsesBool,
}

// mappedPrusaScalars — scalar-typed mapped keys: a ';' in them is content,
// not an item separator, so they cannot fan out.
var mappedPrusaScalars = map[string]func(string) bool{
	"filament_vendor":                    func(string) bool { return true },
	"filament_colour":                    func(string) bool { return true },
	"filament_shrinkage_compensation_xy": parsesShrink,
	"filament_shrinkage_compensation_z":  parsesShrink,
}

func parsesText([]string) bool      { return true }
func parsesFloat(a []string) bool   { return parsePrusaFloat(a) != nil }
func parsesPercent(a []string) bool { return parsePrusaPercent(a) != nil }
func parsesInt(a []string) bool     { return parsePrusaInt(a) != nil }
func parsesBool(a []string) bool    { return parsePrusaBool(a) != nil }
func parsesShrink(s string) bool    { return parsePrusaShrink(s) != nil }

// ingestOwnedPrusaKeys — mapped keys whose issues Ingest emits itself. A
// second line from the classifier would report one key twice.
var ingestOwnedPrusaKeys = map[string]bool{
	"filament_type": true, "compatible_printers": true,
}

// identityPrusaKeys — preset identity and import-gate bindings: they describe
// which preset this is and where it applies, not a print intent. The plain
// print-preset list has no canonical home (compat names machines only).
var identityPrusaKeys = map[string]bool{
	"filament_settings_id": true, "inherits": true, "compatible_prints": true,
}

// intentPrusaKeys — material and cooling properties with no canonical field
// yet (spec #6 lists them as intent). The loss is real, but calling them
// tuning would mislead: the note names the missing field instead.
var intentPrusaKeys = map[string]bool{
	"filament_soluble": true, "filament_spool_weight": true, "fan_below_layer_time": true,
	"overhang_fan_speed_0": true, "overhang_fan_speed_1": true,
	"overhang_fan_speed_2": true, "overhang_fan_speed_3": true,
}

// conditionPrusaKeys — the compat expressions. Never parsed (spec Q4): the
// grammar is PrusaSlicer's and the variables live in printer data outside the
// preset. Ingest keeps the raw rule in overrides.prusa.
var conditionPrusaKeys = map[string]bool{
	"compatible_printers_condition": true, "compatible_prints_condition": true,
}

// classifyKeys buckets every present key of a section, in key order so the
// report is stable. A key that authors nothing (authorsNothing) maps to
// canonical absent and reports nothing.
func classifyKeys(keys map[string]string) []IngestIssue {
	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)

	var issues []IngestIssue
	for _, key := range names {
		raw := keys[key]
		if authorsNothing(raw) {
			continue
		}
		bucket, note := classify(key, raw)
		if bucket == BucketTranslated {
			continue
		}
		issues = append(issues, IngestIssue{Bucket: bucket, Key: key, Value: raw, Note: note})
	}
	return issues
}

// authorsNothing reports the three wire tokens that carry no value: the bare
// empty (an absent list), `""` (one empty item) and "nil" (inherit from the
// print or printer preset). Dropping them loses nothing — canonical absent
// means the same thing on the target — so counting them would inflate the
// report with keys the author never set.
func authorsNothing(raw string) bool {
	return raw == "" || raw == `""` || raw == "nil"
}

// classify assigns one bucket to a present wire key that authors a value. Everything
// not mapped, identity or condition is an engine knob: the toolchange and
// ramming family, the bare retract_* print defaults, and any key newer than
// the schema. No key reaches lost-gcode: a [filament:] section carries only
// the start/end blobs, and both are mapped. The bucket stays in the taxonomy
// for the summary shape; a new filament gcode key belongs in a set here.
func classify(key, raw string) (LossBucket, string) {
	if ingestOwnedPrusaKeys[key] {
		return BucketTranslated, ""
	}
	if parses, ok := mappedPrusaLists[key]; ok {
		items := decodeList(raw)
		if len(items) > 1 {
			return BucketLostFanout, fmt.Sprintf("%d extruder variants collapsed to the first", len(items))
		}
		return parsed(parses(items), items[0])
	}
	if parses, ok := mappedPrusaScalars[key]; ok {
		v := decodeScalar(raw)
		return parsed(parses(v), v)
	}
	switch {
	case conditionPrusaKeys[key]:
		return BucketLostIdentity, "compat rule not parsed, raw kept in overrides.prusa"
	case identityPrusaKeys[key]:
		return BucketLostIdentity, "import-gate key, no print intent"
	case intentPrusaKeys[key]:
		return BucketLostEngine, "intent with no canonical field yet"
	}
	return BucketLostEngine, ""
}

// parsed turns a mapped key's parse check into its bucket. A decoded value
// that authors nothing (a quoted "nil" item) is absent by design, not a
// failed parse. A value the parser rejects left its canonical field absent:
// approximated, the one bucket for "mapped, value changed shape" — the
// shape being none.
func parsed(ok bool, v string) (LossBucket, string) {
	if ok || authorsNothing(v) {
		return BucketTranslated, ""
	}
	return BucketApproximated, "unparseable, canonical field left absent"
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
