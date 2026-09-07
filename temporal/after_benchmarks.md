# temporal after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./temporal`

```
BenchmarkParseDurationCanonical-10             	16446522	        68.09 ns/op	       0 B/op	       0 allocs/op
BenchmarkParseRFC3339Canonical-10              	14523331	        77.76 ns/op	       0 B/op	       0 allocs/op
BenchmarkAggregateDurationDecimalMaximum-10    	 3821091	       319.5 ns/op	      48 B/op	       1 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
