# currency after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./currency`

```
BenchmarkParseDecimal-10     	14366733	        83.42 ns/op	      24 B/op	       1 allocs/op
BenchmarkFormatDecimal-10    	14531355	        84.15 ns/op	      72 B/op	       3 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.


The September 9 package sweep, comparable workloads and retained CPU/memory
profiles are recorded in [the current review](../_docs/currency_upgrade_20260909.md).
The original note above remains historical evidence.
