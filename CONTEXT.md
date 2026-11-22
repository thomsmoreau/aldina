# Aldina

Aldina is ProfileAsCode for 3D printing: a canonical YAML format that describes
a filament or machine profile independent of any slicer, and adapters that
convert it to and from each slicer's format.

## Language

**Canonical**:
The slicer-agnostic profile format. One YAML file describes a filament or
machine intent.
_Avoid_: profile, spec, source of truth

**Provider**:
An adapter that converts between canonical and one slicer's wire format. One
provider per slicer.
_Avoid_: adapter, backend, driver

**Render**:
The canonical → provider direction. A canonical file becomes a slicer file.
_Avoid_: export, emit, write out

**Ingest**:
The provider → canonical direction. A slicer file becomes a canonical file
plus a report.
_Avoid_: parse, import, read in

**Migration**:
Ingest from one slicer, render to a different slicer.
_Avoid_: conversion, port, transfer

**Consolidation**:
Ingest from one slicer, render back to the same slicer. Profile-as-code without
retyping.
_Avoid_: round-trip, back up

**Intent**:
A material property or print intent that transfers across slicers, such as
temperature, flow, shrinkage, or retraction.
_Avoid_: setting, parameter, value

**Engine knob**:
Slicing-engine tuning calibrated to one machine, nozzle, or engine, such as
pressure advance, cooling curves, or fan ramps. Engine knobs do not transfer.
_Avoid_: slicing parameter, engine setting

**Intent/knob portal**:
The admission rule that admits intent into canonical and excludes engine knobs.
_Avoid_: filter, gate, criteria

**Loss bucket**:
A category of the ingest report. The buckets are translated,
translated-with-approximation, lost-identity, lost-gcode, lost-fan-out, and
lost-engine-knob.
_Avoid_: warning, dropped field

**Loop test**:
The cross-provider fidelity test: A → canonical → B → canonical → A′, where A′
is ingested again and compared to A at the canonical level, never textually.
A render warning must declare each deviation; any other deviation is a silent
loss and fails.
_Avoid_: round-trip (reserved for the same-provider ingest→render test),
grand tour, cycle test

**Silent loss**:
A canonical deviation in the loop test that no render warning declares.
_Avoid_: ledger, accepted-loss
