// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

// convert.go — the conversion core the two providers share.
//
// These helpers are the VALUE-FREE part of the render path: a canonical value
// becomes a string, with no opinion about the target format. They used to live
// as identical copies in orca/ and prusa/ (formatFloat, ratioToPercent,
// derefString, formatPoint, scalarFloatPtr) until a bug fix had to land in
// both files at once — the seam between "the formats agree" and "the formats
// disagree" is exactly here.
//
// Everything that encodes a value DIFFERENTLY per slicer stays in the
// provider: broadcast vs one, the "nil" inheritance token, the shrink
// inversion, arrays vs CSV. Those are the two sides' dialects; this file is
// the shared groundwork.

import (
	"math"
	"strconv"
)

// FormatFloat renders a float as the wire string both slicers write:
// 220.0 → "220", 0.945 → "0.945". Shortest round-trip, no trailing zeros.
func FormatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// RatioToPercent converts a canonical 0–1 ratio to the 0–100 string both
// slicers write: 0.30 → "30".
// math.Round, not the +0.5 trick: that one truncates toward zero, so it rounds
// negatives the wrong way (-0.2 → "-19" instead of "-20"). Validate only WARNS
// on a ratio outside [0,1] — the canonical never gates — so out-of-range values
// do reach this helper.
func RatioToPercent(r float64) string {
	return strconv.Itoa(int(math.Round(r * 100)))
}

// DerefString reads an optional string as its wire value: nil → "".
//
// "" is the shared ABSENT token both dialects omit at the wire (an empty
// string never writes a key). It is a COLLAPSE: nil ("never said") and ""
// ("said empty") land on the same token, so a caller that must keep them apart
// — a preset name that has to arrive present AND non-empty — checks presence
// itself, in its own required list, never through this helper.
func DerefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// FormatPoint renders the XxY point mini-format both slicers share:
// 0,256 → "0x256". The two value halves and the separator are identical on
// both wire forms; only the CONTAINER differs (Orca JSON array, Prusa CSV),
// and that container is the caller's.
func FormatPoint(x, y float64) string {
	return FormatFloat(x) + "x" + FormatFloat(y)
}

// ScalarFloatPtr renders an optional scalar: nil → "" (the absent token,
// omitted by the caller's wire). Same collapse as DerefString: the decision
// whether an empty value is meaningful on a given key belongs to the caller,
// never here.
func ScalarFloatPtr(p *float64) string {
	if p == nil {
		return ""
	}
	return FormatFloat(*p)
}
