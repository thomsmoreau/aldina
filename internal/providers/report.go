// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import "fmt"

// LossCounts tallies an ingest report per loss bucket. Each provider keeps its
// own bucket type and counts its own issues; the headline wording is one
// policy, shared so the two reports cannot drift apart.
type LossCounts struct {
	Engine, Fanout, Identity, Gcode, Approximated int
}

// Summary is the one-line report headline. The engine-knob count leads,
// because that is the honest answer to "what happened to my tuning".
func (c LossCounts) Summary() string {
	if c == (LossCounts{}) {
		return "all settings mapped cleanly"
	}
	return fmt.Sprintf("calibration settings don't transfer between slicers — %d engine-knob, %d fan-out, %d identity, %d gcode, %d approximated; re-tune on the target",
		c.Engine, c.Fanout, c.Identity, c.Gcode, c.Approximated)
}
