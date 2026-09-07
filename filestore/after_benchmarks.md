# filestore after

Grow with observed names; ReadDir in 64-entry stdlib batches. Ceiling is admission only.
Delta: lexical sparse ~281x less memory vs ceiling prealloc.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./filestore`

```
BenchmarkLexicalSparseDirectory-10        	   61011	     19031 ns/op	    3768 B/op	      12 allocs/op
BenchmarkWalkLexicalSparseDirectory-10    	   58978	     21226 ns/op	    4264 B/op	      21 allocs/op
BenchmarkWalkNativeSparseDirectory-10     	   61059	     20662 ns/op	    3112 B/op	      20 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
