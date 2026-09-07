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
