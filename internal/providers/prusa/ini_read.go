// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

package prusa

// ini_read.go — reader for PrusaSlicer's INI dialect, the inverse of the
// writer in ini.go. Same traps, read backwards:
//
//   - '#' and ';' are comments only at line START. In a value they are
//     content: filament_colour = #F7F7F7 is a colour, not a comment.
//   - one physical line per key; newlines are ESCAPED inside values (\n as
//     two characters), so a line scanner splits the file — no quote-aware
//     state machine is needed to find value boundaries.
//   - a value is a scalar or a ';'-joined list of items, quoted only when
//     needed. The quoting is never decorative: filament_settings_id = "" is
//     ONE EMPTY ITEM (a list), while compatible_printers =  is an ABSENT
//     list. The wire struct chooses the reading — `string` decodes the whole
//     value as one scalar, `[]string` decodes it as a list.
//
// Two views come out of one parse. Section.Keys is the raw view: values
// exactly as written, escapes intact, every key INCLUDING those the wire
// schema does not model (the classifier's input). DecodeFilament is the
// typed view, derived on demand through the schema.

import (
	"fmt"
	"reflect"
	"strings"
)

// Section is one header-delimited block of a PrusaSlicer INI document.
type Section struct {
	Category string            // "filament", "print", "printer", "presets", …
	Name     string            // everything after the first ':' — "" for bare headers like [presets]
	Keys     map[string]string // wire-form values, escapes intact (see file header)
}

// Parse reads PrusaSlicer's INI dialect. It returns the sections in file
// order; a headerless document (the detached whole-configuration export,
// pitfall #15) yields zero sections and no error — refusing it is the ingest
// layer's decision. Keys outside any section are part of no preset and are
// ignored. Malformed lines and duplicate keys inside one section are errors
// naming the offending line.
func Parse(data []byte) ([]Section, error) {
	var sections []Section
	cur := -1 // index into sections of the section being read
	for n, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		switch line[0] {
		case '#', ';': // comments only at line start
			continue
		case '[':
			if line[len(line)-1] != ']' {
				return nil, fmt.Errorf("prusa: ini line %d: malformed section header %q", n+1, line)
			}
			inner := line[1 : len(line)-1]
			sec := Section{Keys: map[string]string{}}
			if cat, name, found := strings.Cut(inner, ":"); found {
				sec.Category, sec.Name = cat, name
			} else {
				sec.Category = inner
			}
			sections = append(sections, sec)
			cur = len(sections) - 1
		default:
			if cur < 0 {
				continue // keys before any header: the headerless case, ignored
			}
			key, value, found := strings.Cut(line, " = ")
			if !found {
				return nil, fmt.Errorf("prusa: ini line %d: no ' = ' separator: %q", n+1, line)
			}
			if _, dup := sections[cur].Keys[key]; dup {
				return nil, fmt.Errorf("prusa: ini line %d: duplicate key %q in section [%s:%s]",
					n+1, key, sections[cur].Category, sections[cur].Name)
			}
			sections[cur].Keys[key] = value
		}
	}
	return sections, nil
}

// DecodeFilament is the typed view of a filament section: the wire keys the
// schema knows decode into a FilamentSettings, in the exact inverse of the
// writer's encoding. Keys the schema does not model are left untouched — the
// classifier reads them from Section.Keys. Mirror of collect in ini.go.
func (s Section) DecodeFilament() FilamentSettings {
	var out FilamentSettings
	decodeInto(reflect.ValueOf(&out).Elem(), s.Keys)
	return out
}

func decodeInto(v reflect.Value, keys map[string]string) {
	t := v.Type()
	for i := range t.NumField() {
		field, value := t.Field(i), v.Field(i)
		if field.Anonymous {
			decodeInto(value, keys)
			continue
		}
		key := field.Tag.Get("ini")
		if key == "" {
			panic(fmt.Sprintf("prusa: %s.%s has no ini tag", t.Name(), field.Name))
		}
		raw, ok := keys[key]
		if !ok {
			continue // absent key = field left at its zero value, same as the writer's omission
		}
		switch value.Kind() {
		case reflect.String:
			value.SetString(decodeScalar(raw))
		case reflect.Slice:
			items := decodeList(raw)
			if items == nil {
				continue
			}
			value.Set(reflect.ValueOf(items))
		default:
			panic(fmt.Sprintf("prusa: %s.%s has unsupported kind %s", t.Name(), field.Name, value.Kind()))
		}
	}
}

// decodeScalar is the inverse of escapeCStyle for a `string` field. A value
// written quoted (custom_parameters_filament = "", filament_ramming_parameters
// = "…") is unquoted first; then the \\ \n \r escapes decode. A bare '"' that
// is not part of a quoted envelope stays literal, as the writer leaves it.
func decodeScalar(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return unescapeCStyle(s[1:len(s)-1], true)
	}
	return unescapeCStyle(s, false)
}

// decodeList is the inverse of escapeStrings for a `[]string` field. The bare
// empty value is an ABSENT list and `""` is one empty item — the distinction
// the quoting exists to carry. A ';' inside a quoted item is content; inside,
// '\' is copied wholesale so the later unescape sees the escape pair whole.
func decodeList(s string) []string {
	if s == "" {
		return nil
	}
	items := []string{}
	var b strings.Builder
	quoted := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case quoted && c == '\\' && i+1 < len(s):
			b.WriteByte(c)
			b.WriteByte(s[i+1])
			i++
		case c == '"':
			quoted = !quoted
			b.WriteByte(c)
		case c == ';' && !quoted:
			items = append(items, b.String())
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}
	items = append(items, b.String())
	for i, item := range items {
		items[i] = decodeScalar(item)
	}
	return items
}

// unescapeCStyle decodes the C-style escape forms back to their literals.
// quoted also decodes \" — the extra escaping escapeStrings adds INSIDE
// quotes, which escapeCStyle never emits for unquoted scalars.
func unescapeCStyle(s string, quoted bool) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			// Only the escapes the writer can produce. Unknown pairs keep both
			// characters, byte for byte.
			switch n := s[i+1]; n {
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			case 'r':
				b.WriteByte('\r')
				i++
				continue
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			case '"':
				if quoted {
					b.WriteByte('"')
					i++
					continue
				}
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}
