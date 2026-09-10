# Submissionauth after benchmarks — 2026-09-10

Go 1.27.1, Linux/amd64, AMD EPYC 7282, GOMAXPROCS=8. One pass, seven fixed workloads, 30 configured seconds per case, CPU/memory profiles enabled. Setup, signed fixtures and the local provider upload are outside b.Loop. These measure authentication, JSON round trips, projection and receipt reconciliation; they do not measure network throughput. Every timed operation checks its result.

~~~sh
go test ./submissionauth '-run=^$' -bench=. -benchmem -benchtime=30s -count=1 -cpuprofile=/home/d/engineering-evidence/primitive/submissionauth-upgrade-20260910/optimized-bench/cpu.pprof -memprofile=/home/d/engineering-evidence/primitive/submissionauth-upgrade-20260910/optimized-bench/mem.pprof -o /home/d/engineering-evidence/primitive/submissionauth-upgrade-20260910/optimized-bench/submissionauth.test
~~~

~~~text
goos: linux
goarch: amd64
pkg: github.com/deliri/primitive/v2026/submissionauth
cpu: AMD EPYC 7282 16-Core Processor
BenchmarkAuthentication/verify_request-8         	  104583	    337550 ns/op	   10564 B/op	     242 allocs/op
BenchmarkAuthentication/request_json_roundtrip-8 	   87230	    408888 ns/op	   50353 B/op	     887 allocs/op
BenchmarkAuthentication/verify_completion-8      	   53191	    648259 ns/op	   34017 B/op	     561 allocs/op
BenchmarkAuthentication/completion_json_roundtrip-8         	   89845	    395762 ns/op	   54743 B/op	     868 allocs/op
BenchmarkAuthentication/projection_json-8                   	  325692	    110640 ns/op	   17882 B/op	     205 allocs/op
BenchmarkAuthentication/reconcile_receipt-8                 	   90160	    397624 ns/op	   16092 B/op	     417 allocs/op
BenchmarkAssemble-8                                         	14249091	      2519 ns/op	     256 B/op	       8 allocs/op
PASS
ok  	github.com/deliri/primitive/v2026/submissionauth	248.941s
~~~

Base commit: b5f6154e00c9326beda706cf54832ec489af787c. Exact uncommitted source hashes, snapshots, command, output, duration, profiles and matching binary: /home/d/engineering-evidence/primitive/submissionauth-upgrade-20260910/optimized-bench.

There were three explicit phases: baseline, the binding-fix candidate, and the profile-guided candidate. The first candidate added one allocation to projection encoding; the final candidate removes a redundant validation while preserving nomination checks. Benchmark changes after baseline only improve failure diagnostics; the successful timed work, fixture arguments, benchmark names and budgets are unchanged. Each phase ran once. All observations are retained; none is selected as a statistical trend.

| Workload | Before ns/op | After ns/op | Change | Before/after B/op | Before/after allocs/op |
|---|---:|---:|---:|---:|---:|
| BenchmarkAuthentication/verify_request | 337999 | 337550 | -0.1% | 10565/10564 | 242/242 |
| BenchmarkAuthentication/request_json_roundtrip | 405338 | 408888 | +0.9% | 50369/50353 | 887/887 |
| BenchmarkAuthentication/verify_completion | 595840 | 648259 | +8.8% | 34011/34017 | 561/561 |
| BenchmarkAuthentication/completion_json_roundtrip | 394066 | 395762 | +0.4% | 54739/54743 | 868/868 |
| BenchmarkAuthentication/projection_json | 108305 | 110640 | +2.2% | 17878/17882 | 205/205 |
| BenchmarkAuthentication/reconcile_receipt | 392842 | 397624 | +1.2% | 16092/16092 | 417/417 |
| BenchmarkAssemble | 2546 | 2519 | -1.1% | 256/256 | 8/8 |

## Initial binding-fix candidate, before removing redundant validation

~~~text
goos: linux
goarch: amd64
pkg: github.com/deliri/primitive/v2026/submissionauth
cpu: AMD EPYC 7282 16-Core Processor
BenchmarkAuthentication/verify_request-8         	  111056	    333113 ns/op	   10564 B/op	     242 allocs/op
BenchmarkAuthentication/request_json_roundtrip-8 	   86194	    411479 ns/op	   50353 B/op	     887 allocs/op
BenchmarkAuthentication/verify_completion-8      	   54826	    646813 ns/op	   34014 B/op	     561 allocs/op
BenchmarkAuthentication/completion_json_roundtrip-8         	   90213	    394968 ns/op	   54739 B/op	     868 allocs/op
BenchmarkAuthentication/projection_json-8                   	  318390	    112227 ns/op	   17927 B/op	     206 allocs/op
BenchmarkAuthentication/reconcile_receipt-8                 	   89890	    399986 ns/op	   16093 B/op	     417 allocs/op
BenchmarkAssemble-8                                         	13446211	      2553 ns/op	     256 B/op	       8 allocs/op
PASS
ok  	github.com/deliri/primitive/v2026/submissionauth	249.743s
~~~

That candidate was measured before the profile-guided edit; its full command, source snapshot, CPU/memory profiles and binary are retained in /home/d/engineering-evidence/primitive/submissionauth-upgrade-20260910/after-bench. Its observation is historical, not attributed to the final source.
