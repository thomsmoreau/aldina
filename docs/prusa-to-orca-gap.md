# Prusa → Orca conversion — gap inventory

Research note. Question: what is left to build a functional conversion from a
PrusaSlicer file to an OrcaSlicer file (`prusa → canonical → orca`)?

Method: inspect the source of this repo (`internal/`, `cmd/`, `internal/cli/`)
and `docs/mapping.md`. Every claim cites the file it comes from. Date of the
inspection: 2026-08-24 (`HEAD` = `89dbcef`).

## 1. Current state

| Direction       | Orca                                     | Prusa                                                             |
| --------------- | ---------------------------------------- | ----------------------------------------------------------------- |
| render filament | yes (`orca/filament_render.go`)          | yes (`prusa/filament_render.go`)                                  |
| render machine  | yes (`orca/machine_render.go`)           | yes (`prusa/machine_render.go`)                                   |
| ingest filament | yes (`orca/ingest.go:63 IngestFilament`) | **no** — no `Ingest` symbol exists in `internal/providers/prusa/` |
| ingest machine  | **no** — no `IngestMachine` in the repo  | **no**                                                            |
| loss report     | yes (`orca/ingest_report.go`, 6 buckets) | **no**                                                            |
| round-trip test | yes (`orca/ingest_roundtrip_test.go`)    | **no**                                                            |

The CLI confirms the gap: `internal/cli/ingest.go:39-40` rejects every
provider except `orca` ("ingest: unsupported provider %q (only orca)").

Conclusion: the missing piece for `prusa → orca` is **Prusa filament ingest**.
The render side (`canonical → orca`) already works.

## 2. What a Prusa ingest must build

Mirror the Orca package layout: `prusa/ingest.go`, `prusa/ingest_convert.go`,
`prusa/ingest_report.go` + table tests + a round-trip king test.

### 2.1 INI reader — does not exist

`prusa/ini.go` is write-only: its exported API is `MarshalBundle`
(`prusa/ini.go:104`); the rest is escaping and collect helpers. Prusa profiles
are `key = value` text under a `[section]` header. Ingest needs a parser
(nearest model: Orca ingest decodes the wire struct, `orca/ingest.go:67-73`).
The parser must also feed the classifier, which needs keys the struct does
not model (same double-decode pattern, `orca/ingest.go:64-66`).

### 2.2 Headerless-configuration trap

A headerless Prusa INI is a whole printer configuration, not one preset
(mapping.md pitfall #15, `docs/mapping.md:160`). Ingest must detect the
section context (`[filament:NAME]`, `[print:NAME]`, `[sla_print]` …) and
handle a multi-section file deliberately: take the filament section, or
refuse.

### 2.3 Inheritance refusal

Orca ingest refuses a non-empty `inherits` with `ErrInherits`
(`orca/ingest.go:76-78`). The Prusa schema carries the same key
(`inherits` in `prusa/schema.go`). The refusal is required by ADR-0001
(`docs/adr/0001-ingest-scope-intent-only.md`).

### 2.4 Value conversions Prusa needs (all new code, none reusable from Orca ingest)

> **Update 2026-10-24 (`548bf02`):** this statement is no longer true. The
> scalar parsers (float, percent, int, bool, and the `"nil"` token) are now
> shared by the two providers in `internal/providers/convert.go`. The
> conversions below that are specific to Prusa stay in `prusa/`.

Verified against the key list in `prusa/schema.go` and the concept table in
`docs/mapping.md`:

- **Temperatures are scalars**, not per-extruder arrays: `temperature`,
  `bed_temperature`, `first_layer_temperature`, `first_layer_bed_temperature`
  vs Orca's `["240"]` fan-out (mapping.md conformance rows).
- **Fans are plain percents**: `min_fan_speed`/`max_fan_speed` "30"/"100" →
  canonical ratios 0–1 (conformance row `cooling.fan_min`).
- **Shrinkage is NOT the Orca inversion**: Prusa
  `filament_shrinkage_compensation_xy = "2%"` maps directly to canonical
  `0.02`; Orca spells the same intent `filament_shrink = "98%"`
  (mapping.md pitfall #14, `docs/mapping.md:231`; conformance rows).
  The Prusa ingest must NOT copy the Orca `100%−x` decode.
- **Material vocabulary**: Prusa says `"PET"` for PETG, a single value where
  Orca uses an array (conformance row `meta.material`). The Orca side already
  has the vocabulary map (`orca/ingest_convert.go:165 PLA/PETG/ABS/ASA/…`);
  the Prusa side needs its own.
- **Flow rename**: `extrusion_multiplier` → canonical `flow_ratio`
  (mapping.md concept table).
- **Bed rename**: `bed_temperature` → canonical `temps.bed` (concept table).
- **Retraction families**: Prusa filament presets carry BOTH
  `filament_retract_*` (override layer) and bare `retract_*` (machine
  defaults collapsed into the same file), plus toolchange variants
  (`filament_retract_length_toolchange`, `retract_restart_extra_toolchange`).
  Canonical models one filament retraction block with `"nil"` = inherit
  (mapping.md §Retraction, `docs/mapping.md:131`). The ingest must decide
  which family wins — open question, §4.
- **Wipe/z-hop/deretract**: `filament_wipe`, `filament_retract_before_wipe`,
  `filament_retract_lift` (→ `z_hop_mm`), `filament_deretract_speed` — same
  canonical targets Orca ingest already reaches
  (`FilamentWipe`, `FilamentWipeDistance`, `FilamentZHop`,
  `FilamentDeretractionSpeed` in `orca/ingest.go:96-168`).
- **Identity**: Prusa binds filament→machine through
  `compatible_printers_condition` / `printer_settings_id` (mapping.md
  pitfalls #12–#13, `docs/mapping.md:153,183`). Orca ingest already reads
  `CompatiblePrinters` into `compat`; the Prusa condition-DSL cannot be
  parsed to intent — likely a warn-and-drop with the raw kept in
  `overrides.prusa`.

### 2.5 Loss-report buckets (Prusa flavour)

Reuse the bucket taxonomy (`orca/ingest_report.go:86-110`: translated,
lost-engine, lost-fanout, lost-identity, lost-gcode, approximated) with
Prusa-specific key sets. Prusa-only loss candidates in `prusa/schema.go`:
`filament_multitool_ramming*`, `filament_cooling_moves/initial_speed/
final_speed`, `filament_stamping_*`, `filament_purge_multiplier`,
`filament_toolchange_delay`, `filament_travel_*_lift`, loading/unloading
speed and time keys, `custom_parameters_filament`, `max_volumetric_speed`
(map, not drop — canonical `flow.max_volumetric_mm3_s` exists).

The "never silent loss" rule applies: every present wire key
must land in a bucket.

## 3. Small remaining pieces outside the provider

- **CLI**: accept `--provider prusa` in `internal/cli/ingest.go:39-40`; the
  `--kind filament` guard (`:27`) already matches scope.
- **Round-trip king test**: `prusa → canonical → orca` value identity. Orca
  has the pattern (`orca/ingest_roundtrip_test.go`, 149 lines). This new test
  is the proof of agnosticism for the conversion.
- **mapping.md**: document each newly mapped Prusa→canonical field there
  first.
- **Import test**: `docs/import-test.md` gate #2 must run on a
  Prusa-ingested-then-Orca-rendered file.

## 4. Open questions (for the grilling phase)

1. Scope: filament only (matches MVP "filament resource first"), or also
   machine ingest? Machine ingest is missing for BOTH providers.
2. Multi-section and headerless files: slice out `[filament:*]`, or refuse?
3. Dual retraction families: which key set is authoritative in a Prusa
   filament preset?
4. `compatible_printers_condition` DSL: drop-with-warning, or parse the
   trivial `printer == "..."` forms?
5. One-off pipeline (`aldina ingest --provider prusa` then `render --provider
orca`) or a dedicated `aldina convert` command? `cli/ingest.go:19-22`
   already calls the pipeline the intended composition.
6. Which real Prusa exports are the corpus (MK4? XL? SLA excluded)?

## 5. Suggested next step

Take this note into `/grill-with-docs` to settle §4, then `/to-spec` →
`/to-tickets` → `/implement`, one vertical slice per ticket
(parser → filament ingest → report → CLI → king test).
