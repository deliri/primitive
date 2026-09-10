# Filelock before benchmarks — 2026-09-10

Go 1.27.1, Linux/amd64, AMD EPYC 7282, GOMAXPROCS=8. One run per phase, 30 seconds per case; CPU and allocation profiles enabled. Setup is outside b.Loop; every timed acquisition checks the exact Held result and releases successful holds. Contention attempts do not call Release. No payload bytes are read.

~~~sh
go test ./filelock '-run=^$' -bench=. -benchmem -benchtime=30s -count=1 -cpuprofile=/home/d/engineering-evidence/primitive/filelock-upgrade-20260910/baseline-bench/cpu.pprof -memprofile=/home/d/engineering-evidence/primitive/filelock-upgrade-20260910/baseline-bench/mem.pprof -o /home/d/engineering-evidence/primitive/filelock-upgrade-20260910/baseline-bench/filelock.test
~~~

~~~text
goos: linux
goarch: amd64
pkg: github.com/deliri/primitive/v2026/filelock
cpu: AMD EPYC 7282 16-Core Processor
BenchmarkAdvisoryLock/exclusive_acquire_release-8         	25824400	      1394 ns/op	       0 B/op	       0 allocs/op
BenchmarkAdvisoryLock/shared_acquire_release-8            	25201874	      1397 ns/op	       0 B/op	       0 allocs/op
BenchmarkAdvisoryLock/exclusive_contention-8              	47170260	       732.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkAdvisoryLock/shared_compatible_holder-8          	25111788	      1402 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/deliri/primitive/v2026/filelock	140.985s
~~~

Base commit: 2c3ea10fab1e1c036fa03bbac3dfd7c35da390f2. Per-run source hashes and content snapshots bind the uncommitted source exactly. Raw evidence: /home/d/engineering-evidence/primitive/filelock-upgrade-20260910/baseline-bench. CPU/memory profiles and the matching binary remain outside Git.
