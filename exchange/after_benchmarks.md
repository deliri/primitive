# exchange after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./exchange`

```
BenchmarkBoundedReceiveByDeclaredExtent/declared_extent-10         	    8252	    144339 ns/op	3632.33 MB/s	 1055656 B/op	      48 allocs/op
BenchmarkBoundedReceiveByDeclaredExtent/undeclared_extent-10       	    8044	    149696 ns/op	3502.35 MB/s	 1055672 B/op	      48 allocs/op
BenchmarkServerJSONBoundary/128B-10                                	  110394	     11034 ns/op	  23.20 MB/s	   45618 B/op	      96 allocs/op
BenchmarkServerJSONBoundary/1KiB-10                                	   66165	     17639 ns/op	 116.11 MB/s	   53965 B/op	     104 allocs/op
BenchmarkServerJSONBoundary/8KiB-10                                	   17019	     70484 ns/op	 232.45 MB/s	  148887 B/op	     112 allocs/op
BenchmarkRequestConstructionControl-10                             	  719553	      1770 ns/op	    5658 B/op	      18 allocs/op
BenchmarkServerJSONBoundaryByLimit/limit1KiB-10                    	  107048	     10374 ns/op	   42655 B/op	      96 allocs/op
BenchmarkServerJSONBoundaryByLimit/limit64KiB-10                   	  115422	     10649 ns/op	   45611 B/op	      96 allocs/op
BenchmarkServerJSONBoundaryByLimit/limit1MiB-10                    	  101526	     12429 ns/op	   45627 B/op	      96 allocs/op
BenchmarkServerJSONBoundaryParallel-10                             	   72636	     16407 ns/op	 124.82 MB/s	   54352 B/op	     105 allocs/op
BenchmarkJSONRoundTripOverLoopbackParallel-10                      	   13098	     92142 ns/op	  22.23 MB/s	  132774 B/op	     304 allocs/op
BenchmarkUpload10MiBFileOverLoopback-10     	     331	   3389487 ns/op	3093.61 MB/s	  110970 B/op	     135 allocs/op
BenchmarkDownload10MiBFileOverLoopback-10   	     343	   3307545 ns/op	3170.25 MB/s	  110708 B/op	     146 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
