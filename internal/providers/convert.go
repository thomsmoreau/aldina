// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

// convert.go — the conversion core the two providers share.
//
// These helpers are the VALUE-FREE part of both directions: a canonical value
// becomes a string (render), a wire scalar becomes a canonical value (ingest),
// with no opinion about the target format. They used to live
// as identical copies in orca/ and prusa/ (formatFloat, ratioToPercent,
// derefString, formatPoint, scalarFloatPtr) until a bug fix had to land in
// both files at once — the seam between "the formats agree" and "the formats
// disagree" is exactly here.
//
// Everything that encodes a value DIFFERENTLY per slicer stays in the
// provider: broadcast vs one, the shrink inversion, arrays vs CSV. Those are
// the two sides' dialects; this file is the shared groundwork. The "nil"
// inherit token is NOT a dialect: both slicers spell it alike (Slic3r
// heritage), so ingest reads it once, here. Which keys render it stays per
// provider.

import (
	"math"
	"math/big"
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

// RatioToExactPercent converts a 0–1 ratio to a percent string WITHOUT
// rounding, for wire keys that are float percents (shrink, coPercents in both
// slicers): 0.015 → "1.5". The ×100 runs on the ratio's shortest decimal
// (math/big), since 0.07*100 in float64 is 7.000000000000001. Non-finite
// values have no decimal and fall back to the float product.
func RatioToExactPercent(r float64) string {
	d, ok := new(big.Rat).SetString(strconv.FormatFloat(r, 'g', -1, 64))
	if !ok {
		return FormatFloat(r * 100)
	}
	f, _ := d.Mul(d, big.NewRat(100, 1)).Float64()
	return FormatFloat(f)
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

// absent reports the two wire tokens an ingest parser reads as canonical
// absent: "" (the key was omitted) and "nil" (inherit from the parent preset).
// Both dialects spell them alike; how a key is unpacked into one scalar (a
// JSON array, an INI list) stays the caller's.
func absent(s string) bool { return s == "" || s == "nil" }

// ParseFloat reverses FormatFloat: "220" → 220.0. Absent and unparseable
// values return nil, never an error: the report surfaces the loss, the parse
// must not fail a whole file over one bad key.
func ParseFloat(s string) *float64 {
	if absent(s) {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

// ParsePercent reverses RatioToPercent: "30" → 0.30. The /100 is an "e-2"
// exponent handed to ParseFloat — one correctly rounded parse, so "0.7" lands
// as 0.007, where 0.7/100 would write 0.006999999999999999 into the canonical.
func ParsePercent(s string) *float64 {
	if absent(s) {
		return nil
	}
	v, err := strconv.ParseFloat(s+"e-2", 64)
	if err != nil {
		return nil
	}
	return &v
}

// ParseInt reads a layer count: "3" → 3. Atoi, not a float parse then a cast:
// "2.5" is unparseable (absent, reported), never truncated to 2.
func ParseInt(s string) *int {
	if absent(s) {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}

// ParseBool reads the "1"/"0" form both slicers write. Anything else is absent.
func ParseBool(s string) *bool {
	switch s {
	case "1":
		return new(true)
	case "0":
		return new(false)
	}
	return nil
}
