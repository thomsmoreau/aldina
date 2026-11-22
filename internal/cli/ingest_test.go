// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

// ingest_test.go — the migration loop end to end (docs/adr/0001, the demo):
// an Orca filament JSON becomes a canonical YAML, then renders to a Prusa .ini.
// The machine is chosen from BOTH catalogues (the MINI), so the canonical
// reference crosses the provider boundary through the alias table — exactly
// the migration the feature promises.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIngestMigrationLoop(t *testing.T) {
	src := writeFixture(t, "orca-filament.json", `{
  "name": "Round Trip PETG @Prusa MINI 0.4 nozzle",
  "filament_type": ["PETG"],
  "filament_vendor": ["Overture"],
  "nozzle_temperature": ["240"],
  "hot_plate_temp": ["80"],
  "fan_min_speed": ["30"],
  "filament_retraction_length": ["1.4"],
  "compatible_printers": ["Prusa MINI 0.4 nozzle"],
  "inherits": "",
  "version": "2.4.0.0",
  "filament_settings_id": ["Round Trip PETG @Prusa MINI 0.4 nozzle"]
}`)

	// Step 1: ingest → canonical YAML + report.
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"ingest", "--in", src, "--out", outDir}, &stdout, &stderr); err != nil {
		t.Fatalf("ingest: %v\nstderr:\n%s", err, stderr.String())
	}

	// The canonical file is named after the suffix-stripped source name.
	yamlPath := filepath.Join(outDir, "Round Trip PETG.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read canonical: %v", err)
	}
	if !strings.Contains(string(data), "kind: filament") {
		t.Errorf("canonical YAML missing kind:\n%s", data)
	}
	if !strings.Contains(stdout.String(), yamlPath) {
		t.Errorf("stdout = %q, want the YAML path", stdout.String())
	}
	if !strings.Contains(stderr.String(), "transfer") {
		t.Errorf("stderr = %q, want the summary headline", stderr.String())
	}

	// Step 2: render the ingested YAML to Prusa — the other provider.
	prusaDir := t.TempDir()
	var stdout2, stderr2 bytes.Buffer
	if err := Run([]string{"render", "--provider", "prusa", "--kind", "filament", "--in", yamlPath, "--out", prusaDir}, &stdout2, &stderr2); err != nil {
		t.Fatalf("render prusa: %v\nstderr:\n%s", err, stderr2.String())
	}
	ini, err := os.ReadFile(filepath.Join(prusaDir, "Round Trip PETG.ini"))
	if err != nil {
		t.Fatalf("read prusa ini: %v", err)
	}
	// The authored value crossed the boundary and landed on the Prusa side.
	if !strings.Contains(string(ini), "temperature = 240") {
		t.Errorf("prusa ini missing authored temperature:\n%s", ini)
	}
}

// The reverse migration (spec #6): one Prusa bundle carries a library of
// presets, so ingest writes one canonical YAML per preset and reports each.
// A preset that fails — refused (inherits) or unwritable (a name that would
// escape --out) — fails alone: the others still land, and the command fails
// so a script never mistakes a partial migration for a whole.
func TestIngestPrusaPartialBundle(t *testing.T) {
	src := writeFixture(t, "bundle.ini", `[filament:Child PLA]
inherits = Parent PLA
temperature = 210

[filament:Plain PLA]
temperature = 205

[filament:../Escape]
temperature = 200

[filament:Plain PETG]
temperature = 240
`)
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := Run([]string{"ingest", "--provider", "prusa", "--in", src, "--out", outDir}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), `"Child PLA"`) || !strings.Contains(err.Error(), `"../Escape"`) {
		t.Fatalf("ingest: err = %v, want both failed presets named\nstderr:\n%s", err, stderr.String())
	}
	for _, name := range []string{"Plain PLA", "Plain PETG"} {
		path := filepath.Join(outDir, name+".yaml")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("preset %q not written: %v", name, err)
		}
		if !strings.Contains(stdout.String(), path) {
			t.Errorf("stdout = %q, want %s", stdout.String(), path)
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "Child PLA.yaml")); !os.IsNotExist(err) {
		t.Errorf("refused preset written anyway (stat err = %v)", err)
	}
	// Every preset is named on stderr; a report for each ingested one.
	report := stderr.String()
	for _, want := range []string{`preset "Child PLA"`, "Parent PLA", `preset "Plain PLA"`, `preset "../Escape"`, "unsafe preset name", `preset "Plain PETG"`} {
		if !strings.Contains(report, want) {
			t.Errorf("stderr missing %q:\n%s", want, report)
		}
	}
	if n := strings.Count(report, "summary: "); n != 3 {
		t.Errorf("got %d summaries, want 3 (one per ingested preset):\n%s", n, report)
	}
}

// The king test through the CLI: prusa bundle → N canonical YAML → orca JSON,
// and every authored value lands on the Orca side. PET comes back as PETG.
func TestIngestPrusaToOrcaPipeline(t *testing.T) {
	src := writeFixture(t, "bundle.ini", `[filament:Migrated PET]
filament_type = PET
temperature = 235
first_layer_temperature = 240
min_fan_speed = 30
extrusion_multiplier = 0.95
compatible_printers = "Original Prusa MINI & MINI+"

[filament:Migrated PLA]
filament_type = PLA
temperature = 205
first_layer_temperature = 210
min_fan_speed = 100
extrusion_multiplier = 1
compatible_printers = "Original Prusa MINI & MINI+"

[print:Ignored Print]
layer_height = 0.2
`)
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"ingest", "--provider", "prusa", "--in", src, "--out", outDir}, &stdout, &stderr); err != nil {
		t.Fatalf("ingest: %v\nstderr:\n%s", err, stderr.String())
	}

	cases := []struct {
		preset                          string
		material, nozzle, first, fanMin string
		flow                            string
	}{
		{"Migrated PET", "PETG", "235", "240", "30", "0.95"},
		{"Migrated PLA", "PLA", "205", "210", "100", "1"},
	}
	for _, c := range cases {
		t.Run(c.preset, func(t *testing.T) {
			orcaDir := t.TempDir()
			var stdout, stderr bytes.Buffer
			yamlPath := filepath.Join(outDir, c.preset+".yaml")
			if err := Run([]string{"render", "--provider", "orca", "--kind", "filament", "--in", yamlPath, "--out", orcaDir}, &stdout, &stderr); err != nil {
				t.Fatalf("render orca: %v\nstderr:\n%s", err, stderr.String())
			}
			files, err := filepath.Glob(filepath.Join(orcaDir, "*.json"))
			if err != nil || len(files) != 1 {
				t.Fatalf("orca outputs = %v (err %v), want exactly one", files, err)
			}
			data, err := os.ReadFile(files[0])
			if err != nil {
				t.Fatalf("read orca json: %v", err)
			}
			var got struct {
				FilamentType      []string `json:"filament_type"`
				NozzleTemperature []string `json:"nozzle_temperature"`
				NozzleFirstLayer  []string `json:"nozzle_temperature_initial_layer"`
				FanMinSpeed       []string `json:"fan_min_speed"`
				FlowRatio         []string `json:"filament_flow_ratio"`
			}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("decode orca json: %v", err)
			}
			checks := []struct {
				key  string
				got  []string
				want string
			}{
				{"filament_type", got.FilamentType, c.material},
				{"nozzle_temperature", got.NozzleTemperature, c.nozzle},
				{"nozzle_temperature_initial_layer", got.NozzleFirstLayer, c.first},
				{"fan_min_speed", got.FanMinSpeed, c.fanMin},
				{"filament_flow_ratio", got.FlowRatio, c.flow},
			}
			for _, k := range checks {
				if len(k.got) == 0 || k.got[0] != k.want {
					t.Errorf("%s = %v, want [%s]", k.key, k.got, k.want)
				}
			}
		})
	}
}

func TestIngestUnsupportedSource(t *testing.T) {
	src := writeFixture(t, "any.json", `{}`)
	cases := []struct {
		name, provider, kind, want string
	}{
		{"provider", "cura", "filament", `unsupported provider "cura"`},
		{"orca kind", "orca", "machine", `unsupported kind "machine"`},
		{"prusa kind", "prusa", "machine", `unsupported kind "machine"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Run([]string{"ingest", "--provider", c.provider, "--kind", c.kind, "--in", src, "--out", t.TempDir()}, &stdout, &stderr)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
}
