# Loop corpus: export procedure

The loop test (docs/adr/0004) runs a real filament through both slicers and
back. It needs real exports that bind the filament to a machine that BOTH
catalogues know, in a form that Aldina can read. The current references do
not do this:

- the Orca exports target the Bambu Lab P2S, which the Prusa catalogue does
  not contain. The Prusa render refuses them.
- the Prusa bundle binds its filament with a `printer_notes` regex. Aldina
  does not parse a condition that a human wrote, so `compat.machines` stays
  empty and the Orca render refuses it.

These files stay in `references/` as refusal cases. This procedure makes the
two files that the loops run on.

## Target machine

Use the Prusa MINI with the 0.4 mm nozzle. Both catalogues contain it
(`internal/catalog/catalog.json`):

| Slicer      | Printer preset name           |
| ----------- | ----------------------------- |
| OrcaSlicer  | `Prusa MINI 0.4 nozzle`       |
| PrusaSlicer | `Original Prusa MINI & MINI+` |

Do not use the Input Shaper variant (`MINIIS`). It is a different model in
both catalogues.

## OrcaSlicer export

1. Select the printer `Prusa MINI 0.4 nozzle`.
2. Select a filament. Change one value, then save it as a user preset.
3. Remove the parent: the file must be self-contained. If the filament
   settings show `Detach from system preset`, click it. If not, edit the
   JSON file in step 5 and set `"inherits": ""`. Then the file must hold all
   values, not only the changed ones.
4. Make sure that `compatible_printers` contains `Prusa MINI 0.4 nozzle`.
5. Copy the user preset file. On macOS it is in
   `~/Library/Application Support/OrcaSlicer/user/<profile>/filament/`.
6. Put it in `references/orca/filaments/`. Do not change its name.

## PrusaSlicer export

1. Select the printer `Original Prusa MINI & MINI+`.
2. Select a filament. Change one value, then save it as a user preset.
3. Open `Filament Settings → Dependencies`.
4. Click `Detach from system preset`. The file must not have `inherits`.
5. In `Compatible printers`, clear `All`. Click `Set…` and select only
   `Original Prusa MINI & MINI+`.
6. Clear the `Compatible printers condition` field.
7. Save the preset.
8. Click `File → Export → Export Config Bundle…`.
9. Put the file in `references/prusa/`. Give it a name that is not
   `PrusaSlicer_config_bundle.ini`: that file is the refusal case.

## Checks

Do these checks before you commit the files. `<orca>` and `<prusa>` are the
paths of the two new files.

```sh
# Orca: no parent, and bound to the MINI 0.4.
python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(repr(d["inherits"]), d["compatible_printers"])' "<orca>"
# Expected: '' ['Prusa MINI 0.4 nozzle']

# Prusa: the filament section has no parent, a printer list, no condition.
grep -A200 '^\[filament:' "<prusa>" | grep -E '^(inherits|compatible_printers|compatible_printers_condition) ='
# Expected: compatible_printers names Original Prusa MINI & MINI+ (quotes
# are possible), and compatible_printers_condition and inherits are empty.
```

If a check fails, do the export again. Do not edit the Prusa file by hand:
the reference must be what the slicer wrote. The Orca `inherits` edit in
step 3 is the one exception, and only when Orca has no detach control.
