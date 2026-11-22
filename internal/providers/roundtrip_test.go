// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers_test

// roundtrip_test.go — THE king test (CONTRIBUTING.md §Tests).
//
// One canonical value, rendered to Orca AND Prusa, correct on both sides. It
// is the only test that can prove the canonical format is genuinely
// slicer-agnostic rather than a disguised mirror of whichever provider was
// written first — and the only one that catches an encoding INVERSION, where
// each provider is self-consistent and the pair is wrong.
//
// It lives in providers_test (external package) on purpose: importing both
// orca and prusa from either one would create the very coupling the
// architecture forbids (one-way providers → canonical, never sideways).

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/providers/orca"
	"github.com/thomsmoreau/Aldina/internal/providers/prusa"
)

// One of the ~74 machines both catalogues know, so a single canonical
// reference can be resolved on both sides. Spelled the PrusaSlicer way here;
// the Orca catalogue reaches it through the alias table.
var mini = canonical.MachineRef{Model: "Original Prusa MINI & MINI+", NozzleMM: 0.4}

// parseINI turns INI bytes into a key → value map, dropping section headers
// and comments. The separator is " = " exactly (see prusa/ini.go).
func parseINI(raw []byte) map[string]string {
	ini := make(map[string]string)
	for _, line := range strings.Split(string(raw), "\n") {
		if k, v, ok := strings.Cut(line, " = "); ok {
			ini[k] = v
		}
	}
	return ini
}

// render returns the two wire forms of one canonical filament.
func render(t *testing.T, f canonical.Filament) (orca.FilamentSettings, map[string]string) {
	t.Helper()

	orcaOut, _, err := orca.RenderFilament(f, mini)
	if err != nil {
		t.Fatalf("orca.RenderFilament: %v", err)
	}
	prusaOut, _, err := prusa.RenderFilament(f)
	if err != nil {
		t.Fatalf("prusa.RenderFilament: %v", err)
	}
	raw, err := prusa.MarshalBundle("filament", "Round Trip", prusaOut)
	if err != nil {
		t.Fatalf("prusa.Marshal: %v", err)
	}
	return orcaOut, parseINI(raw)
}

func base() canonical.Filament {
	return canonical.Filament{
		Meta:   canonical.Meta{Name: new("Round Trip"), Material: new(canonical.PETG)},
		Temps:  canonical.Temps{Nozzle: new(240.0), Bed: new(80.0)},
		Compat: canonical.Compat{Machines: []canonical.MachineRef{mini}},
	}
}

// miniOrcaCompat is the exact compatible_printers the MINI fixture yields in
// Orca: the catalogue preset name, then the self-link name.
func miniOrcaCompat() []string {
	return []string{"Prusa MINI 0.4 nozzle", "Original Prusa MINI & MINI+ 0.4"}
}

// The linkage counterpart of the shrinkage case: ONE canonical machine
// reference, two bindings with nothing in common on the wire — Orca names the
// preset, Prusa compares an id it never shows the author — and both must land
// on the same physical printer.
//
// No single-provider test can prove this. Each side is self-consistent while
// carrying the other's vocabulary: that is exactly how a filament ended up
// declaring `compatible_printers = Bambu Lab P2S 0.6 nozzle` in a PrusaSlicer
// file, importing cleanly and then never appearing.
func TestOneMachineReferenceBindsOnBothSides(t *testing.T) {
	orcaOut, prusaINI := render(t, base())

	// Orca: its OWN catalogue name for that machine — not the spelling the
	// author used, which is PrusaSlicer's. The alias table bridged them. The
	// second entry is the self-link name (mapping.md, machine pitfall #1).
	if want := miniOrcaCompat(); !reflect.DeepEqual(orcaOut.CompatiblePrinters, want) {
		t.Errorf("orca compatible_printers = %v, want %v", orcaOut.CompatiblePrinters, want)
	}
	// Prusa: an expression over the printer's own fields, and NO name list.
	if got := prusaINI["compatible_printers_condition"]; got != `printer_model=="MINI" and nozzle_diameter[0]==0.4` {
		t.Errorf("prusa compatible_printers_condition = %q", got)
	}
	if got, ok := prusaINI["compatible_printers"]; ok && got != "" {
		t.Errorf("prusa compatible_printers = %q, want omitted", got)
	}
	// And the proof the two are not the same string in disguise: neither side
	// carries the other's spelling.
	if strings.Contains(prusaINI["compatible_printers_condition"], "MINI &") {
		t.Error("prusa condition leaked the human model name instead of the id")
	}
}

// The bug this slice removes, stated as a test: an Orca catalogue name must not
// be renderable into a Prusa file. It is not a Prusa machine, so it stops.
func TestOrcaOnlyMachineCannotRenderToPrusa(t *testing.T) {
	f := base()
	f.Compat.Machines = []canonical.MachineRef{{Model: "Bambu Lab P2S", NozzleMM: 0.6}}

	if _, _, err := orca.RenderFilament(f, f.Compat.Machines[0]); err != nil {
		t.Fatalf("orca knows this machine: %v", err)
	}
	_, _, err := prusa.RenderFilament(f)
	if err == nil {
		t.Fatal("prusa has no Bambu machine: want an error, got a preset that would import and never show up")
	}
	if !strings.Contains(err.Error(), "Bambu Lab P2S") {
		t.Errorf("error = %q, want it to name the machine", err)
	}
}

// ---------------------------------------------------------------------------
// Machine ↔ filament linkage
// ---------------------------------------------------------------------------

// baseMachine names the SAME machine as base(): the pair is what an author
// actually ships, and the whole point is that the two halves must meet.
func baseMachine() canonical.Machine {
	return canonical.Machine{
		Meta: canonical.MachineMeta{
			Name:  new("Round Trip MINI"),
			Model: new(mini.Model),
		},
		Nozzle: canonical.NozzleSpec{DiameterMM: new(mini.NozzleMM)},
		Volume: canonical.BuildVolume{WidthMM: new(180.0), DepthMM: new(180.0), HeightMM: new(180.0)},
	}
}

func renderMachine(t *testing.T, m canonical.Machine) (orca.MachineSettings, map[string]string) {
	t.Helper()

	orcaOut, _, err := orca.RenderMachine(m, "")
	if err != nil {
		t.Fatalf("orca.RenderMachine: %v", err)
	}
	prusaOut, _, err := prusa.RenderMachine(m, "")
	if err != nil {
		t.Fatalf("prusa.RenderMachine: %v", err)
	}
	raw, err := prusa.MarshalBundle("printer", *m.Meta.Name, prusaOut)
	if err != nil {
		t.Fatalf("prusa.MarshalBundle: %v", err)
	}
	return orcaOut, parseINI(raw)
}

// THE case for this slice, and the one no single-provider test can see: the
// filament and the machine are rendered independently, from two canonical
// files, by code paths that never meet — and the expression one emits must
// address the identity the other emits.
//
// This is the exact bug that made a rendered preset import cleanly and stay
// invisible: a condition comparing an id nothing in the bundle carried. It is
// pinned by REBUILDING the condition out of the machine's own rendered values,
// so the two can only agree by actually agreeing.
func TestFilamentConditionAddressesTheRenderedMachine(t *testing.T) {
	_, machineINI := renderMachine(t, baseMachine())
	_, filamentINI := render(t, base())

	want := fmt.Sprintf("printer_model==%q and nozzle_diameter[0]==%s",
		machineINI["printer_model"], machineINI["nozzle_diameter"])
	if got := filamentINI["compatible_printers_condition"]; got != want {
		t.Errorf("the filament does not address the machine we render\n got: %s\nwant: %s", got, want)
	}
	// And the identity is the catalogue id, not the human label the author
	// wrote — otherwise the two would agree on a value PrusaSlicer never tests.
	if machineINI["printer_model"] != "MINI" {
		t.Errorf("machine printer_model = %q, want the catalogue id", machineINI["printer_model"])
	}
}

// The Orca counterpart, pinned as it actually behaves — and it does NOT match
// the Prusa side. Two distinct reasons, only one of which is by design:
//
//  1. By design: Orca binds a filament by exact PRESET NAME, so the filament
//     targets the machine's catalogue preset ("Prusa MINI 0.4 nozzle") while
//     the machine we render carries the author's own label. The linkage is
//     per-format (docs/mapping.md pitfall #1) and only the canonical key —
//     model + nozzle — is shared.
//  2. Debt: orca.RenderMachine emits meta.model VERBATIM, so a canonical
//     written with PrusaSlicer's spelling produces an Orca machine whose
//     printer_model is not Orca's own catalogue name. prusa.RenderMachine
//     resolves the same field through the catalogue;
//     changing Orca is its own slice, not this one.
//
// This test exists so that divergence is a written fact rather than a surprise.
func TestOrcaMachineModelIsNotCatalogueResolved(t *testing.T) {
	orcaMachine, prusaMachineINI := renderMachine(t, baseMachine())
	orcaFilament, _ := render(t, base())

	if orcaMachine.Name != "Round Trip MINI" {
		t.Errorf("orca machine name = %q, want the author's label", orcaMachine.Name)
	}
	if want := miniOrcaCompat(); !reflect.DeepEqual(orcaFilament.CompatiblePrinters, want) {
		t.Fatalf("orca compatible_printers = %v, want %v", orcaFilament.CompatiblePrinters, want)
	}
	if orcaFilament.CompatiblePrinters[0] == orcaMachine.Name {
		t.Error("if these ever match, the linkage model changed — revisit mapping.md pitfall #1")
	}
	// The self-link name is the MODEL string, not the author's label: a machine
	// titled anything but "<model> <nozzle>" is not self-linked yet.
	if orcaFilament.CompatiblePrinters[1] == orcaMachine.Name {
		t.Error("self-link name matched the label — the naming model changed, revisit mapping.md")
	}

	// The debt, stated as an assertion: same canonical field, two policies.
	if orcaMachine.PrinterModel != "Original Prusa MINI & MINI+" {
		t.Errorf("orca printer_model = %q, want meta.model verbatim", orcaMachine.PrinterModel)
	}
	if prusaMachineINI["printer_model"] != "MINI" {
		t.Errorf("prusa printer_model = %q, want the catalogue id", prusaMachineINI["printer_model"])
	}
}
