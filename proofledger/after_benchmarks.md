# proofledger after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./proofledger`

```
BenchmarkProofLedgerEventHash-10                       	  126505	      8536 ns/op	    5940 B/op	     104 allocs/op
BenchmarkProofLedgerReceiptVerification-10             	   19630	     61168 ns/op	   11201 B/op	     193 allocs/op
BenchmarkProofLedgerStreamingChainReplayPerEvent-10    	   73233	     16958 ns/op	   12249 B/op	     213 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
