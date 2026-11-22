// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package canonical

import "gopkg.in/yaml.v3"

// MarshalFilament serializes a canonical filament to YAML — the inverse of
// LoadFilament. Ingest writes it so the result can be re-read and re-rendered
// by the authoring path (the two-step scaffold, docs/adr/0001).
func MarshalFilament(f *Filament) ([]byte, error) {
	return yaml.Marshal(f)
}
