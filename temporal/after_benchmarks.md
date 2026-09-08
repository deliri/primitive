# temporal after

Historical measurements from an earlier inspection. The current upgrade has
[a separate profiled comparison](upgrade_benchmarks.md).

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./temporal`

```
BenchmarkParseDurationCanonical-10             	16446522	        68.09 ns/op	       0 B/op	       0 allocs/op
BenchmarkParseRFC3339Canonical-10              	14523331	        77.76 ns/op	       0 B/op	       0 allocs/op
BenchmarkAggregateDurationDecimalMaximum-10    	 3821091	       319.5 ns/op	      48 B/op	       1 allocs/op
```

These three historical benchmarks discarded their results and used the default
benchmark duration and CPU settings. Their numbers are retained as history;
they are not the baseline for the current upgrade.
