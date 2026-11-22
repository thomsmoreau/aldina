// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

// Warning is one render loss: a canonical field the target cannot express.
// Path is the canonical YAML path the loss is about, typed rather than a
// "path: message" prefix because the loop test matches deviations on it
// (ADR-0004): a text convention is not a contract.
type Warning struct {
	Path, Message string
}

// String is the CLI line, the text render warnings carried before they were
// typed.
func (w Warning) String() string {
	return w.Path + ": " + w.Message
}
