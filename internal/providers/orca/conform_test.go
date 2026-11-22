// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// conform_test.go — content conformance against a REAL OrcaSlicer binary.
//
// WHY this exists: golden files prove we render what we INTENDED. They cannot
// prove Orca understands it. This feeds the rendered preset to the slicer's own
// CLI and, on failure, reports Orca's log instead of leaving us to guess in the
// GUI.
//
// Opt-in: set ALDINA_ORCA_BIN to the binary. Everything here skips otherwise,
// so CI and contributors without OrcaSlicer stay green, and the run stays inside
// t.TempDir() — never the user's real config (CONTRIBUTING.md §Tests).
//
// SCOPE — read this before trusting a PASS. `--load-settings` is a CONTENT
// oracle, NOT the GUI "File > Import > Import Configs" gate. Two adaptations are
// applied before handing a preset over; both were measured on OrcaSlicer 2.4.2
// and both are properties of the CLI, not of the preset format:
//
//   - `type` is required by the CLI ("unknown config type of file ... in
//     load-settings") and is ABSENT from every real GUI-importable user preset
//     on disk. Our own filament preset, which the GUI imports without
//     complaint, fails the CLI until `type` is injected. So it is a CLI
//     artifact: added here, never rendered.
//   - a machine with `from: "User"` is refused by the CLI whatever else it
//     carries — verified against printer_settings_id, is_custom_defined,
//     print_compatible_printers and inherits_group, all of which changed
//     nothing. Filaments with `from: "User"` pass. Machines are therefore
//     handed over as "system" so their CONTENT gets checked. This is a CLI
//     limitation only: the GUI does import a `from: "User"` machine preset
//     (verified 2026-08-01, derived case).
//
// A PASS means: Orca parsed every key, resolved `inherits`, and sliced with the
// result. It does not mean the preset imports in the UI.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomsmoreau/Aldina/internal/canonical"
)

const orcaBinEnv = "ALDINA_ORCA_BIN"

// orcaBin returns the slicer binary, skipping the test when it is not opted in.
func orcaBin(t *testing.T) string {
	t.Helper()
	bin := os.Getenv(orcaBinEnv)
	if bin == "" {
		t.Skipf("%s not set: skipping OrcaSlicer conformance", orcaBinEnv)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("%s=%q: %v", orcaBinEnv, bin, err)
	}
	return bin
}

// seedDatadir builds a throwaway Orca data directory. `models` is the
// installed-printer gate: without it Orca cannot resolve `inherits` against the
// vendor profiles it ships, and reports the preset as incompatible.
//
// Written by hand on purpose — the real OrcaSlicer.conf also holds the printer
// serial, its LAN address and the user access code, none of which belong in a
// fixture.
func seedDatadir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	conf := map[string]any{
		"models": []map[string]string{{
			"vendor":          "BBL",
			"model":           "Bambu Lab P2S",
			"nozzle_diameter": "0.2;0.4;0.6;0.8",
		}},
	}
	writeJSONFile(t, filepath.Join(dir, "OrcaSlicer.conf"), conf)
	return dir
}

// asCLIPreset turns a rendered settings struct into the loosely typed form the
// CLI wants: marshal → map → inject the CLI-only keys. Round-tripping through
// the real JSON keeps the wire form under test rather than the Go field names.
func asCLIPreset(t *testing.T, settings any, kind string) map[string]any {
	t.Helper()
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal %s: %v", kind, err)
	}
	var preset map[string]any
	if err := json.Unmarshal(raw, &preset); err != nil {
		t.Fatalf("unmarshal %s: %v", kind, err)
	}
	preset["type"] = kind
	return preset
}

func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// loadProcessFixture returns the reference process preset, retargeted at the
// machine under test. Orca refuses to run without a process compatible with the
// printer, so one is required even when the machine is what we are checking.
func loadProcessFixture(t *testing.T, machineName string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "conform", "process.json"))
	if err != nil {
		t.Fatalf("read process fixture: %v", err)
	}
	var process map[string]any
	if err := json.Unmarshal(raw, &process); err != nil {
		t.Fatalf("decode process fixture: %v", err)
	}
	process["type"] = "process"
	process["compatible_printers"] = []string{machineName}
	return process
}

// runOrca executes one CLI conformance round and returns the settings Orca
// exported back. A missing export is the failure signal: Orca returns 0 even
// when it refuses the presets, so the exported file — not the exit code — is
// what says whether it got through.
func runOrca(t *testing.T, machine, process, filament map[string]any) map[string]any {
	t.Helper()
	bin := orcaBin(t)
	dir := seedDatadir(t)

	machinePath := filepath.Join(dir, "machine.json")
	processPath := filepath.Join(dir, "process.json")
	outPath := filepath.Join(dir, "out.json")
	logPath := filepath.Join(dir, "orca.log")
	writeJSONFile(t, machinePath, machine)
	writeJSONFile(t, processPath, process)

	// Machine first, process second: the CLI applies them in order and silently
	// ignores the settings of a reversed pair.
	args := []string{
		"--datadir", dir,
		"--debug", "5",
		"--logfile", logPath,
		"--load-settings", machinePath + ";" + processPath,
		"--export-settings", outPath,
	}
	if filament != nil {
		filamentPath := filepath.Join(dir, "filament.json")
		writeJSONFile(t, filamentPath, filament)
		args = append(args, "--load-filaments", filamentPath)
	}

	combined, _ := exec.Command(bin, args...).CombinedOutput()

	exported, err := os.ReadFile(outPath)
	if err != nil {
		orcaLog, _ := os.ReadFile(logPath)
		t.Fatalf("orca exported no settings: %v\n--- orca log ---\n%s\n--- stdout/stderr ---\n%s",
			err, lastLines(string(orcaLog), 12), strings.TrimSpace(string(combined)))
	}
	var out map[string]any
	if err := json.Unmarshal(exported, &out); err != nil {
		t.Fatalf("decode exported settings: %v", err)
	}
	return out
}

// lastLines keeps a failure message readable: Orca's trace log is thousands of
// lines and only the tail carries the verdict.
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// wantExported asserts a key survived the round trip through Orca. Values are
// compared as rendered JSON so a []string stays distinguishable from a scalar —
// the per-extruder array trap that bites every Orca field.
func wantExported(t *testing.T, out map[string]any, key string, want any) {
	t.Helper()
	got, ok := out[key]
	if !ok {
		t.Errorf("%s: absent from exported settings", key)
		return
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("%s = %s, want %s", key, gotJSON, wantJSON)
	}
}

// TestConformMachine is the answer to "does Orca actually understand our
// machine preset?". Until now that could only be asked by hand in the UI, and
// the UI answers with a single unactionable sentence.
func TestConformMachine(t *testing.T) {
	m := baseMachine()
	m.Meta.Name = new("Aldina Conformance 0.6")

	settings, _, err := RenderMachine(m, p2s06MachinePresetName)
	if err != nil {
		t.Fatalf("RenderMachine: %v", err)
	}

	machine := asCLIPreset(t, settings, "machine")
	machine["from"] = "system" // see SCOPE at the top of this file
	out := runOrca(t, machine, loadProcessFixture(t, settings.Name), nil)

	// Spot-check one field per encoding family: the scalar, the per-extruder
	// array, the XxY mini-format and the opaque gcode block. If these four
	// survive, the wire form is understood.
	wantExported(t, out, "printable_height", "256")
	wantExported(t, out, "nozzle_diameter", []string{"0.6"})
	wantExported(t, out, "printable_area", []string{"0x0", "256x0", "256x256", "0x256"})
	wantExported(t, out, "machine_start_gcode", "G90 ; start")
}

// TestConformFilament guards the one preset that is PROVEN to import in the
// Orca UI. Its content must keep passing: it is the reference point that makes
// a machine failure meaningful rather than ambiguous.
func TestConformFilament(t *testing.T) {
	settings, _, err := RenderFilament(baseFilament(), p2s06)
	if err != nil {
		t.Fatalf("RenderFilament: %v", err)
	}

	sysMachine := systemMachineFixture(t)
	filament := asCLIPreset(t, settings, "filament")
	filament["compatible_printers"] = []string{p2s06MachinePresetName}

	out := runOrca(t, sysMachine, loadProcessFixture(t, p2s06MachinePresetName), filament)

	wantExported(t, out, "nozzle_temperature", []string{"220"})
	wantExported(t, out, "filament_type", []string{"PLA"})
}

// systemMachineFixture builds a minimal machine that only exists to give the
// filament something to be compatible with. It is not under test.
func systemMachineFixture(t *testing.T) map[string]any {
	t.Helper()
	m := canonical.Machine{
		Meta:   canonical.MachineMeta{Name: new(p2s06MachinePresetName), Model: new("Bambu Lab P2S")},
		Volume: canonical.BuildVolume{WidthMM: floatP(256), DepthMM: floatP(256), HeightMM: floatP(256)},
		Nozzle: canonical.NozzleSpec{DiameterMM: floatP(0.6)},
	}
	settings, _, err := RenderMachine(m, "")
	if err != nil {
		t.Fatalf("RenderMachine fixture: %v", err)
	}
	preset := asCLIPreset(t, settings, "machine")
	preset["from"] = "system"
	return preset
}
