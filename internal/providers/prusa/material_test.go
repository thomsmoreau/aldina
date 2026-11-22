// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

import (
	"testing"

	"github.com/thomsmoreau/Aldina/internal/canonical"
)

// Every canonical Material. canonical has no list of its enum, so a new value
// must be added here too — or the symmetry below never sees it.
var allMaterials = []canonical.Material{
	canonical.PLA, canonical.PETG, canonical.ABS, canonical.ASA,
	canonical.TPU, canonical.PC, canonical.PA, canonical.PHA,
}

// The vocabulary is symmetric: whatever name render writes without a
// warning, ingest reads back as the same material. A name that exists in one
// direction only would break the Loop test.
func TestMaterialVocabularySymmetric(t *testing.T) {
	for _, m := range allMaterials {
		t.Run(string(m), func(t *testing.T) {
			name, warn := mapMaterialToPrusa(m)
			if warn != "" {
				return // a lossy render (PHA → PLA) has no inverse to prove
			}
			got, ok := materialFromPrusa(name)
			if !ok || got != m {
				t.Errorf("render %q → %q, ingest %q → (%q, %v), want %q", m, name, name, got, ok, m)
			}
		})
	}
}
