// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package catalog

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The two machines the maintainer actually owns, one per slicer: if these break
// the snapshot is wrong in a way that matters immediately.
func TestLookupExactModel(t *testing.T) {
	got, err := Lookup(Orca, "Bambu Lab P2S", 0.6)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(got.Presets) != 1 || got.Presets[0] != "Bambu Lab P2S 0.6 nozzle" {
		t.Errorf("presets = %v, want [Bambu Lab P2S 0.6 nozzle]", got.Presets)
	}

	mini, err := Lookup(Prusa, "Original Prusa MINI & MINI+", 0.4)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	// The id, not the name, is what a compatible_printers_condition compares.
	if mini.ModelID != "MINI" {
		t.Errorf("model_id = %q, want MINI", mini.ModelID)
	}
}

// The whole point of aliases.json: the author writes ONE spelling and both
// renders resolve, whichever slicer's catalogue they took it from.
func TestLookupThroughAlias(t *testing.T) {
	// Orca's spelling, queried against Prusa's catalogue.
	got, err := Lookup(Prusa, "Prusa MINI", 0.4)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got.ModelID != "MINI" {
		t.Errorf("model_id = %q, want MINI", got.ModelID)
	}
	// And the reverse: Prusa's spelling against Orca's catalogue.
	if _, err := Lookup(Orca, "Original Prusa MINI & MINI+", 0.4); err != nil {
		t.Errorf("reverse alias failed: %v", err)
	}
}

// Orca binds by name and a few models ship several presets for one nozzle. All
// of them come back: the filament holds for every variant of the same hardware.
func TestOneKeyCanHoldSeveralPresets(t *testing.T) {
	got, err := Lookup(Orca, "Raise3D Pro3", 0.4)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(got.Presets) < 2 {
		t.Fatalf("presets = %v, want the Left/Right/Dual variants", got.Presets)
	}
}

// Prusa writes printer_variant = 0.50 where Orca writes 0.5. The generator
// parses it as a number precisely so this resolves.
func TestNozzleIsComparedNumerically(t *testing.T) {
	if _, err := Lookup(Prusa, "M3 - Independent Dual Rev.1", 0.5); err != nil {
		t.Errorf("0.50 in the source must match 0.5: %v", err)
	}
}

// The most common failure by far — most models live in one catalogue only —
// so its message must say the machine is absent, not that the author mistyped.
func TestModelAbsentFromTheOtherCatalogue(t *testing.T) {
	_, err := Lookup(Prusa, "Bambu Lab P2S", 0.6)
	if err == nil {
		t.Fatal("Prusa's catalogue has no Bambu machine: want an error")
	}
	for _, want := range []string{"Bambu Lab P2S", "known to orca", "absent from prusa"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to mention %q", err, want)
		}
	}
}

func TestUnknownModel(t *testing.T) {
	_, err := Lookup(Orca, "Nonexistent Printer 9000", 0.4)
	if err == nil {
		t.Fatal("want an error for an invented model")
	}
	if !strings.Contains(err.Error(), "neither catalogue") ||
		!strings.Contains(err.Error(), "aldina-catalog") {
		t.Errorf("error = %q, want it to name both catalogues and the way to refresh them", err)
	}
}

// A known model with the wrong nozzle is a different mistake, and the fix is in
// the message: the sizes that DO exist.
func TestUnknownNozzleListsTheAvailableOnes(t *testing.T) {
	_, err := Lookup(Orca, "Bambu Lab P2S", 1.2)
	if err == nil {
		t.Fatal("want an error for an unavailable nozzle")
	}
	for _, want := range []string{"1.2", "0.2, 0.4, 0.6, 0.8"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	}
}

// aliases.json is hand-written and catalog.json is regenerated: an alias that
// stops resolving after a regeneration must fail here, not in a user's render.
func TestEveryAliasResolvesOnBothSides(t *testing.T) {
	orca := make(map[string]bool)
	for _, m := range Models(Orca) {
		orca[m] = true
	}
	prusa := make(map[string]bool)
	for _, m := range Models(Prusa) {
		prusa[m] = true
	}
	for _, a := range Aliases() {
		if !orca[a[0]] {
			t.Errorf("alias %q -> %q: left side is not an orca model", a[0], a[1])
		}
		if !prusa[a[1]] {
			t.Errorf("alias %q -> %q: right side is not a prusa model", a[0], a[1])
		}
	}
}

// An alias is only needed when the spellings differ; one mapping a name to
// itself is noise that hides the real divergences.
func TestNoRedundantAlias(t *testing.T) {
	for _, a := range Aliases() {
		if a[0] == a[1] {
			t.Errorf("alias %q is redundant: both catalogues already agree", a[0])
		}
	}
}

// A hotend variant must NOT answer a plain query. The maintainer's own CORE
// One is installed as HF0.4: silently returning it would bind a filament tuned
// for a stock hotend to a high-flow one, and silently dropping it at generation
// would make the machine invisible. It resolves to an error that names it.
func TestDecoratedVariantIsRefusedByName(t *testing.T) {
	// The plain 0.4 still resolves...
	got, err := Lookup(Prusa, "Prusa CORE One & CORE One+", 0.4)
	if err != nil {
		t.Fatalf("plain 0.4: %v", err)
	}
	if got.Variant != "0.4" {
		t.Errorf("variant = %q, want the plain one", got.Variant)
	}

	// ...while a diameter that exists ONLY as a hotend variant does not.
	_, err = Lookup(Prusa, "VORON2 250mm (Afterburner)", 1.2)
	if err == nil {
		t.Fatal("volcano-only nozzle must not resolve")
	}
	if !strings.Contains(err.Error(), "volcano") || !strings.Contains(err.Error(), "hotend class") {
		t.Errorf("error = %q, want it to name the variant and why it is refused", err)
	}
}

// Prusa writes "0.50", we would format "0.5": a textual plainness test would
// call that a hotend variant and refuse a perfectly ordinary machine.
func TestTrailingZeroVariantIsStillPlain(t *testing.T) {
	got, err := Lookup(Prusa, "M3 - Independent Dual Rev.1", 0.5)
	if err != nil {
		t.Fatalf("0.50 must be plain: %v", err)
	}
	if got.Variant != "0.50" {
		t.Errorf("variant = %q, want the raw 0.50 from the catalogue", got.Variant)
	}
}

// Snapshot is the ONE definition of the file format, shared with the generator
// that writes it. This test is what makes that safe: decode the committed file
// and re-encode it exactly as cmd/aldina-catalog does, and the bytes must match.
// A field reordered or a tag lost shows up here rather than as a 200 KB diff on
// the next regeneration.
func TestSnapshotRoundTripsByteForByte(t *testing.T) {
	want, err := os.ReadFile("catalog.json")
	if err != nil {
		t.Fatalf("read catalog.json: %v", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(want, &snap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !bytes.Equal(append(got, '\n'), want) {
		t.Error("re-encoding the committed catalogue does not reproduce it byte for byte")
	}
}

func TestUnknownSlicer(t *testing.T) {
	if _, err := Lookup(Slicer("cura"), "whatever", 0.4); err == nil {
		t.Error("want an error for an unknown slicer")
	}
}
