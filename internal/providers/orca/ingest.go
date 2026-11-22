// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// ingest.go — Orca filament JSON → canonical Filament (slice A: scaffold).
//
// Direction is provider → canonical, the reverse of RenderFilament. Same loss
// axis as render, inverted (docs/adr/0001): a hard error when the source
// cannot be read into a FAITHFUL canonical (malformed, or non-empty inherits),
// a loss bucket when a key is present but unmappable. Slice A maps nothing
// yet — it pins the scaffold, the report types, and the refuse seam.

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thomsmoreau/Aldina/internal/canonical"
)

// ErrInherits reports a file whose values live in a parent preset. Ingesting
// its deltas alone would produce an almost-empty canonical and a false
// success, so the file is refused instead (docs/adr/0001). Detection happens
// BEFORE any field mapping: the canonical would be false, not merely lossy.
var ErrInherits = errors.New("orca: file inherits from another preset; ingesting deltas is not supported")

// LossBucket classifies what happened to one source key during ingest.
type LossBucket string

const (
	BucketTranslated   LossBucket = "translated"                    // mapped cleanly to a canonical field
	BucketApproximated LossBucket = "translated-with-approximation" // mapped, value changed shape
	BucketLostIdentity LossBucket = "lost-identity"                 // import-gate key, no intent
	BucketLostGcode    LossBucket = "lost-gcode"                    // opaque gcode blob, never interpreted
	BucketLostFanout   LossBucket = "lost-fan-out"                  // per-plate / per-extruder array
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

// IngestReport is the ingest warning channel. Richer than RenderFilament's
// flat []providers.Warning on purpose: ingest loss is structural (bucket per key), render
// loss is per-field — different shapes, not unified.
type IngestReport struct {
	Summary string
	Issues  []IngestIssue
}

// IngestFilament reads an Orca filament JSON export into a canonical Filament.
//
// On a non-empty inherits it refuses with ErrInherits wrapped around the
// inherited-from preset name, so the caller can both react (errors.Is) and
// explain (the message). Malformed JSON is a decode error, not ErrInherits.
func IngestFilament(raw []byte) (*canonical.Filament, *IngestReport, error) {
	// Decode twice: the raw key map drives the classifier (it must see the
	// keys the struct does not model — the loss is in those), the typed struct
	// drives the field mapping.
	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, nil, fmt.Errorf("orca: decode filament JSON: %w", err)
	}
	var s FilamentSettings
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, nil, fmt.Errorf("orca: decode filament JSON: %w", err)
	}
	if s.Inherits != "" {
		return nil, nil, fmt.Errorf("orca: %q: %w", s.Inherits, ErrInherits)
	}

	f, issues := ingestFilament(s)

	for key, rawVal := range all {
		bucket, note := classify(key, rawVal)
		if bucket == BucketTranslated {
			continue
		}
		issues = append(issues, IngestIssue{Bucket: bucket, Key: key, Value: string(rawVal), Note: note})
	}

	return f, &IngestReport{Summary: buildSummary(issues), Issues: issues}, nil
}

// ingestFilament maps the wire struct to canonical and reports the
// approximations it had to make (name suffix strip, material vocabulary,
// abrasive HRC threshold). Nothing stays deferred: the result is a fully
// renderable canonical (RenderFilament requires meta.name).
func ingestFilament(s FilamentSettings) (*canonical.Filament, []IngestIssue) {
	var issues []IngestIssue

	name, nameNote := parseOrcaName(s.Name)
	if nameNote != "" {
		issues = append(issues, IngestIssue{Bucket: BucketApproximated, Key: "name", Value: *s.Name, Note: nameNote})
	}
	material, matNote := parseOrcaMaterial(s.FilamentType)
	if matNote != "" {
		v, _ := unbroadcast(s.FilamentType)
		issues = append(issues, IngestIssue{Bucket: BucketApproximated, Key: "filament_type", Value: v, Note: matNote})
	}
	abrasive, abrasiveNote := parseOrcaAbrasive(s.RequiredNozzleHRC)
	if abrasiveNote != "" {
		v, _ := unbroadcast(s.RequiredNozzleHRC)
		issues = append(issues, IngestIssue{Bucket: BucketApproximated, Key: "required_nozzle_HRC", Value: v, Note: abrasiveNote})
	}
	machines, compatIssues := parseCompatMachines(s.CompatiblePrinters)
	issues = append(issues, compatIssues...)

	return &canonical.Filament{
		Kind:          "filament",
		SchemaVersion: 1,
		Meta: canonical.Meta{
			Name:     name,
			Vendor:   parseOrcaString(s.FilamentVendor),
			Material: material,
			Notes:    parseOrcaString(s.FilamentNotes),
			ColorHex: parseOrcaColour(s.DefaultFilamentColour),
		},
		Physical: canonical.Physical{
			DiameterMM:  parseOrcaFloat(s.FilamentDiameter),
			DensityGCm3: parseOrcaFloat(s.FilamentDensity),
			CostPerKg:   parseOrcaFloat(s.FilamentCost),
			Abrasive:    abrasive,
			ShrinkageXY: parseOrcaShrink(s.FilamentShrink),
			ShrinkageZ:  parseOrcaShrink(s.FilamentShrinkageCompensationZ),
		},
		Temps: canonical.Temps{
			Nozzle:           parseOrcaFloat(s.NozzleTemperature),
			NozzleFirstLayer: parseOrcaFloat(s.NozzleTemperatureInitialLayer),
			NozzleRange:      parseOrcaRange(s.NozzleTemperatureRangeLow, s.NozzleTemperatureRangeHigh),
			Bed:              parseOrcaFloat(s.HotPlateTemp),
			BedFirstLayer:    parseOrcaFloat(s.HotPlateTempInitialLayer),
			Softening:        parseOrcaFloat(s.TemperatureVitrification),
		},
		Flow: canonical.Flow{
			MaxVolumetricMM3S: parseOrcaFloat(s.FilamentMaxVolumetricSpeed),
			FlowRatio:         parseOrcaFloat(s.FilamentFlowRatio),
		},
		Cooling: canonical.Cooling{
			FanMin:              parseOrcaPercent(s.FanMinSpeed),
			FanMax:              parseOrcaPercent(s.FanMaxSpeed),
			FanOffFirstLayers:   parseOrcaInt(s.CloseFanTheFirstXLayers),
			FullFanAtLayer:      parseOrcaInt(s.FullFanSpeedLayer),
			SlowDownLayerTimeS:  parseOrcaFloat(s.SlowDownLayerTime),
			SlowDownMinSpeedMMS: parseOrcaFloat(s.SlowDownMinSpeed),
			BridgesFan:          parseOrcaPercent(s.OverhangFanSpeed),
		},
		Retraction: canonical.Retraction{
			LengthMM:          parseOrcaFloat(s.FilamentRetractionLength),
			SpeedMMS:          parseOrcaFloat(s.FilamentRetractionSpeed),
			DeretractSpeedMMS: parseOrcaFloat(s.FilamentDeretractionSpeed),
			ZHopMM:            parseOrcaFloat(s.FilamentZHop),
			Wipe:              parseOrcaBool(s.FilamentWipe),
			WipeDistanceMM:    parseOrcaFloat(s.FilamentWipeDistance),
		},
		Compat: canonical.Compat{Machines: machines},
		Gcode: canonical.Gcode{
			Start: parseOrcaGcode(s.FilamentStartGcode),
			End:   parseOrcaGcode(s.FilamentEndGcode),
		},
	}, issues
}
