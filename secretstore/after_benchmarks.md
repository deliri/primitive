# secretstore after

NewValue copies exact payload size. Sparse 112 B/op vs 64 KiB ceiling; maximum still pays 65600 B/op.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./secretstore`

```
BenchmarkNewValueSparse-10     	18337419	        69.79 ns/op	     112 B/op	       3 allocs/op
BenchmarkNewValueMaximum-10    	  100328	     10707 ns/op	6120.88 MB/s	   65600 B/op	       2 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
