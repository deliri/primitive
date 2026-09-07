# gomodule after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./gomodule`

```
BenchmarkParsePath-10          	 2727835	       428.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkParseImportPath-10    	 2285317	       522.3 ns/op	       0 B/op	       0 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
