# attest after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./attest`

```
BenchmarkCanonicalObjectReusedScalarBuffer-10    	 8490302	       136.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkSignCanonicalBody64KiB-10               	   10000	    115753 ns/op	    9252 B/op	      19 allocs/op
BenchmarkSignCanonicalBodyMaximum-10             	    2223	    548853 ns/op	    9251 B/op	      19 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
