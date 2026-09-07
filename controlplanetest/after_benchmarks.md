# controlplanetest after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./controlplanetest`

```
BenchmarkIssueInstallation-10    	    8517	    128956 ns/op	    4630 B/op	     118 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
