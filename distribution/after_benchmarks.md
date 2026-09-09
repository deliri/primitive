Current review measurements: [September 9 review fixes](../_docs/distribution_review_20260909.md).

# Distribution after — full sweep, 2026-09-09

See [the full review report](../_docs/distribution_upgrade_20260909.md) and
[machine evidence](../_docs/distribution_upgrade_20260909_evidence.json).

The twelve-workload candidate-benchmarks run retains stdout, stderr, CPU/memory
profiles and its matching binary outside Git. Exact before/after values, effective
durations and comparison limits are recorded in the report.

## Historical bulk reservation audit (unchanged)

# distribution after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./distribution`

```
BenchmarkParseSigningDomain-10    	112445750	        10.67 ns/op	       0 B/op	       0 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
