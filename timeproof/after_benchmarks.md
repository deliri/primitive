# timeproof after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./timeproof`

```
BenchmarkPrepareRequest-10             	  332266	      3133 ns/op	    2885 B/op	      75 allocs/op
BenchmarkVerifyAuthenticFreeTSA-10     	     914	   1298621 ns/op	  335073 B/op	    1372 allocs/op
BenchmarkRejectOversizedResponse-10    	  872932	      1399 ns/op	    1392 B/op	      38 allocs/op
BenchmarkReplayCanonicalEvidence-10    	   10000	    111781 ns/op	  169986 B/op	     868 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
