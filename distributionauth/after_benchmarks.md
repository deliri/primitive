# distributionauth after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./distributionauth`

```
BenchmarkAssembleUpdate-10    	 1562215	       765.6 ns/op	     170 B/op	       9 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
