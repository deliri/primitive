# Proofledger after — 2026-09-10

Go 1.27.1; Linux/amd64; AMD EPYC 7282; GOMAXPROCS=8. One explicit after phase, three fixed workloads, 30 configured seconds per benchmark. The replay benchmark performs three event observations per operation despite its historical PerEvent name. Its events/op metric is now reported after ResetTimer so it survives into the output. Successful timed work and fixtures are unchanged between phases; setup error checks and that post-loop metric were improved.

Command: ["go", "test", "./proofledger", "-run=^$", "-bench=.", "-benchmem", "-benchtime=30s", "-count=1", "-cpuprofile=/work/engineering-evidence/primitive/proofledger-upgrade-20260910/candidate-bench/cpu.pprof", "-memprofile=/work/engineering-evidence/primitive/proofledger-upgrade-20260910/candidate-bench/mem.pprof", "-o", "/work/engineering-evidence/primitive/proofledger-upgrade-20260910/candidate-bench/proofledger.test"]

```text
goos: linux
goarch: amd64
pkg: github.com/deliri/primitive/v2026/proofledger
cpu: AMD EPYC 7282 16-Core Processor
BenchmarkProofLedgerEventHash-8                      	 1267341	     28471 ns/op	    3716 B/op	      66 allocs/op
BenchmarkProofLedgerReceiptVerification-8            	  196536	    177583 ns/op	    6758 B/op	     117 allocs/op
BenchmarkProofLedgerStreamingChainReplayPerEvent-8   	  864118	     42982 ns/op	         3.000 events/op	    5574 B/op	      99 allocs/op
PASS
ok  	github.com/deliri/primitive/v2026/proofledger	108.187s
```

Exact base revision: a40accdad80780af6ab79d65e4b702e5bc3ee2e6. The dirty source list, every source hash and content snapshot, complete argv/environment, elapsed duration (108.833 seconds), stdout/stderr, aggregate CPU and memory profiles and matching executable are retained under /work/engineering-evidence/primitive/proofledger-upgrade-20260910/candidate-bench. Both phases ran once. Profiles cover the complete three-workload pass, not independently isolated per-workload captures. No distribution or statistical-significance claim is made. Memory profiles describe allocations, not peak resident memory. Local measurements are not an independent acceptance receipt.
