// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// ingest_roundtrip_test.go — the reverse king test (docs/adr/0001, Q8).
//
// ingest(render(canonical)) == canonical for the slice-B field set: the proof
// the reverse conversions are true inverses, not plausible-looking decoders.
// The forward king test (roundtrip_test.go) proved both providers agree on the
// wire; this one proves the wire reads back into the same canonical.
//
// Every field now round-trips: the ingest direction is a true inverse of render.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/thomsmoreau/Aldina/internal/canonical"
)

// renderToJSON runs the forward path and returns the exact bytes ingest sees.
func renderToJSON(t *testing.T, f canonical.Filament, target canonical.MachineRef) []byte {
	t.Helper()
	o, _, err := RenderFilament(f, target)
	if err != nil {
		t.Fatalf("RenderFilament: %v", err)
	}
	raw, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return raw
}

// ptrVal derefs a pointer for a readable failure message.
func ptrVal(v any) string {
	rv := reflect.ValueOf(v)
	if rv.IsValid() && rv.Kind() == reflect.Ptr && !rv.IsNil() {
		return fmt.Sprint(rv.Elem().Interface())
	}
	return fmt.Sprint(v)
}

func assertField(t *testing.T, field string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %s, want %s", field, ptrVal(got), ptrVal(want))
	}
}

func TestIngestRoundTrip(t *testing.T) {
	in := baseFilament()
	in.Physical.ShrinkageXY = new(0.02) // exercise the inverted-encoding inverse
	in.Physical.ShrinkageZ = new(0.015)
	in.Physical.Abrasive = new(true) // exercise the HRC-threshold inverse

	got, _, err := IngestFilament(renderToJSON(t, in, p2s06))
	if err != nil {
		t.Fatalf("IngestFilament: %v", err)
	}

	// Whole-group round-trips: every field in these groups is mapped in slice B.
	assertField(t, "physical", got.Physical, in.Physical)
	assertField(t, "temps", got.Temps, in.Temps)
	assertField(t, "flow", got.Flow, in.Flow)
	assertField(t, "cooling", got.Cooling, in.Cooling)
	assertField(t, "retraction", got.Retraction, in.Retraction)
	assertField(t, "gcode", got.Gcode, in.Gcode)

	// Meta: full round-trip now — name (suffix strip) and material (vocabulary)
	// map back exactly for the base fixture.
	assertField(t, "meta.vendor", got.Meta.Vendor, in.Meta.Vendor)
	assertField(t, "meta.color_hex", got.Meta.ColorHex, in.Meta.ColorHex)
	assertField(t, "meta.notes", got.Meta.Notes, in.Meta.Notes)
	assertField(t, "meta.name", got.Meta.Name, in.Meta.Name)
	assertField(t, "meta.material", got.Meta.Material, in.Meta.Material)

	assertField(t, "compat.machines", got.Compat.Machines, in.Compat.Machines)
}

// TestIngestDedupesCompatMachines: render declares one machine twice
// (catalogue preset + self-link); ingest reads back ONE reference, in the
// self-link's spelling, because that one is the author's.
func TestIngestDedupesCompatMachines(t *testing.T) {
	mini := canonical.MachineRef{Model: "Prusa MINI", NozzleMM: 0.4}
	prusaMini := canonical.MachineRef{Model: "Original Prusa MINI & MINI+", NozzleMM: 0.4}
	withPrinters := func(names ...string) []byte {
		var wire map[string]any
		if err := json.Unmarshal(renderToJSON(t, baseFilament(), p2s06), &wire); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		wire["compatible_printers"] = names
		raw, err := json.Marshal(wire)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		return raw
	}
	cases := []struct {
		name string
		raw  []byte
		want []canonical.MachineRef
	}{
		{"render, prusa spelling", renderToJSON(t, baseFilament(), prusaMini), []canonical.MachineRef{prusaMini}},
		{"render, orca spelling", renderToJSON(t, baseFilament(), mini), []canonical.MachineRef{mini}},
		{"self-link before its preset", withPrinters("Original Prusa MINI & MINI+ 0.4", "Prusa MINI 0.4 nozzle"), []canonical.MachineRef{prusaMini}},
		{"preset alone (orca-native file)", withPrinters("Prusa MINI 0.4 nozzle"), []canonical.MachineRef{mini}},
		{"two machines keep their order", withPrinters("Prusa MINI 0.6 nozzle", "Prusa MINI 0.4 nozzle", "Original Prusa MINI & MINI+ 0.4"),
			[]canonical.MachineRef{{Model: "Prusa MINI", NozzleMM: 0.6}, prusaMini}},
		{"self-link unknown to the catalogue", withPrinters("Homebrew Delta 0.4"), []canonical.MachineRef{{Model: "Homebrew Delta", NozzleMM: 0.4}}},
		{"known model, nozzle the catalogue lacks", withPrinters("Original Prusa MINI & MINI+ 0.9"), []canonical.MachineRef{{Model: "Original Prusa MINI & MINI+", NozzleMM: 0.9}}},
		{"no compatible printers", withPrinters(), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _, err := IngestFilament(c.raw)
			if err != nil {
				t.Fatalf("IngestFilament: %v", err)
			}
			assertField(t, "compat.machines", got.Compat.Machines, c.want)
		})
	}
}

// TestIngestRealWire ingests a REAL export, not render's simplified dialect.
// The Overture file has TWO extruder variants (nozzle ["255","240"]), so the
// first-element semantics of the collapse must hold, and the "nil" retraction
// token must map to a nil canonical field.
func TestIngestRealWire(t *testing.T) {
	got, _, err := IngestFilament(readIngestFixture(t, "self-contained.json"))
	if err != nil {
		t.Fatalf("IngestFilament: %v", err)
	}

	assertField(t, "temps.nozzle", got.Temps.Nozzle, new(255.0))
	assertField(t, "temps.nozzle_first_layer", got.Temps.NozzleFirstLayer, new(257.0))
	assertField(t, "temps.bed", got.Temps.Bed, new(80.0))
	assertField(t, "temps.softening", got.Temps.Softening, new(85.0))
	assertField(t, "temps.nozzle_range", got.Temps.NozzleRange, new([2]float64{220, 260}))
	assertField(t, "cooling.fan_min", got.Cooling.FanMin, new(0.30))
	assertField(t, "cooling.fan_max", got.Cooling.FanMax, new(0.50))
	assertField(t, "cooling.fan_off_first_layers", got.Cooling.FanOffFirstLayers, new(3))
	assertField(t, "cooling.slow_down_layer_time_s", got.Cooling.SlowDownLayerTimeS, new(1.0))
	assertField(t, "physical.density_g_cm3", got.Physical.DensityGCm3, new(1.27))
	assertField(t, "physical.shrinkage_xy", got.Physical.ShrinkageXY, new(0.0))
	assertField(t, "flow.flow_ratio", got.Flow.FlowRatio, new(0.955))
	assertField(t, "flow.max_volumetric_mm3_s", got.Flow.MaxVolumetricMM3S, new(25.0))
	assertField(t, "retraction.length_mm", got.Retraction.LengthMM, new(1.4))
	assertField(t, "retraction.z_hop_mm", got.Retraction.ZHopMM, new(0.4))
	assertField(t, "retraction.wipe", got.Retraction.Wipe, new(true))
	assertField(t, "meta.vendor", got.Meta.Vendor, new("Overture"))
	assertField(t, "meta.color_hex", got.Meta.ColorHex, new("B3B4B2"))
	// Name suffix strip + material vocabulary, on the real export.
	assertField(t, "meta.name", got.Meta.Name, new("Overture PETG Rock White"))
	assertField(t, "meta.material", got.Meta.Material, new(canonical.PETG))

	// "nil" = inherit from machine → canonical nil, not a parsed zero.
	assertField(t, "retraction.speed_mm_s", got.Retraction.SpeedMMS, (*float64)(nil))

	// "3" is below the hardened-steel threshold and ambiguous in real data → nil.
	assertField(t, "physical.abrasive", got.Physical.Abrasive, (*bool)(nil))

	// compatible_printers resolves back to one canonical machine reference.
	assertField(t, "compat.machines", got.Compat.Machines, []canonical.MachineRef{{Model: "Bambu Lab P2S", NozzleMM: 0.6}})
}

// TestParseOrcaAbrasive pins the one-way reverse: only the hardened-steel
// threshold (≥40) is a faithful abrasive signal. Lower values ("0", "3", "30")
// are ambiguous — Orca leaves "0" on abrasive filaments too — so they map to
// nil, never a confident false.
func TestParseOrcaAbrasive(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want *bool
	}{
		{"hardened steel", []string{"40"}, new(true)},
		{"zero is ambiguous", []string{"0"}, nil},
		{"three is ambiguous", []string{"3"}, nil},
		{"below hardened threshold", []string{"30"}, nil},
		{"inherit token", []string{"nil"}, nil},
		{"absent", nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := parseOrcaAbrasive(c.in)
			assertField(t, "abrasive", got, c.want)
		})
	}
}

// TestParseOrcaShrink pins the inverted decode on fractional percents, where
// float arithmetic leaks: (100-99.3)/100 in float64 is 0.007000000000000029,
// and that noise would land in the canonical YAML.
func TestParseOrcaShrink(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want *float64
	}{
		{"no shrink", []string{"100%"}, new(0.0)},
		{"whole percent", []string{"98%"}, new(0.02)},
		{"fractional percent is exact", []string{"99.3%"}, new(0.007)},
		{"fractional percent is exact (2)", []string{"97.2%"}, new(0.028)},
		{"expansion is negative", []string{"101.5%"}, new(-0.015)},
		{"inherit token", []string{"nil"}, nil},
		{"absent", nil, nil},
		{"unparseable", []string{"big%"}, nil},
		{"fraction is not a wire number", []string{"1/3%"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertField(t, "shrinkage", parseOrcaShrink(c.in), c.want)
		})
	}
}

// TestParseOrcaInt pins the layer counts as strict integers, the same policy
// as prusa.parsePrusaInt: a fractional value is unparseable (absent), never
// truncated into a different layer, so both dialects read one wire alike.
func TestParseOrcaInt(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want *int
	}{
		{"layer count", []string{"3"}, new(3)},
		{"authored zero is not absent", []string{"0"}, new(0)},
		{"negative kept, flagged by validate not ingest", []string{"-1"}, new(-1)},
		{"fractional is not truncated", []string{"2.5"}, nil},
		{"inherit token", []string{"nil"}, nil},
		{"empty", []string{""}, nil},
		{"absent", nil, nil},
		{"unparseable", []string{"many"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertField(t, "int", parseOrcaInt(c.in), c.want)
		})
	}
}
