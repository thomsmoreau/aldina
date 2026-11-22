// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// conform_test.go — content conformance against a REAL PrusaSlicer binary.
//
// Same purpose as the Orca harness: golden files prove we render what we
// INTENDED, not that the slicer understands it. Here the oracle is direct —
// `--load` our file, `--save` what PrusaSlicer made of it, compare. Any value
// it silently reinterprets shows up as a diff.
//
// Opt-in: set ALDINA_PRUSA_BIN. Everything skips otherwise, and the run stays
// inside t.TempDir() — never the user's real config (CONTRIBUTING.md §Tests).
//
// This oracle is cleaner than Orca's: no `type` key to inject, no from=User
// restriction, no process preset needed. But the same caveat applies — it
// checks CONTENT. Only a real import proves the preset is usable.

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

const prusaBinEnv = "ALDINA_PRUSA_BIN"

func prusaBin(t *testing.T) string {
	t.Helper()
	bin := os.Getenv(prusaBinEnv)
	if bin == "" {
		t.Skipf("%s not set: skipping PrusaSlicer conformance", prusaBinEnv)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("%s=%q: %v", prusaBinEnv, bin, err)
	}
	return bin
}

var iniLine = regexp.MustCompile(`^([a-z0-9_]+) = (.*)$`)

func parseINI(t *testing.T, raw string) map[string]string {
	t.Helper()
	out := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		if m := iniLine.FindStringSubmatch(line); m != nil {
			out[m[1]] = m[2]
		}
	}
	return out
}

// roundTrip feeds an INI body to PrusaSlicer and returns what it wrote back.
// PrusaSlicer fills in every default, so the result is much larger than the
// input: only the keys we authored are compared.
func roundTrip(t *testing.T, body []byte) map[string]string {
	t.Helper()
	bin := prusaBin(t)
	dir := t.TempDir()

	in := filepath.Join(dir, "in.ini")
	out := filepath.Join(dir, "out.ini")
	if err := os.WriteFile(in, body, 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	cmd := exec.Command(bin, "--datadir", filepath.Join(dir, "data"), "--load", in, "--save", out)
	combined, err := cmd.CombinedOutput()

	saved, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("prusa saved nothing: %v (run: %v)\n--- output ---\n%s",
			readErr, err, strings.TrimSpace(string(combined)))
	}
	return parseINI(t, string(saved))
}

// TestConformFilamentRoundTrip is the point of the harness: every value we
// author must come back untouched. A slicer that silently reinterprets one of
// them would otherwise be invisible until a print goes wrong.
func TestConformFilamentRoundTrip(t *testing.T) {
	settings, _, err := RenderFilament(baseFilament())
	if err != nil {
		t.Fatalf("RenderFilament: %v", err)
	}
	body, err := marshalBody(settings)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got := roundTrip(t, body)
	for key, want := range parseINI(t, string(body)) {
		if have, ok := got[key]; !ok {
			t.Errorf("%s: dropped by prusa (we wrote %q)", key, want)
		} else if have != want {
			t.Errorf("%s: prusa rewrote %q as %q", key, want, have)
		}
	}
}

// The hard cases, isolated so a failure names the trap rather than a random
// key: '#' opening a value, a gcode blob holding BOTH a semicolon and a
// newline, a space-bearing scalar, and the quoted empty item.
func TestConformEscapingTraps(t *testing.T) {
	body, err := marshalBody(FilamentSettings{
		FilamentIdentity: FilamentIdentity{
			FilamentSettingsID: []string{""},
			FilamentColour:     "#F7F7F7",
			FilamentVendor:     "Forward AM",
			FilamentNotes:      []string{"line one\nline two"},
		},
		FilamentGcode: FilamentGcode{
			StartFilamentGcode: []string{`M900 K0.2 ; set K` + "\n" + `M862.3 P "XL"`},
		},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got := roundTrip(t, body)
	for key, want := range parseINI(t, string(body)) {
		if got[key] != want {
			t.Errorf("%s: wrote %q, prusa returned %q", key, want, got[key])
		}
	}
}

// The machine half of the round-trip. A printer preset is where the encoding
// traps concentrate: the comma-joined numeric vectors (machine_max_feedrate_x),
// the CSV point list (bed_shape) and the gcode blobs that carry ';' and '"'
// unquoted. If any of those were spelled with the string-list ';' instead,
// PrusaSlicer would reinterpret or drop the value here rather than in a print.
func TestConformMachineRoundTrip(t *testing.T) {
	m := baseMachine()
	m.Gcode.Start = new("M17 ; enable steppers\nM862.3 P \"MINI\"")
	m.Gcode.End = new("M104 S0 ; turn off temperature\nM84 ; disable motors")
	m.Kinetics.MinTravelMMS = new(0.0)
	m.Kinetics.MinExtrudeMMS = new(0.0)

	settings, _, err := RenderMachine(m, "")
	if err != nil {
		t.Fatalf("RenderMachine: %v", err)
	}
	body, err := marshalBody(settings)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got := roundTrip(t, body)
	for key, want := range parseINI(t, string(body)) {
		if have, ok := got[key]; !ok {
			t.Errorf("%s: dropped by prusa (we wrote %q)", key, want)
		} else if have != want {
			t.Errorf("%s: prusa rewrote %q as %q", key, want, have)
		}
	}
}

// Static check, no slicer needed beyond the reference export: every ini tag
// must name a real PrusaSlicer option. Catches a typo the moment it is typed,
// where a golden would happily freeze it forever.
func TestSchemaTagsExistInRealExport(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "references", "prusa",
		"filaments", "Ultrafuse PET @MINI - Detached.ini"))
	if err != nil {
		t.Fatalf("read reference export: %v", err)
	}
	known := parseINI(t, string(raw))

	settings, _, err := RenderFilament(baseFilament())
	if err != nil {
		t.Fatalf("RenderFilament: %v", err)
	}
	body, err := marshalBody(settings)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	for key := range parseINI(t, string(body)) {
		if _, ok := known[key]; !ok {
			t.Errorf("%s: not a key of the real prusa export — typo or invented field", key)
		}
	}
}

// Same check for the machine, but over the TYPE rather than a rendered
// instance: MachineSettings is a closed struct, so every tag it declares is a
// key we can emit, including those no fixture happens to set.
func TestMachineSchemaTagsExistInRealExport(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "references", "prusa",
		"printers", "Prusa_XL.ini"))
	if err != nil {
		t.Fatalf("read reference export: %v", err)
	}
	known := parseINI(t, string(raw))

	typ := reflect.TypeOf(MachineSettings{})
	for i := range typ.NumField() {
		key := typ.Field(i).Tag.Get("ini")
		if key == "" {
			t.Errorf("%s has no ini tag", typ.Field(i).Name)
			continue
		}
		if _, ok := known[key]; !ok {
			t.Errorf("%s: not a key of the real prusa printer export — typo or invented field", key)
		}
	}
}
