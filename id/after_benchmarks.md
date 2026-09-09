# id after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./id`

```
BenchmarkParseULID-10                     	14097826	        77.93 ns/op	       0 B/op	       0 allocs/op
BenchmarkParseUUIDv7-10                   	11454330	       107.7 ns/op	      48 B/op	       1 allocs/op
BenchmarkULIDAppendTextReusedBuffer-10    	34439997	        35.91 ns/op	       0 B/op	       0 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.

The complete September 9 sweep and source-bound CPU/memory profiles are recorded
in [the current review](../_docs/id_upgrade_20260909.md). The original note above
remains historical evidence.
