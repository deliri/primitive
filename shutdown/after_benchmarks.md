# shutdown after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./shutdown`

```
BenchmarkPlanRunMaximumNoop-10    	   31676	     36732 ns/op	    8592 B/op	     137 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.


Current package-sweep measurements and retained CPU/memory profiles are recorded
in [the September 9 upgrade review](../_docs/shutdown_upgrade_20260909.md).
The values above remain historical reservation-audit evidence.
