# Ingest scope: intent-only, warn-and-drop

Status: accepted

The MVP was authoring only (canonical → render), but hooking users requires
migration (provider → canonical → other provider), so Ingest is added to the
MVP. Ingest is a two-step scaffold: it emits a canonical YAML file plus a
report, the author edits the file, then renders normally. Canonical stays
intent-only: engine knobs are never translated, because a mechanical
translation produces values that are syntactically valid but physically wrong
on the target machine. Unmappable fields are dropped with a warning, classified
by schema nature, never silently. Version 1 ingests Orca filament files only;
a file with a non-empty `inherits` is detected and refused, because ingesting
its deltas without resolving inheritance would produce an almost-empty
canonical file and a false success. Inherits resolution, Prusa ingest, and
machine ingest are later slices.

**Consequences**: `conformance.yaml` pins ingest cases only — it is never a
bidirectional mapping engine; each provider carries explicit ingest code, the
same way it carries render code.

**Amended 2026-09-26**: Prusa filament ingest is in scope (spec #6) and
available in the CLI. Its input shape is in ADR-0002. Its fidelity gate is in
ADR-0004, which supersedes ADR-0003 (the Loop test is not implemented
yet, #16). The scope rules
above apply to it without change: intent-only, warn-and-drop, and refusal of a
non-empty `inherits`. In a Prusa bundle, the refusal applies to that preset
only. Inherits resolution and machine ingest are still later slices.
