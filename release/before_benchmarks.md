# release before

`BuildDependencies` stored modules in `[BuildDependencyMaximumCount]BuildDependency`
(1024 slots) on every value, including a one-module closure. The ceiling was an
admission bound used as a heap reservation.

## Measured

`go test -run=^$ -bench=BenchmarkBuildDependenciesUnmarshal -benchmem -count=1 ./release`

```
BenchmarkBuildDependenciesUnmarshalSparse-10     80258    15186 ns/op   83952 B/op     44 allocs/op
BenchmarkBuildDependenciesUnmarshalMaximum-10      982  1195433 ns/op  566838 B/op   8258 allocs/op
```

Sparse (1 module) paid ~84 KiB, the 1024-slot array plus JSON decode.

## September 8, 2026 retained baseline

The historical measurements above are not the baseline for this sweep. See
[the current baseline](upgrade_review.md#recorded-baseline) and its
[source-bound manifest](baseline_evidence.json). All seven current workloads
requested 30 seconds and retain CPU/memory profiles and matching binaries.

The final sweep contains twelve paired comparisons in the
[upgrade manifest](upgrade_evidence.json). Additional material, indexed-walk and
retained-artifact baselines bind their exact harnesses and preserved production.
The [review](upgrade_review.md#final-measured-comparisons) distinguishes reconstructed
baselines from chronological measurements and records executable fixture hashes.
