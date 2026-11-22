// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// ingest_convert.go — the reverse encodings: Prusa wire → canonical value.
// Mirrors the render bricks (one, oneFloatPtr) inverted. The Prusa dialect:
// wire lists collapse to their first element (a [filament:] section holds one
// value per key; the collapse guards the one-element vector form), and
// shrinkage is a straight percent — no inversion (that one is Orca's). The
// plain scalars and the "nil" inherit token are shared (providers/convert.go).
//
// Unparseable values return nil, not an error: the parse must not fail a
// whole file over one bad key. The report surfaces that nil — the classifier
// re-runs these parsers on every mapped key (ingest_report.go).

import (
	"strings"

	"github.com/thomsmoreau/Aldina/internal/providers"
)

// unbroadcast collapses a per-extruder array to its first element. An empty
// array is the absent token (the key was omitted at render).
func unbroadcast(a []string) (string, bool) {
	if len(a) == 0 {
		return "", false
	}
	return a[0], true
}

// The scalar parsers are the shared ones (providers/convert.go); only the
// list collapse is Prusa's.
func parsePrusaFloat(a []string) *float64 {
	s, _ := unbroadcast(a)
	return providers.ParseFloat(s)
}

func parsePrusaBool(a []string) *bool {
	s, _ := unbroadcast(a)
	return providers.ParseBool(s)
}

// parsePrusaPercent: the wire is a bare percent, no "%" (ConfigOptionInts).
func parsePrusaPercent(a []string) *float64 {
	s, _ := unbroadcast(a)
	return providers.ParsePercent(s)
}

func parsePrusaInt(a []string) *int {
	s, _ := unbroadcast(a)
	return providers.ParseInt(s)
}

// parsePrusaShrink reverses renderShrink: a straight rate, "2%" → 0.02, never
// the Orca inversion (docs/mapping.md pitfall #14): the percent with its "%"
// suffix.
func parsePrusaShrink(s string) *float64 {
	return providers.ParsePercent(strings.TrimSuffix(s, "%"))
}

// parsePrusaScalar reverses DerefString for the bare `string` wire fields:
// "" is absent. "nil" is not special — it is the inherit token of the
// retraction family only.
func parsePrusaScalar(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// parsePrusaColour reverses the render's '#' prefix: "#F7F7F7" → "F7F7F7".
// Canonical keeps the hex bare (docs/mapping.md, the INI comment trap).
func parsePrusaColour(s string) *string {
	return parsePrusaScalar(strings.TrimPrefix(s, "#"))
}

// parsePrusaString reverses one() for a string list field (filament_notes,
// the gcode blobs): the first item, "" absent. The reader has already decoded
// the \n escapes; nothing else is touched, so a gcode blob stays opaque.
func parsePrusaString(a []string) *string {
	s, _ := unbroadcast(a)
	return parsePrusaScalar(s)
}
