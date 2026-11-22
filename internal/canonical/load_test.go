// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package canonical

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// kind and schema_version describe the document: a file loaded through the
// wrong loader, or with no header, must fail at load instead of decoding
// into an empty struct of the other kind.
func TestLoadChecksHeader(t *testing.T) {
	load := map[string]func(string) error{
		"filament": func(p string) error { _, err := LoadFilament(p); return err },
		"machine":  func(p string) error { _, err := LoadMachine(p); return err },
	}
	other := map[string]string{"filament": "machine", "machine": "filament"}
	for kind, loadFn := range load {
		for _, tc := range []struct {
			name, yaml, wantErr string
		}{
			{"valid", "kind: " + kind + "\nschema_version: 1\n", ""},
			{"wrong kind", "kind: " + other[kind] + "\nschema_version: 1\n", "kind"},
			{"missing kind", "schema_version: 1\n", "kind"},
			{"missing version", "kind: " + kind + "\n", "schema_version"},
			{"future version", "kind: " + kind + "\nschema_version: 2\n", "schema_version"},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "doc.yaml")
				if err := os.WriteFile(path, []byte(tc.yaml), 0644); err != nil {
					t.Fatal(err)
				}
				err := loadFn(path)
				switch {
				case tc.wantErr == "" && err != nil:
					t.Errorf("error = %v, want nil", err)
				case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
					t.Errorf("error = %v, want one naming %q", err, tc.wantErr)
				}
			})
		}
	}
}
