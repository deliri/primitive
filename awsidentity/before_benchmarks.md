# AWS identity benchmark baseline

Captured on Go 1.27.1, darwin/arm64, Apple M1 Max. These are local measurements,
not an independent acceptance receipt. No performance improvement is claimed.

The original package supplied only `BenchmarkParseAudience`. Its fixture and
production source were saved before edits. That original workload measured
**22.43 ns/op, 0 B/op, 0 allocs/op** with CPU and memory profiling enabled.
The requested duration was 30 seconds; Go reached its 1,000,000,000-iteration
ceiling first. The package reported 22.880 seconds. This was not a completed
30-second sample, and its shorter effective duration is retained explicitly.

The other workloads did not exist in the original suite. Their baselines below
are **reconstructed comparisons**: the saved original AWS production files are
selected through `baseline-production.overlay.json`, while the new benchmark
fixtures are identical to those used for the proposed-after measurements. They
are not represented as historical runs of benchmarks that did not yet exist.

| Workload | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Original audience, 17 bytes | 22.43 | 0 | 0 |
| Maximum audience, 1,000 bytes | 220.8 | 0 | 0 |
| Exact signed request construction | 37,315 | 3,616 | 46 |
| Provider XML, one-byte token | 38,460 | 4,440 | 92 |
| Provider XML, 16,384-byte token | 483,065 | 69,836 | 101 |
| Bearer disclosure, one-byte token | 77.34 | 8 | 1 |
| Bearer disclosure, 16,384-byte token | 31,033 | 18,432 | 1 |
| Acquire, maximum token, transport seam | 499,249 | 126,549 | 156 |

The acquisition benchmark calls public `Acquire` through a real `exchange.Client`
and Go's `http.Client`, with a prebuilt RoundTripper response. It measures the
owned request/response processing path. It does **not** measure socket, TLS,
DNS, live AWS, or SigV4 authentication performance. Fixture construction occurs
outside the timed loop; reader reset and the fixed transport call occur inside.
The XML workloads measure the private decoder directly and exclude transport.

Each workload runs once per declared phase, serially, with:

```
go test -run=^$ -bench=<exact workload filter> -benchmem -benchtime=30s \
  -count=1 -p=1 -parallel=1 -overlay=<baseline production overlay> \
  -cpuprofile=<workload>.cpu.pprof -memprofile=<workload>.mem.pprof \
  -o=<workload>.test ./awsidentity
```

The initial audience run has no overlay. Absolute commands, complete stdout and
stderr, requested/effective timing, source hashes, revision, dirty-tree fact,
workspace configuration, and artifacts are retained under
`testdata/test-upgrade-20260906`. The original audience record is
`baseline-BenchmarkParseAudience.json`; reconstructed records begin with
`reconstructed-baseline-`. `profile-plan.json` identifies each exact filter.
CPU and heap profiles cover the whole benchmark process, including untimed
fixture setup and runtime work; B/op and allocs/op are the benchmark's timed
allocation counters. Heap profiles use Go's default sampling rate, so their
sampled totals are not exact per-operation allocation counters.

The separately authorized temporal support change also has an unchanged RFC3339
comparator baseline: **151.0 ns/op, 0 B/op, 0 allocs/op**. Its original fixture is
retained in `baseline-temporal-source`; its execution and CPU/memory profiles are
named `baseline-temporal-RFC3339`. The new compact and UTC-only parser APIs have
no historical before measurement.
