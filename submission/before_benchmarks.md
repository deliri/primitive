# Submission benchmark baseline — 2026-09-10

Production base: `67875b51b4bee0a41f7c0a2dba0551b3655745fa` (v2026.1.44). The new benchmark harness was captured before production edits; its exact source hash is `e326785ac225ca68bf70308dae424d514e3ae60009a8d8e1fdad7fa0d52343bb`.

Furnace: Linux/amd64, AMD EPYC 7282, Go 1.27.1, GOWORK=off, GOMAXPROCS=8. Each case ran once with a configured 30-second duration. Parser results describe a fixed batch of 16 parses; all other rows describe one operation. Profile collection was enabled for the complete pass.

The previous note measured only ParseSigningDomain (6.149 ns/op) without the broader workload or matching profile evidence. That historical number is not this comparison's baseline.

The fixture establishes one real local TLS Objectstore transfer before timing; timed work is Submission's metadata operation. Request and completion fixtures describe the fixed bytes `submission benchmark transfer`. Grant cases use the existing fixed grant fixture. UploadCall measures construction of an execution request without reading its source; it does not measure upload throughput.

Command:
```sh
go test ./submission -run=^$ -bench=. -benchmem -benchtime=30s -count=1 -cpuprofile=/home/d/engineering-evidence/primitive/submission-upgrade-20260910/baseline-bench/cpu.pprof -memprofile=/home/d/engineering-evidence/primitive/submission-upgrade-20260910/baseline-bench/mem.pprof -o /home/d/engineering-evidence/primitive/submission-upgrade-20260910/baseline-bench/submission.test
```

```text
goos: linux
goarch: amd64
pkg: github.com/deliri/primitive/v2026/submission
cpu: AMD EPYC 7282 16-Core Processor
BenchmarkParseSigningDomainBatch-8   	320032279	       112.7 ns/op	        16.00 parses/op	       0 B/op	       0 allocs/op
BenchmarkSubmission/RequestCommitment-8         	  907476	     34091 ns/op	    4256 B/op	      91 allocs/op
BenchmarkSubmission/IssueRequest-8              	  144608	    237045 ns/op	    6366 B/op	     126 allocs/op
BenchmarkSubmission/VerifyRequest-8             	  273339	    136533 ns/op	    5387 B/op	     117 allocs/op
BenchmarkSubmission/IssueGrant-8                	   77246	    453058 ns/op	   29013 B/op	     369 allocs/op
BenchmarkSubmission/VerifyGrant-8               	  137656	    254467 ns/op	   19811 B/op	     287 allocs/op
BenchmarkSubmission/IssueCompletion-8           	   54880	    638587 ns/op	   57683 B/op	     766 allocs/op
BenchmarkSubmission/VerifyCompletion-8          	  118672	    306525 ns/op	   21104 B/op	     330 allocs/op
BenchmarkSubmission/DecodeCompletion-8          	  325653	    109219 ns/op	   14735 B/op	     284 allocs/op
BenchmarkSubmission/UploadCall-8                	  168188	    207610 ns/op	   33468 B/op	     425 allocs/op
PASS
ok  	github.com/deliri/primitive/v2026/submission	350.735s
```

Raw stdout, stderr, CPU and memory profiles, executable, exact commands, source patch, full source hashes and content snapshots are retained in `/home/d/engineering-evidence/primitive/submission-upgrade-20260910`. No raw binary or profile is added to Git.
