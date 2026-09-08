# Hostfacts historical allocation checkpoint result

These original measurements are preserved for history. They did not capture
CPU/memory profiles with the measurement and are not the current acceptance
comparison. See [the profiled comparison](upgrade_benchmarks.md).

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

The original OOM benchmarks discarded their result; the previous claim that
all benchmarks observed results was incorrect. The current checked workloads
and exact-result checks are documented in the new comparison.
No `unsafe`, no C.
