# PrusaSlicer references

## `PrusaSlicer_config_bundle.ini` is the trustworthy source

It is an untouched PrusaSlicer export holding three sections — `print:`,
`filament:`, `printer:`. When a value here disagrees with a standalone `.ini`,
**the bundle wins**.

## The standalone exports were corrupted, and are now repaired

`filaments/*.ini` and `printers/*.ini` had been run through a comment stripper
that cut everything from ` ;` or `#` onwards — both are comment markers in
generic INI, neither is one in PrusaSlicer's dialect. Eight values were
truncated, silently and plausibly:

| key | corrupted | real (bundle) |
| --- | --- | --- |
| `start_gcode` | `M17` | `M17 ; enable steppers\nM862.3 P "XL" …` |
| `end_filament_gcode` | `"` | `"; Filament-specific end gcode"` |
| `filament_colour` | *(empty)* | `#F7F7F7` |
| `layer_gcode` | *(empty)* | `;AFTER_LAYER_CHANGE\n;[layer_z]` |

Restored from the bundle on 2026-08-01, verified value-by-value (0 differences
remaining). Only truncated VALUES were restored: the key sets are untouched, so
the filament file still describes a real detached export (85 keys, matching
`internal/providers/prusa/schema.go` 1:1). The bundle's two extra
multi-material keys — `filament_flush_speed`, `filament_flush_volume` — were
deliberately NOT added: they do not appear in a detached export.

Why this is worth a README: a truncated `start_gcode` still looks like valid
G-code. Nothing warns you. Two format conclusions were drawn from these files
and both were wrong — that PrusaSlicer emits unbalanced quotes, and that its
dialect is unusable with a generic INI writer.

## What the repaired files establish about the format

- **One line per key**, `\n` escaped as two characters. There are no real
  newlines inside a value and no multi-line continuations.
- **Quoting is per FIELD, not per value.** `end_filament_gcode` is quoted;
  `before_layer_gcode = ;BEFORE_LAYER_CHANGE\n…` is NOT, though it also starts
  with `;`. Do not infer a rule from the value's content.
- `filament_colour = #F7F7F7` is unquoted and starts with `#` — the trap that a
  stock INI parser falls into, and the one that corrupted these files.
- Values may contain spaces unquoted (`filament_vendor = Forward AM`) and
  double quotes unquoted (`M862.3 P "XL"`).

The authoritative rules are taken from PrusaSlicer itself via `--load` / `--save`
(see `internal/providers/prusa/conform_test.go`), never inferred from a file
collected by hand.
