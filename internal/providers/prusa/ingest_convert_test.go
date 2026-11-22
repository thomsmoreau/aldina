// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// ingest_convert_test.go — the per-field wire → canonical conversions, one
// table per family, every case through the public Ingest seam.

import (
	"strings"
	"testing"

	"github.com/thomsmoreau/Aldina/internal/canonical"
)

// ingestOne runs a one-preset bundle through Ingest and returns its filament.
func ingestOne(t *testing.T, body string) *canonical.Filament {
	t.Helper()
	results, err := Ingest([]byte("[filament:T]\n" + body))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil || results[0].Filament == nil {
		t.Fatalf("results = %+v, want one clean filament", results)
	}
	return results[0].Filament
}

// Temperatures, flow, the physical floats, the cooling slowdown and the
// retraction overrides share one wire shape: a bare float string, one wire
// key → one canonical field, "nil" and "" absent.
func TestIngestScalarFloats(t *testing.T) {
	fields := []struct {
		key string
		get func(*canonical.Filament) *float64
	}{
		{"temperature", func(f *canonical.Filament) *float64 { return f.Temps.Nozzle }},
		{"first_layer_temperature", func(f *canonical.Filament) *float64 { return f.Temps.NozzleFirstLayer }},
		{"bed_temperature", func(f *canonical.Filament) *float64 { return f.Temps.Bed }},
		{"first_layer_bed_temperature", func(f *canonical.Filament) *float64 { return f.Temps.BedFirstLayer }},
		{"extrusion_multiplier", func(f *canonical.Filament) *float64 { return f.Flow.FlowRatio }},
		{"filament_max_volumetric_speed", func(f *canonical.Filament) *float64 { return f.Flow.MaxVolumetricMM3S }},
		{"filament_diameter", func(f *canonical.Filament) *float64 { return f.Physical.DiameterMM }},
		{"filament_density", func(f *canonical.Filament) *float64 { return f.Physical.DensityGCm3 }},
		{"filament_cost", func(f *canonical.Filament) *float64 { return f.Physical.CostPerKg }},
		{"slowdown_below_layer_time", func(f *canonical.Filament) *float64 { return f.Cooling.SlowDownLayerTimeS }},
		{"min_print_speed", func(f *canonical.Filament) *float64 { return f.Cooling.SlowDownMinSpeedMMS }},
		{"filament_retract_length", func(f *canonical.Filament) *float64 { return f.Retraction.LengthMM }},
		{"filament_retract_speed", func(f *canonical.Filament) *float64 { return f.Retraction.SpeedMMS }},
		{"filament_deretract_speed", func(f *canonical.Filament) *float64 { return f.Retraction.DeretractSpeedMMS }},
		{"filament_retract_lift", func(f *canonical.Filament) *float64 { return f.Retraction.ZHopMM }},
	}
	cases := []struct {
		name string
		line string // "" = key absent from the section
		want *float64
	}{
		{"value", " = 215", new(215.0)},
		{"decimal kept exact", " = 0.95", new(0.95)},
		{"authored zero is not absent", " = 0", new(0.0)},
		{"absent", "", nil},
		{"nil inherits", " = nil", nil},
		{"empty", " = ", nil}, // the real export form: separator, no value
		{"unparseable", " = hot", nil},
	}
	for _, fld := range fields {
		for _, c := range cases {
			t.Run(fld.key+"/"+c.name, func(t *testing.T) {
				body := ""
				if c.line != "" {
					body = fld.key + c.line + "\n"
				}
				got := fld.get(ingestOne(t, body))
				switch {
				case c.want == nil && got != nil:
					t.Errorf("got %v, want nil (absent)", *got)
				case c.want != nil && got == nil:
					t.Errorf("got nil, want %v", *c.want)
				case c.want != nil && *got != *c.want:
					t.Errorf("got %v, want %v", *got, *c.want)
				}
			})
		}
	}
}

// Prusa spells abrasive as a plain "1"/"0" bool — the faithful reverse of the
// render, unlike Orca's HRC threshold. An authored "0" is a known-not-abrasive
// intent and must stay distinct from absent.
func TestIngestAbrasive(t *testing.T) {
	cases := []struct {
		name string
		line string // "" = key absent from the section
		want *bool
	}{
		{"true", "filament_abrasive = 1\n", new(true)},
		{"authored false is not absent", "filament_abrasive = 0\n", new(false)},
		{"absent", "", nil},
		{"nil inherits", "filament_abrasive = nil\n", nil},
		{"empty", "filament_abrasive = \n", nil},
		{"unparseable", "filament_abrasive = yes\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ingestOne(t, c.line).Physical.Abrasive
			switch {
			case c.want == nil && got != nil:
				t.Errorf("got %v, want nil (absent)", *got)
			case c.want != nil && got == nil:
				t.Errorf("got nil, want %v", *c.want)
			case c.want != nil && *got != *c.want:
				t.Errorf("got %v, want %v", *got, *c.want)
			}
		})
	}
}

// filament_wipe is a retraction override: "nil" is inherit from the machine,
// and an authored "0" (wipe off) must stay distinct from it.
func TestIngestWipe(t *testing.T) {
	cases := []struct {
		name string
		line string // "" = key absent from the section
		want *bool
	}{
		{"true", "filament_wipe = 1\n", new(true)},
		{"authored false is not inherit", "filament_wipe = 0\n", new(false)},
		{"absent", "", nil},
		{"nil inherits", "filament_wipe = nil\n", nil},
		{"unparseable", "filament_wipe = yes\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ingestOne(t, c.line).Retraction.Wipe
			switch {
			case c.want == nil && got != nil:
				t.Errorf("got %v, want nil (inherit)", *got)
			case c.want != nil && got == nil:
				t.Errorf("got nil, want %v", *c.want)
			case c.want != nil && *got != *c.want:
				t.Errorf("got %v, want %v", *got, *c.want)
			}
		})
	}
}

// Only filament_retract_* carries filament intent (spec Q3). Bare retract_*
// are print-settings defaults and *_toolchange keys are multi-material
// knobs: a value there must never reach canonical retraction, whatever its
// name resembles.
func TestIngestRetractionAuthority(t *testing.T) {
	f := ingestOne(t, "retract_length = 3\n"+
		"retract_speed = 40\n"+
		"deretract_speed = 30\n"+
		"retract_lift = 0.4\n"+
		"wipe = 1\n"+
		"filament_retract_length_toolchange = 5\n"+
		"filament_retract_restart_extra_toolchange = 1\n")
	if f.Retraction != (canonical.Retraction{}) {
		t.Errorf("retraction = %+v, want all nil", f.Retraction)
	}
}

// Prusa shrinkage is a straight rate (docs/mapping.md pitfall #14): an Orca
// inverted decode leaking in would read "2%" as 0.98.
func TestIngestShrinkage(t *testing.T) {
	fields := []struct {
		key string
		get func(*canonical.Filament) *float64
	}{
		{"filament_shrinkage_compensation_xy", func(f *canonical.Filament) *float64 { return f.Physical.ShrinkageXY }},
		{"filament_shrinkage_compensation_z", func(f *canonical.Filament) *float64 { return f.Physical.ShrinkageZ }},
	}
	cases := []struct {
		name string
		line string // "" = key absent from the section
		want *float64
	}{
		{"rate maps directly", " = 2%", new(0.02)},
		{"authored zero is not absent", " = 0%", new(0.0)},
		{"fractional percent kept, not rounded", " = 1.5%", new(0.015)},
		{"decimal shift is exact, no float noise", " = 0.7%", new(0.007)}, // 0.7/100 = 0.006999999999999999
		{"negative (expansion) kept", " = -0.5%", new(-0.005)},
		{"absent", "", nil},
		{"nil inherits", " = nil", nil},
		{"empty", " = ", nil},
		{"unparseable", " = lots%", nil},
	}
	for _, fld := range fields {
		for _, c := range cases {
			t.Run(fld.key+"/"+c.name, func(t *testing.T) {
				body := ""
				if c.line != "" {
					body = fld.key + c.line + "\n"
				}
				got := fld.get(ingestOne(t, body))
				switch {
				case c.want == nil && got != nil:
					t.Errorf("got %v, want nil (absent)", *got)
				case c.want != nil && got == nil:
					t.Errorf("got nil, want %v", *c.want)
				case c.want != nil && *got != *c.want:
					t.Errorf("got %v, want %v", *got, *c.want)
				}
			})
		}
	}
}

// Prusa fan speeds are whole percents (ConfigOptionInts) → canonical 0–1
// ratios. An authored "0" is a fan-off intent and must stay distinct from
// absent; a "%" suffix is not the Prusa spelling and reads as unparseable.
func TestIngestFanRatios(t *testing.T) {
	fields := []struct {
		key string
		get func(*canonical.Filament) *float64
	}{
		{"min_fan_speed", func(f *canonical.Filament) *float64 { return f.Cooling.FanMin }},
		{"max_fan_speed", func(f *canonical.Filament) *float64 { return f.Cooling.FanMax }},
		{"bridge_fan_speed", func(f *canonical.Filament) *float64 { return f.Cooling.BridgesFan }},
	}
	cases := []struct {
		name string
		line string // "" = key absent from the section
		want *float64
	}{
		{"percent to ratio", " = 30", new(0.30)},
		{"full speed", " = 100", new(1.0)},
		{"authored zero is not absent", " = 0", new(0.0)},
		{"decimal shift is exact, no float noise", " = 7", new(0.07)},
		{"hand-edited fractional percent kept, not rounded", " = 30.5", new(0.305)},
		{"negative kept, flagged by validate not ingest", " = -10", new(-0.10)},
		{"absent", "", nil},
		{"nil inherits", " = nil", nil},
		{"empty", " = ", nil},
		{"unparseable", " = fast", nil},
		{"percent sign is not the prusa spelling", " = 30%", nil},
	}
	for _, fld := range fields {
		for _, c := range cases {
			t.Run(fld.key+"/"+c.name, func(t *testing.T) {
				body := ""
				if c.line != "" {
					body = fld.key + c.line + "\n"
				}
				got := fld.get(ingestOne(t, body))
				switch {
				case c.want == nil && got != nil:
					t.Errorf("got %v, want nil (absent)", *got)
				case c.want != nil && got == nil:
					t.Errorf("got nil, want %v", *c.want)
				case c.want != nil && *got != *c.want:
					t.Errorf("got %v, want %v", *got, *c.want)
				}
			})
		}
	}
}

// The cooling layer counts are ConfigOptionInts. A fractional value is not
// truncated into a different layer: it reads as unparseable, so the loss
// stays visible instead of silently shifting the intent.
func TestIngestCoolingLayers(t *testing.T) {
	fields := []struct {
		key string
		get func(*canonical.Filament) *int
	}{
		{"disable_fan_first_layers", func(f *canonical.Filament) *int { return f.Cooling.FanOffFirstLayers }},
		{"full_fan_speed_layer", func(f *canonical.Filament) *int { return f.Cooling.FullFanAtLayer }},
	}
	cases := []struct {
		name string
		line string // "" = key absent from the section
		want *int
	}{
		{"layer count", " = 3", new(3)},
		{"authored zero is not absent", " = 0", new(0)},
		{"absent", "", nil},
		{"nil inherits", " = nil", nil},
		{"empty", " = ", nil},
		{"unparseable", " = many", nil},
		{"fractional is not truncated", " = 2.5", nil},
		{"negative kept, flagged by validate not ingest", " = -1", new(-1)},
	}
	for _, fld := range fields {
		for _, c := range cases {
			t.Run(fld.key+"/"+c.name, func(t *testing.T) {
				body := ""
				if c.line != "" {
					body = fld.key + c.line + "\n"
				}
				got := fld.get(ingestOne(t, body))
				switch {
				case c.want == nil && got != nil:
					t.Errorf("got %v, want nil (absent)", *got)
				case c.want != nil && got == nil:
					t.Errorf("got nil, want %v", *c.want)
				case c.want != nil && *got != *c.want:
					t.Errorf("got %v, want %v", *got, *c.want)
				}
			})
		}
	}
}

// The identity strings. filament_colour is written "#RRGGBB" unquoted — a
// value, not a comment — and canonical stores the hex WITHOUT '#'.
// filament_vendor is an unquoted scalar that keeps its spaces;
// filament_notes is quoted with its newlines escaped on the wire.
func TestIngestMetaStrings(t *testing.T) {
	fields := map[string]func(*canonical.Filament) *string{
		"colour": func(f *canonical.Filament) *string { return f.Meta.ColorHex },
		"vendor": func(f *canonical.Filament) *string { return f.Meta.Vendor },
		"notes":  func(f *canonical.Filament) *string { return f.Meta.Notes },
	}
	cases := []struct {
		name  string
		field string
		line  string // "" = key absent from the section
		want  *string
	}{
		{"colour drops the hash", "colour", "filament_colour = #F7F7F7\n", new("F7F7F7")},
		{"colour already bare", "colour", "filament_colour = F7F7F7\n", new("F7F7F7")},
		{"colour empty", "colour", "filament_colour = \n", nil},
		{"colour absent", "colour", "", nil},
		{"vendor keeps spaces", "vendor", "filament_vendor = Forward AM\n", new("Forward AM")},
		{"vendor empty", "vendor", "filament_vendor = \n", nil},
		{"vendor absent", "vendor", "", nil},
		{"vendor literal nil is a name", "vendor", "filament_vendor = nil\n", new("nil")}, // "nil" inherits only in the retraction family
		{"notes unescaped", "notes", `filament_notes = "Line one\nLine two"` + "\n", new("Line one\nLine two")},
		{"notes empty quoted", "notes", `filament_notes = ""` + "\n", nil},
		{"notes absent", "notes", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := fields[c.field](ingestOne(t, c.line))
			switch {
			case c.want == nil && got != nil:
				t.Errorf("got %q, want nil (absent)", *got)
			case c.want != nil && got == nil:
				t.Errorf("got nil, want %q", *c.want)
			case c.want != nil && *got != *c.want:
				t.Errorf("got %q, want %q", *got, *c.want)
			}
		})
	}
}

// The filament gcode blobs are opaque: carried byte for byte,
// templating included, never interpreted. The real bundle's start blob is the
// fixture — PrusaSlicer macros, a regex, a ';' comment inside the value.
func TestIngestGcodeOpaque(t *testing.T) {
	const blob = "M900 K{if nozzle_diameter[0]==0.6}0.12{else}0.2{endif} ; Filament gcode\n\n" +
		"{if printer_notes=~/.*PRINTER_MODEL_MINIIS.*/}\nM572 S0.36\n{endif}"
	wire := strings.ReplaceAll(blob, "\n", `\n`)
	cases := []struct {
		name string
		body string
		want *string
		get  func(*canonical.Filament) *string
	}{
		{"start", "start_filament_gcode = \"" + wire + "\"\n", new(blob), func(f *canonical.Filament) *string { return f.Gcode.Start }},
		{"end", "end_filament_gcode = \"; Filament-specific end gcode\"\n", new("; Filament-specific end gcode"), func(f *canonical.Filament) *string { return f.Gcode.End }},
		{"start empty", "start_filament_gcode = \"\"\n", nil, func(f *canonical.Filament) *string { return f.Gcode.Start }},
		{"end absent", "", nil, func(f *canonical.Filament) *string { return f.Gcode.End }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.get(ingestOne(t, c.body))
			switch {
			case c.want == nil && got != nil:
				t.Errorf("got %q, want nil (absent)", *got)
			case c.want != nil && got == nil:
				t.Errorf("got nil, want %q", *c.want)
			case c.want != nil && *got != *c.want:
				t.Errorf("got %q, want %q", *got, *c.want)
			}
		})
	}
}

// filament_type: the vocabulary renames come back as the canonical name, and a
// name both sides spell alike passes through. A rename that lands nil is a
// table bug, not a loss.
func TestIngestMaterial(t *testing.T) {
	cases := []struct {
		name string
		line string // "" = key absent from the section
		want *canonical.Material
	}{
		{"PET is PETG", "filament_type = PET\n", new(canonical.PETG)},
		{"FLEX is TPU", "filament_type = FLEX\n", new(canonical.TPU)},
		{"NYLON is PA", "filament_type = NYLON\n", new(canonical.PA)},
		{"PA passes through", "filament_type = PA\n", new(canonical.PA)},
		{"PLA passes through", "filament_type = PLA\n", new(canonical.PLA)},
		{"typed canonical name", "filament_type = PETG\n", new(canonical.PETG)}, // open select: a user may type it
		{"empty", "filament_type = \n", nil},
		{"absent", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ingestOne(t, c.line).Meta.Material
			switch {
			case c.want == nil && got != nil:
				t.Errorf("got %q, want nil (absent)", *got)
			case c.want != nil && got == nil:
				t.Errorf("got nil, want %q", *c.want)
			case c.want != nil && *got != *c.want:
				t.Errorf("got %q, want %q", *got, *c.want)
			}
		})
	}
}

// A filament_type with no canonical home (the rest of Prusa's list, or a
// typed CF variant) leaves meta.material empty, is reported as an
// approximation, and keeps its raw value in overrides.prusa — the blocking
// decision belongs to the render target, never to a silent drop.
func TestIngestMaterialHomeless(t *testing.T) {
	for _, raw := range []string{"HIPS", "PVA", "EDGE", "NGEN", "PP", "PEI", "PEEK", "PEKK", "POM", "PSU", "PVDF", "SCAFF", "PLA-CF", "PA-CF"} {
		t.Run(raw, func(t *testing.T) {
			results, err := Ingest([]byte("[filament:T]\nfilament_type = " + raw + "\n"))
			if err != nil || len(results) != 1 || results[0].Err != nil {
				t.Fatalf("Ingest: err=%v results=%+v", err, results)
			}
			r := results[0]
			if r.Filament.Meta.Material != nil {
				t.Errorf("meta.material = %q, want nil", *r.Filament.Meta.Material)
			}
			if got := r.Filament.Overrides["prusa"]["filament_type"]; got != raw {
				t.Errorf("overrides.prusa.filament_type = %v, want %q", got, raw)
			}
			if len(r.Report.Issues) != 1 {
				t.Fatalf("issues = %+v, want one", r.Report.Issues)
			}
			is := r.Report.Issues[0]
			if is.Bucket != BucketApproximated || is.Key != "filament_type" || is.Value != raw || is.Note == "" {
				t.Errorf("issue = %+v, want approximated filament_type %q with a note", is, raw)
			}
			if r.Report.Summary == "all settings mapped cleanly" {
				t.Errorf("summary claims a clean map despite %d issue(s)", len(r.Report.Issues))
			}
		})
	}
}

// A mapped material is a clean translation: no issue, no override.
func TestIngestMaterialMappedIsClean(t *testing.T) {
	results, err := Ingest([]byte("[filament:T]\nfilament_type = PET\n"))
	if err != nil || len(results) != 1 || results[0].Err != nil {
		t.Fatalf("Ingest: err=%v results=%+v", err, results)
	}
	if r := results[0]; len(r.Report.Issues) != 0 || r.Filament.Overrides != nil {
		t.Errorf("issues = %+v, overrides = %v, want none", r.Report.Issues, r.Filament.Overrides)
	}
}

// filament_type is coStrings: a multi-extruder preset joins one type per
// extruder with ';'. Canonical holds one material, so the first wins and the
// collapse is reported. A homeless first keeps the WHOLE raw list.
func TestIngestMaterialMultiValue(t *testing.T) {
	cases := []struct {
		raw          string
		want         *canonical.Material
		wantOverride bool
	}{
		{"PET;PLA", new(canonical.PETG), false},
		{"FLEX;FLEX", new(canonical.TPU), false},
		{"HIPS;PLA", nil, true},
		{`"PLA CF";"ASA X"`, nil, true}, // typed names with spaces are quoted items; the override keeps the wire text
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			results, err := Ingest([]byte("[filament:T]\nfilament_type = " + c.raw + "\n"))
			if err != nil || len(results) != 1 || results[0].Err != nil {
				t.Fatalf("Ingest: err=%v results=%+v", err, results)
			}
			r := results[0]
			got := r.Filament.Meta.Material
			switch {
			case c.want == nil && got != nil:
				t.Errorf("meta.material = %q, want nil", *got)
			case c.want != nil && (got == nil || *got != *c.want):
				t.Errorf("meta.material = %v, want %q", got, *c.want)
			}
			if o, ok := r.Filament.Overrides["prusa"]["filament_type"]; ok != c.wantOverride || (ok && o != c.raw) {
				t.Errorf("overrides.prusa.filament_type = %v (present=%v), want present=%v", o, ok, c.wantOverride)
			}
			if len(r.Report.Issues) != 1 || r.Report.Issues[0].Bucket != BucketApproximated || r.Report.Issues[0].Value != c.raw {
				t.Errorf("issues = %+v, want one approximated filament_type %q", r.Report.Issues, c.raw)
			}
		})
	}
}
