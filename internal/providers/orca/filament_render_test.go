// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/providers"
)

var update = flag.Bool("update", false, "regenerate golden files")

// The machine every fixture targets, in the two shapes the code needs: the
// canonical reference an author writes, and the exact Orca preset name the
// catalogue resolves it to — the second must never be typed anywhere but here.
const p2s06MachinePresetName = "Bambu Lab P2S 0.6 nozzle"

var p2s06 = canonical.MachineRef{Model: "Bambu Lab P2S", NozzleMM: 0.6}

// baseFilament returns a fully populated canonical filament used as a
// starting point for golden-file tests. Add a new case by copying + tweaking
// this struct, then run `go test ./internal/providers/orca -update`.
func baseFilament() canonical.Filament {
	return canonical.Filament{
		Kind:          "filament",
		SchemaVersion: 1,
		Meta: canonical.Meta{
			Name:     new("Overture PLA Rock White"),
			Vendor:   new("Overture"),
			Material: new(canonical.PLA),
			ColorHex: new("B3B4B2"),
			Notes:    new("test fixture"),
		},
		Physical: canonical.Physical{
			DiameterMM:  new(1.75),
			DensityGCm3: new(1.24),
			CostPerKg:   new(20.0),
			ShrinkageXY: new(0.0),
		},
		Temps: canonical.Temps{
			Nozzle:           new(220.0),
			NozzleFirstLayer: new(225.0),
			NozzleRange:      new([2]float64{210, 240}),
			Bed:              new(60.0),
			BedFirstLayer:    new(65.0),
			Softening:        new(55.0),
		},
		Flow: canonical.Flow{
			MaxVolumetricMM3S: new(25.0),
			FlowRatio:         new(0.95),
		},
		Cooling: canonical.Cooling{
			FanMin:              new(0.3),
			FanMax:              new(0.8),
			FanOffFirstLayers:   new(2),
			FullFanAtLayer:      new(4),
			SlowDownLayerTimeS:  new(10.0),
			SlowDownMinSpeedMMS: new(10.0),
			BridgesFan:          new(0.5),
		},
		Retraction: canonical.Retraction{
			LengthMM:          new(0.8),
			SpeedMMS:          new(40.0),
			DeretractSpeedMMS: new(40.0),
			ZHopMM:            new(0.2),
			Wipe:              new(true),
			WipeDistanceMM:    new(2.0),
		},
		Gcode: canonical.Gcode{
			Start: new("M104 S{nozzle_temperature}"),
		},
		Compat: canonical.Compat{
			Machines: []canonical.MachineRef{p2s06},
		},
	}
}

func TestBroadcast(t *testing.T) {
	got := broadcast("220")
	want := []string{"220"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("broadcast() = %v, want %v", got, want)
	}
}

func TestBroadcastFloat(t *testing.T) {
	got := broadcastFloat(220)
	want := []string{"220"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("broadcastFloat() = %v, want %v", got, want)
	}
}

func TestRenderFloatPtr(t *testing.T) {
	if got := renderFloatPtr(nil); !reflect.DeepEqual(got, []string{"nil"}) {
		t.Errorf("nil = %v, want [nil]", got)
	}
	if got := renderFloatPtr(new(0.8)); !reflect.DeepEqual(got, []string{"0.8"}) {
		t.Errorf("value = %v, want [0.8]", got)
	}
}

func TestRenderBoolPtr(t *testing.T) {
	cases := []struct {
		name string
		in   *bool
		want []string
	}{
		{"nil", nil, []string{"nil"}},
		{"true", new(true), []string{"1"}},
		{"false", new(false), []string{"0"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := renderBoolPtr(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("renderBoolPtr() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestMapMaterialToOrca(t *testing.T) {
	cases := []struct {
		in       canonical.Material
		want     string
		wantWarn string
	}{
		{canonical.PLA, "PLA", ""},
		{canonical.PETG, "PETG", ""},
		{canonical.PHA, "PLA", "PHA unsupported by orca, rendered as PLA"},
		{canonical.ABS, "ABS", ""},
	}
	for _, c := range cases {
		t.Run(string(c.in), func(t *testing.T) {
			got, warn := mapMaterialToOrca(c.in)
			if got != c.want || warn != c.wantWarn {
				t.Errorf("mapMaterialToOrca(%q) = (%q, %q), want (%q, %q)", c.in, got, warn, c.want, c.wantWarn)
			}
		})
	}
}

func TestTruncate6(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"short", "short"},
		{"exactly six", "exactl"},
		{"Overture", "Overtu"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := truncate6(c.in); got != c.want {
				t.Errorf("truncate6(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestMakeFilamentID(t *testing.T) {
	m := canonical.Meta{Vendor: new("Overture"), Material: new(canonical.PLA), Name: new("Overture PLA Rock White")}
	if got := makeFilamentID(m); got != "POvertud3f0f7" {
		t.Errorf("makeFilamentID() = %q, want POvertud3f0f7", got)
	}
}

// Orca groups presets by filament_id: two filaments sharing vendor and
// material but differing by Name must NOT collide.
func TestMakeFilamentIDNoCollision(t *testing.T) {
	a := canonical.Meta{Vendor: new("eSUN"), Material: new(canonical.PLA), Name: new("eSUN PLA+")}
	b := canonical.Meta{Vendor: new("eSUN"), Material: new(canonical.PLA), Name: new("eSUN PLA Matte")}
	if makeFilamentID(a) == makeFilamentID(b) {
		t.Errorf("collision: %q and %q share id %q", *a.Name, *b.Name, makeFilamentID(a))
	}
}

func TestRenderFilamentIdentity(t *testing.T) {
	f := baseFilament()
	got, warns := renderFilamentIdentity(f, p2s06)
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
	want := FilamentIdentity{
		Name:                  new("Overture PLA Rock White @Bambu Lab P2S 0.6"),
		Version:               new(orcaVersion),
		From:                  new("User"),
		FilamentSettingsID:    []string{"Overture PLA Rock White @Bambu Lab P2S 0.6"},
		FilamentID:            "POvertud3f0f7",
		FilamentType:          []string{"PLA"},
		FilamentVendor:        []string{"Overture"},
		FilamentNotes:         []string{"test fixture"},
		DefaultFilamentColour: []string{"#B3B4B2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("identity mismatch:\ngot  %+v\nwant %+v", got, want)
	}
}

// Policy: cool plates are rendered INCOMPATIBLE ("0", not 0°C) whatever the
// canonical bed temp — PETG at 80°C bed has no business on a cool plate.
// Field-by-field values live in the goldens; only the policy is asserted here.
func TestCoolPlateIncompatible(t *testing.T) {
	for _, bed := range []float64{0, 60, 110} {
		got := renderTemperatures(canonical.Temps{Bed: new(bed)})
		plates := map[string][]string{
			"CoolPlateTemp":                     got.CoolPlateTemp,
			"CoolPlateTempInitialLayer":         got.CoolPlateTempInitialLayer,
			"TexturedCoolPlateTemp":             got.TexturedCoolPlateTemp,
			"TexturedCoolPlateTempInitialLayer": got.TexturedCoolPlateTempInitialLayer,
		}
		for name, v := range plates {
			if !reflect.DeepEqual(v, []string{"0"}) {
				t.Errorf("bed=%v: %s = %v, want [0] (plate incompatible)", bed, name, v)
			}
		}
	}
}

// Policy: Orca identity must be coherent or import/linkage misbehaves — Name
// equals filament_settings_id, and compatible_printers carries the catalogue
// preset names plus the reference itself (the self-link name; mapping.md,
// machine pitfall #1).
func TestIdentityCoherence(t *testing.T) {
	got, _, err := RenderFilament(baseFilament(), p2s06)
	if err != nil {
		t.Fatalf("RenderFilament error: %v", err)
	}
	if *got.Name != got.FilamentSettingsID[0] {
		t.Errorf("Name %q != FilamentSettingsID[0] %q", *got.Name, got.FilamentSettingsID[0])
	}
	// The label is the canonical reference, NOT the preset name: it no longer
	// has to match anything in Orca's catalogue.
	if !strings.HasSuffix(*got.Name, " @Bambu Lab P2S 0.6") {
		t.Errorf("Name %q missing the machine label suffix", *got.Name)
	}
	want := []string{p2s06MachinePresetName, p2s06.String()}
	if !reflect.DeepEqual(got.CompatiblePrinters, want) {
		t.Errorf("CompatiblePrinters = %v, want %v", got.CompatiblePrinters, want)
	}
}

// A model/nozzle pair may resolve to SEVERAL presets; all of them must be
// declared, or the filament silently misses hardware it is valid for.
func TestCompatFansOutToEveryMatchingPreset(t *testing.T) {
	got, _, err := RenderFilament(baseFilament(), canonical.MachineRef{Model: "Raise3D Pro3", NozzleMM: 0.4})
	if err != nil {
		t.Fatalf("RenderFilament error: %v", err)
	}
	if len(got.CompatiblePrinters) < 2 {
		t.Errorf("CompatiblePrinters = %v, want the Left/Right/Dual variants", got.CompatiblePrinters)
	}
}

// Policy: a fully-nil canonical Retraction means "inherit from machine" —
// every rendered field must pass Orca's "nil" through, none may be dropped.
func TestRetractionNilInheritance(t *testing.T) {
	got := renderRetractionOverrides(canonical.Retraction{})
	fields := map[string][]string{
		"FilamentRetractionLength":  got.FilamentRetractionLength,
		"FilamentRetractionSpeed":   got.FilamentRetractionSpeed,
		"FilamentDeretractionSpeed": got.FilamentDeretractionSpeed,
		"FilamentZHop":              got.FilamentZHop,
		"FilamentWipe":              got.FilamentWipe,
		"FilamentWipeDistance":      got.FilamentWipeDistance,
	}
	for name, v := range fields {
		if !reflect.DeepEqual(v, []string{"nil"}) {
			t.Errorf("%s = %v, want [nil]", name, v)
		}
	}
}

func TestRenderFilamentNameRequired(t *testing.T) {
	f := canonical.Filament{Meta: canonical.Meta{}}
	if _, _, err := RenderFilament(f, p2s06); err == nil {
		t.Errorf("RenderFilament with nil name: want error, got nil")
	}
	f.Meta.Name = new("")
	if _, _, err := RenderFilament(f, p2s06); err == nil {
		t.Errorf("RenderFilament with empty name: want error, got nil")
	}
}

// An unresolvable target is a HARD error, never a preset rendered without
// compatible_printers: Orca imports the latter without complaint and then
// offers the filament for every machine on the shelf.
func TestRenderFilamentTargetMustResolve(t *testing.T) {
	cases := []struct {
		name   string
		target canonical.MachineRef
	}{
		{"no model", canonical.MachineRef{NozzleMM: 0.6}},
		{"no nozzle", canonical.MachineRef{Model: "Bambu Lab P2S"}},
		{"unknown model", canonical.MachineRef{Model: "Nonexistent 9000", NozzleMM: 0.4}},
		{"nozzle the model never had", canonical.MachineRef{Model: "Bambu Lab P2S", NozzleMM: 1.2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := RenderFilament(baseFilament(), c.target); err == nil {
				t.Error("want an error, got nil")
			}
		})
	}
}

func TestRenderFilamentMaterialOmitted(t *testing.T) {
	f := baseFilament()
	f.Meta.Material = nil
	got, warns, err := RenderFilament(f, p2s06)
	if err != nil {
		t.Fatalf("RenderFilament error: %v", err)
	}
	wantWarn := providers.Warning{Path: "meta.material", Message: "absent, filament_type omitted (slicer default applies)"}
	if !slices.Contains(warns, wantWarn) {
		t.Errorf("warnings = %v, want to contain %v", warns, wantWarn)
	}
	if got.FilamentIdentity.FilamentType != nil {
		t.Errorf("FilamentType = %v, want nil when material absent", got.FilamentIdentity.FilamentType)
	}
}

// A value Orca's own ingest cannot read back identically must warn on its
// exact canonical path, and only when the field is set (ADR-0004).
func TestRenderFilamentDeclaresLosses(t *testing.T) {
	cases := []struct {
		name     string
		edit     func(*canonical.Filament)
		path     string
		wantWarn bool
	}{
		{"abrasive false", func(f *canonical.Filament) { f.Physical.Abrasive = new(false) }, "physical.abrasive", true},
		{"abrasive true", func(f *canonical.Filament) { f.Physical.Abrasive = new(true) }, "physical.abrasive", false},
		{"abrasive nil", func(f *canonical.Filament) { f.Physical.Abrasive = nil }, "physical.abrasive", false},
		{"overrides.prusa set", func(f *canonical.Filament) {
			f.Overrides = map[string]map[string]any{"prusa": {"filament_type": "FLEX"}}
		}, "overrides.prusa", true},
		{"overrides.prusa empty", func(f *canonical.Filament) {
			f.Overrides = map[string]map[string]any{"prusa": {}}
		}, "overrides.prusa", false},
		{"overrides nil", func(f *canonical.Filament) { f.Overrides = nil }, "overrides.prusa", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := baseFilament()
			c.edit(&f)
			_, warns, err := RenderFilament(f, p2s06)
			if err != nil {
				t.Fatalf("RenderFilament: %v", err)
			}
			got := slices.ContainsFunc(warns, func(w providers.Warning) bool { return w.Path == c.path })
			if got != c.wantWarn {
				t.Errorf("warning on %s = %v, want %v (warnings: %v)", c.path, got, c.wantWarn, warns)
			}
		})
	}
}

func TestBroadcastFloatPtrOmitted(t *testing.T) {
	if got := broadcastFloatPtr(nil); got != nil {
		t.Errorf("broadcastFloatPtr(nil) = %v, want nil", got)
	}
	if got := broadcastFloatPtr(new(1.75)); !reflect.DeepEqual(got, []string{"1.75"}) {
		t.Errorf("broadcastFloatPtr(1.75) = %v, want [1.75]", got)
	}
}

func TestRenderFilamentSelfLinksEmittedMachine(t *testing.T) {
	// The filament must also declare the machine's own title, not just the
	// catalogue name, or an imported aldina machine never shows it (mapping.md,
	// machine pitfall #1; verified on OrcaSlicer 2.4.2).
	got, _, err := RenderFilament(baseFilament(), p2s06)
	if err != nil {
		t.Fatalf("RenderFilament: %v", err)
	}
	want := []string{p2s06MachinePresetName, p2s06.String()}
	if !slices.Equal(got.FilamentCompatibility.CompatiblePrinters, want) {
		t.Errorf("compatible_printers = %v, want %v",
			got.FilamentCompatibility.CompatiblePrinters, want)
	}
}

func TestRenderFilament(t *testing.T) {
	cases := []struct {
		name      string
		input     canonical.Filament
		target    canonical.MachineRef
		wantWarns []providers.Warning
	}{
		{
			name:   "pla-full",
			input:  baseFilament(),
			target: p2s06,
		},
		{
			name: "petg-abrasive",
			input: func() canonical.Filament {
				f := baseFilament()
				f.Meta.Name = new("Overture PETG Rock White")
				f.Meta.Material = new(canonical.PETG)
				f.Physical.Abrasive = new(true)
				f.Physical.ShrinkageXY = new(0.01)
				f.Physical.CostPerKg = new(30.0)
				return f
			}(),
			target: p2s06,
		},
		{
			name: "inherit-retraction",
			input: func() canonical.Filament {
				f := baseFilament()
				f.Retraction = canonical.Retraction{}
				return f
			}(),
			target: p2s06,
		},
		{
			name: "pha-fallback",
			input: func() canonical.Filament {
				f := baseFilament()
				f.Meta.Name = new("Overture PHA Rock White")
				f.Meta.Material = new(canonical.PHA)
				return f
			}(),
			target:    p2s06,
			wantWarns: []providers.Warning{{Path: "meta.material", Message: "PHA unsupported by orca, rendered as PLA"}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, warns, err := RenderFilament(c.input, c.target)
			if err != nil {
				t.Fatalf("RenderFilament error: %v", err)
			}

			// Loss policy: warnings are part of the contract — an unexpected
			// warning must fail just like a missing one.
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
