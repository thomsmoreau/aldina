// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/thomsmoreau/Aldina/internal/canonical"
	"github.com/thomsmoreau/Aldina/internal/providers"
)

// baseMachine mirrors examples/machine_prusa_mini_04.yaml. Copy and tweak in a
// table case, then regenerate with `go test ./internal/providers/prusa -update`.
func baseMachine() canonical.Machine {
	return canonical.Machine{
		Kind:          "machine",
		SchemaVersion: 1,
		Meta: canonical.MachineMeta{
			Name:   new("Prusa MINI+ 0.4"),
			Vendor: new("Prusa Research"),
			Model:  new("Original Prusa MINI & MINI+"),
			Tags:   []string{"PG"},
		},
		Volume: canonical.BuildVolume{
			WidthMM:  new(180.0),
			DepthMM:  new(180.0),
			HeightMM: new(180.0),
		},
		Nozzle: canonical.NozzleSpec{
			DiameterMM:         new(0.4),
			FilamentDiameterMM: new(1.75),
			MinLayerHeightMM:   new(0.05),
			MaxLayerHeightMM:   new(0.25),
		},
		Limits: canonical.MachineLimits{
			MaxNozzleTempC:    new(280.0),
			MaxBedTempC:       new(100.0),
			MaxVolumetricMM3S: new(15.0),
		},
		Kinetics: canonical.Kinetics{
			MaxSpeedMMS:  &canonical.Axes{X: 180, Y: 180, Z: 12, E: 80},
			MaxAccelMMS2: &canonical.Axes{X: 2500, Y: 2500, Z: 400, E: 2500},
			MaxJerkMMS:   &canonical.Axes{X: 8, Y: 8, Z: 2, E: 10},
		},
		Retract: canonical.MachineRetract{
			LengthMM:          new(3.2),
			SpeedMMS:          new(35.0),
			DeretractSpeedMMS: new(25.0),
			MinTravelMM:       new(1.5),
			ZHopMM:            new(0.2),
			OnLayerChange:     new(true),
			Wipe:              new(true),
		},
		Firmware: canonical.MachineFirmware{
			RelativeE:          new(true),
			FirmwareRetraction: new(false),
		},
		Features: canonical.MachineFeatures{Enclosed: new(false)},
		Gcode:    canonical.MachineGcode{Flavor: new(canonical.FlavorMarlin2)},
		Defaults: canonical.MachineDefaults{
			PrintProfile:    new("0.15mm QUALITY @MINI"),
			FilamentProfile: new("Prusament PLA @MINI"),
		},
	}
}

// ---------------------------------------------------------------------------
// Bricks
// ---------------------------------------------------------------------------

func TestRectToBedShape(t *testing.T) {
	want := "0x0,180x0,180x180,0x180"
	if got := rectToBedShape(new(180.0), new(180.0)); got != want {
		t.Errorf("rectToBedShape = %q, want %q", got, want)
	}
	// A corner needs BOTH dimensions: half a bed is no bed, not a 0-wide one.
	if got := rectToBedShape(new(180.0), nil); got != "" {
		t.Errorf("missing depth should omit the key, got %q", got)
	}
	if got := rectToBedShape(nil, new(180.0)); got != "" {
		t.Errorf("missing width should omit the key, got %q", got)
	}
}

// The separator trap this schema exists for: numeric vectors are comma-joined,
// string LISTS are ';'-joined. Writing a kinetics pair as []string would emit
// "180;180", which PrusaSlicer cannot read.
func TestPairUsesCommaNotSemicolon(t *testing.T) {
	if got := pair(180); got != "180,180" {
		t.Errorf("pair(180) = %q, want %q", got, "180,180")
	}
	if got := escapeStrings([]string{"180", "180"}); got != "180;180" {
		t.Errorf("escapeStrings = %q, want %q — the encodings must stay distinct", got, "180;180")
	}
}

func TestBool01Ptr(t *testing.T) {
	if got := bool01Ptr(nil); got != "" {
		t.Errorf("nil must omit the key, got %q", got)
	}
	if got := bool01Ptr(new(true)); got != "1" {
		t.Errorf("true = %q, want %q", got, "1")
	}
	// An authored false is an intent, not an absence.
	if got := bool01Ptr(new(false)); got != "0" {
		t.Errorf("false = %q, want %q", got, "0")
	}
}

func TestVendorKeyword(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"Prusa Research", "PRUSA_RESEARCH"},
		{"Bambu Lab", "BAMBU_LAB"},
		{"E3D-Online", "E3D_ONLINE"},
	} {
		if got := vendorKeyword(c.in); got != c.want {
			t.Errorf("vendorKeyword(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Gates
// ---------------------------------------------------------------------------

func TestRenderMachineUnknownModelIsNotFilamentFlavored(t *testing.T) {
	// A machine whose model is absent from Prusa's catalogue must fail with a
	// message about the MACHINE. The catalogue error is shared with the filament
	// path, so it stays noun-neutral (catalog.go) — never "this filament" on
	// the machine render path.
	m := baseMachine()
	m.Meta.Model = new("Bambu Lab P2S")
	_, _, err := RenderMachine(m, "")
	if err == nil {
		t.Fatal("expected an error for a model absent from prusa's catalogue")
	}
	if !strings.Contains(err.Error(), "meta.model") {
		t.Errorf("error %q does not name meta.model", err)
	}
	if strings.Contains(err.Error(), "filament") {
		t.Errorf("error %q leaks the filament wording on the machine path", err)
	}
}

func TestRenderMachineRequiredFields(t *testing.T) {
	for _, c := range []struct {
		name   string
		mutate func(*canonical.Machine)
		want   string
	}{
		{"no name", func(m *canonical.Machine) { m.Meta.Name = nil }, "meta.name"},
		{"empty name", func(m *canonical.Machine) { m.Meta.Name = new("") }, "meta.name"},
		{"no model", func(m *canonical.Machine) { m.Meta.Model = nil }, "meta.model"},
		{"no nozzle", func(m *canonical.Machine) { m.Nozzle.DiameterMM = nil }, "nozzle.diameter_mm"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := baseMachine()
			c.mutate(&m)
			_, _, err := RenderMachine(m, "")
			if err == nil {
				t.Fatalf("expected an error naming %s", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not name %s", err, c.want)
			}
		})
	}
}

func TestRenderMachineRejectsSelfInheritance(t *testing.T) {
	m := baseMachine()
	if _, _, err := RenderMachine(m, *m.Meta.Name); err == nil {
		t.Fatal("a preset inheriting from itself must be refused")
	}
}

// The whole reason the machine render consults the catalogue: a model we cannot
// resolve would produce a preset that imports cleanly and binds to nothing.
func TestUnknownModelIsAnError(t *testing.T) {
	m := baseMachine()
	m.Meta.Model = new("Voron 2.4 350")
	_, _, err := RenderMachine(m, "")
	if err == nil {
		t.Fatal("a model in neither catalogue must be refused")
	}
	if !strings.Contains(err.Error(), "meta.model") {
		t.Errorf("error %q does not name the offending field", err)
	}
}

// A diameter that exists only as a hotend class (volcano 1.2) is something the
// canonical cannot express, so the render must fail by name rather than bind to
// the wrong hardware.
func TestDecoratedVariantIsRefused(t *testing.T) {
	m := baseMachine()
	m.Meta.Model = new("VORON2 250mm (Afterburner)")
	m.Nozzle.DiameterMM = new(1.2)
	_, _, err := RenderMachine(m, "")
	if err == nil {
		t.Fatal("a volcano-only nozzle must not resolve")
	}
	if !strings.Contains(err.Error(), "hotend class") {
		t.Errorf("error %q does not explain why it is refused", err)
	}
}

// The reason printer_variant is taken from the catalogue rather than from
// FormatFloat(nozzle): PrusaSlicer matches variants as STRINGS, and the
// catalogue holds "0.50" where we would have written "0.5".
func TestPrinterVariantKeepsTheCatalogueSpelling(t *testing.T) {
	m := baseMachine()
	m.Meta.Model = new("M3 - Independent Dual Rev.1")
	m.Nozzle.DiameterMM = new(0.5)

	got, _, err := RenderMachine(m, "")
	if err != nil {
		t.Fatalf("RenderMachine: %v", err)
	}
	if got.PrinterVariant != "0.50" {
		t.Errorf("printer_variant = %q, want the catalogue's %q", got.PrinterVariant, "0.50")
	}
	// nozzle_diameter is a NUMBER on the wire, so it keeps our formatting.
	if got.NozzleDiameter != "0.5" {
		t.Errorf("nozzle_diameter = %q, want %q", got.NozzleDiameter, "0.5")
	}
}

// ---------------------------------------------------------------------------
// Content
// ---------------------------------------------------------------------------

// printer_model must be the catalogue ID, never the human name: it is what a
// filament's compatible_printers_condition compares against.
func TestPrinterModelIsTheCatalogueID(t *testing.T) {
	got, _, err := RenderMachine(baseMachine(), "")
	if err != nil {
		t.Fatalf("RenderMachine: %v", err)
	}
	if got.PrinterModel != "MINI" {
		t.Errorf("printer_model = %q, want %q", got.PrinterModel, "MINI")
	}
	if got.PrinterVariant != "0.4" {
		t.Errorf("printer_variant = %q, want %q", got.PrinterVariant, "0.4")
	}
	if got.PrinterSettingsID != *baseMachine().Meta.Name {
		t.Errorf("printer_settings_id = %q, want the preset name", got.PrinterSettingsID)
	}
	if got.PrinterTechnology != "FFF" {
		t.Errorf("printer_technology = %q, want FFF", got.PrinterTechnology)
	}
}

func TestPrinterNotesComposition(t *testing.T) {
	m := baseMachine()
	m.Meta.Notes = new("Dry-box fed.")
	m.Meta.Tags = []string{"PG", "INPUT_SHAPER"}

	got, _, err := RenderMachine(m, "")
	if err != nil {
		t.Fatalf("RenderMachine: %v", err)
	}
	want := "Dry-box fed.\nPRINTER_VENDOR_PRUSA_RESEARCH\nPRINTER_MODEL_MINI\nPG\nINPUT_SHAPER"
	if got.PrinterNotes != want {
		t.Errorf("printer_notes =\n%q\nwant\n%q", got.PrinterNotes, want)
	}
}

// PRINTER_MODEL_ carries the id VERBATIM. Model ids in the catalogue hold '+',
// '-', '.' and spaces; sanitising them would break the regex the keyword exists
// for.
func TestPrinterModelKeywordIsNotSanitised(t *testing.T) {
	m := baseMachine()
	m.Meta.Vendor = nil
	m.Meta.Tags = nil
	m.Meta.Model = new("Original Prusa MINI & MINI+ Input Shaper")

	got, _, err := RenderMachine(m, "")
	if err != nil {
		t.Fatalf("RenderMachine: %v", err)
	}
	if got.PrinterNotes != "PRINTER_MODEL_MINIIS" {
		t.Errorf("printer_notes = %q", got.PrinterNotes)
	}
}

// Never a silent loss: every Orca-only machine property must be announced.
func TestOrcaOnlyFieldsAreWarned(t *testing.T) {
	m := baseMachine()
	m.Nozzle.Type = new(canonical.NozzleHardened)
	m.Nozzle.Extruder = new(canonical.DirectDrive)
	m.Nozzle.HeightMM = new(4.2)
	m.Nozzle.HRC = new(60.0)
	m.Retract.WipeDistanceMM = new(2.0)
	m.Features.AuxFan = new(true)
	m.Features.AirFiltration = new(true)
	m.Features.HeatedChamber = new(false)

	_, warns, err := RenderMachine(m, "")
	if err != nil {
		t.Fatalf("RenderMachine: %v", err)
	}
	var paths []string
	for _, w := range warns {
		paths = append(paths, w.Path)
	}
	for _, want := range []string{
		"nozzle.type", "nozzle.extruder", "nozzle.height_mm", "nozzle.hrc",
		"retraction.wipe_distance_mm", "features.aux_fan",
		"features.air_filtration", "features.heated_chamber",
	} {
		if !slices.Contains(paths, want) {
			t.Errorf("missing warning for %s, got: %v", want, warns)
		}
	}
}

// The canonical-only group feeds `aldina validate`, not slicer files: dropping
// it is expected, so warning about it would be noise the author cannot act on.
func TestCanonicalOnlyFieldsAreSilent(t *testing.T) {
	m := baseMachine()
	m.Features.Plates = []string{"textured_pei"}

	_, warns, err := RenderMachine(m, "")
	if err != nil {
		t.Fatalf("RenderMachine: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("limits/enclosed/plates/filament_diameter must drop silently, got: %v", warns)
	}
}

// A zero axis is an authored value; a nil axis GROUP is silence. The pointer
// rule has to survive all the way to the wire.
func TestAuthoredZeroAxisSurvivesButNilGroupIsOmitted(t *testing.T) {
	m := baseMachine()
	m.Kinetics.MaxJerkMMS = &canonical.Axes{X: 0, Y: 0, Z: 0, E: 0}
	m.Kinetics.MaxAccelMMS2 = nil

	got, _, err := RenderMachine(m, "")
	if err != nil {
		t.Fatalf("RenderMachine: %v", err)
	}
	if got.MachineMaxJerkX != "0,0" {
		t.Errorf("authored zero jerk = %q, want %q", got.MachineMaxJerkX, "0,0")
	}
	if got.MachineMaxAccelerationX != "" {
		t.Errorf("nil accel group must omit the key, got %q", got.MachineMaxAccelerationX)
	}
}

// ---------------------------------------------------------------------------
// Goldens
// ---------------------------------------------------------------------------

func TestRenderMachine(t *testing.T) {
	cases := []struct {
		name      string
		input     canonical.Machine
		inherits  string
		wantWarns []providers.Warning
	}{
		{name: "machine-mini-04", input: baseMachine()},
		{
			// Derived from a system preset the reader already has installed —
			// the shape of the real references/prusa/printers/Prusa_XL.ini.
			name:     "machine-mini-04-derived",
			input:    baseMachine(),
			inherits: "Original Prusa MINI & MINI+",
		},
		{
			// Only what the catalogue lookup needs: proves omission everywhere
			// else, and that an authored zero still renders.
			name: "machine-minimal",
			input: canonical.Machine{
				Meta:   canonical.MachineMeta{Name: new("Bare MINI"), Model: new("Original Prusa MINI & MINI+")},
				Nozzle: canonical.NozzleSpec{DiameterMM: new(0.4)},
				Volume: canonical.BuildVolume{HeightMM: new(0.0)},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			settings, warns, err := RenderMachine(c.input, c.inherits)
			if err != nil {
				t.Fatalf("RenderMachine: %v", err)
			}
			if len(warns) != len(c.wantWarns) || (len(warns) > 0 && !reflect.DeepEqual(warns, c.wantWarns)) {
				t.Errorf("warnings = %v, want %v", warns, c.wantWarns)
			}
			got, err := MarshalBundle("printer", *c.input.Meta.Name, settings)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			path := filepath.Join("testdata", "render", c.name+".golden.ini")
			if *update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update): %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}
