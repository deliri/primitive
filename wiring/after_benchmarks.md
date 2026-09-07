# wiring after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./wiring`

```
BenchmarkDeriveCombinedMaximumRuntimeGraph-10    	     950	   1084109 ns/op	  355530 B/op	    2039 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
