// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package canonical

import (
	"strings"
	"testing"
)

// validMachine returns a complete machine: Validate reports no warning.
func validMachine() Machine {
	return Machine{
		Kind:          "machine",
		SchemaVersion: 1,
		Meta:          MachineMeta{Name: new("Bambu Lab P2S 0.6 nozzle"), Vendor: new("Bambu Lab"), Model: new("Bambu Lab P2S")},
		Volume:        BuildVolume{WidthMM: new(256.0), DepthMM: new(256.0), HeightMM: new(256.0)},
		Nozzle: NozzleSpec{
			DiameterMM:         new(0.6),
			Type:               new(NozzleHardened),
			Extruder:           new(DirectDrive),
			FilamentDiameterMM: new(1.75),
		},
	}
}

func TestMachineValidate(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*Machine)
		wantWarns []string // substrings expected; empty = no warning
		dontWant  []string // substrings that must NOT appear
	}{
		{name: "valid", mutate: nil},

		{name: "missing name", mutate: func(m *Machine) { m.Meta.Name = new("") }, wantWarns: []string{"meta.name: absent"}},
		{name: "missing model", mutate: func(m *Machine) { m.Meta.Model = new("") }, wantWarns: []string{"meta.model: absent"}},
		{name: "zero width", mutate: func(m *Machine) { m.Volume.WidthMM = new(0.0) }, wantWarns: []string{"volume.width_mm: must be > 0"}},
		{name: "zero nozzle", mutate: func(m *Machine) { m.Nozzle.DiameterMM = new(0.0) }, wantWarns: []string{"nozzle.diameter_mm: must be > 0"}},
		{name: "unknown nozzle type", mutate: func(m *Machine) { m.Nozzle.Type = new(NozzleType("unobtainium")) }, wantWarns: []string{"nozzle.type: unknown value"}},
		{name: "unknown extruder", mutate: func(m *Machine) { m.Nozzle.Extruder = new(ExtruderType("delta")) }, wantWarns: []string{"nozzle.extruder: unknown value"}},
		{name: "unknown flavor", mutate: func(m *Machine) { m.Gcode.Flavor = new(GcodeFlavor("gcode")) }, wantWarns: []string{"gcode.flavor: unknown value"}},
		{name: "inverted layer heights", mutate: func(m *Machine) {
			m.Nozzle.MinLayerHeightMM = new(0.4)
			m.Nozzle.MaxLayerHeightMM = new(0.1)
		}, wantWarns: []string{"nozzle.min_layer_height_mm: must be <= nozzle.max_layer_height_mm"}},
		{name: "negative limit", mutate: func(m *Machine) { m.Limits.MaxNozzleTempC = new(-5.0) }, wantWarns: []string{"limits.max_nozzle_temp_c: must be > 0"}},

		// nil limits/optional fields are legal and silent: the cross-check
		// is skipped, nothing is warned
		{name: "one layer height set: no coherence check", mutate: func(m *Machine) {
			m.Nozzle.MaxLayerHeightMM = new(0.3)
		}, dontWant: []string{"min_layer_height"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := validMachine()
			if c.mutate != nil {
				c.mutate(&m)
			}
			warns := m.Validate()
			for _, want := range c.wantWarns {
				found := false
				for _, w := range warns {
					if strings.Contains(w, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("want warning containing %q, got %v", want, warns)
				}
			}
			for _, dont := range c.dontWant {
				for _, w := range warns {
					if strings.Contains(w, dont) {
						t.Errorf("unwanted warning containing %q: %v", dont, w)
					}
				}
			}
			if len(c.wantWarns) == 0 && len(warns) != 0 {
				t.Errorf("unexpected warnings: %v", warns)
			}
		})
	}
}

func TestLoadMachineP2SExample(t *testing.T) {
	m, err := LoadMachine("../../examples/machine_p2s_06.yaml")
	if err != nil {
		t.Fatalf("LoadMachine: %v", err)
	}
	// A label, not Orca's catalogue title: the linkage key is (model, nozzle).
	// Deref before printing — %v on a *string reports an address, which is what
	// made the previous failure of this very check unreadable.
	for _, c := range []struct {
		field, want string
		got         *string
	}{
		{"meta.name", "Bambu Lab P2S 0.6", m.Meta.Name},
		{"meta.model", "Bambu Lab P2S", m.Meta.Model},
	} {
		if c.got == nil {
			t.Errorf("%s is nil, want %q", c.field, c.want)
		} else if *c.got != c.want {
			t.Errorf("%s = %q, want %q", c.field, *c.got, c.want)
		}
	}
	if warns := m.Validate(); len(warns) != 0 {
		t.Errorf("example should validate clean, got %v", warns)
	}
}
