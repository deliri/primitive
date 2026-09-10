# Submissionauth before benchmarks — 2026-09-10

Go 1.27.1, Linux/amd64, AMD EPYC 7282, GOMAXPROCS=8. One pass, seven fixed workloads, 30 configured seconds per case, CPU/memory profiles enabled. Setup, signed fixtures and the local provider upload are outside b.Loop. These measure authentication, JSON round trips, projection and receipt reconciliation; they do not measure network throughput. Every timed operation checks its result.

~~~sh
go test ./submissionauth '-run=^$' -bench=. -benchmem -benchtime=30s -count=1 -cpuprofile=/home/d/engineering-evidence/primitive/submissionauth-upgrade-20260910/baseline-bench/cpu.pprof -memprofile=/home/d/engineering-evidence/primitive/submissionauth-upgrade-20260910/baseline-bench/mem.pprof -o /home/d/engineering-evidence/primitive/submissionauth-upgrade-20260910/baseline-bench/submissionauth.test
~~~

~~~text
goos: linux
goarch: amd64
pkg: github.com/deliri/primitive/v2026/submissionauth
cpu: AMD EPYC 7282 16-Core Processor
BenchmarkAuthentication/verify_request-8         	  104521	    337999 ns/op	   10565 B/op	     242 allocs/op
BenchmarkAuthentication/request_json_roundtrip-8 	   88689	    405338 ns/op	   50369 B/op	     887 allocs/op
BenchmarkAuthentication/verify_completion-8      	   67014	    595840 ns/op	   34011 B/op	     561 allocs/op
BenchmarkAuthentication/completion_json_roundtrip-8         	   90288	    394066 ns/op	   54739 B/op	     868 allocs/op
BenchmarkAuthentication/projection_json-8                   	  325492	    108305 ns/op	   17878 B/op	     205 allocs/op
BenchmarkAuthentication/reconcile_receipt-8                 	   92281	    392842 ns/op	   16092 B/op	     417 allocs/op
BenchmarkAssemble-8                                         	12865825	      2546 ns/op	     256 B/op	       8 allocs/op
PASS
ok  	github.com/deliri/primitive/v2026/submissionauth	251.217s
~~~

Base commit: b5f6154e00c9326beda706cf54832ec489af787c. Exact uncommitted source hashes, snapshots, command, output, duration, profiles and matching binary: /home/d/engineering-evidence/primitive/submissionauth-upgrade-20260910/baseline-bench.
