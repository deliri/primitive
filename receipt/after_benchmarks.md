# receipt after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./receipt`

```
BenchmarkVerifyEvidence-10      	   23139	     51238 ns/op	    3027 B/op	      62 allocs/op
BenchmarkAdvanceWatermark-10    	11067894	       115.0 ns/op	       0 B/op	       0 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
