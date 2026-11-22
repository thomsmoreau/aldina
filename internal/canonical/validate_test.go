// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package canonical

import (
	"strings"
	"testing"
)

// validFilament returns a complete filament: Validate reports no warning.
func validFilament() Filament {
	return Filament{
		Kind:          "filament",
		SchemaVersion: 1,
		Meta:          Meta{Name: new("Test PLA"), Vendor: new("Test"), Material: new(PLA)},
		Physical: Physical{
			DiameterMM:  new(1.75),
			DensityGCm3: new(1.24),
		},
		Temps: Temps{
			Nozzle:      new(220.0),
			NozzleRange: new([2]float64{210, 240}),
		},
		Flow: Flow{
			MaxVolumetricMM3S: new(25.0),
			FlowRatio:         new(0.95),
		},
		Cooling: Cooling{
			FanMin: new(0.3),
			FanMax: new(0.8),
		},
	}
}

// TestZeroIsNotAbsence is the point of the all-pointer canonical: an authored
// zero (`bed: 0` = unheated bed, `fan_min: 0` = fan off) is a real intent and
// must stay distinguishable from a key the author never wrote. Before
// pointers these two states shared the same float64 zero value.
func TestZeroIsNotAbsence(t *testing.T) {
	absent := validFilament()
	authored := validFilament()
	authored.Temps.Bed = new(0.0)
	authored.Cooling.FanMin = new(0.0)

	if absent.Temps.Bed != nil {
		t.Errorf("Temps.Bed = %v, want nil when the key is absent", *absent.Temps.Bed)
	}
	if authored.Temps.Bed == nil || *authored.Temps.Bed != 0 {
		t.Errorf("Temps.Bed = %v, want an authored 0", authored.Temps.Bed)
	}
	// An authored 0 is in range, so it must not be reported as a violation.
	for _, w := range authored.Validate() {
		if strings.Contains(w, "cooling.fan_min") {
			t.Errorf("authored fan_min 0 warned: %q", w)
		}
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*Filament)
		wantWarns []string // substrings expected; empty = no warning
		dontWant  []string // substrings that must NOT appear
	}{
		{name: "valid", mutate: nil, wantWarns: nil},

		// presence: nil is legal but signalled
		{name: "missing name", mutate: func(f *Filament) { f.Meta.Name = nil }, wantWarns: []string{"meta.name: absent"}},
		{name: "empty name", mutate: func(f *Filament) { f.Meta.Name = new("") }, wantWarns: []string{"meta.name: absent"}},
		{name: "missing material", mutate: func(f *Filament) { f.Meta.Material = nil }, wantWarns: []string{"meta.material: absent"}},
		{name: "missing nozzle", mutate: func(f *Filament) { f.Temps.Nozzle = nil }, wantWarns: []string{"temps.nozzle: absent"}},
		// diameter has NO presence warning: 1.75 mm is near-universal, so
		// signalling its absence would be noise, not information.
		{name: "missing diameter is silent", mutate: func(f *Filament) { f.Physical.DiameterMM = nil },
			dontWant: []string{"physical.diameter_mm"}},
		{name: "missing flow", mutate: func(f *Filament) {
			f.Flow.MaxVolumetricMM3S = nil
			f.Flow.FlowRatio = nil
		}, wantWarns: []string{"flow.max_volumetric_mm3_s: absent", "flow.flow_ratio: absent"}},

		// no double jeopardy: nil = presence warning ONLY (no bounds check)
		{name: "nil nozzle skips bounds", mutate: func(f *Filament) { f.Temps.Nozzle = nil },
			wantWarns: []string{"temps.nozzle: absent"}, dontWant: []string{"temps.nozzle: must be > 0"}},

		// ratios outside [0,1]
		{name: "fan_min above 1", mutate: func(f *Filament) { f.Cooling.FanMin = new(1.2) }, wantWarns: []string{"cooling.fan_min: must be in [0,1]"}},
		{name: "fan_max negative", mutate: func(f *Filament) { f.Cooling.FanMax = new(-0.1) }, wantWarns: []string{"cooling.fan_max: must be in [0,1]"}},
		{name: "bridges_fan above 1", mutate: func(f *Filament) { f.Cooling.BridgesFan = new(2.0) }, wantWarns: []string{"cooling.bridges_fan: must be in [0,1]"}},
		{name: "shrinkage_xy negative", mutate: func(f *Filament) { f.Physical.ShrinkageXY = new(-0.5) }, wantWarns: []string{"physical.shrinkage_xy: must be in [0,1]"}},
		{name: "shrinkage_z above 1", mutate: func(f *Filament) { f.Physical.ShrinkageZ = new(1.01) }, wantWarns: []string{"physical.shrinkage_z: must be in [0,1]"}},
		{name: "fan_min above fan_max", mutate: func(f *Filament) { f.Cooling.FanMin = new(0.9) }, wantWarns: []string{"cooling.fan_min: must be <= cooling.fan_max"}},

		// strictly positive values
		{name: "zero diameter", mutate: func(f *Filament) { f.Physical.DiameterMM = new(0.0) }, wantWarns: []string{"physical.diameter_mm: must be > 0"}},
		{name: "negative density", mutate: func(f *Filament) { f.Physical.DensityGCm3 = new(-1.0) }, wantWarns: []string{"physical.density_g_cm3: must be > 0"}},
		{name: "zero nozzle", mutate: func(f *Filament) { f.Temps.Nozzle = new(0.0) }, wantWarns: []string{"temps.nozzle: must be > 0"}},
		{name: "zero max volumetric", mutate: func(f *Filament) { f.Flow.MaxVolumetricMM3S = new(0.0) }, wantWarns: []string{"flow.max_volumetric_mm3_s: must be > 0"}},
		{name: "zero flow ratio", mutate: func(f *Filament) { f.Flow.FlowRatio = new(0.0) }, wantWarns: []string{"flow.flow_ratio: must be > 0"}},

		// range inversé
		{name: "nozzle_range inverted", mutate: func(f *Filament) { f.Temps.NozzleRange = new([2]float64{240, 210}) }, wantWarns: []string{"temps.nozzle_range: min must be <= max"}},

		// all violations surface at once (no fail-fast)
		{
			name: "multiple violations",
			mutate: func(f *Filament) {
				f.Cooling.FanMin = new(2.0)
				f.Temps.Nozzle = new(0.0)
			},
			wantWarns: []string{"cooling.fan_min: must be in [0,1]", "temps.nozzle: must be > 0"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := validFilament()
			if c.mutate != nil {
				c.mutate(&f)
			}
			warns := f.Validate()
			joined := strings.Join(warns, "\n")

			if len(c.wantWarns) == 0 && len(warns) != 0 {
				t.Fatalf("Validate() = %v, want no warning", warns)
			}
			for _, want := range c.wantWarns {
				if !strings.Contains(joined, want) {
					t.Errorf("Validate() = %q, missing %q", joined, want)
				}
			}
			for _, dont := range c.dontWant {
				if strings.Contains(joined, dont) {
					t.Errorf("Validate() = %q, unexpected %q", joined, dont)
				}
			}
		})
	}
}
