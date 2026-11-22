# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `aldina ingest`: reads an OrcaSlicer filament export into canonical YAML plus
  a loss report. Intent-only and warn-and-drop; refuses a file with a
  non-empty `inherits`. Orca filament only
  (`docs/adr/0001-ingest-scope-intent-only.md`).
- `aldina ingest --provider prusa`: reads a PrusaSlicer config bundle into one
  canonical YAML per filament preset, with one loss report per preset. A preset
  with a non-empty `inherits` is refused alone; the command writes the others
  and exits non-zero.
- Reverse preset lookup in the catalogue, so ingest can resolve an Orca preset
  name back to its machine.

### Fixed

- Orca render: `physical.abrasive` emits `required_nozzle_HRC` as `40`, the
  slicer's spelling for an abrasive filament.

## [0.1.0] - 2026-08-22

### Added

- `aldina render` for `filament` and `machine`, OrcaSlicer (JSON) and
  PrusaSlicer (INI config bundle).
- `aldina validate` (machine ↔ filament compatibility).
- Committed machine-catalogue snapshot (`internal/catalog`); rendering and
  testing never touch the network.
- Golden-file tests per provider and the round-trip test proving one canonical
  value lands correctly on both slicers.
- Machine import verified in the GUI of OrcaSlicer 2.4.2 and PrusaSlicer 2.9.6
  (`docs/import-test.md`).
