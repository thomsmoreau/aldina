# Contributing to Aldina

Thanks for looking. This document covers what the project accepts, how it is
built, and the rules that keep the canonical format from turning into a copy of
one slicer's schema.

## Scope and governance

Aldina is an author-led project. That is a statement of process, not of
ownership: anyone may fork it under the AGPL, but what lands in this repository
is a maintainer decision.

**Contributions that fit well**

- A new provider — an adapter for a slicer that is not yet supported.
- A new mapping, or a correction to an existing one, **verified against real
  exported profiles** rather than against documentation or memory.
- A test reproducing a bug, ideally as the first commit of the fix.
- Documentation fixes.

**Discuss in an issue before opening a pull request**

- Any change to the canonical format itself — new fields, renamed concepts,
  changed units or semantics. The canonical format is the one thing every
  provider depends on, and it grows on evidence rather than on anticipation.
- Anything that reorganises the tree, renames packages, or adds a dependency.
  The current dependency set is the standard library plus `gopkg.in/yaml.v3`,
  and additions need justification.

Discuss it first. An unannounced large refactor is usually refused. The
refusal costs you more time than it costs the maintainer. A refusal states
clear reasons and arrives quickly.

## Architecture invariants

These are not style preferences. Breaking one of them breaks the reason the
project exists.

- **`internal/canonical` never imports a provider.** The dependency direction is
  one-way: `providers → canonical`. If you see `import ".../internal/providers/orca"`
  inside `canonical/`, the architecture is broken. Go's import graph enforces
  this, so a violation will not compile.
- **All format-specific detail lives in the provider.** Stringly-typed numbers
  (`"220"`), `"nil"` inheritance markers, `"70%"` percentages, per-extruder
  arrays, and sentinel values like `"-1"` belong in
  `internal/providers/<slicer>`, at the boundary. They never belong in the
  core.
- **Start and end gcode are opaque.** Carried through, never parsed or
  interpreted.
- **`docs/mapping.md` is updated first.** A newly mapped field is documented
  there before it is implemented. That file is the project's central artefact.
- **Pointers mean presence.** Every tunable canonical scalar is a pointer, so
  `nil` ("not set") stays distinct from an authored zero — `bed: 0` is an
  unheated bed, which is an intent, not an absence. The exceptions are slices
  and maps (where `nil` is already distinct from empty), `Kind` and
  `SchemaVersion` (whose absence is a load error), and the fields inside `Axes`
  (where `*Axes` is the unit of presence).
- **The provider blocks, never the canonical.** `Validate()` returns warnings
  and never gates anything: a PLA at 190 °C is unusual, not invalid. "Required"
  is a property of the _target format_ — each provider declares its own list
  (`internal/providers/orca/required.go`) surfaced through
  `providers.MissingError`.
- **Never a silent loss.** If a provider cannot express a canonical field, the
  renderer collects a warning and surfaces it to the caller. The renderer never
  drops the field without a warning.

## Code conventions

- **Reuse before writing.** Look for an existing conversion function
  (`formatFloat`, `broadcast`, `ratioToPercent`, `renderFloatPtr`…) before you
  add one. A conversion that appears twice becomes a named pure function in
  the provider.
- **Small pure functions** for conversions: input to output, no state, no I/O.
- **No generic mapping engine.** No DSL, no reflection magic. Explicit code per
  provider. Abstract after the second proven repetition, not before.
- **Idiomatic errors**: wrap with `%w` and useful context (`"temps.bed: %w"`),
  `errors.Join` for multiple validation failures. No `panic` outside a proven
  internal bug.
- **Wire-schema struct fields follow the target format's key exactly**, json or
  ini tag included, even when the name is ugly. The schema describes the file,
  not our taste.

## Tests

```bash
go test ./...
```

Everything must be green before a commit. Tests never touch the network or your
home directory: fixtures live in `testdata/` and scratch space comes from
`t.TempDir()`.

### Golden files

Reference renders live in each provider's `testdata/render/`. Regenerate them
deliberately:

```bash
go test ./internal/providers/orca -update
go test ./internal/providers/prusa -update
```

**Then read the diff, value by value, before committing it.** If a golden
file changes, that change is either a regression or an intended change. The
commit message must say which. Do not regenerate golden files just to turn a
red test green without understanding what moved. The maintainer refuses a
pull request that does this outright. The golden files are the only record
of what the slicers actually accept.

### Edge cases every conversion needs

The traps in this domain are known, so cover them:

- a valid `0` versus an absent field (the pointer/zero-value trap)
- `"nil"` (inheritance), `""`, empty array versus one element versus N
- sentinels (`"-1"` = auto), percentages (`"70%"`)
- inverted encodings — Orca shrink `"100%"` is Prusa `"0%"` is canonical `0.0`
- ratio rounding, negative values, `nozzle_range` bounds

### The round-trip test

`internal/providers/roundtrip_test.go` renders one canonical value to Orca _and_
Prusa and asserts it lands correctly on both sides. It is the proof that the
canonical format is not just a renamed copy of one slicer. **Every newly mapped
concept needs a case there.**

### Conformance against a real slicer (optional)

```bash
ALDINA_ORCA_BIN=/path/to/orca-slicer go test ./internal/providers/orca
```

Skipped when the variable is unset, so contributors without the slicer stay
green. Note the limit: this checks that the slicer parses and slices with the
preset, which is not the same as the GUI's import gate.

## Commits

- Imperative mood, first line 72 characters or fewer, scope prefix where it
  helps: `orca: render bed temps fan-out`.
- **One concept per commit.** A vertical slice is one concept end to end —
  canonical type, mapping doc, Orca render, Prusa render, tests — not
  twenty-five half-finished fields.
- **Never mix a refactor and a feature** in the same commit.
- `gofmt -l .` and `go vet ./...` must both be silent before you commit.

## Contributor licence agreement

Contributions require signing the [CLA](CLA.md). The signature is collected
automatically on your first pull request.

The reason, stated plainly rather than left for you to guess: Aldina's core is
AGPL-3.0, and commercial licences exempting the network clause are offered
alongside it to fund development. This requires the right to sublicense every
line in the tree. Without a CLA, each contributor keeps copyright over their
own patch. A future commercial licence would then need every contributor's
individual consent. One refusal, or one contributor the maintainer cannot
reach, ends that option for good.

The CLA is a **licence grant, not a copyright assignment**. You keep the
copyright to your work and remain free to use it anywhere else.

**Trivial contributions are exempt** — a typo, a one-line fix, a comment
correction. These sit below the threshold of originality and asking for
paperwork would be disproportionate.
