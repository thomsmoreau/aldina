// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidateReportsCompatibilityError(t *testing.T) {
	machine := writeFixture(t, "machine.yaml", `kind: machine
schema_version: 1
meta:
  name: Test machine
  model: Test
volume:
  width_mm: 100
  depth_mm: 100
  height_mm: 100
nozzle:
  diameter_mm: 0.4
limits:
  max_nozzle_temp_c: 250
`)
	filament := writeFixture(t, "filament.yaml", `kind: filament
schema_version: 1
meta:
  name: Test PETG
temps:
  nozzle: 255
`)
	var stdout, stderr bytes.Buffer
	err := Run([]string{"validate", "--machine", machine, "--filament", filament}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "limits.max_nozzle_temp_c: filament nozzle 255 °C exceeds machine max 250 °C") {
		t.Fatalf("run validate error = %v, want nozzle limit error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

// A compat warning means compatibility is degraded: a bare "OK" right after
// it would tell the reader the opposite, so the line carries the count.
func TestValidateOKCountsCompatWarnings(t *testing.T) {
	machine := writeFixture(t, "machine.yaml", `kind: machine
schema_version: 1
meta:
  name: Test machine
  model: Test
volume:
  width_mm: 100
  depth_mm: 100
  height_mm: 100
nozzle:
  diameter_mm: 0.4
limits:
  max_volumetric_mm3_s: 20
`)
	filament := writeFixture(t, "filament.yaml", `kind: filament
schema_version: 1
meta:
  name: Test PETG
flow:
  max_volumetric_mm3_s: 25
`)
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"validate", "--machine", machine, "--filament", filament}, &stdout, &stderr); err != nil {
		t.Fatalf("run validate: %v", err)
	}
	if got, want := stdout.String(), "compatibility: OK (1 warning)\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}
