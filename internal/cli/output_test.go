// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// The preset name comes from user YAML and becomes both a file name and a
// stdout line. A control character (a YAML block scalar carries a newline
// easily) would split that line in two and break one-path-per-line output.
func TestWritePresetRejectsUnsafeNames(t *testing.T) {
	for _, tc := range []struct {
		name   string
		unsafe bool
	}{
		{"Plain PLA", false},
		{"PLA @Bambu Lab P2S 0.4", false},
		{"", true},
		{".", true},
		{"..", true},
		{"a/b", true},
		{`a\b`, true},
		{"foo\nbar", true},
		{"tab\tname", true},
		{"del\x7fname", true},
		{"c1\u0085name", true},
	} {
		var stdout bytes.Buffer
		err := writePreset(t.TempDir(), tc.name, ".yaml", []byte("x\n"), &stdout)
		switch {
		case tc.unsafe && (err == nil || !strings.Contains(err.Error(), "unsafe preset name")):
			t.Errorf("writePreset(%q) error = %v, want unsafe preset name", tc.name, err)
		case !tc.unsafe && err != nil:
			t.Errorf("writePreset(%q) error = %v, want nil", tc.name, err)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// The printed path IS the command's result for a script: when it cannot be
// written (stdout redirected to a full disk), the command must not succeed.
func TestWritePresetReportsStdoutFailure(t *testing.T) {
	err := writePreset(t.TempDir(), "Plain PLA", ".yaml", []byte("x\n"), failingWriter{})
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error = %v, want the stdout write error", err)
	}
}
