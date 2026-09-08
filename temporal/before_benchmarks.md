# temporal before

Historical inspection note, not a measured baseline for the Temporal upgrade.
See [the profiled upgrade comparison](upgrade_benchmarks.md) for that baseline.

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. ParseDuration/RFC3339 wrap stdlib time.
