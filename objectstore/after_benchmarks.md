# objectstore after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./objectstore`

```
BenchmarkGoogleCloudStorageDownloadCRC32CProviderProjection-10    	 7465135	       152.8 ns/op	      16 B/op	       2 allocs/op
BenchmarkInspectThreeDigestsAcrossStreamExtents/one_kibibyte-10   	  208274	      6570 ns/op	 155.86 MB/s	   43992 B/op	       9 allocs/op
BenchmarkInspectThreeDigestsAcrossStreamExtents/one_mebibyte-10   	     546	   2217043 ns/op	 472.96 MB/s	   43992 B/op	       9 allocs/op
BenchmarkInspectThreeDigestsAcrossStreamExtents/sixteen_mebibytes-10         	      33	  35549111 ns/op	 471.94 MB/s	   43992 B/op	       9 allocs/op
BenchmarkUploadStreaming1KiB-10                                              	   82352	     14799 ns/op	  69.19 MB/s	   73051 B/op	      99 allocs/op
BenchmarkUploadStreaming10MiB-10                                             	     192	   6216289 ns/op	1686.82 MB/s	   72972 B/op	      99 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
