// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// ingest_test.go — the scaffold contract (ticket T2): one bundle in, N
// results out, every refusal seam, and the minimal end-to-end proof through
// the render. Field mapping lives in ingest_convert_test.go, the loss
// classifier in ingest_report_test.go.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The minimal fixture the ticket names: name + one temperature, ingested and
// rendered back to a valid bundle. Round-trip at the wire level — the same
// value must land back in the INI.
func TestIngestMinimalBundleRoundTrip(t *testing.T) {
	results, err := Ingest([]byte("[filament:Fixture]\ntemperature = 215\n"))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	r := results[0]
	if r.Err != nil {
		t.Fatalf("result error: %v", r.Err)
	}
	if r.Name != "Fixture" {
		t.Errorf("Name = %q, want Fixture (the [filament:NAME] header)", r.Name)
	}
	f := r.Filament
	if f == nil {
		t.Fatal("Filament is nil")
	}
	if f.Kind != "filament" || f.SchemaVersion != 1 {
		t.Errorf("Kind/SchemaVersion = %q/%d, want filament/1", f.Kind, f.SchemaVersion)
	}
	if f.Meta.Name == nil || *f.Meta.Name != "Fixture" {
		t.Errorf("meta.name = %v, want Fixture", f.Meta.Name)
	}
	if f.Temps.Nozzle == nil || *f.Temps.Nozzle != 215 {
		t.Errorf("temps.nozzle = %v, want 215", f.Temps.Nozzle)
	}
	if r.Report == nil || r.Report.Summary == "" {
		t.Error("report missing or without summary")
	}

	// Render back: meta.name present → requiredFilament passes, and the
	// temperature must land back in the INI, byte for byte.
	settings, _, err := RenderFilament(*f)
	if err != nil {
		t.Fatalf("RenderFilament: %v", err)
	}
	bundle, err := MarshalBundle("filament", r.Name, settings)
	if err != nil {
		t.Fatalf("MarshalBundle: %v", err)
	}
	sections, err := Parse(bundle)
	if err != nil {
		t.Fatalf("re-parse bundle: %v", err)
	}
	got := sections[0].DecodeFilament().Temperature
	if len(got) != 1 || got[0] != "215" {
		t.Errorf("temperature after round trip = %v, want [215]", got)
	}
}

// A preset with a non-empty inherits is refused — THAT preset only, per user
// decision (a). The bundle's other presets still ingest.
func TestInheritsRefusedPerPreset(t *testing.T) {
	raw := "[filament:A]\ninherits = Base\ntemperature = 210\n\n" +
		"[filament:B]\ntemperature = 220\n"
	results, err := Ingest([]byte(raw))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	a := results[0]
	if a.Err == nil || !errors.Is(a.Err, ErrInherits) {
		t.Errorf("A: Err = %v, want ErrInherits", a.Err)
	}
	if a.Err == nil || !strings.Contains(a.Err.Error(), "Base") || !strings.Contains(a.Err.Error(), "A") {
		t.Errorf("A: Err must name the preset and the parent, got %v", a.Err)
	}
	if a.Filament != nil {
		t.Error("refused preset must carry no filament")
	}
	b := results[1]
	if b.Err != nil || b.Filament == nil {
		t.Fatalf("B must ingest cleanly, got Err=%v", b.Err)
	}
	if b.Name != "B" || *b.Filament.Temps.Nozzle != 220 {
		t.Errorf("B = %q / %v", b.Name, b.Filament.Temps.Nozzle)
	}
}

// Headerless files are the detached whole-configuration export: refused with
// an actionable pointer, not a generic parse error (pitfall #15).
func TestIngestHeaderlessRefused(t *testing.T) {
	raw := readReference(t, "filaments", "Ultrafuse PET @MINI - Detached.ini")
	_, err := Ingest(raw)
	if err == nil {
		t.Fatal("headerless export must be refused")
	}
	for _, want := range []string{"config bundle", "no section"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal missing %q: %v", want, err)
		}
	}
}

// A bundle that carries sections but no filament has nothing to ingest — the
// error must name what the file DOES contain.
func TestIngestNoFilamentSection(t *testing.T) {
	_, err := Ingest([]byte("[print:X]\nlayer_height = 0.2\n\n[printer:Y]\nprinter_model = MINI\n"))
	if err == nil || !strings.Contains(err.Error(), "[print:") || !strings.Contains(err.Error(), "filament") {
		t.Errorf("must refuse listing the sections found, got %v", err)
	}

	// [presets] alone is not a preset library.
	if _, err := Ingest([]byte("[presets]\nfilament = Ultrafuse PET @XL\n")); err == nil {
		t.Error("a [presets]-only file must be refused")
	}
}

// Two presets with the same name would write the same canonical file, and the
// CLI writer would silently overwrite one — refused at the source.
func TestIngestDuplicatePresetNames(t *testing.T) {
	raw := "[filament:X]\ntemperature = 210\n\n[filament:X]\ntemperature = 220\n"
	_, err := Ingest([]byte(raw))
	if err == nil || !strings.Contains(err.Error(), "X") || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("duplicate name must error naming it, got %v", err)
	}
}

// macOS and Windows filesystems ignore case by default: PLA.yaml and pla.yaml
// are one file there, so names equal after case folding are duplicates too.
func TestIngestDuplicatePresetNamesIgnoreCase(t *testing.T) {
	raw := "[filament:PLA]\ntemperature = 210\n\n[filament:pla]\ntemperature = 220\n"
	_, err := Ingest([]byte(raw))
	if err == nil || !strings.Contains(err.Error(), `"PLA"`) || !strings.Contains(err.Error(), `"pla"`) {
		t.Errorf("case-only duplicate must error naming both presets, got %v", err)
	}
}

// The real 2.9.6 bundle: its one filament preset has an EMPTY inherits (all
// values inline), so it ingests — and the print/printer/presets sections
// neither hijack the results nor trip the refusals.
func TestIngestRealBundleSmoke(t *testing.T) {
	raw := readReference(t, "PrusaSlicer_config_bundle.ini")
	results, err := Ingest(raw)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want exactly the 1 filament preset", len(results))
	}
	r := results[0]
	if r.Err != nil || r.Filament == nil {
		t.Fatalf("result: %v", r.Err)
	}
	if r.Name != "Ultrafuse PET @MINI - Detached" {
		t.Errorf("Name = %q", r.Name)
	}
	if *r.Filament.Temps.Nozzle != 215 {
		t.Errorf("temps.nozzle = %v, want 215 (measured off the file)", r.Filament.Temps.Nozzle)
	}
}

// Malformed INI is a whole-file error, wrapped with the parse failure.
func TestIngestMalformedBundle(t *testing.T) {
	if _, err := Ingest([]byte("[filament:X]\ngarbage line\n")); err == nil {
		t.Error("malformed bundle must error")
	}
}

// The real bundle's filament, as canonical YAML, pinned as a golden: every
// mapped family shows up here, reviewed against the measured wire values
// (vendor Forward AM, filament_type PET → PETG, colour #F7F7F7, the
// two-paragraph notes, temperature 215, first layer 220, bed 70/70,
// extrusion_multiplier 1, filament_max_volumetric_speed 8, diameter 1.75,
// density 1.33, cost 0, abrasive 0, shrinkage 0%/0%, fans min 75 / max 100
// / bridge 100, fan off first 1 layer, full fan at layer 0, slowdown below
// 15 s, min print speed 15, retract speed 45, deretract speed 25, wipe 0 —
// retract length and lift are "nil", so they stay absent — the start/end
// gcode blobs verbatim, and the compat condition kept raw in
// overrides.prusa).
// Regenerate with -update when a slice maps a new family, and review the
// diff.
func TestIngestRealBundleGolden(t *testing.T) {
	results, err := Ingest(readReference(t, "PrusaSlicer_config_bundle.ini"))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	got, err := yaml.Marshal(results[0].Filament)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	path := filepath.Join("testdata", "ingest", "real-bundle.golden.yaml")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
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
}
