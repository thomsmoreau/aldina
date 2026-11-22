# Aldina Mapping Table

Canonical concepts → OrcaSlicer / PrusaSlicer fields, with tag per row.

Status: Phase 1 work-in-progress.

## Legend

- **rename** — same value, different field name
- **unit-conv** — same concept, different unit (percent vs ratio, mm/s², etc.)
- **1:N** — one canonical field maps to several native fields
- **no-equiv** — concept has no native equivalent; goes to `overrides.<provider>`

## Concept table

| canonical           | OrcaSlicer              | PrusaSlicer            | tag       |
| ------------------- | ----------------------- | ---------------------- | --------- |
| nozzle_temperature  | `nozzle_temperature`    | `nozzle_temperature`   | rename    |
| bed_temperature     | `hot_plate_temp`        | `bed_temperature`      | rename    |
| filament_flow_ratio | `filament_flow_ratio`   | `extrusion_multiplier` | rename    |
| retraction_length   | `retraction_length`     | `retraction_length`    | rename    |
| retraction_speed    | `retraction_speed`      | `retraction_speed`     | rename    |
| cooling_fan_speed   | `fan_max_speed`         | `fan_speed`            | rename    |
| layer_height        | `layer_height`          | `layer_height`         | rename    |
| wall_count          | `wall_loops`            | `perimeters`           | rename    |
| infill_density      | `sparse_infill_density` | `fill_density`         | unit-conv |
| infill_pattern      | `sparse_infill_pattern` | `fill_pattern`         | 1:N       |
| print_speed         | `speed_print`           | `max_print_speed`      | no-equiv  |

<!-- conformance:start -->

| resource | concept | value | OrcaSlicer | PrusaSlicer | note |
| -------- | ------- | ----- | ---------- | ----------- | ---- |
| filament | temps.nozzle | 240 | `nozzle_temperature` = ["240"] | `temperature` = "240" | |
| filament | temps.bed | 0 | `hot_plate_temp` = ["0"] | `bed_temperature` = "0" | authored zero is a value, not an absence |
| filament | cooling.fan_min | 0.3 | `fan_min_speed` = ["30"] | `min_fan_speed` = "30" | ratio 0–1 → 0–100 on both sides; the key names swap word order |
| filament | cooling.fan_max | 1 | `fan_max_speed` = ["100"] | `max_fan_speed` = "100" | |
| filament | physical.shrinkage_xy | 0 | `filament_shrink` = ["100%"] | `filament_shrinkage_compensation_xy` = "0%" | INVERSE — Orca asks "how big it ends up", Prusa asks "how much it shrinks" |
| filament | physical.shrinkage_xy | 0.02 | `filament_shrink` = ["98%"] | `filament_shrinkage_compensation_xy` = "2%" | |
| filament | physical.shrinkage_xy | ∅ | `filament_shrink` = absent | `filament_shrinkage_compensation_xy` = absent | absence stays absence on both sides |
| filament | meta.material | "PETG" | `filament_type` = ["PETG"] | `filament_type` = "PET" | one material, two vocabularies — PETG is "PETG" at Orca, "PET" at Prusa |
| filament | retraction.length_mm | ∅ | `filament_retraction_length` = ["nil"] | `filament_retract_length` = "nil" | nil = inherit from the machine, spelled "nil" by both |
| machine | nozzle.diameter_mm | 0.4 | `nozzle_diameter` = ["0.4"] | `nozzle_diameter` = "0.4" | |
| machine | volume.height_mm | 180 | `printable_height` = "180" | `max_print_height` = "180" | |
| machine | nozzle.min_layer_height_mm | 0.05 | `min_layer_height` = ["0.05"] | `min_layer_height` = "0.05" | |
| machine | nozzle.max_layer_height_mm | 0.25 | `max_layer_height` = ["0.25"] | `max_layer_height` = "0.25" | |
| machine | retract.speed_mms | 35 | `retraction_speed` = ["35"] | `retract_speed` = "35" | |
<!-- conformance:end -->

## Identity / metadata (Orca)

Orca treats these top-level JSON keys as import gates. A missing `version` or
`filament_settings_id` silently drops the preset on import. Pointers in
`internal/providers/orca/schema.go` encode which fields are mandatory.

One place declares which canonical inputs block the render:
`internal/providers/orca/required.go`. Of the gates below, only `meta.name`
qualifies. `version` and `from` are render constants, and
`filament_settings_id` derives from `name`.

| canonical                           | OrcaSlicer                                                         | rule                                                                                           |
| ----------------------------------- | ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------- |
| `meta.name`                         | `name`, `filament_settings_id[0]`                                  | required; `filament_settings_id` IS the preset name                                            |
| — render target `canonical.MachineRef` | `name` suffix, `filament_settings_id[0]`, `compatible_printers` | resolved against Orca's catalogue (`internal/catalog`) — pitfall #13            |
| `meta.vendor`                       | `filament_vendor`                                                  | rename; broadcast to per-extruder array                                                        |
| `meta.material`                     | `filament_type`                                                    | vocabulary map (e.g. `PETG` → `PETG`)                                                          |
| —                                   | `version`                                                          | required Semver; render constant `2.4.0.0`                                                     |
| —                                   | `from`                                                             | required; always `"User"` for rendered presets                                                 |
| — render option                     | `inherits`                                                         | `""` = root; exact existing machine `name` = derived child; MUST differ from child `meta.name` |
| —                                   | `filament_id`                                                      | stable hash of `vendor/name` (used by Orca to group presets)                                   |

## Filament resource (v0)

Sources — REAL exports only:
`references/orca/filaments/*.json` (Orca 2.6.0.2) and
`references/prusa/filaments/Ultrafuse PET @MINI - Detached.ini`
(PrusaSlicer 2.9.4, 85 keys — **repaired**, see `references/prusa/README.md`).
Every Prusa value below was read off that file, not inferred.

Cardinality reminder: Orca is per-extruder arrays throughout (`["220"]`); a
Prusa DETACHED export is scalar (`temperature = 215`) while a config bundle is
per-extruder. v0 renders the detached form.

### Identity & compatibility

| canonical       | OrcaSlicer                                     | PrusaSlicer                                | tag                                  |
| --------------- | ---------------------------------------------- | ------------------------------------------ | ------------------------------------ |
| `meta.name`     | `name`, `filament_settings_id[0]`, `filament_id` | `filament_settings_id`                     | 1:N both sides — pitfall #12         |
| `meta.vendor`   | `filament_vendor`                              | `filament_vendor`                          | 1:1 (unquoted, may contain spaces)   |
| `meta.material` | `filament_type` (`PETG` → `PETG`)              | `filament_type` (`PETG` → **`PET`**)       | vocabulary map — two different ones  |
| `meta.color_hex` | `default_filament_colour` = `"#" + hex`       | `filament_colour` = `#F7F7F7`              | rename — canon stores WITHOUT `#`    |
| `meta.notes`    | `filament_notes`                               | `filament_notes` (quoted, `\n` escaped)    | rename                               |
| `compat.machines[]` | `compatible_printers` (exact preset names)  | `compatible_printers_condition` (expression) | NOT isomorphic — pitfall #13       |

### Physical

| canonical               | OrcaSlicer                | PrusaSlicer                            | tag                       |
| ----------------------- | ------------------------- | -------------------------------------- | ------------------------- |
| `physical.diameter_mm`  | `filament_diameter`       | `filament_diameter`                    | 1:1                       |
| `physical.density_g_cm3` | `filament_density`       | `filament_density`                     | 1:1                       |
| `physical.cost_per_kg`  | `filament_cost`           | `filament_cost`                        | 1:1                       |
| `physical.abrasive`     | `required_nozzle_HRC` = `"55"` | `filament_abrasive` = `"1"`       | 1:N — hardness vs boolean |
| `physical.shrinkage_xy` | `filament_shrink` = `"100%"` | `filament_shrinkage_compensation_xy` = `"0%"` | **inverted — pitfall #14** |
| `physical.shrinkage_z`  | — (not rendered v0)       | `filament_shrinkage_compensation_z`    | Prusa-only                |

### Temperatures

| canonical                  | OrcaSlicer                              | PrusaSlicer                    | tag                          |
| -------------------------- | --------------------------------------- | ------------------------------ | ---------------------------- |
| `temps.nozzle`             | `nozzle_temperature`                    | `temperature`                  | rename                       |
| `temps.nozzle_first_layer` | `nozzle_temperature_initial_layer`      | `first_layer_temperature`      | rename                       |
| `temps.bed`                | 6 plate types × 2 = **12 fields**       | `bed_temperature`              | 1:N vs 1:1 — the asymmetry   |
| `temps.bed_first_layer`    | the `*_initial_layer` half of those 12  | `first_layer_bed_temperature`  | idem                         |
| `temps.nozzle_range`       | `nozzle_temperature_range_low`/`_high`  | — no equivalent                | no-equiv → **warning**       |
| `temps.softening`          | `temperature_vitrification`             | — no equivalent                | no-equiv → **warning**       |

### Flow & cooling

| canonical                        | OrcaSlicer                  | PrusaSlicer                 | tag                        |
| -------------------------------- | --------------------------- | --------------------------- | -------------------------- |
| `flow.max_volumetric_mm3_s`      | `filament_max_volumetric_speed` | `filament_max_volumetric_speed` | 1:1                    |
| `flow.flow_ratio`                | `filament_flow_ratio`       | `extrusion_multiplier`      | rename                     |
| `cooling.fan_min` (ratio 0–1)    | `fan_min_speed` = `"30"`    | `min_fan_speed` = `"75"`    | unit-conv + **word order** |
| `cooling.fan_max`                | `fan_max_speed`             | `max_fan_speed`             | idem                       |
| `cooling.bridges_fan`            | `overhang_fan_speed`        | `bridge_fan_speed`          | unit-conv + rename         |
| `cooling.fan_off_first_layers`   | `close_fan_the_first_x_layers` | `disable_fan_first_layers` | rename                    |
| `cooling.full_fan_at_layer`      | `full_fan_speed_layer`      | `full_fan_speed_layer`      | 1:1                        |
| `cooling.slow_down_layer_time_s` | `slow_down_layer_time`      | `slowdown_below_layer_time` | rename                     |
| `cooling.slow_down_min_speed_mm_s` | `slow_down_min_speed`     | `min_print_speed`           | rename                     |

### Retraction (filament OVERRIDES the machine; `"nil"` = inherit)

The `"nil"` string idiom is **identical on both sides** — the one place the two
formats agree exactly. Verified: Orca `filament_retraction_length: ["nil"]`,
Prusa `filament_retract_length = nil`.

| canonical                        | OrcaSlicer                    | PrusaSlicer                 | tag         |
| -------------------------------- | ----------------------------- | --------------------------- | ----------- |
| `retraction.length_mm`           | `filament_retraction_length`  | `filament_retract_length`   | rename      |
| `retraction.speed_mm_s`          | `filament_retraction_speed`   | `filament_retract_speed`    | rename      |
| `retraction.deretract_speed_mm_s` | `filament_deretraction_speed` | `filament_deretract_speed` | rename      |
| `retraction.z_hop_mm`            | `filament_z_hop`              | `filament_retract_lift`     | rename      |
| `retraction.wipe`                | `filament_wipe`               | `filament_wipe`             | 1:1         |
| `retraction.wipe_distance_mm`    | `filament_wipe_distance`      | — no equivalent             | no-equiv → **warning** |

### Gcode (opaque blobs — carried, never interpreted)

| canonical      | OrcaSlicer              | PrusaSlicer             | tag                      |
| -------------- | ----------------------- | ----------------------- | ------------------------ |
| `gcode.start`  | `filament_start_gcode`  | `start_filament_gcode`  | **word order reversed**  |
| `gcode.end`    | `filament_end_gcode`    | `end_filament_gcode`    | idem                     |

#### Pitfall #12 — identity is redundant on both sides, differently

Orca needs THREE coherent fields (`name`, `filament_settings_id`,
`filament_id`); Prusa takes it from the **`[filament:NAME]` section header**,
and leaves `filament_settings_id` EMPTY in a bundle. Generate them together or
not at all.

#### Pitfall #15 — a headerless Prusa INI is a whole CONFIGURATION

A flat `key = value` file with no section header is what PrusaSlicer writes for
"Export Config": the complete current setup, all three categories at once.
If you import one, PrusaSlicer adopts the file as the print, the filament,
and the printer, all three named after it. This was observed on 2.9.6: all
three slots of `PrusaSlicer.ini` pointed at the same imported file.

A shareable single preset must therefore be a **config bundle**:

```ini
# generated by aldina
[filament:Overture PETG Rock White]
bed_temperature = 80
…
```

`PresetBundle::load_configbundle` accepts `print:`, `filament:`, `sla_print:`,
`sla_material:`, `printer:`, `physical_printer:`, plus `presets` and
`obsolete_presets`; unknown sections are ignored and **`[presets]` is not
required**. We omit `[presets]` on purpose — it names the ACTIVE selection, and
sharing a filament must not switch the recipient's printer or print profile.

#### Pitfall #13 — the two slicers bind a filament to a machine DIFFERENTLY

A preset name belongs to a slicer's catalogue, not to the machine. The canonical
therefore names a MACHINE (`compat.machines[] = {model, nozzle_mm}`) and each
provider resolves it in its own catalogue — see `internal/catalog`.

Counted over the catalogues both slicers ship
(`<App>/Contents/Resources/profiles/`):

| system filament presets | Orca     | Prusa    |
| ----------------------- | -------- | -------- |
| total                   | 4455     | 7528     |
| bound by **exact name** | **3927** | 17       |
| bound by **condition**  | **0**    | **1149** |

Orca never uses a condition. Prusa almost only uses one. Variables actually used
in Prusa conditions: `nozzle_diameter` (1321), `printer_model` (1264),
`printer_notes` (498), `nozzle_high_flow` (240), `num_extruders` (6) — so
`printer_model=="MEGA0"` is a valid and common form, not just the
`printer_notes` regex. For this reason, each provider renders as follows:

- Orca → `compatible_printers: [<exact preset names>]`, condition left empty
  (0/4455 of its own presets use one — untested ground);
- Prusa → `compatible_printers` empty, and
  `compatible_printers_condition = printer_model=="MINI" and nozzle_diameter[0]==0.4`.

The resolution key exists on both sides: every machine preset carries
`printer_model` + `printer_variant` (the nozzle). Measured: Orca 1001
instantiable presets → 981 `(vendor, model, nozzle)` keys, Prusa 1066 → 516.
Model names are globally UNIQUE inside each catalogue (0 collision across
vendors, on 384 and 264 models), which is why `MachineRef` carries no vendor.

Two traps in that key:

- a key may designate SEVERAL presets (8 collisions in Orca — Raise3D
  Left/Right/Dual, Volumic COPY/MIRROR MODE, Sovol High-Speed; 36 in Prusa —
  MMU Single, BL-TOUCH, Duplication Mode). We emit them all: the filament holds
  for every variant of the same model and nozzle;
- the nozzle must be compared as a NUMBER. Prusa writes `printer_variant = 0.50`
  where Orca writes `0.5`; string equality would miss the machine.

The two catalogues do not spell models alike — `Prusa MINI` (Orca) vs
`Original Prusa MINI & MINI+`, id `MINI` (Prusa). 63 of them match exactly, 74
after normalisation, so only 11 differ by punctuation
(`Elegoo Neptune 3`/`Elegoo Neptune-3`, `UltiMaker 2`/`Ultimaker 2`,
`FLSun QQ-S Pro`/`FLSun QQS Pro`…). `internal/catalog/aliases.json` bridges the
equivalence classes; there is no "canonical spelling" to invent.

#### Pitfall #14 — shrinkage is INVERTED between the two formats

The same physical reality, three encodings:

| | value for "no shrinkage compensation" |
| --- | --- |
| canonical `physical.shrinkage_xy` | `0.0` (a RATE) |
| Orca `filament_shrink` | `"100%"` (a SIZE RATIO) |
| Prusa `filament_shrinkage_compensation_xy` | `"0%"` (a RATE) |

Orca asks "how big should it end up", Prusa asks "how much does it shrink".
A naive copy between the two turns a neutral profile into a 100 % error. This
is the main case for the value round-trip test (CONTRIBUTING.md, "the king
test"). No single-provider test can catch it.

## Machine resource (v0)

Sources — REAL exports only, no composites:
`references/orca/printers/Bambu Lab P2S 0.6 nozzle.json` (Orca 2.1.0.16,
156 keys) and `references/prusa/printers/Prusa_XL.ini` (PrusaSlicer 2.9.x,
91 keys, user profile of "Original Prusa XL Input Shaper 0.4 nozzle").

Extra tags for this section:

- **1:1** — same name, same encoding both sides
- **conversion** — same concept, different serialization (cardinality, mini-format)
- **canonical-only** — absent from BOTH formats by design; dropped at render
  WITHOUT warning (it feeds `aldina validate`, not slicer files)
- **Orca-only / Prusa-only** — no equivalent on the other side; concept stays
  canonical when it is a machine PROPERTY, otherwise it would be an override

### Identity & linkage

| canonical     | OrcaSlicer                                     | PrusaSlicer                                                                    | tag                                                |
| ------------- | ---------------------------------------------- | ------------------------------------------------------------------------------ | -------------------------------------------------- |
| `meta.name`   | `name`                                         | `[printer:...]` section header                                                 | structural — pitfall #1                            |
| `meta.vendor` | vendor dir + `<Vendor>.json` (not rendered v0) | `printer_vendor` (often empty) + `PRINTER_VENDOR_*` keyword in `printer_notes` | 1:N                                                |
| `meta.model`  | `printer_model` (`"Bambu Lab P2S"`) — verbatim | `printer_model` (`"XLIS"`) — catalogue ID, resolved                            | conversion — pitfall #11                           |
| `meta.tags`   | `printer_notes` left empty v0                  | extra keywords appended in `printer_notes` (`INPUT_SHAPER`, `PG`)              | 1:N — pitfall #1                                   |
| — (derived)   | `printer_variant` = `"0.6"`                    | `printer_variant` = `0.4`                                                      | derived from `nozzle.diameter_mm`, never canonized |
| `meta.name`   | `printer_settings_id` — **scalar**             | `printer_settings_id`                                                          | IS the preset name — pitfall #10                   |
| —             | `from`, `inherits`, `version`                  | `inherits`, `printer_settings_id`                                              | render constants — pitfall #7                      |
| —             | `printer_technology` = `"FFF"`                 | `printer_technology` = `FFF`                                                   | constant                                           |

#### Pitfall #10 — `<kind>_settings_id` is per-kind, and its cardinality varies

Every user preset Orca accepts carries its own settings id, but **not in the
same shape**. Read off real files, never inferred from a sibling:

| kind     | key                    | form                                              |
| -------- | ---------------------- | ------------------------------------------------- |
| filament | `filament_settings_id` | **array** — `["Francofil PLA Wood @BBL P2S 0.6"]` |
| process  | `print_settings_id`    | **scalar** — `"Fast-Strong Technical 0.30mm …"`   |
| machine  | `printer_settings_id`  | **scalar** — `""` on the system export            |

#### Pitfall #11 — `type` is a CLI key, not a preset key

`"type": "machine"|"filament"|"process"` is **required by
`orca-slicer --load-settings`** ("unknown config type of file … in
load-settings") and is **absent from every GUI-importable user preset** on
disk. Our own filament preset, which the Orca UI imports without complaint,
carries no `type`. The conformance harness
(`internal/providers/orca/conform_test.go`) injects it, and Aldina **never
renders** it.

Same file records a second CLI-only constraint, measured on 2.4.2: a machine
with `from: "User"` is refused by `--load-settings` whatever else it carries,
while the GUI imports it fine. The CLI validates preset CONTENT; it is not the
GUI import gate, and a green harness never replaces one real import.

On import, Orca rewrites the preset down to a minimal overlay on `inherits`.
It drops every key whose value matches the resolved parent (41 → 27 on the
verified machine), and it adds `printer_extruder_id` and
`printer_extruder_variant`. It alters no value we sent. A non-minimal
overlay is therefore safe to render.

### Volume & geometry

| canonical                             | OrcaSlicer                                                          | PrusaSlicer                                              | tag                     |
| ------------------------------------- | ------------------------------------------------------------------- | -------------------------------------------------------- | ----------------------- |
| `volume.width_mm` + `volume.depth_mm` | `printable_area` = `["0x0","256x0","256x256","0x256"]` (JSON array) | `bed_shape` = `0x0,360x0,360x360,0x360` (ONE CSV string) | conversion — pitfall #2 |
| `volume.height_mm`                    | `printable_height`                                                  | `max_print_height`                                       | rename                  |

### Nozzle

| canonical                     | OrcaSlicer                                                              | PrusaSlicer                 | tag                                                             |
| ----------------------------- | ----------------------------------------------------------------------- | --------------------------- | --------------------------------------------------------------- |
| `nozzle.diameter_mm`          | `nozzle_diameter` = `["0.6"]`                                           | `nozzle_diameter` = `0.4`   | conversion — pitfall #3                                         |
| `nozzle.type`                 | `nozzle_type` per-variant array (`["hardened_steel","hardened_steel"]`) | ABSENT                      | Orca-only; real hardware property, drives abrasive cross-check  |
| `nozzle.extruder`             | `extruder_type` = `["Direct Drive"]` (WITH space)                       | ABSENT                      | Orca-only                                                       |
| `nozzle.filament_diameter_mm` | ABSENT                                                                  | ABSENT                      | canonical-only (cross-check vs `filament.physical.diameter_mm`) |
| `nozzle.height_mm`            | `nozzle_height` = `"4.2"` (scalar)                                      | ABSENT                      | Orca-only                                                       |
| `nozzle.hrc`                  | `nozzle_hrc` = `"0"` (0 = unknown on the real P2S)                      | ABSENT                      | Orca-only; refines abrasive cross-check, nil/0 = unknown → skip |
| `nozzle.min_layer_height_mm`  | `min_layer_height` = `["0.12"]`                                         | `min_layer_height` = `0.07` | 1:1 — M0-verified both sides                                    |
| `nozzle.max_layer_height_mm`  | `max_layer_height` = `["0.42"]`                                         | `max_layer_height` = `0.3`  | 1:1                                                             |

### Limits (canonical-only, the `aldina validate` payload)

Absent from BOTH formats (firmware/vendor-spec data) — verified on both real
exports. Providers drop them silently: expected, not a loss.

`limits.max_nozzle_temp_c`, `limits.max_bed_temp_c`, `limits.max_chamber_temp_c`,
`limits.max_volumetric_mm3_s`.

### Kinetics

Both formats store [normal, silent/stealth] value pairs per field — convergent
concept. Canonical carries NORMAL mode only; providers duplicate; silent-mode
tuning goes to overrides (promotion candidate, parked). Cardinality: pitfall #3.

| canonical                            | OrcaSlicer                   | PrusaSlicer                  | tag                                    |
| ------------------------------------ | ---------------------------- | ---------------------------- | -------------------------------------- |
| `kinetics.max_speed_mm_s.{x,y,z,e}`  | `machine_max_speed_*`        | `machine_max_feedrate_*`     | rename (speed ≠ feedrate false friend) |
| `kinetics.max_accel_mm_s2.{x,y,z,e}` | `machine_max_acceleration_*` | `machine_max_acceleration_*` | 1:1                                    |
| `kinetics.max_jerk_mm_s.{x,y,z,e}`   | `machine_max_jerk_*`         | `machine_max_jerk_*`         | 1:1                                    |
| `kinetics.min_travel_mm_s`           | `machine_min_travel_rate`    | `machine_min_travel_rate`    | 1:1                                    |
| `kinetics.min_extrude_mm_s`          | `machine_min_extruding_rate` | `machine_min_extruding_rate` | 1:1                                    |

### Retraction (machine DEFAULTS — filament overrides or inherits via "nil")

| canonical                      | OrcaSlicer                    | PrusaSlicer             | tag                                      |
| ------------------------------ | ----------------------------- | ----------------------- | ---------------------------------------- |
| `retract.length_mm`            | `retraction_length`           | `retract_length`        | rename (retraction/retract false friend) |
| `retract.speed_mm_s`           | `retraction_speed`            | `retract_speed`         | rename                                   |
| `retract.deretract_speed_mm_s` | `deretraction_speed`          | `deretract_speed`       | rename                                   |
| `retract.min_travel_mm`        | `retraction_minimum_travel`   | `retract_before_travel` | rename                                   |
| `retract.z_hop_mm`             | `z_hop`                       | `retract_lift`          | rename (no common word)                  |
| `retract.on_layer_change`      | `retract_when_changing_layer` | `retract_layer_change`  | rename                                   |
| `retract.wipe`                 | `wipe`                        | `wipe`                  | 1:1                                      |
| `retract.wipe_distance_mm`     | `wipe_distance`               | ABSENT                  | Orca-only                                |

### Firmware behavior

| canonical                      | OrcaSlicer                         | PrusaSlicer                      | tag              |
| ------------------------------ | ---------------------------------- | -------------------------------- | ---------------- |
| `firmware.relative_e`          | `use_relative_e_distances` = `"1"` | `use_relative_e_distances` = `1` | 1:1 — pitfall #4 |
| `firmware.firmware_retraction` | `use_firmware_retraction`          | `use_firmware_retraction`        | 1:1              |

### Features

| canonical                 | OrcaSlicer                     | PrusaSlicer | tag                                          |
| ------------------------- | ------------------------------ | ----------- | -------------------------------------------- |
| `features.enclosed`       | ABSENT                         | ABSENT      | canonical-only (cross-check ABS/ASA chamber) |
| `features.plates`         | ABSENT                         | ABSENT      | canonical-only (future plate fan-out driver) |
| `features.aux_fan`        | `auxiliary_fan`                | ABSENT      | Orca-only                                    |
| `features.air_filtration` | `support_air_filtration`       | ABSENT      | Orca-only                                    |
| `features.heated_chamber` | `support_chamber_temp_control` | ABSENT      | Orca-only                                    |

### Gcode (opaque blobs, NEVER interpreted — templating incompatible, pitfall #6)

| canonical               | OrcaSlicer                  | PrusaSlicer                | tag                       |
| ----------------------- | --------------------------- | -------------------------- | ------------------------- |
| `gcode.flavor`          | `gcode_flavor` (`marlin`)   | `gcode_flavor` (`marlin2`) | enum vocabulary alignment |
| `gcode.start`           | `machine_start_gcode`       | `start_gcode`              | rename                    |
| `gcode.end`             | `machine_end_gcode`         | `end_gcode`                | rename                    |
| `gcode.before_layer`    | `before_layer_change_gcode` | `before_layer_gcode`       | rename                    |
| `gcode.layer_change`    | `layer_change_gcode`        | `layer_gcode`              | rename                    |
| `gcode.pause`           | `machine_pause_gcode`       | `pause_print_gcode`        | rename                    |
| `gcode.filament_change` | `change_filament_gcode`     | `color_change_gcode`       | rename                    |

### Default profiles

| canonical                   | OrcaSlicer                                     | PrusaSlicer                                           | tag                     |
| --------------------------- | ---------------------------------------------- | ----------------------------------------------------- | ----------------------- |
| `defaults.print_profile`    | `default_print_profile` (scalar)               | `default_print_profile` (scalar)                      | 1:1                     |
| `defaults.filament_profile` | `default_filament_profile` = `["..."]` (array) | `default_filament_profile` = `"..."` (scalar, quoted) | conversion — pitfall #5 |

### Machine pitfalls

1. **Linkage is not isomorphic, and `meta.name` is NOT the key.** Orca: filament
   lists EXACT machine names in `compatible_printers`. Prusa: machine exposes
   keywords in `printer_notes` (`PRINTER_VENDOR_PRUSA3D`, `PRINTER_MODEL_XLIS`,
   plus capability tags like `INPUT_SHAPER`, `PG`); filament matches by
   expression in `compatible_printers_condition` (`printer_model==`, a regex
   over `printer_notes`, `nozzle_diameter[0]`, `nozzle_high_flow[0]`). Full
   figures in pitfall #13.
   Canonical model: the linkage key is `(meta.model, nozzle.diameter_mm)`, which
   `internal/catalog` resolves to each slicer's own syntax. `meta.name` is a
   HUMAN LABEL — the title the rendered preset gets, nothing more. Writing
   Orca's catalogue title there (`"Bambu Lab P2S 0.6 nozzle"`) restates
   `meta.model` and the nozzle in one slicer's convention: the exact violation
   `compat.printers` used to carry. Derived `PRINTER_VENDOR_/PRINTER_MODEL_`
   still come from `meta.vendor`/`meta.model`, and `meta.tags` carry
   non-derivable capabilities.
   Orca additionally appends the reference's own spelling (`MachineRef.String()`,
   e.g. `"Bambu Lab P2S 0.6"`) to `compatible_printers`, so a machine titled
   `<model> <nozzle>` self-links when both are imported. A machine titled
   anything else is not self-linked yet.
2. **Mini-formats inside the format.** Point `"XxY"` (printable_area,
   bed_shape, extruder_offset, wrapping_exclude_area); comma pair `"0.5,0.5"`
   (best_object_pos — COMMA, not `x`); percent `"80%"` (retract_before_wipe).
   One `formatPoint` brick, two area serializers (JSON array vs CSV string).
3. **Array cardinality families (Orca).** Per-extruder `[v]` (nozzle*diameter,
   extruder_offset, extruder_colour, min/max_layer_height); per-extruder-VARIANT
   `[v]×N` (nozzle_type, nozzle_volume, retraction*\*); kinetics
   `[normal,silent]×N` variants — the P2S 0.6 export has 2 variants
   (Standard/High Flow) → kinetics arrays of 4, retraction of 2. Confirmed by
   observation: the A1 mini system profile (1 variant) has kinetics arrays of
   2 and `nozzle_type` of 1 — array length = 2 × variants. v0 renders
   ONE variant: kinetics `[v,v]`, variant fields `[v]`, extruder fields `[v]`.
   Extruder variants parked.
4. **Bool encodings differ.** Orca: `"1"`/`"0"` strings; Prusa: `1`/`0` ints.
5. **Default-profile asymmetry.** Orca wraps `default_filament_profile` in an
   array, Prusa keeps it scalar (and quotes it). `default_print_profile` is
   scalar both sides.
6. **Gcode is slicer-templated on both sides** (`{if ...}{endif}`,
   `nozzle_diameter[0]`, `[printer_model]`). Blobs are opaque; a canonical
   profile carries neutral blobs + per-provider overrides, never a translation.
7. **Inheritance mechanics differ.** Orca inherits across files
   (`fdm_machine_common`); Prusa user profiles carry `inherits` = parent SYSTEM
   preset name yet still export every setting flat. v0 renders standalone:
   Orca `inherits: ""`, Prusa `inherits =`.
8. **Thumbnails converged** (fork heritage): ONE string `"48x48/PNG,300x300/PNG"`
   (Orca) / `"16x16/QOI, 313x173/QOI"` (Prusa), `thumbnails_format` both.
   Overrides v0; canonization candidate.
9. **Schema freezes on real exports, not composites.** The Orca 2.1.0.16
   export has NO `setting_id`/`instantiation`, `from: "System"` (capital S),
   `extruder_type: "Direct Drive"` (with space), `best_object_pos` with a
   comma. Prusa 2.9.x dropped `extrusion_axis`. Earlier composites were wrong
   on all of these.
10. **Fork-convergent keys** (same name both sides, PrusaSlicer heritage):
    `machine_max_junction_deviation`, `travel_slope`, MMU cluster
    (`cooling_tube_length`, `cooling_tube_retraction`, `parking_pos_retraction`,
    `extra_loading_move`, `high_current_on_filament_swap`), `z_offset`,
    `single_extruder_multi_material`, `silent_mode`, host keys. Overrides v0;
    MMU cluster becomes 1:1 when multi-material un-parks.
11. **`printer_model` is an ID on the Prusa side, and the machine render must
    resolve it.** A filament's `compatible_printers_condition` compares
    `printer_model=="MINI"` — the catalogue id, never the human name the author
    writes. `prusa.RenderMachine` therefore runs the SAME `catalog.Lookup` the
    filament's compat clause runs, and refuses a model in neither catalogue: a
    preset carrying `printer_model = Original Prusa MINI & MINI+` would import
    cleanly and match nothing (the "accepted but invisible" failure). Two
    consequences:
    - `printer_variant` is taken from the catalogue VERBATIM (`"0.40"`,
      `"0.50"`), not from `formatFloat(nozzle)`: PrusaSlicer matches variants as
      strings.
    - **`PRINTER_VENDOR_` is NOT mechanically derivable.** Real exports say
      `PRINTER_VENDOR_PRUSA3D`, which no uppercase/underscore rule produces from
      `"Prusa Research"`. Aldina derives it best-effort (uppercase,
      non-alphanumeric → `_`), so a rendered printer is NOT matched by stock
      vendor filaments that regex on the canonical keyword. `PRINTER_MODEL_`
      has no such problem: it takes the catalogue id verbatim. Fixing the vendor
      half needs a `Vendor` field on `catalog.Machine` — parked.
    Orca does NOT do any of this: `orca.RenderMachine` emits `meta.model`
    verbatim, because Orca binds filaments by exact preset name and never
    compares the model. That asymmetry is deliberate today and pinned by
    `TestOrcaMachineModelIsNotCatalogueResolved`.

### Known overrides (v0)

- **overrides.orca**: extruder clearances (`extruder_clearance_radius`,
  `_height_to_rod`, `_height_to_lid`), `head_wrap_detect_zone`,
  `upward_compatible_machine`, `scan_first_layer`,
  `machine_load/unload_filament_time`, `machine_tool_change_time`,
  `input_shaping_*`, `resonance_avoidance*`, `bed_mesh_*`,
  `bed_temperature_formula`, `default_bed_type`, `bed_exclude_area`,
  `nozzle_volume` (promotion candidate, parked with variants),
  `default_nozzle_volume_type`, `extruder_variant_list`,
  `printer_extruder_variant`, `printer_extruder_id`,
  `emit_machine_limits_to_gcode`, `silent_mode`, `thumbnails*`, `wrapping_*`,
  `travel_slope`, `best_object_pos`, extra gcode blobs (`time_lapse_gcode`,
  `file_start_gcode`, `change_extrusion_role_gcode`,
  `printing_by_object_gcode`, `wrapping_detection_gcode`),
  `parallel_printheads_*`, `wipe_tower_*`.
- **overrides.prusa**: `machine_limits_usage` (policy enum: emit_to_gcode /
  time_estimate_only / ignore — Orca has only the bool
  `emit_machine_limits_to_gcode`), `silent_mode`, `binary_gcode`,
  `use_volumetric_e`, `autoemit_temperature_commands`, `remaining_times`,
  `nozzle_high_flow` (compat-condition capability, parked with variants),
  `travel_lift_before_obstacle`, `travel_max_lift`, `travel_ramping_lift`,
  `travel_slope`, `prefer_clockwise_movements`, `variable_layer_height`,
  `multimaterial_purging`, `custom_parameters_printer`,
  `extruder_clearance_height`, `extruder_clearance_radius`,
  `between_objects_gcode`, `toolchange_gcode`, `thumbnails*`,
  `bed_custom_model`, `bed_custom_texture`.

### Excluded forever

Host/deployment keys are NOT machine descriptions and may contain secrets:
`print_host`, `printhost_apikey`, `printhost_cafile`, `host_type`,
`serial_port`, `serial_speed`, `bbl_use_printhost`,
`printhost_authorization_type`, `printhost_ssl_ignore_revoke`,
`printer_agent`. Never canonized, never rendered from canonical.

## Notes

- Fill rows one concept at a time from real Orca/Prusa exports.
- Keep values in canonical units (°C, mm, mm/s, ratio 0.0–1.0).
- When in doubt, use `overrides` passthrough rather than forcing an equivalence.
- Nil pointer on a canonical scalar = key omitted at render; the slicer default
  or parent preset applies. The only blocking validation lives in the provider
  when an importable file cannot be produced at all (e.g. Orca without name).
