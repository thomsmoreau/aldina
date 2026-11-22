// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers_test

// conformance_test.go — the harness that makes conformance.yaml checkable.
//
// Every case in the YAML is rendered to BOTH providers and asserted against
// the stated wire forms. The YAML is the single source of truth; this file
// only reads it and checks it. The one engine-ish thing here is the builder
// registry: a typed setter per concept, because a YAML value cannot set a Go
// struct field on its own — no reflection, no eval, just ~10 one-liners.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	_ "embed"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/providers"
	"github.com/thomsmoreau/Aldina/internal/providers/orca"
	"github.com/thomsmoreau/Aldina/internal/providers/prusa"
)

//go:embed conformance.yaml
var conformanceYAML []byte

func loadTable(t *testing.T) providers.ConformanceTable {
	t.Helper()
	tab, err := providers.ParseConformance(conformanceYAML)
	if err != nil {
		t.Fatalf("conformance.yaml: %v", err)
	}
	return tab
}

// ---------------------------------------------------------------------------
// Builders — the typed binding from a concept name to its canonical field.
// ---------------------------------------------------------------------------

// setFloatPtr sets *dst from a YAML number, or to nil when the value is null
// (absence, not zero).
func setFloatPtr(v any, dst **float64) error {
	if v == nil {
		*dst = nil
		return nil
	}
	n, err := num(v)
	if err != nil {
		return err
	}
	*dst = &n
	return nil
}

func num(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("want a number, got %T", v)
	}
}

var filamentBuilders = map[string]func(*canonical.Filament, any) error{
	"temps.nozzle": func(f *canonical.Filament, v any) error {
		return setFloatPtr(v, &f.Temps.Nozzle)
	},
	"temps.bed": func(f *canonical.Filament, v any) error {
		return setFloatPtr(v, &f.Temps.Bed)
	},
	"cooling.fan_min": func(f *canonical.Filament, v any) error {
		return setFloatPtr(v, &f.Cooling.FanMin)
	},
	"cooling.fan_max": func(f *canonical.Filament, v any) error {
		return setFloatPtr(v, &f.Cooling.FanMax)
	},
	"physical.shrinkage_xy": func(f *canonical.Filament, v any) error {
		return setFloatPtr(v, &f.Physical.ShrinkageXY)
	},
	"meta.material": func(f *canonical.Filament, v any) error {
		if v == nil {
			f.Meta.Material = nil
			return nil
		}
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("meta.material: want a string, got %T", v)
		}
		m := canonical.Material(s)
		f.Meta.Material = &m
		return nil
	},
	"retraction.length_mm": func(f *canonical.Filament, v any) error {
		return setFloatPtr(v, &f.Retraction.LengthMM)
	},
}

var machineBuilders = map[string]func(*canonical.Machine, any) error{
	"nozzle.diameter_mm": func(m *canonical.Machine, v any) error {
		return setFloatPtr(v, &m.Nozzle.DiameterMM)
	},
	"volume.height_mm": func(m *canonical.Machine, v any) error {
		return setFloatPtr(v, &m.Volume.HeightMM)
	},
	"nozzle.min_layer_height_mm": func(m *canonical.Machine, v any) error {
		return setFloatPtr(v, &m.Nozzle.MinLayerHeightMM)
	},
	"nozzle.max_layer_height_mm": func(m *canonical.Machine, v any) error {
		return setFloatPtr(v, &m.Nozzle.MaxLayerHeightMM)
	},
	"retract.speed_mms": func(m *canonical.Machine, v any) error {
		return setFloatPtr(v, &m.Retract.SpeedMMS)
	},
}

// ---------------------------------------------------------------------------
// Render helpers — wire forms as comparable maps.
// ---------------------------------------------------------------------------

// orcaMap marshals a rendered Orca struct back to its wire JSON, so the
// assertion compares the same key names the real export uses.
func orcaMap(t *testing.T, o any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return m
}

func renderFilamentWire(t *testing.T, f canonical.Filament) (map[string]any, map[string]string) {
	t.Helper()
	o, _, err := orca.RenderFilament(f, mini)
	if err != nil {
		t.Fatalf("orca.RenderFilament: %v", err)
	}
	p, _, err := prusa.RenderFilament(f)
	if err != nil {
		t.Fatalf("prusa.RenderFilament: %v", err)
	}
	raw, err := prusa.MarshalBundle("filament", "Round Trip", p)
	if err != nil {
		t.Fatalf("prusa.Marshal: %v", err)
	}
	return orcaMap(t, o), parseINI(raw)
}

func renderMachineWire(t *testing.T, m canonical.Machine) (map[string]any, map[string]string) {
	t.Helper()
	o, _, err := orca.RenderMachine(m, "")
	if err != nil {
		t.Fatalf("orca.RenderMachine: %v", err)
	}
	p, _, err := prusa.RenderMachine(m, "")
	if err != nil {
		t.Fatalf("prusa.RenderMachine: %v", err)
	}
	raw, err := prusa.MarshalBundle("printer", *m.Meta.Name, p)
	if err != nil {
		t.Fatalf("prusa.MarshalBundle: %v", err)
	}
	return orcaMap(t, o), parseINI(raw)
}

// ---------------------------------------------------------------------------
// Assertions — the YAML map is a subset of the wire map; null means absent.
// ---------------------------------------------------------------------------

func assertOrcaWire(t *testing.T, concept string, want, got map[string]any) {
	t.Helper()
	for k, wv := range want {
		gv, ok := got[k]
		if wv == nil {
			if ok {
				t.Errorf("%s: orca %s should be absent, got %v", concept, k, gv)
			}
			continue
		}
		if !ok {
			t.Errorf("%s: orca %s missing, want %v", concept, k, wv)
			continue
		}
		if !reflect.DeepEqual(gv, wv) {
			t.Errorf("%s: orca %s = %v, want %v", concept, k, gv, wv)
		}
	}
}

func assertPrusaWire(t *testing.T, concept string, want map[string]any, got map[string]string) {
	t.Helper()
	for k, wv := range want {
		gv, ok := got[k]
		if wv == nil {
			if ok {
				t.Errorf("%s: prusa %s should be absent, got %q", concept, k, gv)
			}
			continue
		}
		wantStr, ok := wv.(string)
		if !ok {
			t.Fatalf("%s: prusa %s expectation must be a string, got %T", concept, k, wv)
		}
		if !ok || gv != wantStr {
			t.Errorf("%s: prusa %s = %q, want %q", concept, k, gv, wantStr)
		}
	}
}

// ---------------------------------------------------------------------------
// Drivers — one explicit loop per resource (no generic engine).
// ---------------------------------------------------------------------------

func TestConformanceFilament(t *testing.T) {
	for _, c := range loadTable(t).Filament {
		t.Run(c.Concept, func(t *testing.T) {
			b, ok := filamentBuilders[c.Concept]
			if !ok {
				t.Fatalf("no builder for concept %q", c.Concept)
			}
			f := base()
			if err := b(&f, c.Value); err != nil {
				t.Fatalf("build %s: %v", c.Concept, err)
			}
			o, ini := renderFilamentWire(t, f)
			assertOrcaWire(t, c.Concept, c.Orca, o)
			assertPrusaWire(t, c.Concept, c.Prusa, ini)
		})
	}
}

func TestConformanceMachine(t *testing.T) {
	for _, c := range loadTable(t).Machine {
		t.Run(c.Concept, func(t *testing.T) {
			b, ok := machineBuilders[c.Concept]
			if !ok {
				t.Fatalf("no builder for concept %q", c.Concept)
			}
			m := baseMachine()
			if err := b(&m, c.Value); err != nil {
				t.Fatalf("build %s: %v", c.Concept, err)
			}
			o, ini := renderMachineWire(t, m)
			assertOrcaWire(t, c.Concept, c.Orca, o)
			assertPrusaWire(t, c.Concept, c.Prusa, ini)
		})
	}
}
