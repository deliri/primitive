# gotoolchain after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./gotoolchain`

```
BenchmarkParseToolchainVersion-10    	33787285	        35.47 ns/op	       0 B/op	       0 allocs/op
BenchmarkParsePackageName-10         	47129364	        26.20 ns/op	       0 B/op	       0 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
