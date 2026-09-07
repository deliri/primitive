# gcsobjects after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./gcsobjects`

```
BenchmarkGCSDownloadCapabilityRequestValidation-10    	 2832189	       419.8 ns/op	     168 B/op	       6 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
