# Ingest returns one canonical document per preset

Status: accepted

A PrusaSlicer bundle is designed to carry many presets (`[filament:A]`,
`[filament:B]`, …), while an OrcaSlicer JSON export physically carries exactly
one. Ingest therefore keeps the shape of the wire format it reads:
`prusa.Ingest` returns N canonical filaments with N reports;
`orca.IngestFilament` stays one-in/one-out. We rejected a unified
cross-provider interface because the asymmetry IS the format difference the
provider exists to expose, and forcing unity would hide it in the core. The
canonical document unit is one preset per file, on every side. The accepted
input shape for Prusa is a sectioned bundle only: a headerless file is refused
with a pointer to the bundle export, because PrusaSlicer's single-import of
headerless files is a silent no-op in the GUI and a headerless "Export Config"
file is a whole mixed configuration (pitfall #15).

**Consequences**: a future provider whose format packs N presets into one
file carries that packing (and its unpacking) inside its own package, never in
canonical. The bundle-only refusal can be relaxed later by adding a parser
mode; nothing downstream has to change, because both shapes end as one preset
per canonical file. Section header name becomes `meta.name` of each document.
