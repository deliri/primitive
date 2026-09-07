# keygen after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./keygen`

```
BenchmarkGenerateSigningKey-10       	   33687	     35308 ns/op	     256 B/op	       5 allocs/op
BenchmarkGenerateMaximumSecret-10    	 2312518	       564.7 ns/op	     160 B/op	       2 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
