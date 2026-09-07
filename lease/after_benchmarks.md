# lease after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./lease`

```
BenchmarkEvaluate-10                 	  955819	      1124 ns/op	     128 B/op	       4 allocs/op
BenchmarkVerify-10                   	   20282	     59720 ns/op	    2377 B/op	      68 allocs/op
BenchmarkDecisionCanonicalJSON-10    	  307743	      4150 ns/op	    1625 B/op	      53 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
