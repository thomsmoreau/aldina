// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// ingest.go — PrusaSlicer config bundle → canonical filaments.
//
// Direction is provider → canonical, the reverse of RenderFilament. One
// bundle in, N results out — one per [filament:NAME] section (docs/adr/0002).
// Loss axis mirrors the render, inverted (docs/adr/0001): a hard error when
// the whole file cannot be read into FAITHFUL canonicals (malformed INI, a
// headerless detached export, no filament section, duplicate preset names), a
// per-preset refusal when one preset's values live in a parent (non-empty
// inherits), a loss bucket when a present key is unmappable — every key of
// the section lands in one (ingest_report.go).

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/thomsmoreau/Aldina/internal/canonical"
)

// ErrInherits reports a preset whose values live in a parent preset.
// Ingesting its deltas alone would produce an almost-empty canonical and a
// false success, so THAT preset is refused — not the bundle: a bundle is a
// library, and one inherited preset must not block the others (user decision
// (a)). Detection happens before any field mapping: the canonical would be
// false, not merely lossy.
var ErrInherits = errors.New("prusa: preset inherits from another preset; ingesting deltas is not supported")

// LossBucket classifies what happened to one source key during ingest. The
// taxonomy matches the orca side (same six buckets) but is its OWN copy: the
// provider boundary rule, and the two dialects' losses differ at the key
// level — a prusa []string list is not an orca JSON array.
type LossBucket string

const (
	BucketTranslated   LossBucket = "translated"                    // mapped cleanly to a canonical field
	BucketApproximated LossBucket = "translated-with-approximation" // mapped, value changed shape
	BucketLostIdentity LossBucket = "lost-identity"                 // import-gate key, no print intent
	BucketLostGcode    LossBucket = "lost-gcode"                    // opaque gcode blob, never interpreted
	BucketLostFanout   LossBucket = "lost-fan-out"                  // per-extruder array collapsed to the first
	BucketLostEngine   LossBucket = "lost-engine-knob"              // slicing-engine tuning, does not transfer
)

// IngestIssue is one source key's disposition. Bucket says what kind of loss
// (or none); Key/Value carry the wire form; Note carries the reason.
type IngestIssue struct {
	Bucket LossBucket
	Key    string
	Value  string
	Note   string
}

// IngestReport is the ingest warning channel, one per result. Richer than the
// render's flat []providers.Warning on purpose: ingest loss is structural (a bucket per
// key), render loss is per-field — different shapes, not unified.
type IngestReport struct {
	Summary string
	Issues  []IngestIssue
}

// IngestResult is one preset's outcome: a canonical filament plus its report,
// or a per-preset refusal with Err set (Filament and Report are nil then).
// Name is always set — the [filament:NAME] header — so callers can report a
// refusal without a filament.
type IngestResult struct {
	Name     string
	Filament *canonical.Filament
	Report   *IngestReport
	Err      error
}

// Ingest reads a PrusaSlicer config bundle and returns one result per
// [filament:NAME] section, in file order.
//
// Whole-file failures are hard errors: malformed INI, a headerless file (the
// detached whole-configuration export — the message points at the bundle
// export), a bundle with no filament section (the message lists what it does
// contain), or duplicate preset names (two presets would write the same
// canonical file, case ignored). Refusals are per preset: a non-empty inherits skips that
// preset only.
func Ingest(data []byte) ([]IngestResult, error) {
	sections, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("prusa: read bundle: %w", err)
	}
	if len(sections) == 0 {
		return nil, errors.New("prusa: no section headers — a detached export is a whole configuration, not a preset; " +
			"export a config bundle (File → Export Config Bundle) and ingest that")
	}

	var filaments []Section
	for _, s := range sections {
		if s.Category == "filament" {
			filaments = append(filaments, s)
		}
	}
	if len(filaments) == 0 {
		if cats := describeCategories(sections); cats == "" {
			return nil, errors.New("prusa: bundle carries no [filament:*] section, only [presets] — nothing to ingest")
		} else {
			return nil, fmt.Errorf("prusa: bundle carries no [filament:*] section (found %s); nothing to ingest", cats)
		}
	}

	// Keyed by the lowercased name: the default macOS and Windows filesystems
	// ignore case, so "PLA" and "pla" would write the same canonical file.
	seen := make(map[string]string, len(filaments))
	for _, s := range filaments {
		key := strings.ToLower(s.Name)
		if first, ok := seen[key]; ok {
			if first == s.Name {
				return nil, fmt.Errorf("prusa: duplicate preset name %q — two sections would write the same canonical file", s.Name)
			}
			return nil, fmt.Errorf("prusa: preset names %q and %q differ only in case — they would write the same canonical file on a case-insensitive filesystem", first, s.Name)
		}
		seen[key] = s.Name
	}

	results := make([]IngestResult, 0, len(filaments))
	for _, s := range filaments {
		wire := s.DecodeFilament()
		if wire.Inherits != "" {
			results = append(results, IngestResult{
				Name: s.Name,
				Err:  fmt.Errorf("prusa: preset %q inherits from %q: %w", s.Name, wire.Inherits, ErrInherits),
			})
			continue
		}
		name := s.Name
		var issues []IngestIssue
		// Value and override carry the wire text as written, quotes and ';'
		// included: the list is what the user authored, not its first item.
		rawType := s.Keys["filament_type"]
		material, matNote := parsePrusaMaterial(wire.FilamentType)
		if matNote != "" {
			issues = append(issues, IngestIssue{Bucket: BucketApproximated, Key: "filament_type", Value: rawType, Note: matNote})
		}
		machines, compatIssues := parseCompatMachines(wire.CompatiblePrinters)
		issues = append(issues, compatIssues...)
		issues = append(issues, classifyKeys(s.Keys)...)

		// What has no canonical home stays in the YAML: a homeless material,
		// so the render target — not ingest — decides whether its absence
		// blocks; a compat rule, so the author can re-declare it by hand.
		prusaOverrides := map[string]any{}
		if material == nil && matNote != "" {
			prusaOverrides["filament_type"] = rawType
		}
		for key := range conditionPrusaKeys {
			if raw := s.Keys[key]; !authorsNothing(raw) {
				prusaOverrides[key] = raw
			}
		}
		var overrides map[string]map[string]any
		if len(prusaOverrides) > 0 {
			overrides = map[string]map[string]any{"prusa": prusaOverrides}
		}
		results = append(results, IngestResult{
			Name: s.Name,
			Filament: &canonical.Filament{
				Kind:          "filament",
				SchemaVersion: 1,
				Meta: canonical.Meta{
					Name:     &name,
					Material: material,
					Vendor:   parsePrusaScalar(wire.FilamentVendor),
					ColorHex: parsePrusaColour(wire.FilamentColour),
					Notes:    parsePrusaString(wire.FilamentNotes),
				},
				Physical: canonical.Physical{
					DiameterMM:  parsePrusaFloat(wire.FilamentDiameter),
					DensityGCm3: parsePrusaFloat(wire.FilamentDensity),
					CostPerKg:   parsePrusaFloat(wire.FilamentCost),
					Abrasive:    parsePrusaBool(wire.FilamentAbrasive),
					ShrinkageXY: parsePrusaShrink(wire.FilamentShrinkageCompensationXY),
					ShrinkageZ:  parsePrusaShrink(wire.FilamentShrinkageCompensationZ),
				},
				Temps: canonical.Temps{
					Nozzle:           parsePrusaFloat(wire.Temperature),
					NozzleFirstLayer: parsePrusaFloat(wire.FirstLayerTemperature),
					Bed:              parsePrusaFloat(wire.BedTemperature),
					BedFirstLayer:    parsePrusaFloat(wire.FirstLayerBedTemperature),
				},
				Flow: canonical.Flow{
					FlowRatio:         parsePrusaFloat(wire.ExtrusionMultiplier),
					MaxVolumetricMM3S: parsePrusaFloat(wire.FilamentMaxVolumetricSpeed),
				},
				Cooling: canonical.Cooling{
					FanMin:              parsePrusaPercent(wire.MinFanSpeed),
					FanMax:              parsePrusaPercent(wire.MaxFanSpeed),
					BridgesFan:          parsePrusaPercent(wire.BridgeFanSpeed),
					FanOffFirstLayers:   parsePrusaInt(wire.DisableFanFirstLayers),
					FullFanAtLayer:      parsePrusaInt(wire.FullFanSpeedLayer),
					SlowDownLayerTimeS:  parsePrusaFloat(wire.SlowdownBelowLayerTime),
					SlowDownMinSpeedMMS: parsePrusaFloat(wire.MinPrintSpeed),
				},
				// Only filament_retract_* is filament intent (spec Q3): bare
				// retract_* are print defaults, *_toolchange are
				// multi-material knobs — both engine knobs (ingest_report.go).
				// "nil" parses to canonical nil, so inherit stays inherit.
				Retraction: canonical.Retraction{
					LengthMM:          parsePrusaFloat(wire.FilamentRetractLength),
					SpeedMMS:          parsePrusaFloat(wire.FilamentRetractSpeed),
					DeretractSpeedMMS: parsePrusaFloat(wire.FilamentDeretractSpeed),
					ZHopMM:            parsePrusaFloat(wire.FilamentRetractLift),
					Wipe:              parsePrusaBool(wire.FilamentWipe),
				},
				Compat: canonical.Compat{Machines: machines},
				// Opaque blobs: carried, never interpreted.
				Gcode: canonical.Gcode{
					Start: parsePrusaString(wire.StartFilamentGcode),
					End:   parsePrusaString(wire.EndFilamentGcode),
				},
				Overrides: overrides,
			},
			Report: &IngestReport{Summary: buildSummary(issues), Issues: issues},
		})
	}
	return results, nil
}

// describeCategories lists the section kinds a filamentless bundle does carry,
// so the refusal names reality instead of guessing. [presets] names ACTIVE
// selections, not presets, and is dropped — except that a [presets]-only file
// must still be distinguishable ("" return, caller special-cases it).
func describeCategories(sections []Section) string {
	names := map[string]bool{}
	for _, s := range sections {
		if s.Category != "presets" {
			names[s.Category] = true
		}
	}
	cats := make([]string, 0, len(names))
	for c := range names {
		cats = append(cats, "["+c+":…]")
	}
	sort.Strings(cats)
	return strings.Join(cats, ", ")
}
