# gitrepo after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./gitrepo`

```
BenchmarkDefaultConfiguration-10       	172488244	         6.905 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorktreeSelectionString-10    	529643871	         2.260 ns/op	       0 B/op	       0 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
