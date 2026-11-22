// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import "testing"

func TestLossCountsSummary(t *testing.T) {
	cases := []struct {
		in   LossCounts
		want string
	}{
		{LossCounts{}, "all settings mapped cleanly"},
		{LossCounts{Engine: 51, Identity: 2},
			"calibration settings don't transfer between slicers — 51 engine-knob, 0 fan-out, 2 identity, 0 gcode, 0 approximated; re-tune on the target"},
		// One approximation alone is still not a clean map.
		{LossCounts{Approximated: 1},
			"calibration settings don't transfer between slicers — 0 engine-knob, 0 fan-out, 0 identity, 0 gcode, 1 approximated; re-tune on the target"},
	}
	for _, c := range cases {
		if got := c.in.Summary(); got != c.want {
			t.Errorf("%+v.Summary() = %q, want %q", c.in, got, c.want)
		}
	}
}
