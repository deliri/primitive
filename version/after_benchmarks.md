# version after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./version`

```
BenchmarkFromProject-10    	 1825311	       653.1 ns/op	      21 B/op	       3 allocs/op
BenchmarkParseTag-10       	 5943859	       201.4 ns/op	      26 B/op	       4 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
