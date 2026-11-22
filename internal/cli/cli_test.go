// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// Asking for help is a success: a script probing --help must see exit 0, and
// the usage must not be followed by an error line.
func TestHelpIsNotAnError(t *testing.T) {
	for _, args := range [][]string{
		{"help"}, {"-h"}, {"--help"},
		{"render", "--help"}, {"validate", "-h"}, {"ingest", "--help"},
	} {
		var stdout, stderr bytes.Buffer
		if err := Run(args, &stdout, &stderr); err != nil {
			t.Errorf("Run(%q) error = %v, want nil", args, err)
		}
		if stdout.Len()+stderr.Len() == 0 {
			t.Errorf("Run(%q) printed no usage", args)
		}
	}
}

// Top-level help is the requested output, so it goes to stdout.
func TestTopLevelHelpGoesToStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(--help) error = %v", err)
	}
	if !strings.Contains(stdout.String(), "usage:") || stderr.Len() != 0 {
		t.Errorf("stdout = %q, stderr = %q, want usage on stdout only", stdout.String(), stderr.String())
	}
}
