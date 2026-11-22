// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

// conformance.go — the cross-provider wire contract, as one parseable schema.
//
// conformance.yaml is the single source of truth for how one canonical
// concept lands on each slicer's wire. Two consumers read it, and both must
// see the same fields, so the schema and its parser live here once:
//   - the king-test harness (conformance_test.go) asserts every case;
//   - cmd/aldina-mapping regenerates the table section of docs/mapping.md.

import "gopkg.in/yaml.v3"

// ConformanceCase pins one authored canonical value to both wire forms.
// A nil Value means the canonical field stays absent; a nil entry inside
// Orca/Prusa means that wire key must be absent (see conformance.yaml header).
type ConformanceCase struct {
	Concept string         `yaml:"concept"`
	Note    string         `yaml:"note"`
	Value   any            `yaml:"value"`
	Orca    map[string]any `yaml:"orca"`
	Prusa   map[string]any `yaml:"prusa"`
}

// ConformanceTable holds one case list per canonical resource.
type ConformanceTable struct {
	Filament []ConformanceCase `yaml:"filament"`
	Machine  []ConformanceCase `yaml:"machine"`
}

// ParseConformance decodes the YAML contract.
func ParseConformance(raw []byte) (ConformanceTable, error) {
	var tab ConformanceTable
	if err := yaml.Unmarshal(raw, &tab); err != nil {
		return ConformanceTable{}, err
	}
	return tab, nil
}
