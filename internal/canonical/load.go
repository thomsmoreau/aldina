// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package canonical

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// loadYAML reads a canonical YAML file into T.
// Unmarshal is strict: unknown keys are rejected, so typos in hand-written
// files fail at load instead of silently dropping values.
// Validation is NOT done here: call Validate() separately — it returns
// warnings, never errors, so it must not gate loading.
func loadYAML[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var v T
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &v, nil
}

// LoadFilament reads a canonical filament YAML file. See loadYAML.
func LoadFilament(path string) (*Filament, error) {
	return loadYAML[Filament](path)
}

// LoadMachine reads a canonical machine YAML file. See loadYAML.
func LoadMachine(path string) (*Machine, error) {
	return loadYAML[Machine](path)
}
