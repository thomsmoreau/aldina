// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// machine_render_test.go — golden tests (the whole mapping) + unit tests
// for the machine-only bricks and the loss policy.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/providers"
)

func floatP(v float64) *float64 { return &v }
func boolP(v bool) *bool        { return &v }

func axesP(a canonical.Axes) *canonical.Axes { return &a }
func flavorP(g canonical.GcodeFlavor) *canonical.GcodeFlavor {
	return &g
}
func nozzleTypeP(n canonical.NozzleType) *canonical.NozzleType {
	return &n
}
func extruderP(e canonical.ExtruderType) *canonical.ExtruderType {
	return &e
}

// baseMachine mirrors examples/machine_p2s_06.yaml — a full P2S 0.6 nozzle.
// Copy + tweak it for new golden cases, then `go test ./internal/providers/orca -update`.
func baseMachine() canonical.Machine {
	return canonical.Machine{
		Kind: "machine",
		Meta: canonical.MachineMeta{
			Name:   new("Bambu Lab P2S 0.6 nozzle"),
			Vendor: new("Bambu Lab"),
			Model:  new("Bambu Lab P2S"),
			Notes:  new("P2S 0.6 hardened, enclosed, aux fan."),
		},
		Volume: canonical.BuildVolume{WidthMM: floatP(256), DepthMM: floatP(256), HeightMM: floatP(256)},
		Nozzle: canonical.NozzleSpec{
			DiameterMM:         floatP(0.6),
			Type:               nozzleTypeP(canonical.NozzleHardened),
			Extruder:           extruderP(canonical.DirectDrive),
			FilamentDiameterMM: floatP(1.75),
			HeightMM:           floatP(4.2),
			MinLayerHeightMM:   floatP(0.12),
			MaxLayerHeightMM:   floatP(0.42),
		},
		Limits: canonical.MachineLimits{
			MaxNozzleTempC:    floatP(300),
			MaxBedTempC:       floatP(100),
			MaxVolumetricMM3S: floatP(24),
		},
		Kinetics: canonical.Kinetics{
			MaxSpeedMMS:   axesP(canonical.Axes{X: 600, Y: 600, Z: 20, E: 30}),
			MaxAccelMMS2:  axesP(canonical.Axes{X: 20000, Y: 20000, Z: 500, E: 5000}),
			MaxJerkMMS:    axesP(canonical.Axes{X: 9, Y: 9, Z: 3, E: 2.5}),
			MinTravelMMS:  floatP(0),
			MinExtrudeMMS: floatP(0),
		},
		Retract: canonical.MachineRetract{
			LengthMM:          floatP(1.4),
			SpeedMMS:          floatP(30),
			DeretractSpeedMMS: floatP(25),
			MinTravelMM:       floatP(1.5),
			ZHopMM:            floatP(0.4),
			OnLayerChange:     boolP(true),
			Wipe:              boolP(true),
		},
		Firmware: canonical.MachineFirmware{
			RelativeE:          boolP(false),
			FirmwareRetraction: boolP(false),
		},
		Features: canonical.MachineFeatures{
			Enclosed:      boolP(true),
			Plates:        []string{"textured", "engineering"},
			AuxFan:        boolP(true),
			AirFiltration: boolP(true),
			HeatedChamber: boolP(false),
		},
		Gcode: canonical.MachineGcode{
			Flavor: flavorP(canonical.FlavorMarlin),
			Start:  new("G90 ; start"),
			End:    new("M400 ; end"),
			Pause:  new("M400 U1"),
		},
		Defaults: canonical.MachineDefaults{
			PrintProfile:    new("0.30mm Standard @BBL P2S 0.6 nozzle"),
			FilamentProfile: new("Bambu PLA Basic @BBL P2S 0.6 nozzle"),
		},
	}
}

func TestRectToPrintableArea(t *testing.T) {
	want := []string{"0x0", "256x0", "256x256", "0x256"} // real export order
	if got := rectToPrintableArea(floatP(256), floatP(256)); !reflect.DeepEqual(got, want) {
		t.Errorf("rectToPrintableArea = %v, want %v", got, want)
	}
}

func TestBroadcastPair(t *testing.T) {
	// kinetics cardinality: [normal, silent] for the single v0 variant
	want := []string{"600", "600"}
	if got := pairFloat(600); !reflect.DeepEqual(got, want) {
		t.Errorf("pairFloat = %v, want %v", got, want)
	}
}

func TestBool01Forms(t *testing.T) {
	cases := []struct {
		name       string
		p          *bool
		wantScalar string
		wantArray  []string
	}{
		{"nil: omitted in both forms", nil, "", nil},
		{"true", boolP(true), "1", []string{"1"}},
		{"false", boolP(false), "0", []string{"0"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := boolScalarPtr(c.p); got != c.wantScalar {
				t.Errorf("boolScalarPtr = %q, want %q", got, c.wantScalar)
			}
			if got := boolArrayPtr(c.p); !reflect.DeepEqual(got, c.wantArray) {
				t.Errorf("boolArrayPtr = %v, want %v", got, c.wantArray)
			}
		})
	}
}

func TestMapExtruderTypeToOrca(t *testing.T) {
	// "Direct Drive" WITH space — the real export wins over composites
	got, warn := mapExtruderTypeToOrca(canonical.DirectDrive)
	if got != "Direct Drive" || warn != "" {
		t.Errorf("DirectDrive → %q, %q", got, warn)
	}
	if _, warn := mapExtruderTypeToOrca("magnetic_levitation"); warn == "" {
		t.Error("unknown extruder type must warn, not drop silently")
	}
}

func TestRenderMachineNameRequired(t *testing.T) {
	if _, _, err := RenderMachine(canonical.Machine{}, ""); err == nil {
		t.Error("empty meta.name must be a render error (no named preset)")
	}
}

// printer_settings_id IS the preset name — same rule as the filament's
// filament_settings_id, so the two must never drift apart.
func TestMachineIdentityCoherence(t *testing.T) {
	got, _, err := RenderMachine(baseMachine(), "")
	if err != nil {
		t.Fatalf("RenderMachine error: %v", err)
	}
	if got.PrinterSettingsID != got.Name {
		t.Errorf("PrinterSettingsID %q != Name %q", got.PrinterSettingsID, got.Name)
	}
}

func TestRenderMachineRejectsSelfInheritance(t *testing.T) {
	m := baseMachine()
	if _, _, err := RenderMachine(m, *m.Meta.Name); err == nil {
		t.Error("inherits == meta.name must be a render error")
	}
}

func TestRenderMachine(t *testing.T) {
	cases := []struct {
		name      string
		input     canonical.Machine
		inherits  string
		wantWarns []providers.Warning
	}{
		{
			name:  "machine-p2s-06",
			input: baseMachine(),
			// heated_chamber has no Orca field; limits/enclosed/plates are
			// the validate payload, not render data — no warning for them.
			wantWarns: []providers.Warning{{Path: "features.heated_chamber", Message: "no orca machine field, dropped"}},
		},
		{
			name: "machine-import-root",
			input: func() canonical.Machine {
				m := baseMachine()
				m.Meta.Name = new("Aldina P2S 0.6 root import test")
				return m
			}(),
			inherits:  "",
			wantWarns: []providers.Warning{{Path: "features.heated_chamber", Message: "no orca machine field, dropped"}},
		},
		{
			name: "machine-import-derived",
			input: func() canonical.Machine {
				m := baseMachine()
				m.Meta.Name = new("Aldina P2S 0.6 derived import test")
				return m
			}(),
			inherits:  "Bambu Lab P2S 0.6 nozzle",
			wantWarns: []providers.Warning{{Path: "features.heated_chamber", Message: "no orca machine field, dropped"}},
		},
		{
			name: "machine-minimal",
			input: canonical.Machine{
				Meta:   canonical.MachineMeta{Name: new("Bare 0.4"), Model: new("Bare")},
				Volume: canonical.BuildVolume{WidthMM: floatP(180), DepthMM: floatP(180), HeightMM: floatP(180)},
				Nozzle: canonical.NozzleSpec{DiameterMM: floatP(0.4)},
			},
		},
		{
			name: "machine-bowden-tags",
			input: func() canonical.Machine {
				m := baseMachine()
				m.Meta.Name = new("Bowden Rig 0.4")
				m.Meta.Tags = []string{"INPUT_SHAPER"}
				m.Nozzle.Extruder = extruderP(canonical.Bowden)
				m.Features.HeatedChamber = nil
				return m
			}(),
			wantWarns: []providers.Warning{{Path: "meta.tags", Message: "no orca consumer, dropped (prusa renders them into printer_notes)"}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, warns, err := RenderMachine(c.input, c.inherits)
			if err != nil {
				t.Fatalf("RenderMachine error: %v", err)
			}

			// Loss policy: warnings are part of the contract.
			if !reflect.DeepEqual(warns, c.wantWarns) {
				t.Errorf("warnings = %v, want %v", warns, c.wantWarns)
			}

			gotJSON, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal got: %v", err)
			}

			goldenPath := filepath.Join("testdata", "render", c.name+".golden.json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
					t.Fatalf("mkdir golden: %v", err)
				}
				if err := os.WriteFile(goldenPath, gotJSON, 0644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("updated golden: %s", goldenPath)
				return
			}

			wantRaw, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v", goldenPath, err)
			}

			var gotMap, wantMap map[string]any
			if err := json.Unmarshal(gotJSON, &gotMap); err != nil {
				t.Fatalf("unmarshal got: %v", err)
			}
			if err := json.Unmarshal(wantRaw, &wantMap); err != nil {
				t.Fatalf("unmarshal golden: %v", err)
			}

			if !reflect.DeepEqual(gotMap, wantMap) {
				t.Errorf("golden mismatch\n got:\n%s\n\n want:\n%s", gotJSON, wantRaw)
			}
		})
	}
}
