The September 8 upgrade skips recovery on normal Context.Err returns, strengthens the tests, and checks each benchmark fixture before timing. See [the review](upgrade_review.md) for all eight paired measurements and the full earlier attempt history, with [source-bound evidence](upgrade_evidence.json).

The following figures and “no production change” statement describe the earlier filestore-specific pass only. They are preserved as historical notes, not current profiled results.

# contextstate after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./contextstate`

```
BenchmarkValidateLiveContext-10    	133048098	         9.391 ns/op	       0 B/op	       0 allocs/op
BenchmarkObserveLiveContext-10     	142829086	         8.429 ns/op	       0 B/op	       0 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
