// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// ingest_convert.go — the reverse encodings: Orca wire → canonical value.
// Mirrors the render bricks (broadcast, renderFloatPtr, renderShrink) inverted.
// These are the Orca DIALECT: arrays collapse to their first element, shrink
// is the inverted percent. The plain scalars and the "nil" inherit token are
// shared (providers/convert.go).
//
// Unparseable values return nil, not an error: the loss is surfaced as a
// bucket by the report (slice C), never silently — but the parse itself must
// not fail a whole file over one bad key.

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/providers"
)

// unbroadcast reverses broadcast: the first element of a per-extruder array.
// An empty array is the absent token (the key was omitted at render).
func unbroadcast(a []string) (string, bool) {
	if len(a) == 0 {
		return "", false
	}
	return a[0], true
}

// The scalar parsers are the shared ones (providers/convert.go); only the
// array collapse is Orca's.
func parseOrcaFloat(a []string) *float64 {
	s, _ := unbroadcast(a)
	return providers.ParseFloat(s)
}

func parseOrcaInt(a []string) *int {
	s, _ := unbroadcast(a)
	return providers.ParseInt(s)
}

func parseOrcaPercent(a []string) *float64 {
	s, _ := unbroadcast(a)
	return providers.ParsePercent(s)
}

func parseOrcaBool(a []string) *bool {
	s, _ := unbroadcast(a)
	return providers.ParseBool(s)
}

// parseOrcaShrink reverses renderShrink: the inverted encoding, "100%" (no
// shrink) → 0.0, "98%" → 0.02. The inversion runs on the exact decimal
// (math/big) and rounds once at the end: float arithmetic makes "99.3%"
// 0.007000000000000029, noise that would land in the canonical YAML.
func parseOrcaShrink(a []string) *float64 {
	s, ok := unbroadcast(a)
	if !ok || s == "" || s == "nil" {
		return nil
	}
	num := strings.TrimSuffix(s, "%")
	// ParseFloat gates first: big.Rat alone would also accept "1/3" and
	// allocate for any exponent a file carries.
	if _, err := strconv.ParseFloat(num, 64); err != nil {
		return nil
	}
	size, ok := new(big.Rat).SetString(num)
	if !ok {
		return nil
	}
	hundred := big.NewRat(100, 1)
	rate, _ := new(big.Rat).Quo(new(big.Rat).Sub(hundred, size), hundred).Float64()
	return &rate
}

// parseOrcaString reverses broadcastStringPtr: first element, nil when absent.
// ("nil" is not special here — only the retraction/float family uses it.)
func parseOrcaString(a []string) *string {
	s, ok := unbroadcast(a)
	if !ok || s == "" {
		return nil
	}
	return &s
}

// parseOrcaColour reverses broadcastColour: "#B3B4B2" → "B3B4B2" (bare hex,
// because a leading '#' is an INI comment on the Prusa side).
func parseOrcaColour(a []string) *string {
	s, ok := unbroadcast(a)
	if !ok || s == "" {
		return nil
	}
	return new(strings.TrimPrefix(s, "#"))
}

// parseOrcaRange pairs the two nozzle-range bounds: both must be present for
// the [2]float64 to be declared (the render emits them as a block).
func parseOrcaRange(low, high []string) *[2]float64 {
	lo, hi := parseOrcaFloat(low), parseOrcaFloat(high)
	if lo == nil || hi == nil {
		return nil
	}
	return new([2]float64{*lo, *hi})
}

// parseOrcaGcode reverses gcodePtr: the blob is carried opaque, never parsed.
func parseOrcaGcode(a []string) *string {
	s, ok := unbroadcast(a)
	if !ok || s == "" {
		return nil
	}
	return &s
}

// parseOrcaName reverses renderFilamentIdentity's preset label: wire
// "Label @Machine" → canonical "Label". The " @<machine>" suffix is render's
// convention, never part of the author's label, so stripping it is an
// approximation the report must surface (BucketApproximated).
func parseOrcaName(p *string) (*string, string) {
	if p == nil || *p == "" {
		return nil, ""
	}
	s := *p
	if i := strings.LastIndex(s, " @"); i >= 0 {
		label := s[:i]
		return &label, fmt.Sprintf("stripped machine suffix from %q", s)
	}
	return &s, ""
}

// parseOrcaMaterial maps Orca's vocabulary to the canonical enum. Exact names
// pass through; the filled/CF variants (PLA-CF, PAHT-CF…) have no canonical
// home and return a note the caller turns into an approximation issue.
func parseOrcaMaterial(a []string) (*canonical.Material, string) {
	s, ok := unbroadcast(a)
	if !ok || s == "" {
		return nil, ""
	}
	m := canonical.Material(s)
	switch m {
	case canonical.PLA, canonical.PETG, canonical.ABS, canonical.ASA,
		canonical.TPU, canonical.PC, canonical.PA, canonical.PHA:
		return &m, ""
	}
	return nil, fmt.Sprintf("filament_type %q has no canonical material", s)
}

// parseOrcaAbrasive reverses the abrasive bool from required_nozzle_HRC. The
// reverse is one-way by the data: Orca's system library leaves the field "0"
// even on abrasive filaments (31 PA-CF, 37 PLA-CF carry "0"), so a low value
// can never be trusted as "not abrasive". Only the hardened-steel threshold
// (≥40) is a faithful abrasive signal — 85/85 presets at "40" are filled/CF.
func parseOrcaAbrasive(a []string) (*bool, string) {
	s, ok := unbroadcast(a)
	if !ok || s == "" || s == "nil" {
		return nil, ""
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 40 {
		return nil, ""
	}
	t := true
	return &t, fmt.Sprintf("required_nozzle_HRC %s collapsed to abrasive bool", s)
}
