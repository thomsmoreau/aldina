// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

// aldina-mapping regenerates the marked wire-contract section of
// docs/mapping.md from internal/providers/conformance.yaml.
//
// The YAML is the single source of truth for the cross-provider value
// contract. This command is the human-facing half: run it and the table in
// mapping.md always matches the table the king test asserts. Run from the
// repo root:
//
//	go run ./cmd/aldina-mapping
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/thomsmoreau/Aldina/internal/providers"
)

const (
	startMarker = "<!-- conformance:start -->"
	endMarker   = "<!-- conformance:end -->"
)

func loadTable(path string) (providers.ConformanceTable, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return providers.ConformanceTable{}, err
	}
	tab, err := providers.ParseConformance(raw)
	if err != nil {
		return providers.ConformanceTable{}, err
	}
	return tab, nil
}

// formatValue renders a YAML scalar as a wire-comparable string. Strings are
// quoted, floats use the same 'f' -1 policy as the renderers, arrays and maps
// are JSON.
func formatValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "∅"
	case string:
		return strconv.Quote(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case bool:
		return strconv.FormatBool(x)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(b)
	}
}

// renderWire renders one side of a case (the orca or prusa map) as one table
// cell: `key` = value, several keys joined by <br>. Keys are sorted so the
// output is deterministic. A nil value means the key must be absent.
func renderWire(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		v := m[k]
		if v == nil {
			parts = append(parts, fmt.Sprintf("`%s` = absent", k))
		} else {
			parts = append(parts, fmt.Sprintf("`%s` = %s", k, formatValue(v)))
		}
	}
	return strings.Join(parts, "<br>")
}

func escapeCell(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

// cell pads a field to one space each side; an empty field collapses to a
// single space, so the row reads "| |" (markdownlint-clean) instead of "|  |".
// The caller adds the pipes.
func cell(s string) string {
	if s == "" {
		return " "
	}
	return " " + s + " "
}

// renderSection emits the full markdown table for every case in both
// resources.
func renderSection(tab providers.ConformanceTable) string {
	var b strings.Builder
	b.WriteString("| resource | concept | value | OrcaSlicer | PrusaSlicer | note |\n")
	b.WriteString("| -------- | ------- | ----- | ---------- | ----------- | ---- |\n")

	emit := func(resource string, cases []providers.ConformanceCase) {
		for _, c := range cases {
			row := []string{
				cell(resource),
				cell(escapeCell(c.Concept)),
				cell(escapeCell(formatValue(c.Value))),
				cell(escapeCell(renderWire(c.Orca))),
				cell(escapeCell(renderWire(c.Prusa))),
				cell(escapeCell(c.Note)),
			}
			b.WriteString("|" + strings.Join(row, "|") + "|\n")
		}
	}
	emit("filament", tab.Filament)
	emit("machine", tab.Machine)
	return b.String()
}

// replaceSection swaps the text between the markers, keeping the markers
// themselves so the section can be regenerated again.
func replaceSection(doc, section string) (string, error) {
	start := strings.Index(doc, startMarker)
	end := strings.Index(doc, endMarker)
	if start == -1 || end == -1 {
		return "", fmt.Errorf("mapping.md is missing %s / %s markers", startMarker, endMarker)
	}
	if end <= start {
		return "", fmt.Errorf("markers out of order in mapping.md")
	}
	return doc[:start] + startMarker + "\n\n" + section + doc[end:], nil
}

func run() error {
	tab, err := loadTable("internal/providers/conformance.yaml")
	if err != nil {
		return fmt.Errorf("conformance.yaml: %w", err)
	}
	raw, err := os.ReadFile("docs/mapping.md")
	if err != nil {
		return err
	}
	out, err := replaceSection(string(raw), renderSection(tab))
	if err != nil {
		return err
	}
	return os.WriteFile("docs/mapping.md", []byte(out), 0o644)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "aldina-mapping:", err)
		os.Exit(1)
	}
}
