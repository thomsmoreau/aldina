# Import test

The import test is the last gate of a slice. A render is proven only when the
slicer imports it AND keeps the authored values. An import that succeeds while
overwriting the values with defaults proves nothing.

The test has two parts:

1. The slicer imports the file without error.
2. The imported preset keeps the values of the source YAML.

A pass has three states:

- **PASS** — the file imports and the values match.
- **PARTIAL PASS** — the file imports, but some values differ or the preset is
  not usable on the intended machine.
- **FAIL** — the import is refused.

## Generate the files

```sh
# Orca pair — a Bambu machine (in Orca's catalogue, absent from Prusa's).
go run ./cmd/aldina render --provider orca --kind filament \
  --in examples/example_mk1.yaml --out /tmp/import-test
go run ./cmd/aldina render --provider orca --kind machine \
  --in examples/machine_p2s_06.yaml --out /tmp/import-test

# Prusa pair — a MINI machine (in Prusa's catalogue).
go run ./cmd/aldina render --provider prusa --kind filament \
  --in examples/filament_prusa_mini.yaml --out /tmp/import-test
go run ./cmd/aldina render --provider prusa --kind machine \
  --in examples/machine_prusa_mini_04.yaml --out /tmp/import-test
```

The honest demo is one filament per ecosystem: a filament rendered toward its
own machine in that slicer, never the same machine reference on both sides.
`example_mk1.yaml` targets a Bambu machine; rendering it to Prusa fails at
render time because Prusa's catalogue has no Bambu models. That refusal is by
design, not a defect.

## OrcaSlicer (2.4.2)

1. `File → Import → Import Configs…`.
2. Import the machine file, then the filament file.
3. In the selector, choose the machine, then the filament.
4. Open the filament and compare four values with the source YAML: nozzle
   temperature, bed temperature, flow, material type.

The filament declares two machine names in `compatible_printers`: the catalogue
name and the reference spelling (`MachineRef.String()`). The first keeps the
system machine bound; the second self-links a machine rendered by Aldina and
titled `<model> <nozzle>`.

## PrusaSlicer (2.9.6)

1. `File → Import → Import Config Bundle…`, one file at a time. Use the
   **Bundle** item, not `Import Config…`: `Import Config…` is a silent no-op on
   single-preset files.
2. Import the printer file, then the filament file.
3. In the printer selector, choose the MINI+ machine. A filament preset is
   hidden until a compatible machine is selected — the compatibility condition
   filters the list. That is by design, not an import failure.
4. Open the filament and compare four values with the source YAML: nozzle
   temperature, bed temperature, extrusion multiplier, material type.

## Value fidelity check

The slicer pads an imported preset with its defaults. The check is that it never
changes the authored values. Diff the installed copy against the generated
file:

- PrusaSlicer: `~/Library/Application Support/PrusaSlicer/filament/` and
  `…/printer/`.
- OrcaSlicer: `~/Library/Application Support/OrcaSlicer/user/<profile>/…`.

The diff must show only added defaults, never a changed authored value.

## Known findings

- PrusaSlicer `Import Config…` (single) is a silent no-op on Aldina's
  single-preset files; `Import Config Bundle…` works. Use the Bundle item.
- PrusaSlicer hides incompatible filament presets from the selector; the preset
  is present but not listed until a compatible machine is selected.
- The identity keys of the Prusa printer preset (`printer_settings_id`,
  `printer_vendor`) may be non-empty. The slicer accepts them on import; the
  real exports leave them empty. This is a deliberate deviation, not a defect.
- PrusaSlicer 2.9.6 CLI accepts Aldina's files via `--load` (exit 0, valid
  G-code), but profile-based slicing on a copied datadir segfaults. The GUI
  import is the reference path.
