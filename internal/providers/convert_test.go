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
