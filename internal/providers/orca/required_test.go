// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

import (
	"strings"
	"testing"

	"github.com/thomsmoreau/Aldina/internal/canonical"
)

// meta.name is required PRESENT AND NON-EMPTY: nil and "" are different
// canonical states (absent key vs authored empty string) but neither can
// produce an importable preset, so both must be caught.
func TestRequiredFilamentName(t *testing.T) {
	cases := []struct {
		name        string
		metaName    *string
		wantMissing bool
	}{
		{name: "absent key", metaName: nil, wantMissing: true},
		{name: "authored empty string", metaName: new(""), wantMissing: true},
		{name: "named", metaName: new("Overture PLA"), wantMissing: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := canonical.Filament{Meta: canonical.Meta{Name: c.metaName}}
			missing := requiredFilament(f)
			if got := len(missing) != 0; got != c.wantMissing {
				t.Fatalf("requiredFilament = %v, want missing=%v", missing, c.wantMissing)
			}
			if c.wantMissing && missing[0] != "meta.name" {
				t.Errorf("requiredFilament = %v, want [meta.name]", missing)
			}
		})
	}
}

func TestRequiredMachineName(t *testing.T) {
	if missing := requiredMachine(canonical.Machine{}); len(missing) != 1 || missing[0] != "meta.name" {
		t.Errorf("requiredMachine = %v, want [meta.name]", missing)
	}
	named := canonical.Machine{Meta: canonical.MachineMeta{Name: new("Bare 0.4")}}
	if missing := requiredMachine(named); len(missing) != 0 {
		t.Errorf("requiredMachine = %v, want none", missing)
	}
}

// Fields that are merely advisory must NOT block: temps.nozzle is strongly
// recommended for a good print but does not stop Orca importing the preset,
// so it stays a canonical warning and never reaches the required list.
func TestAdvisoryFieldsDoNotBlockRender(t *testing.T) {
	f := baseFilament()
	f.Temps.Nozzle = nil
	f.Physical.DiameterMM = nil
	f.Meta.Material = nil

	if _, _, err := RenderFilament(f, p2s06); err != nil {
		t.Errorf("RenderFilament = %v, want nil: advisory gaps must not block", err)
	}
}

// The error must name the offending canonical path: "something is missing"
// is not actionable.
func TestRenderFilamentErrorNamesThePath(t *testing.T) {
	f := baseFilament()
	f.Meta.Name = nil
	_, _, err := RenderFilament(f, p2s06)
	if err == nil {
		t.Fatal("RenderFilament = nil, want an error")
	}
	if !strings.Contains(err.Error(), "meta.name") {
		t.Errorf("error = %q, want it to name meta.name", err)
	}
}
