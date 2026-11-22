// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// ingest_report_test.go — the loss classifier (ticket T6). Every key present
// in a [filament:] section lands in exactly one bucket, or is mapped and
// silent. The real bundle pins the whole report as a golden.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomsmoreau/Aldina/internal/canonical"
)

// ingestResult ingests a one-preset bundle and fails on any refusal.
func ingestResult(t *testing.T, raw string) IngestResult {
	t.Helper()
	results, err := Ingest([]byte(raw))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("want 1 clean result, got %+v", results)
	}
	return results[0]
}

// issuesFor returns every issue reported for key — more than one is a
// double report.
func issuesFor(r *IngestReport, key string) []IngestIssue {
	var out []IngestIssue
	for _, is := range r.Issues {
		if is.Key == key {
			out = append(out, is)
		}
	}
	return out
}

// reportLines renders the report the way the CLI prints it: the headline,
// then one line per issue.
func reportLines(r *IngestReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "summary: %s\n", r.Summary)
	for _, is := range r.Issues {
		fmt.Fprintf(&b, "  [%s] %s = %s", is.Bucket, is.Key, is.Value)
		if is.Note != "" {
			fmt.Fprintf(&b, " — %s", is.Note)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestClassifyBuckets(t *testing.T) {
	cases := []struct {
		name, line, key string
		want            LossBucket // "" = no issue
	}{
		{"mapped scalar list is silent", "temperature = 215", "temperature", ""},
		{"mapped nil inherit is silent", "filament_retract_length = nil", "filament_retract_length", ""},
		{"mapped list, N items collapses", "temperature = 215;220", "temperature", BucketLostFanout},
		{"mapped scalar keeps its ';' as content", "filament_vendor = A;B", "filament_vendor", ""},
		{"quoted ';' inside one item is content", `start_filament_gcode = "M900 K0 ; comment"`, "start_filament_gcode", ""},
		{"unparseable float is reported", "temperature = hot", "temperature", BucketApproximated},
		{"fractional layer count is reported", "disable_fan_first_layers = 2.5", "disable_fan_first_layers", BucketApproximated},
		{"unparseable percent is reported", "min_fan_speed = full", "min_fan_speed", BucketApproximated},
		{"unparseable bool is reported", "filament_wipe = yes", "filament_wipe", BucketApproximated},
		{"unparseable shrink is reported", "filament_shrinkage_compensation_xy = lots", "filament_shrinkage_compensation_xy", BucketApproximated},
		{"free text always parses", "filament_notes = anything at all", "filament_notes", ""},
		{"identity key", "filament_settings_id = My PLA", "filament_settings_id", BucketLostIdentity},
		{"quoted empty item authors nothing", `filament_settings_id = ""`, "filament_settings_id", ""},
		{"inherit token authors nothing", "idle_temperature = nil", "idle_temperature", ""},
		{"print-preset binding", "compatible_prints = Some print", "compatible_prints", BucketLostIdentity},
		{"toolchange knob", "filament_loading_speed = 28", "filament_loading_speed", BucketLostEngine},
		{"bare retract_ is a print default", "retract_length = 0.8", "retract_length", BucketLostEngine},
		{"key the schema does not model", "filament_flush_speed = 0", "filament_flush_speed", BucketLostEngine},
		{"bare empty value is an absent key", "compatible_prints_condition = ", "compatible_prints_condition", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := ingestResult(t, "[filament:X]\n"+c.line+"\n")
			got := issuesFor(r.Report, c.key)
			switch {
			case c.want == "" && len(got) != 0:
				t.Errorf("%s: got %+v, want no issue", c.key, got)
			case c.want != "" && (len(got) != 1 || got[0].Bucket != c.want):
				t.Errorf("%s: got %+v, want exactly one %s", c.key, got, c.want)
			}
		})
	}
}

// Ingest already reports a multi-value or homeless filament_type; the
// classifier must not add a second line for it (T5 carry-over). A clean
// rename is translated, not a loss.
func TestClassifyFilamentTypeReportedOnce(t *testing.T) {
	for line, want := range map[string]int{
		"filament_type = PLA;PETG": 1,
		"filament_type = HIPS":     1,
		"filament_type = PET":      0,
	} {
		r := ingestResult(t, "[filament:X]\n"+line+"\n")
		if got := issuesFor(r.Report, "filament_type"); len(got) != want {
			t.Errorf("%s: %d issue(s) %+v, want %d", line, len(got), got, want)
		}
	}
}

// The condition rules are never parsed (spec Q4): lost-identity, and the raw
// string survives in overrides.prusa so the author can re-declare it by hand.
// They share the override map with a homeless filament_type.
func TestConditionRulesPreserved(t *testing.T) {
	r := ingestResult(t, "[filament:X]\n"+
		"compatible_printers_condition = printer_notes=~/.*PRINTER_MODEL_MINI.*/\n"+
		`compatible_prints_condition = layer_height > 0.1`+"\n"+
		"filament_type = HIPS\n")

	for key, raw := range map[string]string{
		"compatible_printers_condition": "printer_notes=~/.*PRINTER_MODEL_MINI.*/",
		"compatible_prints_condition":   "layer_height > 0.1",
	} {
		got := issuesFor(r.Report, key)
		if len(got) != 1 || got[0].Bucket != BucketLostIdentity || got[0].Value != raw {
			t.Errorf("%s: got %+v, want one lost-identity carrying the raw rule", key, got)
		}
		if v := r.Filament.Overrides["prusa"][key]; v != raw {
			t.Errorf("overrides.prusa.%s = %v, want %q", key, v, raw)
		}
	}
	if v := r.Filament.Overrides["prusa"]["filament_type"]; v != "HIPS" {
		t.Errorf("overrides.prusa.filament_type = %v, want HIPS kept beside the rules", v)
	}
}

// An empty condition carries no rule: nothing to preserve.
func TestEmptyConditionNotPreserved(t *testing.T) {
	r := ingestResult(t, "[filament:X]\ncompatible_printers_condition = \n")
	if r.Filament.Overrides != nil {
		t.Errorf("overrides = %v, want none", r.Filament.Overrides)
	}
}

// compatible_printers names resolve through the prusa catalogue back to
// canonical machines; an unknown name is a named lost-identity, never a
// silent drop.
func TestCompatiblePrintersResolve(t *testing.T) {
	r := ingestResult(t, "[filament:X]\n"+
		`compatible_printers = "Original Prusa MINI & MINI+";"Original Prusa MINI & MINI+ 0.6 nozzle";"Original Prusa MINI & MINI+";"My Custom Printer"`+"\n")

	got := r.Filament.Compat.Machines
	if len(got) != 2 ||
		got[0].Model != "Original Prusa MINI & MINI+" || got[0].NozzleMM != 0.4 ||
		got[1].Model != "Original Prusa MINI & MINI+" || got[1].NozzleMM != 0.6 {
		t.Errorf("compat.machines = %+v, want MINI 0.4 then MINI 0.6, deduplicated", got)
	}
	issues := issuesFor(r.Report, "compatible_printers")
	if len(issues) != 1 || issues[0].Bucket != BucketLostIdentity || issues[0].Value != "My Custom Printer" {
		t.Errorf("issues = %+v, want one lost-identity naming the unknown preset", issues)
	}
}

func TestSummaryClean(t *testing.T) {
	r := ingestResult(t, "[filament:X]\ntemperature = 215\n")
	if r.Report.Summary != "all settings mapped cleanly" || len(r.Report.Issues) != 0 {
		t.Errorf("report = %+v, want clean", r.Report)
	}
}

// The real 2.9.6 bundle, report pinned whole. Its two keys the schema does
// not model (filament_flush_speed, filament_flush_volume) must be in it —
// the classifier walks the raw map precisely for them. Regenerate with
// -update and review the diff.
func TestIngestRealBundleReportGolden(t *testing.T) {
	results, err := Ingest(readReference(t, "PrusaSlicer_config_bundle.ini"))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	report := results[0].Report
	for _, key := range []string{"filament_flush_speed", "filament_flush_volume"} {
		if got := issuesFor(report, key); len(got) != 1 {
			t.Errorf("%s: %+v, want exactly one bucket", key, got)
		}
	}
	got := reportLines(report)

	path := filepath.Join("testdata", "ingest", "real-bundle.report.golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update): %v", err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// Every key render writes must be known to the classifier: a mapped key
// missing from mappedPrusaLists/Scalars would come back lost-engine-knob. A
// fully authored canonical, rendered and read back, reports only the compat
// condition — the one rendered key canonical keeps in overrides, not a field.
func TestRenderedKeysClassifyAsMapped(t *testing.T) {
	f := baseFilament()
	f.Retraction = canonical.Retraction{
		LengthMM: new(0.8), SpeedMMS: new(35.0), DeretractSpeedMMS: new(25.0),
		ZHopMM: new(0.2), Wipe: new(true),
	}
	settings, _, err := RenderFilament(f)
	if err != nil {
		t.Fatalf("RenderFilament: %v", err)
	}
	bundle, err := MarshalBundle("filament", *f.Meta.Name, settings)
	if err != nil {
		t.Fatalf("MarshalBundle: %v", err)
	}
	r := ingestResult(t, string(bundle))
	if len(r.Report.Issues) != 1 || r.Report.Issues[0].Key != "compatible_printers_condition" {
		t.Errorf("issues = %+v, want only compatible_printers_condition", r.Report.Issues)
	}
}

// Spec #6 names these as material or cooling intent, but canonical has no
// field for them yet. The taxonomy has no intent bucket, so they stay
// engine knobs, and the note says why instead of calling them tuning.
func TestUnmodelledIntentNamed(t *testing.T) {
	for _, key := range []string{
		"filament_soluble", "filament_spool_weight", "fan_below_layer_time",
		"overhang_fan_speed_0", "overhang_fan_speed_1", "overhang_fan_speed_2", "overhang_fan_speed_3",
	} {
		r := ingestResult(t, "[filament:X]\n"+key+" = 1\n")
		got := issuesFor(r.Report, key)
		if len(got) != 1 || got[0].Bucket != BucketLostEngine || !strings.Contains(got[0].Note, "no canonical field") {
			t.Errorf("%s: %+v, want one lost-engine-knob naming the missing canonical field", key, got)
		}
	}
}
