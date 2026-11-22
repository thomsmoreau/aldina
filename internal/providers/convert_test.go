// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import "testing"

// The three conversion tests moved here from the providers' own test files
// (they used to run the same assertions twice and could not catch the two
// copies drifting). One home, one truth.

func TestFormatFloat(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want string
	}{
		{"integer", 220, "220"},
		{"trailing zeros dropped", 220.0, "220"},
		{"decimal kept", 0.945, "0.945"},
		{"zero", 0, "0"},
		{"one seventy five", 1.75, "1.75"},
		{"negative", -1, "-1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FormatFloat(c.in); got != c.want {
				t.Errorf("FormatFloat(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// Float-percent keys keep every digit the ratio carries, and none of the
// float noise a plain r*100 adds.
func TestRatioToExactPercent(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want string
	}{
		{"zero", 0.0, "0"},
		{"whole", 0.02, "2"},
		{"float noise on x100", 0.07, "7"}, // 0.07*100 = 7.000000000000001
		{"fraction kept", 0.015, "1.5"},
		{"sub-percent kept", 0.007, "0.7"},
		{"tiny ratio, exponent form", 0.00001, "0.001"},
		{"negative", -0.005, "-0.5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RatioToExactPercent(c.in); got != c.want {
				t.Errorf("RatioToExactPercent(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestRatioToPercent(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want string
	}{
		{"zero", 0.0, "0"},
		{"third", 0.333, "33"},
		{"half", 0.5, "50"},
		{"full", 1.0, "100"},
		{"round up", 0.305, "31"},
		{"nearly full", 0.999, "100"},
		// Out of the documented [0,1] range, but Validate only WARNS, so it
		// reaches the renderer. Rounds away from zero like every other value:
		// the +0.5 trick this brick used to carry returned "-19".
		{"negative rounds away from zero", -0.2, "-20"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RatioToPercent(c.in); got != c.want {
				t.Errorf("RatioToPercent(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestFormatPoint(t *testing.T) {
	cases := []struct {
		name string
		x, y float64
		want string
	}{
		{"origin on x axis", 256, 0, "256x0"},
		{"fractions", 0.5, 2.5, "0.5x2.5"},
		{"origin on y axis", 0, 180, "0x180"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FormatPoint(c.x, c.y); got != c.want {
				t.Errorf("FormatPoint(%v,%v) = %q, want %q", c.x, c.y, got, c.want)
			}
		})
	}
}

// The nil-vs-empty pins: these helpers COLLAPSE nil and "" onto the same
// absent token on purpose (see convert.go), and the collapse is what makes
// the presence gates (required.go) the ONLY place that may tell them apart.
// Locked here so a future "clean-up" cannot reintroduce a distinction the
// wire does not have.
func TestDerefStringCollapsesAbsence(t *testing.T) {
	if got := DerefString(nil); got != "" {
		t.Errorf("DerefString(nil) = %q, want \"\"", got)
	}
	empty := ""
	if got := DerefString(&empty); got != "" {
		t.Errorf("DerefString(&\"\") = %q, want \"\"", got)
	}
	value := "P2S"
	if got := DerefString(&value); got != "P2S" {
		t.Errorf("DerefString(&\"P2S\") = %q, want \"P2S\"", got)
	}
}

func TestScalarFloatPtrCollapsesAbsence(t *testing.T) {
	if got := ScalarFloatPtr(nil); got != "" {
		t.Errorf("ScalarFloatPtr(nil) = %q, want \"\"", got)
	}
	zero := 0.0
	if got := ScalarFloatPtr(&zero); got != "0" {
		t.Errorf("ScalarFloatPtr(&0) = %q, want \"0\" — an authored zero is a value", got)
	}
	val := 220.0
	if got := ScalarFloatPtr(&val); got != "220" {
		t.Errorf("ScalarFloatPtr(&220) = %q, want \"220\"", got)
	}
}

// The ingest parsers read one wire scalar back. "" (absent) and "nil"
// (inherit) both map to canonical absent, and so does an unparseable value:
// the report surfaces it, the parse never fails a whole file.
func TestParseFloat(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want *float64
	}{
		{"integer", "220", new(220.0)},
		{"decimal", "0.945", new(0.945)},
		{"authored zero is not absent", "0", new(0.0)},
		{"negative", "-1", new(-1.0)},
		{"inherit token", "nil", nil},
		{"empty", "", nil},
		{"unparseable", "abc", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPtr(t, "ParseFloat", c.in, ParseFloat(c.in), c.want)
		})
	}
}

// A fractional percent stays exact: "0.7"/100 in float64 is
// 0.006999999999999999 — the drift orca used to carry.
func TestParsePercent(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want *float64
	}{
		{"whole", "30", new(0.3)},
		{"zero", "0", new(0.0)},
		{"full", "100", new(1.0)},
		{"sub-percent exact", "0.7", new(0.007)},
		{"fraction exact", "33.3", new(0.333)},
		{"negative", "-20", new(-0.2)},
		{"suffixed is not the bare wire", "70%", nil},
		{"inherit token", "nil", nil},
		{"empty", "", nil},
		{"unparseable", "abc", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPtr(t, "ParsePercent", c.in, ParsePercent(c.in), c.want)
		})
	}
}

// Layer counts are strict integers: a fractional value is unparseable, never
// truncated into a different layer.
func TestParseInt(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want *int
	}{
		{"layer count", "3", new(3)},
		{"authored zero is not absent", "0", new(0)},
		{"negative", "-1", new(-1)},
		{"fractional is not truncated", "2.5", nil},
		{"unparseable", "abc", nil},
		{"inherit token", "nil", nil},
		{"empty", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPtr(t, "ParseInt", c.in, ParseInt(c.in), c.want)
		})
	}
}

func TestParseBool(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want *bool
	}{
		{"one", "1", new(true)},
		{"zero is an authored false", "0", new(false)},
		{"word form is not the wire", "true", nil},
		{"inherit token", "nil", nil},
		{"empty", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPtr(t, "ParseBool", c.in, ParseBool(c.in), c.want)
		})
	}
}

func assertPtr[T comparable](t *testing.T, fn, in string, got, want *T) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Errorf("%s(%q) = %v, want %v", fn, in, got, want)
	case *got != *want:
		t.Errorf("%s(%q) = %v, want %v", fn, in, *got, *want)
	}
}
