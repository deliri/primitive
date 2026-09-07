# runnercontrol after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./runnercontrol`

```
BenchmarkCompileExperimentObservation-10    	   17673	     67303 ns/op	   30091 B/op	     605 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
