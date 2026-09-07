# runworkspace after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./runworkspace`

```
BenchmarkParseGoDeclarations-10     	  605466	      1800 ns/op	    2256 B/op	      59 allocs/op
BenchmarkParseResidueCount-10       	55010016	      22.94 ns/op	       0 B/op	       0 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
