# process after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./process`

```
BenchmarkRunStreamingStdout64KiB-10                	     246	   4670266 ns/op	  114023 B/op	      80 allocs/op
BenchmarkRunStreamingStdout1MiB-10                 	     229	   5128678 ns/op	  113713 B/op	      80 allocs/op
BenchmarkRunStreamingStdin64KiB-10                 	     262	   4607507 ns/op	  113719 B/op	      80 allocs/op
BenchmarkRunStreamingStdin1MiB-10                  	     240	   5005852 ns/op	  113680 B/op	      80 allocs/op
BenchmarkParseExactEnvironmentAtMaximumCount-10    	    2673	    464395 ns/op	  578880 B/op	    4116 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
