# Attest benchmark baseline — 2026-09-06

Captured before changing any attest Go source in this pass. The previous report
contained no baseline measurements; it is retained in
[testdata/test-upgrade-20260906/previous-before-report.md](testdata/test-upgrade-20260906/previous-before-report.md).

Revision: `8db3ace7a4353a09e81cb8da846f232a10bfbd8c`, dirty workspace.
Toolchain: Go 1.27.1, darwin/arm64. Machine: Apple M1 Max, AC power.
Each execution record includes the precise argument vector, source SHA-256s,
stdout/stderr hashes and sizes, profile hashes, binary hash, and actual duration.
These are local measurements, not independent acceptance of a committed revision.

| Benchmark | Iterations | ns/op | B/op | allocs/op | Process seconds |
| --- | ---: | ---: | ---: | ---: | ---: |
| CanonicalObjectReusedScalarBuffer | 100000000 | 367.2 | 0 | 0 | 38.94 |
| SignCanonicalBody64KiB | 308541 | 748896 | 9245 | 19 | 232.12 |
| SignCanonicalBodyMaximum (1 MiB) | 10000 | 3857513 | 9251 | 19 | 42.32 |

Each target was run separately and serially with:

```sh
go test -run='^$' -bench='^TARGET$' -benchmem -benchtime=30s \
  -count=1 -p=1 -parallel=1 \
  -cpuprofile=attest/testdata/test-upgrade-20260906/baseline-TARGET.cpu.pprof \
  -memprofile=attest/testdata/test-upgrade-20260906/baseline-TARGET.mem.pprof \
  -o=attest/testdata/test-upgrade-20260906/baseline-TARGET.test ./attest
```

The signing workload emits zero bytes in 8 KiB chunks through the caller's
canonical-body callback. Its original `sizedBody` fixture allocates the chunk
on every callback. The maximum input is the owner-defined
`attest.CanonicalBodyMaximumBytes` (1 MiB).

| Workload | Raw result | CPU profile | Memory profile | Execution record |
| --- | --- | --- | --- | --- |
| Scalars | [stdout](testdata/test-upgrade-20260906/baseline-BenchmarkCanonicalObjectReusedScalarBuffer.stdout.txt) | [CPU](testdata/test-upgrade-20260906/baseline-BenchmarkCanonicalObjectReusedScalarBuffer.cpu.pprof) | [memory](testdata/test-upgrade-20260906/baseline-BenchmarkCanonicalObjectReusedScalarBuffer.mem.pprof) | [record](testdata/test-upgrade-20260906/baseline-BenchmarkCanonicalObjectReusedScalarBuffer.json) |
| Sign 64 KiB | [stdout](testdata/test-upgrade-20260906/baseline-BenchmarkSignCanonicalBody64KiB.stdout.txt) | [CPU](testdata/test-upgrade-20260906/baseline-BenchmarkSignCanonicalBody64KiB.cpu.pprof) | [memory](testdata/test-upgrade-20260906/baseline-BenchmarkSignCanonicalBody64KiB.mem.pprof) | [record](testdata/test-upgrade-20260906/baseline-BenchmarkSignCanonicalBody64KiB.json) |
| Sign maximum | [stdout](testdata/test-upgrade-20260906/baseline-BenchmarkSignCanonicalBodyMaximum.stdout.txt) | [CPU](testdata/test-upgrade-20260906/baseline-BenchmarkSignCanonicalBodyMaximum.cpu.pprof) | [memory](testdata/test-upgrade-20260906/baseline-BenchmarkSignCanonicalBodyMaximum.mem.pprof) | [record](testdata/test-upgrade-20260906/baseline-BenchmarkSignCanonicalBodyMaximum.json) |

## What the profiles show

The [64 KiB allocation profile](testdata/test-upgrade-20260906/baseline-sign64-memory.stdout.txt)
attributes **88.56% of sampled allocation volume to the test fixture's
`sizedBody.WriteCanonical`**, which allocates its 8 KiB chunk inside the measured
operation. The test upgrade moves that chunk and a `bytes.Reader` into setup,
then streams identical chunks using the standard library's `io.Copy`.
Any resulting allocation reduction is a correction to benchmark overhead,
not a production optimization.

The [CPU profile](testdata/test-upgrade-20260906/baseline-sign64-cpu.stdout.txt)
shows SHA-256 and Ed25519 standard-library work, plus substantial runtime work.
Only 36.71 seconds were sampled over 231.15 seconds of profile duration. The
configured 30-second benchmark substantially overran during loop calibration;
the actual duration and [diagnostic sample](testdata/test-upgrade-20260906/baseline-sign64-scheduler.sample.txt)
are preserved. These single, noisy wall-clock samples do not establish a speed
trend. Allocation bytes stayed approximately constant across a 16× body-size
increase, which is the useful baseline for bounded working memory.
