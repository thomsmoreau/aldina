// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/thomsmoreau/Aldina/internal/providers"
	"github.com/thomsmoreau/Aldina/internal/providers/prusa"
)

// printWarnings takes both shapes because canonical cannot import providers:
// CheckCompat stays []string, renders return providers.Warning, and both
// print as the same "warning: path: message" line.
func printWarnings[W string | providers.Warning](w io.Writer, warnings []W) {
	for _, warning := range warnings {
		fmt.Fprintf(w, "warning: %s\n", warning)
	}
}

// writePreset is everything the two wire formats share: the preset name comes
// from user YAML and becomes a file name, so it is checked before it can
// escape outDir — and a stdout line, so a control character (a newline from
// a YAML block scalar) must not split the one-path-per-line output.
func writePreset(outDir, presetName, ext string, data []byte, stdout io.Writer) error {
	if presetName == "" || strings.ContainsAny(presetName, `/\\`) || presetName == "." || presetName == ".." ||
		strings.IndexFunc(presetName, unicode.IsControl) >= 0 {
		return fmt.Errorf("unsafe preset name %q", presetName)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	path := filepath.Join(outDir, presetName+ext)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if _, err := fmt.Fprintln(stdout, path); err != nil {
		return fmt.Errorf("report %s: %w", path, err)
	}
	return nil
}

func writeJSON(outDir, presetName string, value any, stdout io.Writer) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	return writePreset(outDir, presetName, ".json", append(data, '\n'), stdout)
}

// writeINI renders PrusaSlicer's dialect. The escaping lives in the provider
// (internal/providers/prusa/ini.go), never here: the CLI must not know how a
// slicer spells a value. section is the bundle category ("filament",
// "printer") — it decides which slot PrusaSlicer installs the preset into, so
// it belongs to the caller that knows the kind, not to this helper.
func writeINI(outDir, presetName, section string, value any, stdout io.Writer) error {
	data, err := prusa.MarshalBundle(section, presetName, value)
	if err != nil {
		return fmt.Errorf("marshal INI: %w", err)
	}
	return writePreset(outDir, presetName, ".ini", data, stdout)
}
