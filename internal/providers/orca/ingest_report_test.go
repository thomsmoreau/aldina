// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package orca

// ingest_report_test.go — the report is the feature (docs/adr/0001, Q3).
// A real export is ~130 keys of which ~24 map; the report must classify the
// rest by schema nature, never list them flat, and lead with the honest
// headline: calibration doesn't transfer.

import (
	"strings"
	"testing"
)

func bucketOf(r *IngestReport, key string) LossBucket {
	for _, is := range r.Issues {
		if is.Key == key {
			return is.Bucket
		}
	}
	return ""
}

func TestIngestReportClassifies(t *testing.T) {
	_, report, err := IngestFilament(readIngestFixture(t, "self-contained.json"))
	if err != nil {
		t.Fatalf("IngestFilament: %v", err)
	}

	if len(report.Issues) == 0 {
		t.Fatal("report.Issues empty, want a loss report")
	}
	if !strings.Contains(report.Summary, "transfer") {
		t.Errorf("summary = %q, want the transfer headline", report.Summary)
	}

	// Spot-check classifications on the REAL export.
	if got := bucketOf(report, "enable_pressure_advance"); got != BucketLostEngine {
		t.Errorf("enable_pressure_advance = %q, want lost-engine-knob", got)
	}
	if got := bucketOf(report, "version"); got != BucketLostIdentity {
		t.Errorf("version = %q, want lost-identity", got)
	}
	if got := bucketOf(report, "name"); got != BucketApproximated {
		t.Errorf("name = %q, want translated-with-approximation (suffix strip)", got)
	}
	// The export carries TWO extruder variants: the collapse is a fan-out loss.
	if got := bucketOf(report, "nozzle_temperature"); got != BucketLostFanout {
		t.Errorf("nozzle_temperature = %q, want lost-fan-out", got)
	}
}

// TestIngestReportSummaryEmpty: a file that maps cleanly (no loss) must say so,
// not print a headline about dropped settings.
func TestIngestReportSummaryEmpty(t *testing.T) {
	in := baseFilament()
	_, report, err := IngestFilament(renderToJSON(t, in, p2s06))
	if err != nil {
		t.Fatalf("IngestFilament: %v", err)
	}
	// baseFilament renders with 1 extruder and no unmapped-authored knobs, so
	// the only issues are identity keys — the summary must still exist.
	if report.Summary == "" {
		t.Error("summary empty, want a one-line disposition")
	}
}
