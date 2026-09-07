# payment after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./payment`

```
BenchmarkParseSigningDomain-10    	211384524	         5.616 ns/op	       0 B/op	       0 allocs/op
BenchmarkParsePaymentID-10        	10098781	       115.3 ns/op	      48 B/op	       1 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
