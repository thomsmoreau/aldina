// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers_test

// roundtrip_test.go — THE king test (CONTRIBUTING.md §Tests).
//
// One canonical value, rendered to Orca AND Prusa, correct on both sides. It
// is the only test that can prove the canonical format is genuinely
// slicer-agnostic rather than a disguised mirror of whichever provider was
// written first — and the only one that catches an encoding INVERSION, where
// each provider is self-consistent and the pair is wrong.
//
// It lives in providers_test (external package) on purpose: importing both
// orca and prusa from either one would create the very coupling the
// architecture forbids (one-way providers → canonical, never sideways).

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/providers/orca"
	"github.com/thomsmoreau/Aldina/internal/providers/prusa"
)

// One of the ~74 machines both catalogues know, so a single canonical
// reference can be resolved on both sides. Spelled the PrusaSlicer way here;
// the Orca catalogue reaches it through the alias table.
var mini = canonical.MachineRef{Model: "Original Prusa MINI & MINI+", NozzleMM: 0.4}

// parseINI turns INI bytes into a key → value map, dropping section headers
// and comments. The separator is " = " exactly (see prusa/ini.go).
func parseINI(raw []byte) map[string]string {
	ini := make(map[string]string)
	for _, line := range strings.Split(string(raw), "\n") {
		if k, v, ok := strings.Cut(line, " = "); ok {
			ini[k] = v
		}
	}
	return ini
}

// render returns the two wire forms of one canonical filament.
func render(t *testing.T, f canonical.Filament) (orca.FilamentSettings, map[string]string) {
	t.Helper()

	orcaOut, _, err := orca.RenderFilament(f, mini)
	if err != nil {
		t.Fatalf("orca.RenderFilament: %v", err)
	}
	prusaOut, _, err := prusa.RenderFilament(f)
	if err != nil {
		t.Fatalf("prusa.RenderFilament: %v", err)
	}
	raw, err := prusa.MarshalBundle("filament", "Round Trip", prusaOut)
	if err != nil {
		t.Fatalf("prusa.Marshal: %v", err)
	}
	return orcaOut, parseINI(raw)
}

func base() canonical.Filament {
	return canonical.Filament{
		Meta:   canonical.Meta{Name: new("Round Trip"), Material: new(canonical.PETG)},
		Temps:  canonical.Temps{Nozzle: new(240.0), Bed: new(80.0)},
		Compat: canonical.Compat{Machines: []canonical.MachineRef{mini}},
	}
}

// miniOrcaCompat is the exact compatible_printers the MINI fixture yields in
// Orca: the catalogue preset name, then the self-link name.
func miniOrcaCompat() []string {
	return []string{"Prusa MINI 0.4 nozzle", "Original Prusa MINI & MINI+ 0.4"}
}

// The linkage counterpart of the shrinkage case: ONE canonical machine
// reference, two bindings with nothing in common on the wire — Orca names the
// preset, Prusa compares an id it never shows the author — and both must land
// on the same physical printer.
//
// No single-provider test can prove this. Each side is self-consistent while
// carrying the other's vocabulary: that is exactly how a filament ended up
// declaring `compatible_printers = Bambu Lab P2S 0.6 nozzle` in a PrusaSlicer
// file, importing cleanly and then never appearing.
func TestOneMachineReferenceBindsOnBothSides(t *testing.T) {
	orcaOut, prusaINI := render(t, base())

	// Orca: its OWN catalogue name for that machine — not the spelling the
	// author used, which is PrusaSlicer's. The alias table bridged them. The
	// second entry is the self-link name (mapping.md, machine pitfall #1).
	if want := miniOrcaCompat(); !reflect.DeepEqual(orcaOut.CompatiblePrinters, want) {
		t.Errorf("orca compatible_printers = %v, want %v", orcaOut.CompatiblePrinters, want)
	}
	// Prusa: an expression over the printer's own fields, and NO name list.
	if got := prusaINI["compatible_printers_condition"]; got != `printer_model=="MINI" and nozzle_diameter[0]==0.4` {
		t.Errorf("prusa compatible_printers_condition = %q", got)
	}
	if got, ok := prusaINI["compatible_printers"]; ok && got != "" {
		t.Errorf("prusa compatible_printers = %q, want omitted", got)
	}
	// And the proof the two are not the same string in disguise: neither side
	// carries the other's spelling.
	if strings.Contains(prusaINI["compatible_printers_condition"], "MINI &") {
		t.Error("prusa condition leaked the human model name instead of the id")
	}
}

// The bug this slice removes, stated as a test: an Orca catalogue name must not
// be renderable into a Prusa file. It is not a Prusa machine, so it stops.
func TestOrcaOnlyMachineCannotRenderToPrusa(t *testing.T) {
	f := base()
	f.Compat.Machines = []canonical.MachineRef{{Model: "Bambu Lab P2S", NozzleMM: 0.6}}

	if _, _, err := orca.RenderFilament(f, f.Compat.Machines[0]); err != nil {
		t.Fatalf("orca knows this machine: %v", err)
	}
	_, _, err := prusa.RenderFilament(f)
	if err == nil {
		t.Fatal("prusa has no Bambu machine: want an error, got a preset that would import and never show up")
	}
	if !strings.Contains(err.Error(), "Bambu Lab P2S") {
		t.Errorf("error = %q, want it to name the machine", err)
	}
}

// ---------------------------------------------------------------------------
// Machine ↔ filament linkage
// ---------------------------------------------------------------------------

// baseMachine names the SAME machine as base(): the pair is what an author
// actually ships, and the whole point is that the two halves must meet.
func baseMachine() canonical.Machine {
	return canonical.Machine{
		Meta: canonical.MachineMeta{
			Name:  new("Round Trip MINI"),
			Model: new(mini.Model),
		},
		Nozzle: canonical.NozzleSpec{DiameterMM: new(mini.NozzleMM)},
		Volume: canonical.BuildVolume{WidthMM: new(180.0), DepthMM: new(180.0), HeightMM: new(180.0)},
	}
}

func renderMachine(t *testing.T, m canonical.Machine) (orca.MachineSettings, map[string]string) {
	t.Helper()

	orcaOut, _, err := orca.RenderMachine(m, "")
	if err != nil {
		t.Fatalf("orca.RenderMachine: %v", err)
	}
	prusaOut, _, err := prusa.RenderMachine(m, "")
	if err != nil {
		t.Fatalf("prusa.RenderMachine: %v", err)
	}
	raw, err := prusa.MarshalBundle("printer", *m.Meta.Name, prusaOut)
	if err != nil {
		t.Fatalf("prusa.MarshalBundle: %v", err)
	}
	return orcaOut, parseINI(raw)
}

// THE case for this slice, and the one no single-provider test can see: the
// filament and the machine are rendered independently, from two canonical
// files, by code paths that never meet — and the expression one emits must
// address the identity the other emits.
//
// This is the exact bug that made a rendered preset import cleanly and stay
// invisible: a condition comparing an id nothing in the bundle carried. It is
// pinned by REBUILDING the condition out of the machine's own rendered values,
// so the two can only agree by actually agreeing.
func TestFilamentConditionAddressesTheRenderedMachine(t *testing.T) {
	_, machineINI := renderMachine(t, baseMachine())
	_, filamentINI := render(t, base())

	want := fmt.Sprintf("printer_model==%q and nozzle_diameter[0]==%s",
		machineINI["printer_model"], machineINI["nozzle_diameter"])
	if got := filamentINI["compatible_printers_condition"]; got != want {
		t.Errorf("the filament does not address the machine we render\n got: %s\nwant: %s", got, want)
	}
	// And the identity is the catalogue id, not the human label the author
	// wrote — otherwise the two would agree on a value PrusaSlicer never tests.
	if machineINI["printer_model"] != "MINI" {
		t.Errorf("machine printer_model = %q, want the catalogue id", machineINI["printer_model"])
	}
}

// The Orca counterpart, pinned as it actually behaves — and it does NOT match
// the Prusa side. Two distinct reasons, only one of which is by design:
//
//  1. By design: Orca binds a filament by exact PRESET NAME, so the filament
//     targets the machine's catalogue preset ("Prusa MINI 0.4 nozzle") while
//     the machine we render carries the author's own label. The linkage is
//     per-format (docs/mapping.md pitfall #1) and only the canonical key —
//     model + nozzle — is shared.
//  2. Debt: orca.RenderMachine emits meta.model VERBATIM, so a canonical
//     written with PrusaSlicer's spelling produces an Orca machine whose
//     printer_model is not Orca's own catalogue name. prusa.RenderMachine
//     resolves the same field through the catalogue;
//     changing Orca is its own slice, not this one.
//
// This test exists so that divergence is a written fact rather than a surprise.
func TestOrcaMachineModelIsNotCatalogueResolved(t *testing.T) {
	orcaMachine, prusaMachineINI := renderMachine(t, baseMachine())
	orcaFilament, _ := render(t, base())

	if orcaMachine.Name != "Round Trip MINI" {
		t.Errorf("orca machine name = %q, want the author's label", orcaMachine.Name)
	}
	if want := miniOrcaCompat(); !reflect.DeepEqual(orcaFilament.CompatiblePrinters, want) {
		t.Fatalf("orca compatible_printers = %v, want %v", orcaFilament.CompatiblePrinters, want)
	}
	if orcaFilament.CompatiblePrinters[0] == orcaMachine.Name {
		t.Error("if these ever match, the linkage model changed — revisit mapping.md pitfall #1")
	}
	// The self-link name is the MODEL string, not the author's label: a machine
	// titled anything but "<model> <nozzle>" is not self-linked yet.
	if orcaFilament.CompatiblePrinters[1] == orcaMachine.Name {
		t.Error("self-link name matched the label — the naming model changed, revisit mapping.md")
	}

	// The debt, stated as an assertion: same canonical field, two policies.
	if orcaMachine.PrinterModel != "Original Prusa MINI & MINI+" {
		t.Errorf("orca printer_model = %q, want meta.model verbatim", orcaMachine.PrinterModel)
	}
	if prusaMachineINI["printer_model"] != "MINI" {
		t.Errorf("prusa printer_model = %q, want the catalogue id", prusaMachineINI["printer_model"])
	}
}

// ---------------------------------------------------------------------------
// Migration: prusa → canonical → orca
// ---------------------------------------------------------------------------

// The migration direction of the king test: values authored in PrusaSlicer,
// read by prusa.Ingest and rendered by orca, must land with the same number
// on the Orca side. Values are distinct per key, so a crossed wire (first
// layer into the base field, bed into nozzle) cannot pass by coincidence, and
// a decimal flow ratio catches any formatting drift.
func TestPrusaIngestRendersToOrca(t *testing.T) {
	bundle := "[filament:Migrated]\n" +
		"filament_type = PET\n" +
		"temperature = 235\n" +
		"first_layer_temperature = 240\n" +
		"bed_temperature = 85\n" +
		"first_layer_bed_temperature = 90\n" +
		"extrusion_multiplier = 0.95\n" +
		"filament_max_volumetric_speed = 11.5\n" +
		"filament_diameter = 2.85\n" +
		"filament_density = 1.24\n" +
		"filament_cost = 27.5\n" +
		"filament_abrasive = 1\n" +
		"filament_shrinkage_compensation_z = 2%\n" +
		"min_fan_speed = 35\n" +
		"max_fan_speed = 90\n" +
		"bridge_fan_speed = 65\n" +
		"disable_fan_first_layers = 2\n" +
		"full_fan_speed_layer = 4\n" +
		"slowdown_below_layer_time = 12.5\n" +
		"min_print_speed = 17\n" +
		"filament_wipe = 0\n" +
		"filament_vendor = Forward AM\n" +
		"filament_colour = #F7F7F7\n" +
		"filament_notes = \"Line one\\nLine two\"\n" +
		"start_filament_gcode = \"M900 K{if nozzle_diameter[0]==0.6}0.12{else}0.2{endif}\\nM572 S0.36\"\n" +
		"end_filament_gcode = \"; Filament-specific end gcode\"\n"
	results, err := prusa.Ingest([]byte(bundle))
	if err != nil || len(results) != 1 || results[0].Err != nil {
		t.Fatalf("prusa.Ingest: err=%v results=%+v", err, results)
	}
	o, _, err := orca.RenderFilament(*results[0].Filament, mini)
	if err != nil {
		t.Fatalf("orca.RenderFilament: %v", err)
	}

	for _, c := range []struct {
		key  string
		got  []string
		want string
	}{
		{"filament_type", o.FilamentType, "PETG"}, // one material, two vocabularies
		{"nozzle_temperature", o.NozzleTemperature, "235"},
		{"nozzle_temperature_initial_layer", o.NozzleTemperatureInitialLayer, "240"},
		{"hot_plate_temp", o.HotPlateTemp, "85"},
		{"hot_plate_temp_initial_layer", o.HotPlateTempInitialLayer, "90"},
		{"textured_plate_temp", o.TexturedPlateTemp, "85"},
		{"textured_plate_temp_initial_layer", o.TexturedPlateTempInitialLayer, "90"},
		{"filament_flow_ratio", o.FilamentFlowRatio, "0.95"},
		{"filament_max_volumetric_speed", o.FilamentMaxVolumetricSpeed, "11.5"},
		{"filament_diameter", o.FilamentDiameter, "2.85"},
		{"filament_density", o.FilamentDensity, "1.24"},
		{"filament_cost", o.FilamentCost, "27.5"},
		{"required_nozzle_HRC", o.RequiredNozzleHRC, "40"},                             // abrasive → the hardened-steel threshold
		{"filament_shrinkage_compensation_z", o.FilamentShrinkageCompensationZ, "98%"}, // same key, inverted meaning
		{"fan_min_speed", o.FanMinSpeed, "35"},                                         // word order swaps, the percent survives the ratio hop
		{"fan_max_speed", o.FanMaxSpeed, "90"},
		{"overhang_fan_speed", o.OverhangFanSpeed, "65"},
		{"close_fan_the_first_x_layers", o.CloseFanTheFirstXLayers, "2"},
		{"full_fan_speed_layer", o.FullFanSpeedLayer, "4"},
		{"slow_down_layer_time", o.SlowDownLayerTime, "12.5"},
		{"slow_down_min_speed", o.SlowDownMinSpeed, "17"},
		{"filament_wipe", o.FilamentWipe, "0"}, // authored off, never mistaken for the "nil" inherit
		{"filament_vendor", o.FilamentVendor, "Forward AM"},
		{"default_filament_colour", o.DefaultFilamentColour, "#F7F7F7"}, // bare in canonical, prefixed again on the wire
		{"filament_notes", o.FilamentNotes, "Line one\nLine two"},
		{"filament_start_gcode", o.FilamentStartGcode, "M900 K{if nozzle_diameter[0]==0.6}0.12{else}0.2{endif}\nM572 S0.36"}, // opaque: Prusa macros travel as-is
		{"filament_end_gcode", o.FilamentEndGcode, "; Filament-specific end gcode"},
	} {
		if len(c.got) != 1 || c.got[0] != c.want {
			t.Errorf("orca %s = %v, want [%s]", c.key, c.got, c.want)
		}
	}
}

// Spec story 8: a retraction override left to inherit in PrusaSlicer must
// still inherit in Orca. Ingest reads "nil" as canonical nil and the Orca
// render spells nil back as "nil" — a machine default never gets frozen into
// the filament. The authored neighbours prove the family is not simply
// dropped.
func TestPrusaRetractionInheritSurvivesToOrca(t *testing.T) {
	bundle := "[filament:Retract]\n" +
		"filament_retract_length = nil\n" +
		"filament_retract_lift = nil\n" +
		"filament_wipe = nil\n" +
		"filament_retract_speed = 40\n" +
		"filament_deretract_speed = 25\n"
	results, err := prusa.Ingest([]byte(bundle))
	if err != nil || len(results) != 1 || results[0].Err != nil {
		t.Fatalf("prusa.Ingest: err=%v results=%+v", err, results)
	}
	o, _, err := orca.RenderFilament(*results[0].Filament, mini)
	if err != nil {
		t.Fatalf("orca.RenderFilament: %v", err)
	}
	for _, c := range []struct {
		key  string
		got  []string
		want string
	}{
		{"filament_retraction_length", o.FilamentRetractionLength, "nil"},
		{"filament_z_hop", o.FilamentZHop, "nil"},
		{"filament_wipe", o.FilamentWipe, "nil"},
		{"filament_retraction_speed", o.FilamentRetractionSpeed, "40"},
		{"filament_deretraction_speed", o.FilamentDeretractionSpeed, "25"},
	} {
		if len(c.got) != 1 || c.got[0] != c.want {
			t.Errorf("orca %s = %v, want [%s]", c.key, c.got, c.want)
		}
	}
}

// Pitfall #14 (docs/mapping.md) end to end: the same shrink intent crosses
// from Prusa's rate spelling to Orca's inverted one through canonical.
func TestPrusaShrinkIngestRendersToOrca(t *testing.T) {
	for _, c := range []struct {
		prusa string
		rate  float64
		orca  string
	}{
		{"2%", 0.02, "98%"},
		{"0%", 0.0, "100%"},
	} {
		t.Run(c.prusa, func(t *testing.T) {
			bundle := "[filament:Shrink]\nfilament_shrinkage_compensation_xy = " + c.prusa + "\n"
			results, err := prusa.Ingest([]byte(bundle))
			if err != nil || len(results) != 1 || results[0].Err != nil {
				t.Fatalf("prusa.Ingest: err=%v results=%+v", err, results)
			}
			got := results[0].Filament.Physical.ShrinkageXY
			if got == nil {
				t.Fatalf("canonical shrinkage_xy = nil, want %v", c.rate)
			}
			if *got != c.rate {
				t.Fatalf("canonical shrinkage_xy = %v, want %v", *got, c.rate)
			}
			o, _, err := orca.RenderFilament(*results[0].Filament, mini)
			if err != nil {
				t.Fatalf("orca.RenderFilament: %v", err)
			}
			if len(o.FilamentShrink) != 1 || o.FilamentShrink[0] != c.orca {
				t.Errorf("orca filament_shrink = %v, want [%s]", o.FilamentShrink, c.orca)
			}
		})
	}
}

// A shrink rate renders on both sides without rounding to a whole percent:
// both wire keys are float percents (coPercents in each slicer's
// PrintConfig.cpp), so "1.5%" is expressible and rounding it to "2%" would be
// a silent loss. Expected strings are clean decimals: float noise from the
// ×100 or the Orca inversion (1−0.07 = 0.9299999999999999) must not leak.
func TestShrinkRendersExactOnBoth(t *testing.T) {
	for _, c := range []struct {
		rate  float64
		prusa string
		orca  string
	}{
		{0.0, "0%", "100%"},
		{0.02, "2%", "98%"},
		{0.07, "7%", "93%"},
		{0.015, "1.5%", "98.5%"},
		{0.007, "0.7%", "99.3%"},
		{-0.005, "-0.5%", "100.5%"},
	} {
		t.Run(fmt.Sprint(c.rate), func(t *testing.T) {
			f := base()
			f.Physical.ShrinkageXY = new(c.rate)
			o, p := render(t, f)
			if got := p["filament_shrinkage_compensation_xy"]; got != c.prusa {
				t.Errorf("prusa filament_shrinkage_compensation_xy = %q, want %q", got, c.prusa)
			}
			if len(o.FilamentShrink) != 1 || o.FilamentShrink[0] != c.orca {
				t.Errorf("orca filament_shrink = %v, want [%s]", o.FilamentShrink, c.orca)
			}
		})
	}
}

// Shrinkage Z shares one key name across the two slicers with opposite
// meanings (docs/mapping.md pitfall #14): Prusa's rate, Orca's size.
func TestShrinkZRendersInvertedOnOrca(t *testing.T) {
	f := base()
	f.Physical.ShrinkageZ = new(0.015)
	o, p := render(t, f)
	if got := p["filament_shrinkage_compensation_z"]; got != "1.5%" {
		t.Errorf("prusa filament_shrinkage_compensation_z = %q, want %q", got, "1.5%")
	}
	if len(o.FilamentShrinkageCompensationZ) != 1 || o.FilamentShrinkageCompensationZ[0] != "98.5%" {
		t.Errorf("orca filament_shrinkage_compensation_z = %v, want [98.5%%]", o.FilamentShrinkageCompensationZ)
	}
}

// Ingest agnosticism: one cooling intent, spelled in each slicer's own
// dialect (swapped word order, bridge vs overhang, renamed layer keys), must
// land as the SAME canonical from both ingests. The render-side king test
// cannot see a decode that drifts on one side only.
func TestCoolingIngestAgreesAcrossProviders(t *testing.T) {
	orcaJSON := `{"name": "Cool",
		"fan_min_speed": ["35"], "fan_max_speed": ["90"], "overhang_fan_speed": ["65"],
		"close_fan_the_first_x_layers": ["2"], "full_fan_speed_layer": ["4"],
		"slow_down_layer_time": ["12.5"], "slow_down_min_speed": ["17"]}`
	prusaINI := "[filament:Cool]\n" +
		"min_fan_speed = 35\n" +
		"max_fan_speed = 90\n" +
		"bridge_fan_speed = 65\n" +
		"disable_fan_first_layers = 2\n" +
		"full_fan_speed_layer = 4\n" +
		"slowdown_below_layer_time = 12.5\n" +
		"min_print_speed = 17\n"

	fromOrca, _, err := orca.IngestFilament([]byte(orcaJSON))
	if err != nil {
		t.Fatalf("orca.IngestFilament: %v", err)
	}
	results, err := prusa.Ingest([]byte(prusaINI))
	if err != nil || len(results) != 1 || results[0].Err != nil {
		t.Fatalf("prusa.Ingest: err=%v results=%+v", err, results)
	}
	fromPrusa := results[0].Filament

	want := canonical.Cooling{
		FanMin:              new(0.35),
		FanMax:              new(0.90),
		FanOffFirstLayers:   new(2),
		FullFanAtLayer:      new(4),
		SlowDownLayerTimeS:  new(12.5),
		SlowDownMinSpeedMMS: new(17.0),
		BridgesFan:          new(0.65),
	}
	if !reflect.DeepEqual(fromOrca.Cooling, want) {
		t.Errorf("orca cooling = %s, want %s", coolingString(fromOrca.Cooling), coolingString(want))
	}
	if !reflect.DeepEqual(fromPrusa.Cooling, want) {
		t.Errorf("prusa cooling = %s, want %s", coolingString(fromPrusa.Cooling), coolingString(want))
	}
}

// coolingString derefs the pointers so a mismatch prints values, not addresses.
func coolingString(c canonical.Cooling) string {
	d := func(p any) string {
		switch v := p.(type) {
		case *float64:
			if v != nil {
				return fmt.Sprint(*v)
			}
		case *int:
			if v != nil {
				return fmt.Sprint(*v)
			}
		}
		return "nil"
	}
	return fmt.Sprintf("{min %s max %s off %s full %s time %s speed %s bridge %s}",
		d(c.FanMin), d(c.FanMax), d(c.FanOffFirstLayers), d(c.FullFanAtLayer),
		d(c.SlowDownLayerTimeS), d(c.SlowDownMinSpeedMMS), d(c.BridgesFan))
}
