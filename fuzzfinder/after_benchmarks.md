# fuzzfinder after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./fuzzfinder`

```
BenchmarkCacheFormatGeneratedName/fuzz-corpus-10         	45184513	        26.57 ns/op	       0 B/op	       0 allocs/op
BenchmarkCacheFormatGeneratedName/fuzz-crasher-10        	46953867	        26.24 ns/op	       0 B/op	       0 allocs/op
BenchmarkFindRealDirectory128-10                         	    4525	    259791 ns/op	   53969 B/op	     669 allocs/op
BenchmarkFindRealDirectory8192-10                        	      54	  20592608 ns/op	 3181825 B/op	   42123 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
