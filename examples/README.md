# Aldina — Canonical FILAMENT format (v0.2)

> Derived from comparing REAL files: `Overture PETG @P2S 0.6` (Orca JSON, exported) × `Ultrafuse PET @MINI - Detached` (PrusaSlicer 2.9.4, exported).
> Conventions: clean types (float, int, bool), enforced units (°C, mm, mm/s, mm³/s, 0–1 ratios), `null` = inherit from machine.
> Version 0.2 fixes irregularity #5, a GitHub artefact. It adds the PET/PETG material enum, the abrasive concept, the promotion rule, and the loss policy.

## 1. Proposed canonical format (YAML)

```yaml
# filament.yaml — Aldina canonical profile
kind: filament
schema_version: 0

meta:
  name: "Overture PETG Rock White"
  vendor: "Overture"
  material: PETG            # canonical enum: PLA, PETG, ABS, ASA, TPU, PC, PA...
                            # rendered: Orca="PETG", Prusa="PET" (vocabulary mapping)
  color_hex: "B3B4B2"       # WITHOUT the '#' (INI trap, see irregularity #5)
  notes: "Sécher 60°C 5h si stringing."

physical:
  diameter_mm: 1.75
  density_g_cm3: 1.27
  cost_per_kg: 21.99
  abrasive: false           # shared concept in 2 forms:
                            # Prusa filament_abrasive (bool) / Orca required_nozzle_HRC (hardness)

temps:                       # °C, float
  nozzle: 255.0
  nozzle_first_layer: 257.0
  nozzle_range: [220.0, 260.0]
  bed: 80.0
  bed_first_layer: 80.0
  softening: 85.0            # vitrification. Orca-only → drop+warning on Prusa side

flow:
  max_volumetric_mm3_s: 25.0
  flow_ratio: 0.945          # Orca filament_flow_ratio / Prusa extrusion_multiplier

cooling:
  fan_min: 0.30              # 0–1 ratios
  fan_max: 0.50
  fan_off_first_layers: 3
  full_fan_at_layer: 0
  slow_down_layer_time_s: 1.0
  slow_down_min_speed_mm_s: 20.0
  bridges_fan: 0.80

retraction:                  # absent / null = inherit from machine
  length_mm: 1.4
  speed_mm_s: null
  deretract_speed_mm_s: null # Orca filament_deretraction_speed / Prusa filament_deretract_speed (!)
  z_hop_mm: 0.4
  wipe: true
  wipe_distance_mm: 2.0

gcode:                       # opaque per-provider blocks — never canonized
  start: "; filament start gcode\n"
  end: "; filament end gcode\n"

compat:                      # the MACHINE, never a preset name
  machines:
    - model: "Bambu Lab P2S"
      nozzle_mm: 0.6

overrides:                   # escape hatch for irreducibles — NOTHING is lost
  orca:
    pressure_advance: ["0.014"]
    adaptive_pressure_advance: ["1"]
    filament_z_hop_types: ["Spiral Lift", "Spiral Lift"]
  prusa:
    filament_ramming_parameters: "120 100 6.6 ..."
    cooling_slowdown_logic: uniform_cooling
```

## 2. Mapping table (docs/mapping.md — excerpt verified against real files)

| canonical concept | Orca (JSON) | Prusa (INI 2.9.4) | nature | trap |
|---|---|---|---|---|
| `meta.vendor` | `filament_vendor[]` | `filament_vendor` | rename+array | Prusa: unquoted WITH space (`Forward AM`) |
| `meta.material` | `filament_type[]` = `PETG` | `filament_type` = `PET` | **enum vocab** | same family, different names → enum table |
| `meta.color_hex` | `default_filament_colour[]` = `#B3B4B2` | `filament_colour` = `#F7F7F7` | rename | unquoted `#` = INI comment! canon = hex WITHOUT `#` |
| `physical.diameter_mm` | `filament_diameter[]` | `filament_diameter` | 1:1 | — |
| `physical.density_g_cm3` | `filament_density[]` | `filament_density` | 1:1 | — |
| `physical.abrasive` | `required_nozzle_HRC[]` (hardness) | `filament_abrasive` (bool) | **concept, different forms** | 2 encodings of the same fact |
| `temps.nozzle` | `nozzle_temperature[]` | `temperature` | rename+array | — |
| `temps.nozzle_first_layer` | `nozzle_temperature_initial_layer[]` | `first_layer_temperature` | rename | — |
| `temps.bed` | `hot_plate_temp[]` + 5 more/plate | `bed_temperature` | **1:N** | Orca `cool_plate: 0` = incompatible (hidden semantic) |
| `temps.bed_first_layer` | `*_initial_layer` ×6 | `first_layer_bed_temperature` | 1:N | same |
| `temps.softening` | `temperature_vitrification[]` | ∅ | Orca-only | drop+warning on Prusa side |
| `flow.max_volumetric_mm3_s` | `filament_max_volumetric_speed[]` | `filament_max_volumetric_speed` | 1:1 | values differ per variant on Orca side |
| `flow.flow_ratio` | `filament_flow_ratio[]` | `extrusion_multiplier` | treacherous rename | no word in common |
| `cooling.fan_min/max` | `fan_min/max_speed[]` (0–100) | `min/max_fan_speed` (0–100) | unit | canon = 0–1 ratio; word order reversed! |
| `cooling.fan_off_first_layers` | `close_fan_the_first_x_layers[]` | `disable_fan_first_layers` | rename | — |
| `cooling.slow_down_layer_time_s` | `slow_down_layer_time[]` | `slowdown_below_layer_time` | rename | verify identical semantics |
| `cooling.bridges_fan` | `overhang_fan_speed[]` + enable | `bridge_fan_speed` + `overhang_fan_speed_0..3` | **bilateral 1:N** | non-isomorphic models → simple case in canon |
| `retraction.length_mm` | `filament_retraction_length[]` | `filament_retract_length` | rename | `retraction` vs `retract` |
| `retraction.deretract_speed_mm_s` | `filament_deretraction_speed[]` | `filament_deretract_speed` | rename | near-homonyms |
| `retraction.z_hop_mm` | `filament_z_hop[]` | `filament_retract_lift` | rename | no word in common |
| `retraction.* = null` | `["nil","nil"]` | literal `nil` | **shared convention** | common Slic3r inheritance |
| `gcode.start/end` | `filament_start/end_gcode[]` | `start/end_filament_gcode` | opaque | Prusa: `{if ...}` templating → never canonize |
| `compat.machines[]` | `compatible_printers[]` (exact preset names) | `compatible_printers_condition` (expression) | **not isomorphic** | resolved per slicer via `internal/catalog` |

## 3. Canonical design rules

### 3.1 Admission criterion: property vs knob

A field enters the canon if it describes **the filament or the print intent** (portable by nature), not if it tunes **a slicing engine's implementation**. For example, `temperature_vitrification` is a material property, so it enters the canon even though only Orca expresses it. In contrast, `filament_ramming_parameters` is a Prusa multi-material knob, so it always stays in overrides.

### 3.2 Promotion rule

When in doubt, use `overrides`. The canon promotes a concept only on evidence: when a second slicer expresses the same concept, even under another name or encoding (compare `abrasive` with `HRC`), we promote it. We then bump `schema_version`, and providers gain the mapping. The canon grows from evidence, pulled by reality, never added "just in case".

### 3.3 Loss policy (render)

The renderer never drops a field silently. A canonical field not expressible by a provider → **warning + drop** by default (`warning: temps.softening: not supported by prusa, ignored`), error with `--strict` (farms/industrial where fidelity is contractual). Loss is visible and chosen.

### 3.4 Standing v0 decisions

- **`null` = inherit from machine** (shared `nil` concept of both slicers). Go: pointers or `Inheritable[T]`.
- **0–1 ratios everywhere** (fan, flow_ratio).
- **Single `temps.bed`**, Orca multi-plate generated at render (hot+textured = value, incompatible plates = 0), refinable via `overrides.orca`.
- **Per-extruder granularity NOT canonical in v0**: one value, the provider broadcasts; the High Flow variant goes to `overrides.orca`.
- **`pressure_advance` as Orca override** (no clean 1:1 mapping on Prusa side).

## 4. Observed irregularities (adapter material)

1. **Orca plate: 12 fields** (6 types × initial layer) vs 2 at Prusa. And `cool_plate_temp: 0` = "incompatible plate" — semantics hidden in a value.
2. **Inconsistent Orca array cardinality** within the same file (`["30"]` vs `["255","240"]`). Read `filament_extruder_variant` to know N; 1 element = broadcast.
3. **Orca strings = 7 hidden types**: float, int, bool, `"70%"`, `"nil"`, `"-1"` sentinel (= auto), serialized matrix (`"0,0,0\n0,0,0"`).
4. **Overhang cooling: bilateral 1:N** (threshold+speed at Orca vs 4 tiers + bridge at Prusa). Non-isomorphic → simple case in canon, rest in overrides.
5. **Prusa INI, real traps (fixed v0.2)**: 2.9.4 exports are well-formed (gcode = one quoted line with escaped `\n` — NO dangling quote, that was a GitHub file artefact). The real traps: `filament_colour = #F7F7F7` **unquoted starting with `#`** (= comment for a naïve INI parser); `filament_vendor = Forward AM` unquoted with space; *detached* export WITHOUT section header vs Config Bundle WITH `[filament:...]` — two variants of the same format.
6. **Lexical false friends**: `filament_retraction_length`/`filament_retract_length`, `filament_deretraction_speed`/`filament_deretract_speed`, `fan_min_speed`/`min_fan_speed` (reversed order), `slow_down_layer_time`/`slowdown_below_layer_time`.
7. **The shared `nil` convention** = inherit from machine → free canonical concept (`null`).
8. **Redundant Orca identity**: `name`, `filament_settings_id[]`, `filament_id` — ALL must be generated consistently at render, otherwise erratic import.
9. **Shrinkage: INVERTED encodings.** Orca `filament_shrink = "100%"` (relative size: 100% = no shrink) vs Prusa `filament_shrinkage_compensation_xy = 0%` (compensation: 0% = no compensation). Same concept, opposite zeros — the nastiest conversion trap in the corpus. Proposed canon: `physical.shrinkage_xy: 0.0` (shrink rate, 0 = none) and each provider converts.
10. **Version fields**: 2.9.4 introduces keys absent from older examples (`cooling_slowdown_logic`, `filament_infill_max_speed`, the `filament_travel_*` family, `filament_abrasive`). Living confirmation of "providers pinned to a version".

## 5. Next action (vertical slice, unchanged)

1. The YAML in §1 → `canonical/filament.example.yaml`.
2. Go structs `canonical.Filament` (clean types, `*float64` for inheritable).
3. `orca.RenderFilament` on **`temps.nozzle` only** → OrcaSlicer import test.
4. `prusa.RenderFilament` same → PrusaSlicer import test.
5. Next concept from the table.
