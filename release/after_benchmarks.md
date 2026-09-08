# release after

BuildDependencies uses exact-size []BuildDependency. Sparse 83952 → 1891 B/op (~44x).

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./release`

```
BenchmarkVerifyLatest-10                                    	     987	   1173974 ns/op	  640176 B/op	   13638 allocs/op
BenchmarkAssessLatest-10                                    	 2279118	       521.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkLatestDocumentJSON-10                              	    2824	    437681 ns/op	  271318 B/op	    5349 allocs/op
BenchmarkBuildDependenciesUnmarshalSparse-10                	  421138	      2961 ns/op	    1892 B/op	      44 allocs/op
BenchmarkBuildDependenciesUnmarshalMaximum-10               	    1006	   1167312 ns/op	  484967 B/op	    8258 allocs/op
BenchmarkInspectBuiltArtifactRealExecutable-10              	     417	   2889306 ns/op	1315.97 MB/s	  175614 B/op	     642 allocs/op
BenchmarkInspectBuiltArtifactRealExecutablePlusTenMiB-10    	     122	   9713852 ns/op	1470.89 MB/s	  175678 B/op	     642 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.

## September 8, 2026 sweep

The measurements above are historical. The final comparisons and their
limitations are recorded in [the review](upgrade_review.md#final-measured-comparisons)
and [source-bound evidence](upgrade_evidence.json). The maximum dependency sample
reduced memory from 485,339 to 361,990 B/op while timing increased from 1,195,153
to 1,307,867 ns/op. Seed decoding changed from 256 B / 5 allocations to 112 B /
2 allocations. Walking 1,024 dependencies changed from 330,201,305 to 11,252 ns/op
after removing repeated whole-collection validation from sealed indexed access.
All twelve comparisons retain CPU/memory profiles and matching binaries. Each is
one paired sample; timing results are mixed and establish no statistical trend.
The user reviewed and approved this package for v2026.1.24 publication.
