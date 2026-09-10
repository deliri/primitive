# Filelock after benchmarks — 2026-09-10

Go 1.27.1, Linux/amd64, AMD EPYC 7282, GOMAXPROCS=8. One run per phase, 30 seconds per case; CPU and allocation profiles enabled. Setup is outside b.Loop; every timed acquisition checks the exact Held result and releases successful holds. Contention attempts do not call Release. No payload bytes are read.

~~~sh
go test ./filelock '-run=^$' -bench=. -benchmem -benchtime=30s -count=1 -cpuprofile=/home/d/engineering-evidence/primitive/filelock-upgrade-20260910/after-bench/cpu.pprof -memprofile=/home/d/engineering-evidence/primitive/filelock-upgrade-20260910/after-bench/mem.pprof -o /home/d/engineering-evidence/primitive/filelock-upgrade-20260910/after-bench/filelock.test
~~~

~~~text
goos: linux
goarch: amd64
pkg: github.com/deliri/primitive/v2026/filelock
cpu: AMD EPYC 7282 16-Core Processor
BenchmarkAdvisoryLock/exclusive_acquire_release-8         	23908167	      1478 ns/op	       0 B/op	       0 allocs/op
BenchmarkAdvisoryLock/shared_acquire_release-8            	23884423	      1473 ns/op	       0 B/op	       0 allocs/op
BenchmarkAdvisoryLock/exclusive_contention-8              	43541235	       794.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkAdvisoryLock/shared_compatible_holder-8          	24297253	      1481 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/deliri/primitive/v2026/filelock	141.113s
~~~

Base commit: 2c3ea10fab1e1c036fa03bbac3dfd7c35da390f2. Per-run source hashes and content snapshots bind the uncommitted source exactly. Raw evidence: /home/d/engineering-evidence/primitive/filelock-upgrade-20260910/after-bench. CPU/memory profiles and the matching binary remain outside Git.

The only benchmark-source change since baseline is b.ReportAllocs on the parent dispatcher; all child loops, fixtures and observed work are identical. This run is one observation, not a statistical speedup claim.

| Case | Before ns/op | After ns/op | Change | Before/after B/op | Before/after allocs/op |
|---|---:|---:|---:|---:|---:|
| exclusive_acquire_release | 1394 | 1478 | +6.0% | 0/0 | 0/0 |
| shared_acquire_release | 1397 | 1473 | +5.4% | 0/0 | 0/0 |
| exclusive_contention | 732.5 | 794.8 | +8.5% | 0/0 | 0/0 |
| shared_compatible_holder | 1402 | 1481 | +5.6% | 0/0 | 0/0 |
