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
// warnings, never errors, so it must not gate loading. The header is the
// exception (checkHeader): it says what the document IS, not how good it is.
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

// schemaVersion is the only canonical format version this build reads.
const schemaVersion = 1

// checkHeader rejects a document loaded as the wrong kind. Without it, a
// filament passed where a machine is expected decodes cleanly (every
// machine field is optional) into an empty Machine.
func checkHeader(path, want, kind string, version int) error {
	switch {
	case kind == "":
		return fmt.Errorf("%s: kind: required (want %q)", path, want)
	case kind != want:
		return fmt.Errorf("%s: kind: %q, want %q", path, kind, want)
	case version == 0:
		return fmt.Errorf("%s: schema_version: required", path)
	case version != schemaVersion:
		return fmt.Errorf("%s: schema_version: %d unsupported, this build reads %d", path, version, schemaVersion)
	}
	return nil
}

// LoadFilament reads a canonical filament YAML file. See loadYAML.
func LoadFilament(path string) (*Filament, error) {
	f, err := loadYAML[Filament](path)
	if err != nil {
		return nil, err
	}
	if err := checkHeader(path, "filament", f.Kind, f.SchemaVersion); err != nil {
		return nil, err
	}
	return f, nil
}

// LoadMachine reads a canonical machine YAML file. See loadYAML.
func LoadMachine(path string) (*Machine, error) {
	m, err := loadYAML[Machine](path)
	if err != nil {
		return nil, err
	}
	if err := checkHeader(path, "machine", m.Kind, m.SchemaVersion); err != nil {
		return nil, err
	}
	return m, nil
}
