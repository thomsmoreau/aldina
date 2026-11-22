// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package canonical

import (
	"slices"
	"strings"
	"testing"
)

// compatMachine and compatFilament form a fully compatible pair:
// CheckCompat reports nothing.
func compatMachine() Machine {
	return Machine{
		Meta:   MachineMeta{Name: new("Test 0.4 nozzle"), Model: new("Test")},
		Volume: BuildVolume{WidthMM: new(256.0), DepthMM: new(256.0), HeightMM: new(256.0)},
		Nozzle: NozzleSpec{DiameterMM: new(0.4), Type: new(NozzleHardened), FilamentDiameterMM: new(1.75)},
		Limits: MachineLimits{
			MaxNozzleTempC:    new(300.0),
			MaxBedTempC:       new(100.0),
			MaxVolumetricMM3S: new(24.0),
		},
		Features: MachineFeatures{Enclosed: new(true)},
	}
}

func compatFilament() Filament {
	return Filament{
		Meta:     Meta{Name: new("Test PETG"), Material: new(PETG)},
		Physical: Physical{DiameterMM: new(1.75)},
		Temps:    Temps{Nozzle: new(255.0), Bed: new(80.0)},
		Flow:     Flow{MaxVolumetricMM3S: new(20.0)},
	}
}

func TestCheckCompat(t *testing.T) {
	cases := []struct {
		name     string
		mutateM  func(*Machine)
		mutateF  func(*Filament)
		warnings []string
		errs     []string
	}{
		{name: "compatible pair"},

		// The declared target. A filament tuned for another printer may still
		// print here, so this only warns — and an absent compat claims nothing.
		{name: "compat names this very machine",
			mutateF: func(f *Filament) {
				f.Compat.Machines = []MachineRef{{Model: "Test", NozzleMM: 0.4}}
			}},
		{name: "compat names another machine",
			mutateF: func(f *Filament) {
				f.Compat.Machines = []MachineRef{{Model: "Other", NozzleMM: 0.6}}
			},
			warnings: []string{"compat.machines: this filament targets Other 0.6, not this machine"}},
		{name: "compat names several machines, one of them this one",
			mutateF: func(f *Filament) {
				f.Compat.Machines = []MachineRef{{Model: "Other", NozzleMM: 0.6}, {Model: "Test", NozzleMM: 0.4}}
			}},
		// Same model, wrong nozzle: the pair IS the key, not the model alone.
		{name: "compat names this model with another nozzle",
			mutateF: func(f *Filament) {
				f.Compat.Machines = []MachineRef{{Model: "Test", NozzleMM: 0.6}}
			},
			warnings: []string{"compat.machines: this filament targets Test 0.6, not this machine"}},
		// An undeclared model is UNKNOWN, not different — warning here would
		// fire on every half-written machine and train the reader to ignore it.
		{name: "machine declares no model: nothing claimed",
			mutateM: func(m *Machine) { m.Meta.Model = nil },
			mutateF: func(f *Filament) {
				f.Compat.Machines = []MachineRef{{Model: "Other", NozzleMM: 0.6}}
			}},
		{name: "machine declares no nozzle: nothing claimed",
			mutateM: func(m *Machine) { m.Nozzle.DiameterMM = nil },
			mutateF: func(f *Filament) {
				f.Compat.Machines = []MachineRef{{Model: "Other", NozzleMM: 0.6}}
			}},

		// The king test: PETG at 255 °C on a 250 °C machine must error.
		{name: "nozzle temp exceeds machine max",
			mutateM: func(m *Machine) { m.Limits.MaxNozzleTempC = new(250.0) },
			errs:    []string{"limits.max_nozzle_temp_c: filament nozzle 255 °C exceeds machine max 250 °C"}},
		{name: "bed temp exceeds machine max",
			mutateF: func(f *Filament) { f.Temps.Bed = new(110.0) },
			errs:    []string{"limits.max_bed_temp_c: filament bed 110 °C exceeds machine max 100 °C"}},
		{name: "flow exceeds hotend capacity: capped, not blocked",
			mutateM:  func(m *Machine) { m.Limits.MaxVolumetricMM3S = new(10.0) },
			warnings: []string{"limits.max_volumetric_mm3_s: filament flow 20.0 mm³/s exceeds hotend capacity 10.0 mm³/s: it will be capped"}},
		{name: "abrasive on brass",
			mutateM:  func(m *Machine) { m.Nozzle.Type = new(NozzleBrass) },
			mutateF:  func(f *Filament) { f.Physical.Abrasive = new(true) },
			warnings: []string{"nozzle.type: abrasive filament wears a brass nozzle"}},
		{name: "abrasive on stainless",
			mutateM:  func(m *Machine) { m.Nozzle.Type = new(NozzleStainless) },
			mutateF:  func(f *Filament) { f.Physical.Abrasive = new(true) },
			warnings: []string{"nozzle.type: abrasive filament wears a stainless_steel nozzle"}},
		{name: "abrasive on hardened: fine",
			mutateF: func(f *Filament) { f.Physical.Abrasive = new(true) }},
		{name: "diameter mismatch",
			mutateM: func(m *Machine) { m.Nozzle.FilamentDiameterMM = new(2.85) },
			errs:    []string{"nozzle.filament_diameter_mm: filament diameter 1.75 mm does not match machine 2.85 mm"}},
		{name: "ABS on open frame",
			mutateM:  func(m *Machine) { m.Features.Enclosed = new(false) },
			mutateF:  func(f *Filament) { f.Meta.Material = new(ABS) },
			warnings: []string{"features.enclosed: ABS prints best in an enclosed chamber"}},

		// A blocking issue never swallows the warnings found alongside it.
		{name: "error and warning together",
			mutateM: func(m *Machine) {
				m.Limits.MaxNozzleTempC = new(250.0)
				m.Features.Enclosed = new(false)
			},
			mutateF:  func(f *Filament) { f.Meta.Material = new(ABS) },
			warnings: []string{"features.enclosed: ABS prints best in an enclosed chamber"},
			errs:     []string{"limits.max_nozzle_temp_c: filament nozzle 255 °C exceeds machine max 250 °C"}},

		// nil = unknown → the check is skipped, never failed
		{name: "nil filament nozzle: temp check skipped",
			mutateM: func(m *Machine) { m.Limits.MaxNozzleTempC = new(250.0) },
			mutateF: func(f *Filament) { f.Temps.Nozzle = nil }},
		{name: "nil machine limit: temp check skipped",
			mutateM: func(m *Machine) { m.Limits.MaxNozzleTempC = nil },
			mutateF: func(f *Filament) { f.Temps.Nozzle = new(400.0) }},
		{name: "nil abrasive: wear check skipped",
			mutateM: func(m *Machine) { m.Nozzle.Type = new(NozzleBrass) }},
		{name: "nil filament diameter: diameter check skipped",
			mutateM: func(m *Machine) { m.Nozzle.FilamentDiameterMM = new(2.85) },
			mutateF: func(f *Filament) { f.Physical.DiameterMM = nil }},
		{name: "nil material: chamber check skipped",
			mutateM: func(m *Machine) { m.Features.Enclosed = new(false) },
			mutateF: func(f *Filament) { f.Meta.Material = nil }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, f := compatMachine(), compatFilament()
			if c.mutateM != nil {
				c.mutateM(&m)
			}
			if c.mutateF != nil {
				c.mutateF(&f)
			}
			warnings, err := CheckCompat(m, f)
			if !slices.Equal(warnings, c.warnings) {
				t.Errorf("warnings = %q, want %q", warnings, c.warnings)
			}
			if len(c.errs) == 0 {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("err = nil, want %q", strings.Join(c.errs, "\n"))
			}
			if got := err.Error(); got != strings.Join(c.errs, "\n") {
				t.Errorf("err = %q, want %q", got, strings.Join(c.errs, "\n"))
			}
		})
	}
}
