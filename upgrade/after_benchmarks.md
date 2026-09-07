# upgrade after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./upgrade`

```
BenchmarkStageDownloadStreamingFourKiB-10    	      40	  26689277 ns/op	   0.15 MB/s	   44346 B/op	     183 allocs/op
BenchmarkStageDownloadStreamingTenMiB-10     	      27	  41298465 ns/op	 253.90 MB/s	   44353 B/op	     183 allocs/op
BenchmarkResolvePrimaryFourKiB-10            	    7244	    164941 ns/op	  24.83 MB/s	  115443 B/op	    1191 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
