// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Orca names a filament preset after the machine it targets, so one target
// means one file — and each file carries the preset name the CATALOGUE
// resolved, which is not the label in the file name.
func TestRenderFilamentPerTargetMachine(t *testing.T) {
	input := writeFixture(t, "filament.yaml", `kind: filament
schema_version: 1
meta:
  name: Test PETG
compat:
  machines:
    - model: Bambu Lab P2S
      nozzle_mm: 0.4
    - model: Bambu Lab P2S
      nozzle_mm: 0.6
`)
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"render", "--provider", "orca", "--kind", "filament", "--in", input, "--out", outDir}, &stdout, &stderr); err != nil {
		t.Fatalf("run render: %v\nstderr:\n%s", err, stderr.String())
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("output files = %d, want 2", len(entries))
	}
	for label, preset := range map[string]string{
		"Bambu Lab P2S 0.4": "Bambu Lab P2S 0.4 nozzle",
		"Bambu Lab P2S 0.6": "Bambu Lab P2S 0.6 nozzle",
	} {
		path := filepath.Join(outDir, "Test PETG @"+label+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %q: %v", path, err)
		}
		var got struct {
			CompatiblePrinters []string `json:"compatible_printers"`
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("decode %q: %v", path, err)
		}
		if len(got.CompatiblePrinters) != 2 || got.CompatiblePrinters[0] != preset || got.CompatiblePrinters[1] != label {
			t.Errorf("%q compatible_printers = %v, want [%q %q]", path, got.CompatiblePrinters, preset, label)
		}
	}
}

func TestRenderFilamentRejectsEmptyCompat(t *testing.T) {
	input := writeFixture(t, "filament.yaml", `kind: filament
schema_version: 1
meta:
  name: Test PETG
compat:
  machines: []
`)
	var stdout, stderr bytes.Buffer
	err := Run([]string{"render", "--kind", "filament", "--in", input, "--out", t.TempDir()}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "compat.machines") {
		t.Fatalf("run render error = %v, want compat.machines error", err)
	}
}

// Orca writes one file per target, so a repeated machine would have the second
// render silently overwrite the first. The message must name the machine the
// way MachineRef.String does — one formatter, so the CLI and the CheckCompat
// warning can never disagree on the unit.
func TestRenderFilamentRejectsDuplicateMachine(t *testing.T) {
	input := writeFixture(t, "filament.yaml", `kind: filament
schema_version: 1
meta:
  name: Test PETG
compat:
  machines:
    - model: Bambu Lab P2S
      nozzle_mm: 0.6
    - model: Bambu Lab P2S
      nozzle_mm: 0.6
`)
	var stdout, stderr bytes.Buffer
	err := Run([]string{"render", "--kind", "filament", "--in", input, "--out", t.TempDir()}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "duplicate machine") {
		t.Fatalf("run render error = %v, want a duplicate machine error", err)
	}
	if want := "Bambu Lab P2S 0.6"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to name the machine as %q", err, want)
	}
}

// The strict loader is what turns the old shape into a diagnosable failure
// instead of a filament silently compatible with nothing.
func TestOldCompatPrintersIsALoadError(t *testing.T) {
	input := writeFixture(t, "filament.yaml", `kind: filament
schema_version: 1
meta:
  name: Test PETG
compat:
  printers:
    - Bambu Lab P2S 0.6 nozzle
`)
	var stdout, stderr bytes.Buffer
	err := Run([]string{"render", "--kind", "filament", "--in", input, "--out", t.TempDir()}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "printers") {
		t.Fatalf("run render error = %v, want the loader to reject the removed key", err)
	}
}

// A derived machine preset is now PROVEN to import in the Orca UI, so the
// blanket "import is unvalidated" warning is gone: keeping it would train the
// user to ignore stderr, where the real per-field losses are reported.
func TestRenderMachineDerived(t *testing.T) {
	input := writeFixture(t, "machine.yaml", `kind: machine
schema_version: 1
meta:
  name: Aldina derived test
  model: Bambu Lab P2S
volume:
  width_mm: 256
  depth_mm: 256
  height_mm: 256
nozzle:
  diameter_mm: 0.6
`)
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"render", "--kind", "machine", "--in", input, "--out", outDir, "--inherits", "Bambu Lab P2S 0.6 nozzle"}, &stdout, &stderr); err != nil {
		t.Fatalf("run render: %v\nstderr:\n%s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "unvalidated") {
		t.Errorf("stderr still carries the retired import warning:\n%s", stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(outDir, "Aldina derived test.json"))
	if err != nil {
		t.Fatalf("read machine output: %v", err)
	}
	var got struct {
		Inherits          string `json:"inherits"`
		PrinterSettingsID string `json:"printer_settings_id"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode machine output: %v", err)
	}
	if got.Inherits != "Bambu Lab P2S 0.6 nozzle" {
		t.Errorf("inherits = %q, want parent", got.Inherits)
	}
	// The key that made the import work — assert it end to end, not just in
	// the provider unit test.
	if got.PrinterSettingsID != "Aldina derived test" {
		t.Errorf("printer_settings_id = %q, want the preset name", got.PrinterSettingsID)
	}
}

// Prusa writes ONE file whatever compat holds, where Orca writes one per
// target machine: the two providers structure their output differently and the
// CLI must follow each, not impose a shape.
func TestRenderPrusaFilamentWritesOneFile(t *testing.T) {
	input := writeFixture(t, "filament.yaml", `kind: filament
schema_version: 1
meta:
  name: Test PETG
  material: PETG
temps:
  nozzle: 240
compat:
  machines:
    - model: Original Prusa MINI & MINI+
      nozzle_mm: 0.4
    - model: Original Prusa XL
      nozzle_mm: 0.6
`)
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"render", "--provider", "prusa", "--kind", "filament", "--in", input, "--out", outDir}, &stdout, &stderr); err != nil {
		t.Fatalf("run render: %v\nstderr:\n%s", err, stderr.String())
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("output files = %d, want 1 (prusa binds compatibility inside the preset)", len(entries))
	}

	data, err := os.ReadFile(filepath.Join(outDir, "Test PETG.ini"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	body := string(data)
	// PETG is spelled PET by Prusa: the vocabulary map must survive to the file.
	for _, want := range []string{
		"filament_type = PET\n",
		"temperature = 240\n",
		// Two machines, one expression: alternatives joined by `or`, each
		// parenthesised because `and` binds tighter.
		`compatible_printers_condition = (printer_model=="MINI" and nozzle_diameter[0]==0.4) ` +
			`or (printer_model=="XL" and nozzle_diameter[0]==0.6)` + "\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("output missing %q:\n%s", want, body)
		}
	}
}

// The section header is the whole difference between a printer preset and a
// whole configuration: a headerless INI makes PrusaSlicer adopt the file as the
// print AND the filament AND the printer at once (prusa.MarshalBundle).
func TestRenderPrusaMachineWritesAPrinterBundle(t *testing.T) {
	input := writeFixture(t, "machine.yaml", `kind: machine
schema_version: 1
meta:
  name: Some MINI
  model: Original Prusa MINI & MINI+
nozzle:
  diameter_mm: 0.4
volume:
  width_mm: 180
  depth_mm: 180
  height_mm: 180
`)
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"render", "--provider", "prusa", "--kind", "machine", "--in", input, "--out", outDir, "--inherits", "Original Prusa MINI & MINI+"}, &stdout, &stderr); err != nil {
		t.Fatalf("run render: %v\nstderr:\n%s", err, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(outDir, "Some MINI.ini"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	body := string(data)
	for _, want := range []string{
		"[printer:Some MINI]\n",
		// The catalogue id, not the human model name the author wrote.
		"printer_model = MINI\n",
		"bed_shape = 0x0,180x0,180x180,0x180\n",
		"inherits = Original Prusa MINI & MINI+\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("output missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "[filament:") {
		t.Error("a machine must not be installed in the filament slot")
	}
}

// A machine PrusaSlicer does not know cannot be rendered: emitting the author's
// spelling as printer_model would produce a preset that imports and binds to
// nothing.
func TestRenderPrusaMachineRefusesUnknownModel(t *testing.T) {
	input := writeFixture(t, "machine.yaml", `kind: machine
schema_version: 1
meta:
  name: Some Printer
  model: Voron 2.4 350
nozzle:
  diameter_mm: 0.4
`)
	var stdout, stderr bytes.Buffer
	err := Run([]string{"render", "--provider", "prusa", "--kind", "machine", "--in", input, "--out", t.TempDir()}, &stdout, &stderr)
	if err == nil {
		t.Fatal("a machine outside the prusa catalogue must be refused")
	}
	if !strings.Contains(err.Error(), "meta.model") {
		t.Errorf("error = %q, want it to name the offending field", err)
	}
}

func TestRenderUnknownProviderStillRejected(t *testing.T) {
	input := writeFixture(t, "filament.yaml", "kind: filament\nschema_version: 1\nmeta:\n  name: X\n")
	var stdout, stderr bytes.Buffer
	err := Run([]string{"render", "--provider", "cura", "--kind", "filament", "--in", input, "--out", t.TempDir()}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unsupported provider") {
		t.Errorf("error = %v, want unsupported provider", err)
	}
}
