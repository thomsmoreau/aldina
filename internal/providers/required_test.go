// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"strings"
	"testing"
)

func TestMissingError(t *testing.T) {
	cases := []struct {
		name    string
		missing []string
		want    string // "" = expect nil
	}{
		{name: "nothing missing", missing: nil},
		{name: "empty slice is not an error", missing: []string{}},
		{name: "one field", missing: []string{"meta.name"},
			want: "orca filament: meta.name is required"},
		// No fail-fast: every missing field surfaces in one message, same
		// policy as canonical.Validate.
		{name: "several fields", missing: []string{"meta.name", "meta.vendor"},
			want: "orca filament: required fields missing: meta.name, meta.vendor"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := MissingError("orca filament", c.missing)
			if c.want == "" {
				if err != nil {
					t.Fatalf("MissingError = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("MissingError = nil, want %q", c.want)
			}
			if got := err.Error(); got != c.want {
				t.Errorf("MissingError = %q, want %q", got, c.want)
			}
		})
	}
}

// The kind is what tells the reader WHICH provider and WHICH resource
// refused, so it must reach the message.
func TestMissingErrorCarriesKind(t *testing.T) {
	err := MissingError("prusa machine", []string{"meta.name"})
	if err == nil || !strings.Contains(err.Error(), "prusa machine") {
		t.Errorf("MissingError = %v, want the kind in the message", err)
	}
}
