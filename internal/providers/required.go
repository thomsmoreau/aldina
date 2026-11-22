// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"fmt"
	"strings"
)

// MissingError turns a provider's list of missing canonical paths into one
// error, or nil when nothing is missing.
//
// WHY the list lives in the provider, not in canonical: "required" means
// "the target slicer cannot import the rendered file without it", which is a
// property of the TARGET FORMAT, not of the canonical format. Orca silently
// drops a preset with no `name` (docs/mapping.md §Identity); Prusa's INI has
// its own gates. The canonical layer stays warnings-only (canonical.Validate)
// and never decides what blocks.
//
// WHY each provider declares its fields as a function rather than a []string
// of paths: "required" covers more than one rule. meta.name must be present
// AND non-empty because no name can be invented, whereas a generated id would
// tolerate present-but-empty. A flat path list cannot express that, and
// resolving paths would need reflection — where a typo silently disables the
// guard instead of failing to compile.
//
// kind names the provider and resource ("orca filament") and appears in the
// message. All missing fields are reported at once: no fail-fast, same policy
// as canonical.Validate.
func MissingError(kind string, missing []string) error {
	switch len(missing) {
	case 0:
		return nil
	case 1:
		// One field reads as a sentence, not as a list of one: this is the
		// message an author sees most often.
		return fmt.Errorf("%s: %s is required", kind, missing[0])
	default:
		return fmt.Errorf("%s: required fields missing: %s", kind, strings.Join(missing, ", "))
	}
}
