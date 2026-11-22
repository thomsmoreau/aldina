# The loop test compares canonicals, and render warnings declare its losses

Status: accepted (supersedes ADR-0003)

The loop test runs A → canonical → B → canonical → A′ and compares the
canonical of A with the canonical of A′. It never compares wire files: the
loop changes identity, collapses encodings and drops engine knobs, so a
textual diff can never pass. A canonical deviation passes only when a render
warning names its path. The match is a prefix match at a segment boundary
(`.` or `[`): a warning on `compat.machines` covers `compat.machines[0]`, but
not `compat.machinesX`. A deviation that no warning names is a silent loss,
and the test fails.

We rejected the expected-loss ledger of ADR-0003. The first implementation
(branch `wip/t8-ledger`) showed that about 125 of its 142 lines only repeated
what the ingest reports already declare: two sources of truth for one quality
goal, and one of them drifts. The code is now the single source of declared
losses.

The loop does not retarget. When the target catalogue does not know the
machine, the render stops with an error. The test never uses a substitute
machine.

**Consequences**: to accept a new loss, a provider adds a render warning on
the canonical path. Code review replaces the review of a ledger line. The
match is on a typed `Path`, not on the warning text. The corpus must
bind each filament to a machine that both catalogues know
(`docs/loop-corpus.md`).
