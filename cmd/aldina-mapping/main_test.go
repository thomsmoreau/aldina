// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/thomsmoreau/Aldina/internal/providers"
)

// TestRenderSection pins the generated markdown so a change to the schema or
// the cases is a deliberate, reviewed diff.
func TestRenderSection(t *testing.T) {
	tab := providers.ConformanceTable{
		Filament: []providers.ConformanceCase{
			{Concept: "temps.bed", Value: 0.0,
				Orca:  map[string]any{"hot_plate_temp": []any{"0"}},
				Prusa: map[string]any{"bed_temperature": "0"},
				Note:  "authored zero is a value"},
		},
		Machine: []providers.ConformanceCase{
			{Concept: "retract.speed_mms", Value: 35.0,
				Orca:  map[string]any{"retraction_speed": []any{"35"}},
				Prusa: map[string]any{"retract_speed": "35"}},
		},
	}

	got := renderSection(tab)
	for _, want := range []string{
		"| resource | concept | value | OrcaSlicer | PrusaSlicer | note |",
		"| filament | temps.bed | 0 | `hot_plate_temp` = [\"0\"] | `bed_temperature` = \"0\" | authored zero is a value |",
		"| machine | retract.speed_mms | 35 | `retraction_speed` = [\"35\"] | `retract_speed` = \"35\" | |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderSection output missing %q\n%s", want, got)
		}
	}
}

// TestMappingDocIsInSync is the anti-drift gate: it regenerates the table from
// conformance.yaml and asserts the committed docs/mapping.md section matches.
// If you edited the generated section by hand, this fails and tells you to run
// the generator instead.
func TestMappingDocIsInSync(t *testing.T) {
	tab, err := loadTable("../../internal/providers/conformance.yaml")
	if err != nil {
		t.Fatalf("conformance.yaml: %v", err)
	}
	raw, err := os.ReadFile("../../docs/mapping.md")
	if err != nil {
		t.Fatalf("mapping.md: %v", err)
	}

	doc := string(raw)
	start := strings.Index(doc, startMarker)
	end := strings.Index(doc, endMarker)
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("mapping.md is missing the %s / %s markers", startMarker, endMarker)
	}
	committed := doc[start+len(startMarker) : end]
	want := renderSection(tab)

	if committed != "\n\n"+want {
		t.Errorf("mapping.md wire-contract section is out of sync.\n"+
			"Run: go run ./cmd/aldina-mapping\n"+
			"committed:\n%s\nwant:\n%s", committed, want)
	}
}
