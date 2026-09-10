# Proofledger before — 2026-09-10

Go 1.27.1; Linux/amd64; AMD EPYC 7282; GOMAXPROCS=8. One explicit before phase, three fixed workloads, 30 configured seconds per benchmark. The replay benchmark performs three event observations per operation despite its historical PerEvent name. Its events/op metric is now reported after ResetTimer so it survives into the output. Successful timed work and fixtures are unchanged between phases; setup error checks and that post-loop metric were improved.

Command: ["go", "test", "./proofledger", "-run=^$", "-bench=.", "-benchmem", "-benchtime=30s", "-count=1", "-cpuprofile=/work/engineering-evidence/primitive/proofledger-upgrade-20260910/baseline-bench/cpu.pprof", "-memprofile=/work/engineering-evidence/primitive/proofledger-upgrade-20260910/baseline-bench/mem.pprof", "-o", "/work/engineering-evidence/primitive/proofledger-upgrade-20260910/baseline-bench/proofledger.test"]

```text
goos: linux
goarch: amd64
pkg: github.com/deliri/primitive/v2026/proofledger
cpu: AMD EPYC 7282 16-Core Processor
BenchmarkProofLedgerEventHash-8                      	  898011	     43026 ns/op	    5943 B/op	     104 allocs/op
BenchmarkProofLedgerReceiptVerification-8            	  165076	    218069 ns/op	   11213 B/op	     193 allocs/op
BenchmarkProofLedgerStreamingChainReplayPerEvent-8   	  411400	     87615 ns/op	   12254 B/op	     213 allocs/op
PASS
ok  	github.com/deliri/primitive/v2026/proofledger	110.749s
```

Exact base revision: a40accdad80780af6ab79d65e4b702e5bc3ee2e6. The dirty source list, every source hash and content snapshot, complete argv/environment, elapsed duration (111.362 seconds), stdout/stderr, aggregate CPU and memory profiles and matching executable are retained under /work/engineering-evidence/primitive/proofledger-upgrade-20260910/baseline-bench. Both phases ran once. Profiles cover the complete three-workload pass, not independently isolated per-workload captures. No distribution or statistical-significance claim is made. Memory profiles describe allocations, not peak resident memory. Local measurements are not an independent acceptance receipt.
