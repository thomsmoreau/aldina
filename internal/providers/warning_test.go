// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import "testing"

func TestWarningString(t *testing.T) {
	w := Warning{Path: "temps.softening", Message: "no prusa field, dropped"}
	if got, want := w.String(), "temps.softening: no prusa field, dropped"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
