# core after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./core`

```
BenchmarkEncodeValidatedJSONAbsolutePath-10                           	 1096749	      1089 ns/op	       818.0 ratchet-B/op	     681 B/op	      17 allocs/op
BenchmarkRejectDuplicateJSONFieldsMaximum-10                          	   22790	     52414 ns/op	   46272 B/op	     545 allocs/op
BenchmarkRejectDuplicateJSONFieldsGlobalMaximumLongSharedPrefix-10    	     297	   4060577 ns/op	 252.66 MB/s	 7352221 B/op	     578 allocs/op
BenchmarkDecodeStrictJSONAdvertisedMaximumComposition-10              	      36	  33402295 ns/op	  31.37 MB/s	  36544208 ratchet-B/op	36544022 B/op	  327050 allocs/op
BenchmarkJSONMarshalAbsolutePath-10                                   	 4000435	       302.7 ns/op	     112 B/op	       6 allocs/op
BenchmarkParsePathComponent-10                                        	60182931	        20.59 ns/op	       0 B/op	       0 allocs/op
BenchmarkParseRelativePath-10                                         	14746807	        77.00 ns/op	       0 B/op	       0 allocs/op
BenchmarkDecodeCanonicalHexSHA256-10                                  	16155885	        80.42 ns/op	       0 B/op	       0 allocs/op
BenchmarkDigestWriter1KiB-10                                          	 2179718	       554.2 ns/op	1847.78 MB/s	     160 B/op	       2 allocs/op
BenchmarkDigestWriter1MiB-10                                          	    2665	    462342 ns/op	2267.97 MB/s	     160 B/op	       2 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
