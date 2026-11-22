// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

import (
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

// The fixture machine. Written the way PrusaSlicer's own catalogue spells it;
// TestLookupThroughAlias in internal/catalog covers the Orca spelling.
var mini04 = canonical.MachineRef{Model: "Original Prusa MINI & MINI+", NozzleMM: 0.4}

func baseFilament() canonical.Filament {
	return canonical.Filament{
		Kind:          "filament",
		SchemaVersion: 1,
		Meta: canonical.Meta{
			Name:     new("Overture PLA Rock White"),
			Vendor:   new("Overture"),
			Material: new(canonical.PLA),
			ColorHex: new("B3B4B2"),
			Notes:    new("Dry before use.\nStore sealed."),
		},
		Physical: canonical.Physical{
			DiameterMM:  new(1.75),
			DensityGCm3: new(1.24),
			CostPerKg:   new(19.99),
			Abrasive:    new(false),
			ShrinkageXY: new(0.0),
			ShrinkageZ:  new(0.0),
		},
		Temps: canonical.Temps{
			Nozzle:           new(220.0),
			NozzleFirstLayer: new(225.0),
			Bed:              new(60.0),
			BedFirstLayer:    new(65.0),
		},
		Flow: canonical.Flow{
			MaxVolumetricMM3S: new(21.0),
			FlowRatio:         new(0.98),
		},
		Cooling: canonical.Cooling{
			FanMin:              new(0.3),
			FanMax:              new(1.0),
			FanOffFirstLayers:   new(1),
			FullFanAtLayer:      new(4),
			SlowDownLayerTimeS:  new(8.0),
			SlowDownMinSpeedMMS: new(20.0),
			BridgesFan:          new(0.8),
		},
		Gcode: canonical.Gcode{
			Start: new("M900 K0.02 ; set K\nG92 E0"),
			End:   new("; nothing to do"),
		},
		Compat: canonical.Compat{Machines: []canonical.MachineRef{mini04}},
	}
}

// The inversion that no single-provider test can catch. Prusa asks "how much
// does it shrink", so the canonical rate passes straight through; Orca asks
// "how big should it end up" and writes "100%" for the same zero.
func TestRenderShrink(t *testing.T) {
	cases := []struct {
		name string
		in   *float64
		want string
	}{
		{"absent omits the key", nil, ""},
		{"no shrink is 0%, NOT 100%", new(0.0), "0%"},
		{"2% shrink", new(0.02), "2%"},
		// Out of the documented [0,1] range, but Validate only WARNS, so it
		// reaches the renderer. Rounds away from zero like every other value:
		// the +0.5 trick this brick used to carry returned "-19%".
		{"negative rounds away from zero", new(-0.2), "-20%"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := renderShrink(c.in); got != c.want {
				t.Errorf("renderShrink = %q, want %q", got, c.want)
			}
		})
	}
}

// nil retraction means "inherit from the machine", spelled "nil" — the one
// convention both slicers share verbatim.
func TestRetractionNilInheritance(t *testing.T) {
	got, _, err := RenderFilament(baseFilament())
	if err != nil {
		t.Fatalf("RenderFilament: %v", err)
	}
	for name, field := range map[string][]string{
		"filament_retract_length":  got.FilamentRetractLength,
		"filament_retract_speed":   got.FilamentRetractSpeed,
		"filament_deretract_speed": got.FilamentDeretractSpeed,
		"filament_retract_lift":    got.FilamentRetractLift,
		"filament_wipe":            got.FilamentWipe,
	} {
		if !reflect.DeepEqual(field, []string{"nil"}) {
			t.Errorf("%s = %v, want [nil]", name, field)
		}
	}
}

func TestMapMaterialToPrusa(t *testing.T) {
	cases := []struct {
		in       canonical.Material
		want     string
		wantWarn bool
	}{
		{canonical.PLA, "PLA", false},
		{canonical.PETG, "PET", false}, // the vocabulary trap
		{canonical.ABS, "ABS", false},
		{canonical.ASA, "ASA", false},
		{canonical.TPU, "FLEX", false},
		{canonical.PC, "PC", false},
		{canonical.PA, "PA", false},
		{canonical.PHA, "PLA", true}, // not in prusa's list → warned, never silent
	}
	for _, c := range cases {
		t.Run(string(c.in), func(t *testing.T) {
			got, warn := mapMaterialToPrusa(c.in)
			if got != c.want {
				t.Errorf("mapMaterialToPrusa(%q) = %q, want %q", c.in, got, c.want)
			}
			if (warn != "") != c.wantWarn {
				t.Errorf("warning = %q, wantWarn=%v", warn, c.wantWarn)
			}
		})
	}
}

func TestRenderFilamentNameRequired(t *testing.T) {
	for _, name := range []*string{nil, new("")} {
		f := baseFilament()
		f.Meta.Name = name
		_, _, err := RenderFilament(f)
		if err == nil {
			t.Errorf("meta.name=%v must be a render error", name)
			continue
		}
		if !strings.Contains(err.Error(), "meta.name") {
			t.Errorf("error = %q, want it to name meta.name", err)
		}
	}
}

// compat is NOT required here, unlike Orca: Prusa renders ONE file whatever the
// targets, so an empty list is legal and means "compatible with every printer"
// (mapping.md pitfall #13).
func TestEmptyCompatIsLegal(t *testing.T) {
	f := baseFilament()
	f.Compat.Machines = nil
	got, _, err := RenderFilament(f)
	if err != nil {
		t.Fatalf("RenderFilament = %v, want nil: empty compat is legal for prusa", err)
	}
	if got.CompatiblePrinters != nil {
		t.Errorf("compatible_printers = %v, want omitted", got.CompatiblePrinters)
	}
	if got.CompatiblePrintersCondition != "" {
		t.Errorf("condition = %q, want empty: no target means no restriction", got.CompatiblePrintersCondition)
	}
}

// Prusa binds by expression, so the two facts that matter are WHICH id is
// compared (the catalogue's, never the human name the author wrote) and that
// the nozzle is tested per-extruder.
func TestCompatConditionShape(t *testing.T) {
	got, _, err := RenderFilament(baseFilament())
	if err != nil {
		t.Fatalf("RenderFilament: %v", err)
	}
	want := `printer_model=="MINI" and nozzle_diameter[0]==0.4`
	if got.CompatiblePrintersCondition != want {
		t.Errorf("condition = %q, want %q", got.CompatiblePrintersCondition, want)
	}
	// The list form stays empty, like 7511 of the 7528 system presets.
	if got.CompatiblePrinters != nil {
		t.Errorf("compatible_printers = %v, want omitted", got.CompatiblePrinters)
	}
}

// Several targets are alternatives. Each clause is parenthesised because `and`
// binds tighter than `or`: an unparenthesised join would mean something else.
func TestSeveralMachinesJoinWithOr(t *testing.T) {
	f := baseFilament()
	f.Compat.Machines = []canonical.MachineRef{mini04, {Model: "Original Prusa MK4S", NozzleMM: 0.6}}
	got, _, err := RenderFilament(f)
	if err != nil {
		t.Fatalf("RenderFilament: %v", err)
	}
	want := `(printer_model=="MINI" and nozzle_diameter[0]==0.4) or ` +
		`(printer_model=="MK4S" and nozzle_diameter[0]==0.6)`
	if got.CompatiblePrintersCondition != want {
		t.Errorf("condition = %q, want %q", got.CompatiblePrintersCondition, want)
	}
}

// An unresolvable machine stops the render. The alternative — a filament with
// no condition — imports fine and then shows up for every printer, which is the
// failure this whole slice exists to remove.
func TestUnresolvableMachineIsAnError(t *testing.T) {
	for _, ref := range []canonical.MachineRef{
		{Model: "", NozzleMM: 0.4},
		{Model: "Original Prusa MINI & MINI+"},
		{Model: "Bambu Lab P2S", NozzleMM: 0.6}, // real machine, absent from Prusa's catalogue
	} {
		f := baseFilament()
		f.Compat.Machines = []canonical.MachineRef{ref}
		if _, _, err := RenderFilament(f); err == nil {
			t.Errorf("%+v: want an error, got nil", ref)
		}
	}
}

// The catalogue reports a catalogue failure; this provider is what turns it
// into a canonical field path. Prusa folds every machine into ONE file, so the
// index is the only way to tell which entry of the list was wrong.
func TestErrorNamesTheOffendingEntry(t *testing.T) {
	f := baseFilament()
	f.Compat.Machines = []canonical.MachineRef{mini04, {Model: "Bambu Lab P2S", NozzleMM: 0.6}}

	_, _, err := RenderFilament(f)
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"compat.machines[1]", "Bambu Lab P2S", "absent from prusa"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	}
}

// A canonical field Prusa cannot express must surface, never vanish.
func TestLossesAreWarned(t *testing.T) {
	f := baseFilament()
	f.Temps.NozzleRange = new([2]float64{190, 230})
	f.Temps.Softening = new(60.0)
	f.Retraction.WipeDistanceMM = new(2.0)

	_, warns, err := RenderFilament(f)
	if err != nil {
		t.Fatalf("RenderFilament: %v", err)
	}
	var paths []string
	for _, w := range warns {
		paths = append(paths, w.Path)
	}
	for _, want := range []string{"temps.nozzle_range", "temps.softening", "retraction.wipe_distance_mm"} {
		if !slices.Contains(paths, want) {
			t.Errorf("missing warning for %s, got: %v", want, warns)
		}
	}
}

// overrides.orca never reaches a Prusa file, so Prusa's ingest cannot read it
// back: it must warn on its exact path, and only when set (ADR-0004).
func TestOrcaOverridesAreWarned(t *testing.T) {
	cases := []struct {
		name      string
		overrides map[string]map[string]any
		wantWarn  bool
	}{
		{"set", map[string]map[string]any{"orca": {"filament_id": "GFL99"}}, true},
		{"empty", map[string]map[string]any{"orca": {}}, false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := baseFilament()
			f.Overrides = c.overrides
			_, warns, err := RenderFilament(f)
			if err != nil {
				t.Fatalf("RenderFilament: %v", err)
			}
			got := slices.ContainsFunc(warns, func(w providers.Warning) bool { return w.Path == "overrides.orca" })
			if got != c.wantWarn {
				t.Errorf("warning on overrides.orca = %v, want %v (warnings: %v)", got, c.wantWarn, warns)
			}
		})
	}
}

func TestRenderFilament(t *testing.T) {
	cases := []struct {
		name  string
		input canonical.Filament
	}{
		{name: "pla-full", input: baseFilament()},
		{
			name: "petg-abrasive",
			input: func() canonical.Filament {
				f := baseFilament()
				f.Meta.Name = new("Overture PETG Rock White")
				f.Meta.Material = new(canonical.PETG)
				f.Physical.Abrasive = new(true)
				f.Physical.ShrinkageXY = new(0.02)
				f.Temps.Bed = new(80.0)
				return f
			}(),
		},
		{
			// Everything optional absent: proves omission, and that an authored
			// zero (bed: 0 = unheated) still renders.
			name: "minimal",
			input: canonical.Filament{
				Meta:  canonical.Meta{Name: new("Bare")},
				Temps: canonical.Temps{Bed: new(0.0)},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			settings, _, err := RenderFilament(c.input)
			if err != nil {
				t.Fatalf("RenderFilament: %v", err)
			}
			got, err := MarshalBundle("filament", *c.input.Meta.Name, settings)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			path := filepath.Join("testdata", "render", c.name+".golden.ini")
			if *update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update): %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}
