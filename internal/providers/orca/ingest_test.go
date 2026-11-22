// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// ingest_test.go — the provider → canonical direction.
//
// Slice A pins the scaffold and the refuse seam: a file whose values live in a
// parent preset (non-empty inherits) is refused, never ingesting its deltas
// into an almost-empty canonical and a false success. Self-contained files
// yield a current-version canonical and an empty report (no field mapping yet).
//
// Fixtures are REAL exports copied from references/ (see docs/adr/0001;
// everything goes through testdata/, never the user's HOME).

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readIngestFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "ingest", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

// TestIngestRefusesInherits: the Elegoo export is a "- Copy" preset whose
// values live in "Elegoo PLA @System". Ingesting it would produce a false
// canonical, so it must fail with ErrInherits and no canonical, and the error
// must name the inherited-from preset so the user knows why.
func TestIngestRefusesInherits(t *testing.T) {
	f, report, err := IngestFilament(readIngestFixture(t, "inherits.json"))

	if !errors.Is(err, ErrInherits) {
		t.Fatalf("err = %v, want ErrInherits", err)
	}
	if f != nil {
		t.Error("canonical = non-nil, want nil on refuse")
	}
	if report != nil {
		t.Error("report = non-nil, want nil on refuse")
	}
	if !strings.Contains(err.Error(), "Elegoo PLA @System") {
		t.Errorf("err = %q, want it to name the inherited-from preset", err)
	}
}

// TestIngestSelfContained: the Overture export carries its own values
// (inherits: ""), so ingest yields a current-version canonical with zero
// issues — slice A maps nothing yet.
func TestIngestSelfContained(t *testing.T) {
	f, report, err := IngestFilament(readIngestFixture(t, "self-contained.json"))

	if err != nil {
		t.Fatalf("IngestFilament: %v", err)
	}
	if f == nil {
		t.Fatal("canonical = nil, want non-nil")
	}
	if f.Kind != "filament" {
		t.Errorf("Kind = %q, want \"filament\"", f.Kind)
	}
	if f.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", f.SchemaVersion)
	}
	if report == nil {
		t.Fatal("report = nil, want non-nil")
	}
	if report.Summary == "" {
		t.Error("report.Summary empty, want a one-line disposition")
	}
}

// TestIngestFractionalFanPercent: a hand-edited fractional percent lands on
// the exact ratio. "0.7"/100 in float64 is 0.006999999999999999, noise that
// would land in the canonical YAML; prusa already reads the same wire exactly.
func TestIngestFractionalFanPercent(t *testing.T) {
	raw := []byte(`{"name": "X", "fan_min_speed": ["0.7"], "fan_max_speed": ["33.3"]}`)

	f, _, err := IngestFilament(raw)
	if err != nil {
		t.Fatalf("IngestFilament: %v", err)
	}
	assertField(t, "cooling.fan_min", f.Cooling.FanMin, new(0.007))
	assertField(t, "cooling.fan_max", f.Cooling.FanMax, new(0.333))
}

// TestIngestMalformedJSON: garbage bytes are a decode failure, distinct from
// the inherits refusal.
func TestIngestMalformedJSON(t *testing.T) {
	_, _, err := IngestFilament([]byte("not json"))

	if err == nil {
		t.Fatal("err = nil, want a decode error")
	}
	if errors.Is(err, ErrInherits) {
		t.Fatal("err is ErrInherits, want a malformed-input error")
	}
}
