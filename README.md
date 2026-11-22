# Aldina

One canonical 3D-printing profile, rendered to each slicer's native format.

Aldina is a Go CLI that reads a slicer-independent YAML description of a
filament or a machine and writes the corresponding preset for OrcaSlicer or
PrusaSlicer.

## The problem

A profile you tune in OrcaSlicer cannot be used in PrusaSlicer, and the reverse
is equally true. The consequences are familiar to anyone who has switched:

- Changing slicer means re-tuning every profile from scratch.
- Filament vendors publish per-printer, per-slicer tables that nobody can
  reproduce mechanically.
- A print farm accumulates unversioned profiles that drift between machines,
  with none of the review or rollback that infrastructure-as-code gave servers
  fifteen years ago.

The settings themselves are not the hard part. The hard part is that each
slicer encodes the same physical facts differently. A nozzle temperature is
`"220"` in Orca's JSON and `220` in Prusa's INI. A shrinkage of zero is
`100%` in one format and `0%` in the other.

## The approach

Aldina keeps one canonical description and renders outward, the way a compiler
keeps one intermediate representation and emits several targets:

```
                    ┌──► OrcaSlicer  (.json)
   canonical.yaml ──┼──► PrusaSlicer (.ini)
                    └──► …
```

Converting directly between N slicers needs N×(N−1) converters. Going through a
pivot needs N+M, and each new slicer is one adapter rather than a row and a
column. The same reasoning powers GCC/LLVM and Pandoc.

Two rules keep the pivot honest, and the import graph enforces the first:

- **`internal/canonical` never imports a provider.** The core does not know
  OrcaSlicer exists.
- **All format grime lives in the provider.** Stringly-typed numbers,
  per-extruder arrays, `"nil"` inheritance markers, percentage encodings — these
  stay at the boundary in `internal/providers/<slicer>`, never in the core.

A canonical field is admitted only if it describes the machine, the material,
or the print intent. It is never admitted for a knob that belongs to one
slicing engine. It must also appear in at least two formats. Start and end gcode are carried as opaque
blocks and never interpreted.

## Status

This is a working tool with a deliberately narrow scope, not a finished
product.

**Working**

- `filament` rendered to OrcaSlicer (JSON) and PrusaSlicer (INI config bundle),
  covering temperatures, flow, cooling, retraction, shrinkage, material
  vocabulary and per-machine compatibility.
- `validate`, which cross-checks a filament against a machine and exits
  non-zero on an incompatible pair — for example a 255 °C nozzle target against
  a machine whose maximum is 250 °C.
- A committed snapshot of both slicers' machine catalogues, so rendering and
  testing never touch the network.
- Golden-file tests per provider, plus the round-trip test that renders one
  canonical value to both slicers and asserts it lands correctly on each side.
- `machine` rendered to OrcaSlicer (JSON) and PrusaSlicer (INI), imported in
  the GUI of both — OrcaSlicer 2.4.2 and PrusaSlicer 2.9.6 — with every
  authored value preserved. The filament self-links to an imported machine
  through `compatible_printers`. Procedure: `docs/import-test.md`.

**Caveats — read before relying on it**

- PrusaSlicer must use `File → Import → Import Config Bundle…`. The plain
  `Import Config…` is a silent no-op on Aldina's single-preset files.
- An Orca machine self-links only when titled `<model> <nozzle>`. A machine
  titled anything else still imports but does not self-link its filament
  (mapping.md, machine pitfall #1).
- The optional conformance tests feed rendered presets to a real slicer binary
  (`ALDINA_ORCA_BIN`, `ALDINA_PRUSA_BIN`) and skip when those are unset. They
  use Orca's `--load-settings` and PrusaSlicer's `--load`/`--save`, which check
  that the slicer _parses_ the preset and returns every authored value
  unchanged. That is a content check, not the GUI's File → Import gate; the UI
  gate itself is exercised and recorded in `docs/import-test.md`.
- PrusaSlicer's stock vendor filaments will not match a printer preset that
  Aldina renders. Those filaments match on a `PRINTER_VENDOR_*` keyword whose
  real spelling (`PRUSA3D`) is not derivable from a vendor name, so Aldina
  derives an approximation. Filaments Aldina renders are unaffected: they bind
  on `printer_model`, which is catalogue-resolved.

**Not started**

The `process` resource (layer heights, speeds, infill, supports) and the import
direction — parsing an existing slicer profile back into canonical form, along
with `inherits` resolution. Authoring is the whole of the current scope.

## Install

Requires Go 1.26 or later.

```bash
go install github.com/thomsmoreau/Aldina/cmd/aldina@latest
```

Or from a clone:

```bash
git clone https://github.com/thomsmoreau/Aldina.git
cd aldina
go build -o aldina ./cmd/aldina
```

## Usage

```text
aldina render --provider orca  --kind filament --in FILAMENT.yaml --out DIR
aldina render --provider orca  --kind machine  --in MACHINE.yaml  --out DIR [--inherits PARENT]
aldina render --provider prusa --kind filament --in FILAMENT.yaml --out DIR
aldina render --provider prusa --kind machine  --in MACHINE.yaml  --out DIR [--inherits PARENT]
aldina validate --machine MACHINE.yaml --filament FILAMENT.yaml
```

### render

Writes presets into `--out`, which is a directory.

```bash
aldina render --provider orca --kind filament \
  --in examples/example_mk1.yaml --out ./dist
```

Import the result through **File → Import → Import Configs** in OrcaSlicer.

A canonical filament names the machines it is for, and it names the _machine_
rather than a preset:

```yaml
compat:
  machines:
    - model: "Bambu Lab P2S"
      nozzle_mm: 0.6
```

Each provider resolves that against the slicer's own catalogue: OrcaSlicer
needs the exact preset names it binds by, so one file is written per target
machine; PrusaSlicer expresses the same thing as a
`compatible_printers_condition` inside a single file, whatever the number of
targets.

A machine absent from the target slicer's catalogue is a hard error rather than
a silent fallback. Most printer models exist in one catalogue only, so
rendering the Bambu example above to PrusaSlicer legitimately refuses.
`examples/filament_prusa_mini.yaml` targets a machine both catalogues carry and
so renders to either:

```bash
aldina render --provider prusa --kind filament \
  --in examples/filament_prusa_mini.yaml --out ./dist
```

Anything the target format cannot express is reported as a warning. Aldina does
not drop a canonical field silently.

### validate

Checks a filament against a machine before you render it.

```bash
aldina validate --machine examples/machine_p2s_06.yaml \
  --filament examples/example_mk1.yaml
```

Exit status is non-zero for an incompatible combination.

### Refreshing the machine catalogue

`aldina-catalog` is a maintenance tool, run by hand and committed like a golden
file. It is the only part of the project that reaches the network.

```bash
go run ./cmd/aldina-catalog -out internal/catalog/catalog.json
```

## Documentation

`docs/mapping.md` records how each canonical concept maps to each slicer's
fields, the nature of every mapping, and the traps found in real exported
profiles. It is the central artefact of the project: a field is documented
there before it is implemented.

`examples/` holds canonical profiles you can render as-is.

## Contributing

Contributions are welcome, particularly new providers and mappings verified
against real exported profiles. Please read [CONTRIBUTING.md](CONTRIBUTING.md)
first — it covers the architecture invariants, how to run the tests and
regenerate golden files, the commit convention, and the contributor licence
agreement.

Changes to the canonical format itself are discussed in an issue before any
pull request.

## Licence

Aldina is licensed under the [GNU AGPL-3.0](LICENSE).

The choice is deliberate: the format, the CLI and every provider stay free
software, and the network clause means anyone offering Aldina as a hosted
service must publish their modifications. Hosted services are intended to be
the commercial model, so the copyleft protects that model rather than
restricting the tool. Commercial licences exempting the network clause are
available from the author — see [NOTICE](NOTICE).

---

Named after the Aldine Press of Aldus Manutius (Venice, c. 1500), which
industrialised book reproduction: one authoritative original, faithfully
reproduced. Its device was _festina lente_ — make haste slowly.
