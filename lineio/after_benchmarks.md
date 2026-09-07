# lineio after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./lineio`

```
BenchmarkScanStreaming64Lines-10      	  958252	      1248 ns/op	 307.65 MB/s	     280 B/op	       5 allocs/op
BenchmarkScanStreaming4096Lines-10    	   17160	     70321 ns/op	 349.48 MB/s	     280 B/op	       5 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
