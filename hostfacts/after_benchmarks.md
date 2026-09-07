# hostfacts after

readBoundedValue grows from 4 KiB instead of reserving 1 MiB+1. Sparse 4144 B/op.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./hostfacts`

```
BenchmarkClassifyGoOOMBanner1KiB-10         	  134973	      8391 ns/op	   41008 B/op	       2 allocs/op
BenchmarkClassifyGoOOMBanner1MiB-10         	     260	   4566967 ns/op	   41008 B/op	       2 allocs/op
BenchmarkCgroupMountInfoStreaming1KiB-10    	  179272	      7826 ns/op	   69704 B/op	       5 allocs/op
BenchmarkCgroupMountInfoStreaming1MiB-10    	     502	   2380407 ns/op	   69704 B/op	       5 allocs/op
BenchmarkReadBoundedValueSparse-10          	 1944958	       621.2 ns/op	    4144 B/op	       2 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
