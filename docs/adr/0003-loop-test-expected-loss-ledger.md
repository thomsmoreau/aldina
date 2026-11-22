# Migration fidelity is gated by the loop test and the expected-loss ledger

Status: superseded by ADR-0004

Value round-trip tests prove one direction at one hop. To gate a full
migration (prusa → canonical → orca → canonical → prusa′) we compare A and A′
at the canonical level, never textually: a textual diff can never pass by
design, because the loop deliberately rewrites identity (`condition DSL →
name list`), collapses encodings (arrays → scalars, per-extruder fan-out), and
warn-and-drops engine knobs. The test parses both endpoints with the same
provider ingest and diffs the canonical structs; every wire key or value that
disappears must be covered by a line in the expected-loss ledger — a golden
file where each entry names its loss bucket and reason. Anything outside the
ledger fails CI. We rejected both the bare textual diff (brittle, reinvents
the parser in the test) and a soft report-only gate (silent loss stays
invisible between hops).

**Consequences**: silent loss becomes impossible by construction rather than
disciplined: an undeclared loss is a red build, and the only way to green it
is a human-approved ledger line. Localizing a failure to one step stays the
job of the per-step golden tests, not of the loop.
