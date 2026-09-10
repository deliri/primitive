# Googleidentity baseline — 2026-09-10

Base revision: 5bd5bcf74e5f8b0b37c2fa532212f05479debcc7. Harness expanded before production edits. Go 1.27.1, Linux/amd64, AMD EPYC 7282, GOMAXPROCS=8, GOWORK=off.

Seven cases ran in baseline-bench; the disjoint service-account case ran in baseline-service-bench after discovering the SDK transport defect and before production edits. Each case ran once for 30 configured seconds, with explicit CPU/memory profiles and a retained binary.

~~~text
baseline-bench: go test ./googleidentity -run=^$ -bench=. -benchmem -benchtime=30s -count=1 -cpuprofile=/home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910/baseline-bench/cpu.pprof -memprofile=/home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910/baseline-bench/mem.pprof -o /home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910/baseline-bench/googleidentity.test
baseline-service-bench: go test ./googleidentity -run=^$ -bench=^BenchmarkServiceAccountAcquisition$ -benchmem -benchtime=30s -count=1 -cpuprofile=/home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910/baseline-service-bench/cpu.pprof -memprofile=/home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910/baseline-service-bench/mem.pprof -o /home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910/baseline-service-bench/googleidentity.test
~~~

[All measurements](after_benchmarks.md). Raw receipts, exact source snapshots and profiles: /home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910. Historical measurements remain in Git history and are not used for this comparison.
