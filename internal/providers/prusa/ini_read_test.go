// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// ini_read_test.go — the reader's contract, anchored on the same realities as
// the writer's tests: the real config bundle and the headerless detached
// export in references/. Every expectation is read off a real file, never
// inferred from a generic idea of what INI looks like.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func readReference(t *testing.T, rel ...string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(append([]string{"..", "..", "..", "references", "prusa"}, rel...)...))
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Join(rel...), err)
	}
	return raw
}

// The real 2.9.6 bundle: four sections, three of them named, [presets] bare.
// The filament section is the ingest target: 87 key lines, of which exactly 2
// (filament_flush_speed, filament_flush_volume) are absent from the wire
// schema — the raw map must carry them anyway, the classifier feeds on keys
// the typed struct does not model.
func TestParseRealBundle(t *testing.T) {
	sections, err := Parse(readReference(t, "PrusaSlicer_config_bundle.ini"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []struct{ cat, name string }{
		{"print", "0.20mm SPEED @MINIIS 0.4 - Copy"},
		{"filament", "Ultrafuse PET @MINI - Detached"},
		{"printer", "Original Prusa XL Input Shaper 0.4 nozzle - Export"},
		{"presets", ""},
	}
	if len(sections) != len(want) {
		t.Fatalf("got %d sections, want %d", len(sections), len(want))
	}
	for i, w := range want {
		if sections[i].Category != w.cat || sections[i].Name != w.name {
			t.Errorf("section %d = %q:%q, want %q:%q", i, sections[i].Category, sections[i].Name, w.cat, w.name)
		}
	}

	presets := sections[3].Keys
	for k, want := range map[string]string{
		"print":     "0.20mm SPEED @XLIS 0.4",
		"printer":   "Original Prusa XL Input Shaper 0.4 nozzle - Export",
		"filament":  "Ultrafuse PET @XL",
		"sla_print": "",
	} {
		if got := presets[k]; got != want {
			t.Errorf("presets[%q] = %q, want %q", k, got, want)
		}
	}
}

// The raw map is the classifier's input: it must be complete (all 87 key
// lines) and keep the wire form exactly (escapes intact), including the two
// keys the schema does not model.
func TestRawKeysAreCompleteAndWireForm(t *testing.T) {
	sections, err := Parse(readReference(t, "PrusaSlicer_config_bundle.ini"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var keys map[string]string
	for _, s := range sections {
		if s.Category == "filament" {
			keys = s.Keys
		}
	}
	if len(keys) != 87 {
		t.Errorf("filament section: %d raw keys, want 87 (measured off the file)", len(keys))
	}
	// Keys the typed struct does not model — the classifier's reason to exist.
	if keys["filament_flush_speed"] != "0" {
		t.Errorf("filament_flush_speed = %q, want \"0\"", keys["filament_flush_speed"])
	}
	if keys["filament_flush_volume"] != "0" {
		t.Errorf("filament_flush_volume = %q, want \"0\"", keys["filament_flush_volume"])
	}
	// Wire form stays wire form: still quoted, escapes still escaped.
	if keys["filament_settings_id"] != `""` {
		t.Errorf("filament_settings_id = %q, want raw `\"\"`", keys["filament_settings_id"])
	}
	if !strings.HasPrefix(keys["filament_notes"], `"Material Description\nUltrafuse PET is made from a premium PET`) {
		t.Errorf("filament_notes = %q, want the quoted \\n-escaped wire form", keys["filament_notes"])
	}
	if keys["filament_colour"] != "#F7F7F7" {
		t.Errorf("filament_colour = %q, want #F7F7F7 (the '#' trap)", keys["filament_colour"])
	}
}

// The typed view of the real filament section: values as they decode, not as
// they are spelled. These are the ground truths T3–T6 convert to canonical.
func TestDecodeRealBundleFilament(t *testing.T) {
	sections, err := Parse(readReference(t, "PrusaSlicer_config_bundle.ini"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := sections[1].DecodeFilament()

	// Identity: `filament_settings_id = ""` is one EMPTY item, not an absent
	// list; a quoted scalar may decode to an empty string.
	if !reflect.DeepEqual(got.FilamentSettingsID, []string{""}) {
		t.Errorf("filament_settings_id = %#v, want []string{\"\"}", got.FilamentSettingsID)
	}
	if got.FilamentVendor != "Forward AM" {
		t.Errorf("filament_vendor = %q, want \"Forward AM\" (unquoted scalar with space)", got.FilamentVendor)
	}
	if !reflect.DeepEqual(got.FilamentType, []string{"PET"}) {
		t.Errorf("filament_type = %#v, want []string{\"PET\"}", got.FilamentType)
	}
	if got.FilamentColour != "#F7F7F7" {
		t.Errorf("filament_colour = %q, want #F7F7F7", got.FilamentColour)
	}
	if got.CustomParameters != "" {
		t.Errorf("custom_parameters_filament = %q, want \"\" (quoted empty scalar)", got.CustomParameters)
	}

	// Notes: one quoted item whose \n escapes decode to REAL newlines.
	notes := got.FilamentNotes
	if len(notes) != 1 || !strings.HasPrefix(notes[0], "Material Description\n") ||
		!strings.Contains(notes[0], "\n\nPrinting Recommendations:") {
		t.Errorf("filament_notes decode wrong: %d items, prefix %q",
			len(notes), strings.TrimPrefix(notes[0], "Material Description\n"))
	}

	// Temperatures: scalars on the prusa side, one item per list.
	if !reflect.DeepEqual(got.Temperature, []string{"215"}) {
		t.Errorf("temperature = %v, want [215]", got.Temperature)
	}
	if !reflect.DeepEqual(got.FirstLayerTemperature, []string{"220"}) {
		t.Errorf("first_layer_temperature = %v, want [220]", got.FirstLayerTemperature)
	}
	if !reflect.DeepEqual(got.BedTemperature, []string{"70"}) {
		t.Errorf("bed_temperature = %v, want [70]", got.BedTemperature)
	}

	// Absent lists: `key = ` (bare) reads as nil — nothing set.
	if got.CompatiblePrinters != nil {
		t.Errorf("compatible_printers = %#v, want nil (absent list)", got.CompatiblePrinters)
	}
	if got.CompatiblePrints != nil {
		t.Errorf("compatible_prints = %#v, want nil (absent list)", got.CompatiblePrints)
	}
	if got.CompatiblePrintersCondition != "printer_notes=~/.*PRINTER_MODEL_MINI.*/" {
		t.Errorf("compatible_printers_condition = %q", got.CompatiblePrintersCondition)
	}

	// The "nil" keyword is a LIST ITEM on the wire — inherit means nothing at
	// the reader layer, the canonical nil conversion belongs to the ingest.
	if !reflect.DeepEqual(got.FilamentRetractLength, []string{"nil"}) {
		t.Errorf("filament_retract_length = %#v, want []string{\"nil\"}", got.FilamentRetractLength)
	}
	if !reflect.DeepEqual(got.IdleTemperature, []string{"nil"}) {
		t.Errorf("idle_temperature = %#v, want []string{\"nil\"}", got.IdleTemperature)
	}

	// Physical fields, authored zeros included ("0" is a value, not absence).
	if !reflect.DeepEqual(got.FilamentSpoolWeight, []string{"0"}) {
		t.Errorf("filament_spool_weight = %#v, want [\"0\"]", got.FilamentSpoolWeight)
	}
	if !reflect.DeepEqual(got.FilamentAbrasive, []string{"0"}) {
		t.Errorf("filament_abrasive = %#v, want [\"0\"]", got.FilamentAbrasive)
	}
	if got.FilamentShrinkageCompensationXY != "0%" {
		t.Errorf("filament_shrinkage_compensation_xy = %q, want \"0%%\"", got.FilamentShrinkageCompensationXY)
	}

	// Scalar-with-spaces, quoted on the real file: decode strips the quotes.
	if !strings.HasPrefix(got.FilamentRammingParameters, "120 100 ") ||
		!strings.Contains(got.FilamentRammingParameters, "10.0|") {
		t.Errorf("filament_ramming_parameters = %q", got.FilamentRammingParameters)
	}

	// Gcode: one quoted item, \n escapes decoded, {if} templating untouched.
	gcode := got.StartFilamentGcode
	if len(gcode) != 1 || !strings.Contains(gcode[0], "M900 K{if nozzle_diameter") ||
		!strings.Contains(gcode[0], "\nM572 S") {
		t.Errorf("start_filament_gcode decode wrong: %d items", len(gcode))
	}
	if !reflect.DeepEqual(got.EndFilamentGcode, []string{"; Filament-specific end gcode"}) {
		t.Errorf("end_filament_gcode = %#v (the ';' inside quotes trap)", got.EndFilamentGcode)
	}
}

// The reader is the exact inverse of the writer: marshal a struct hitting
// every encoding trap, parse it back, the struct must come out identical.
func TestRoundTripWriterReader(t *testing.T) {
	want := FilamentSettings{
		FilamentIdentity: FilamentIdentity{
			FilamentSettingsID: []string{""},
			FilamentVendor:     "Forward AM",
			FilamentType:       []string{"PET"},
			FilamentColour:     "#F7F7F7",
			FilamentNotes:      []string{"line one\nline two"},
			CustomParameters:   "k=v",
		},
		FilamentPhysical: FilamentPhysical{
			FilamentDiameter:                []string{"1.75"},
			FilamentShrinkageCompensationXY: "2%",
		},
		FilamentCompatibility: FilamentCompatibility{
			CompatiblePrintersCondition: "printer_notes=~/.*PRINTER_MODEL_MINI.*/",
		},
		FilamentTemp: FilamentTemp{
			Temperature:           []string{"215"},
			FirstLayerTemperature: []string{"220"},
			IdleTemperature:       []string{"nil"},
		},
		FilamentFlow: FilamentFlow{
			ExtrusionMultiplier:     []string{"1"},
			FilamentPurgeMultiplier: "100%",
		},
		FilamentRetraction: FilamentRetraction{
			FilamentRetractLength: []string{"nil"},
			FilamentRetractSpeed:  []string{"45"},
			FilamentWipe:          []string{"1"},
		},
		FilamentToolchange: FilamentToolchange{
			FilamentRammingParameters: "120 100 | 0.05 6.6",
		},
		FilamentGcode: FilamentGcode{
			StartFilamentGcode: []string{"M900 K0.2 ; set K\nM862.3 P \"XL\""},
		},
	}

	raw, err := MarshalBundle("filament", "Echo", want)
	if err != nil {
		t.Fatalf("MarshalBundle: %v", err)
	}
	sections, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sections) != 1 || sections[0].Category != "filament" || sections[0].Name != "Echo" {
		t.Fatalf("parse of own output: %+v", sections)
	}
	if got := sections[0].DecodeFilament(); !reflect.DeepEqual(got, want) {
		t.Errorf("round trip changed the struct:\ngot  %#v\nwant %#v", got, want)
	}
}

// A non-nil EMPTY list is the one shape the wire cannot carry back: the
// writer emits a bare `key = ` for it, which any reader — PrusaSlicer's
// included — reads as an absent list. Documented here, not hidden.
func TestEmptyNonNilSliceRoundTripsAsNil(t *testing.T) {
	raw, err := MarshalBundle("filament", "Echo", FilamentSettings{
		FilamentCompatibility: FilamentCompatibility{CompatiblePrinters: []string{}},
	})
	if err != nil {
		t.Fatalf("MarshalBundle: %v", err)
	}
	sections, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := sections[0].DecodeFilament().CompatiblePrinters; got != nil {
		t.Errorf("compatible_printers = %#v, want nil (the wire cannot express an empty present list)", got)
	}
}

// Dialect traps, spelled out individually so a failure names the trap.
func TestParseDialectTraps(t *testing.T) {
	raw := strings.Join([]string{
		"# comment before any section",
		"; also a comment",
		"",
		"[filament:a:b]",                      // name is everything after the FIRST colon
		"filament_colour = #F7F7F7",           // '#' after '=' is a VALUE, not a comment
		"end_filament_gcode = \"; Fil line\"", // ';' inside quotes is content
		"filament_vendor = Forward AM",        // unquoted scalar with spaces
		"",
		"[presets]",
		"filament = Ultrafuse PET @XL",
		"",
	}, "\n")
	sections, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(sections))
	}
	a := sections[0]
	if a.Category != "filament" || a.Name != "a:b" {
		t.Errorf("header [filament:a:b] = %q:%q", a.Category, a.Name)
	}
	if a.Keys["filament_colour"] != "#F7F7F7" {
		t.Errorf("filament_colour = %q, want #F7F7F7", a.Keys["filament_colour"])
	}
	if got := a.DecodeFilament().EndFilamentGcode; !reflect.DeepEqual(got, []string{"; Fil line"}) {
		t.Errorf("end_filament_gcode = %#v", got)
	}
	p := sections[1]
	if p.Category != "presets" || p.Name != "" {
		t.Errorf("[presets] = %q:%q, want presets:\"\"", p.Category, p.Name)
	}
	if p.Keys["filament"] != "Ultrafuse PET @XL" {
		t.Errorf("presets filament = %q", p.Keys["filament"])
	}
}

// The same key in different sections is normal (every section of a bundle
// carries compatible_printers); inside ONE section a duplicate is an error.
func TestDuplicateKeyRules(t *testing.T) {
	if _, err := Parse([]byte("[filament:A]\ntemperature = 215\ntemperature = 220\n")); err == nil ||
		!strings.Contains(err.Error(), "temperature") {
		t.Errorf("duplicate key must error naming the key, got %v", err)
	}
	sections, err := Parse([]byte("[filament:A]\ntemperature = 215\n\n[filament:B]\ntemperature = 220\n"))
	if err != nil {
		t.Fatalf("same key in different sections must parse: %v", err)
	}
	if got := sections[0].DecodeFilament().Temperature; !reflect.DeepEqual(got, []string{"215"}) {
		t.Errorf("A.temperature = %#v", got)
	}
	if got := sections[1].DecodeFilament().Temperature; !reflect.DeepEqual(got, []string{"220"}) {
		t.Errorf("B.temperature = %#v", got)
	}
}

// Malformed input is an error naming the line — the reader never panics.
func TestParseErrors(t *testing.T) {
	cases := []struct{ name, raw, want string }{
		{"line without separator", "[filament:X]\ngarbage without equals\n", "garbage without equals"},
		{"unterminated header", "[filament:X\ntemperature = 215\n", "section header"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Parse([]byte(c.raw)); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("Parse error = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

// CRLF exports round-trip: the trailing \r is a line terminator, not a
// character of the value.
func TestParseAcceptsCRLF(t *testing.T) {
	raw := "[filament:X]\r\nfilament_colour = #F7F7F7\r\nfilament_vendor = Forward AM\r\n\r\n"
	sections, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := sections[0].Keys["filament_colour"]; got != "#F7F7F7" {
		t.Errorf("filament_colour = %q (CR leaked into value?)", got)
	}
	if got := sections[0].Keys["filament_vendor"]; got != "Forward AM" {
		t.Errorf("filament_vendor = %q", got)
	}
}

// The detached export is the headerless whole-configuration form (pitfall
// #15): zero sections, no error — the REFUSAL is the ingest layer's job.
func TestHeaderlessExportYieldsNoSections(t *testing.T) {
	sections, err := Parse(readReference(t, "filaments", "Ultrafuse PET @MINI - Detached.ini"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("headerless file: got %d sections, want 0", len(sections))
	}
}

func TestDecodeScalar(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"plain", "M900 K0.2", "M900 K0.2"},
		{"escaped newline", `line one\nline two`, "line one\nline two"},
		{"escaped carriage return", `a\rb`, "a\rb"},
		{"doubled backslash", `C:\\temp`, `C:\temp`},
		{"quoted scalar", `"120 100 | 0.05"`, "120 100 | 0.05"},
		{"quoted empty scalar", `""`, ""},
		{"bare quotes stay literal", `M862.3 P "XL"`, `M862.3 P "XL"`},
		{"quoted escaped inner quote", `"say \"hi\""`, `say "hi"`},
		{"nil keyword is plain text here", "nil", "nil"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decodeScalar(c.in); got != c.want {
				t.Errorf("decodeScalar(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestDecodeList(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"bare empty is absent", "", nil},
		{"sole empty item is quoted", `""`, []string{""}},
		{"plain items", "a;b", []string{"a", "b"}},
		{"semicolon inside quotes is content", `"a;b";c`, []string{"a;b", "c"}},
		{"quoted inner quote", `"M862.3 P \"XL\""`, []string{`M862.3 P "XL"`}},
		{"nil is just a one-item list", "nil", []string{"nil"}},
		{"trailing empty slot", "a;", []string{"a", ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decodeList(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("decodeList(%q) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}
