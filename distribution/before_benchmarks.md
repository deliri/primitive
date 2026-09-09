# Distribution before — full sweep, 2026-09-09

See [the full review report](../_docs/distribution_upgrade_20260909.md) and
[machine evidence](../_docs/distribution_upgrade_20260909_evidence.json).

The twelve-workload baseline-benchmarks-compiled run retains stdout, stderr, CPU/memory
profiles and its matching binary outside Git. Exact before/after values, effective
durations and comparison limits are recorded in the report.

## Historical bulk reservation audit (unchanged)

# distribution before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Hot path is a typed parse/assemble door.

## Measured

ParseSigningDomain 10.63 ns/op 0 B/op 0 allocs
