# chit after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./chit`

```
BenchmarkManifestAccumulatorStreaming128-10     	     705	   1591129 ns/op	  73.86 MB/s	  992331 B/op	   14225 allocs/op
BenchmarkManifestAccumulatorStreaming4096-10    	      24	  51224594 ns/op	  73.54 MB/s	31775211 B/op	  458868 allocs/op
BenchmarkVerifyCatalogPageOne-10                	   16170	     74340 ns/op	  24.25 MB/s	   15922 B/op	     261 allocs/op
BenchmarkVerifyCatalogPageMaximum-10            	     936	   1243751 ns/op	  69.98 MB/s	 1178532 B/op	    9899 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
