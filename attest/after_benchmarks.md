# Attest profiled benchmarks after the test upgrade — 2026-09-06

These measurements use the final attest test sources, Go 1.27.1, darwin/arm64,
Apple M1 Max and AC power. Production Go source is unchanged. The dirty base
revision is `8db3ace7a4353a09e81cb8da846f232a10bfbd8c`; every record retains the
source hashes, complete arguments, machine/power facts, profiles and matching
test binary. These are local measurements, not independent acceptance of the
subsequent commit.

The final phase follows the package tests and changed signature fuzz target.
Each benchmark runs once, separately and serially, with 30 seconds configured:

```sh
go test -run='^$' -bench='^TARGET$' -benchmem -benchtime=30s \
  -count=1 -p=1 -parallel=1 \
  -cpuprofile=attest/testdata/test-upgrade-20260906/boundary-TARGET.cpu.pprof \
  -memprofile=attest/testdata/test-upgrade-20260906/boundary-TARGET.mem.pprof \
  -o=attest/testdata/test-upgrade-20260906/boundary-TARGET.test ./attest
```

| Workload | Iterations | ns/op | B/op | allocs/op | Process seconds |
| --- | ---: | ---: | ---: | ---: | ---: |
| CanonicalObjectReusedScalarBuffer | 262202545 | 137.0 | 0 | 0 | 37.17 |
| SignCanonicalBody64KiB | 317769 | 113092 | 1053 | 18 | 36.80 |
| SignCanonicalBodyMaximum | 66410 | 551934 | 1054 | 18 | 37.48 |
| VerifyCanonicalBody64KiB | 484189 | 73594 | 424 | 10 | 36.62 |
| VerifyCanonicalBodyMaximum | 71629 | 564348 | 425 | 10 | 41.24 |
| EnvelopeMarshalJSON | 16935841 | 2143 | 1794 | 30 | 37.70 |
| EnvelopeUnmarshalJSON | 3535987 | 10894 | 3731 | 48 | 42.42 |

All timed results remain observable. Scalar output is checked against a typed
standard-library JSON projection. Signing checks length and independent SHA-256
facts, then verifies the resulting envelope. Verification checks the exact
retained envelope. JSON encode/decode checks the complete typed value after
timing. Setup and these checks sit outside `b.Loop()`.

## Interpretation

See the [original baseline](before_benchmarks.md). Its allocation profile
identified the test body callback allocating an 8 KiB chunk on every operation.
The replacement fixture reuses an 8 KiB array and `bytes.Reader`, passing each
chunk through `io.Copy`. No production optimization was made.

The scalar allocation ratchet remains zero. Signing drops from 19 to 18
allocations per operation and removes the fixture chunk allocation. Signing
and verification keep the same allocation counts across the 16× increase
from 64 KiB to the owner-defined 1 MiB maximum. This supports bounded working
memory for these measured streaming workloads; it does not claim constant
time for hashing N bytes.

Wall-clock timing varied substantially between attempts on this machine.
The baseline and all subsequent attempts are retained below. The source did
not receive a production speed optimization, and these timings are not a
statistical speedup claim. The verification and JSON benchmarks are newly
introduced workloads, so they have no original pre-upgrade measurements.

## What the final profiles show

The [64 KiB signing CPU profile](testdata/test-upgrade-20260906/boundary-BenchmarkSignCanonicalBody64KiB-cpu-top.stdout.txt)
is dominated by Go SHA-256 and Ed25519 arithmetic. At the maximum body size,
[verification spends 85.28% of sampled CPU in Go SHA-256](testdata/test-upgrade-20260906/boundary-BenchmarkVerifyCanonicalBodyMaximum-cpu-top.stdout.txt).
Execution remains on the standard library's cryptographic path.

The [signing allocation profile](testdata/test-upgrade-20260906/boundary-BenchmarkSignCanonicalBody64KiB-mem-top.stdout.txt)
now shows the fixed signing capability/frame, Go key/hash setup and returned
values; the old per-operation 8 KiB body fixture allocation is gone. Caller
callbacks still contribute real measured work, including domain text emission.

The [JSON decoding allocation profile](testdata/test-upgrade-20260906/boundary-BenchmarkEnvelopeUnmarshalJSON-mem-top.stdout.txt)
attributes 41.20% of sampled allocation volume to Core's bounded document read,
with subsequent allocations in Go's JSON decoder and the typed structural scan.
These identify costs for a future, separately reviewed optimization pass;
this test upgrade changes none of those production paths.

## Final artifacts

| Workload | Raw output | CPU | Memory | Matching binary | Record |
| --- | --- | --- | --- | --- | --- |
| CanonicalObjectReusedScalarBuffer | [stdout](testdata/test-upgrade-20260906/boundary-BenchmarkCanonicalObjectReusedScalarBuffer.stdout.txt) | [CPU](testdata/test-upgrade-20260906/boundary-BenchmarkCanonicalObjectReusedScalarBuffer.cpu.pprof) | [memory](testdata/test-upgrade-20260906/boundary-BenchmarkCanonicalObjectReusedScalarBuffer.mem.pprof) | [binary](testdata/test-upgrade-20260906/boundary-BenchmarkCanonicalObjectReusedScalarBuffer.test) | [record](testdata/test-upgrade-20260906/boundary-BenchmarkCanonicalObjectReusedScalarBuffer.json) |
| SignCanonicalBody64KiB | [stdout](testdata/test-upgrade-20260906/boundary-BenchmarkSignCanonicalBody64KiB.stdout.txt) | [CPU](testdata/test-upgrade-20260906/boundary-BenchmarkSignCanonicalBody64KiB.cpu.pprof) | [memory](testdata/test-upgrade-20260906/boundary-BenchmarkSignCanonicalBody64KiB.mem.pprof) | [binary](testdata/test-upgrade-20260906/boundary-BenchmarkSignCanonicalBody64KiB.test) | [record](testdata/test-upgrade-20260906/boundary-BenchmarkSignCanonicalBody64KiB.json) |
| SignCanonicalBodyMaximum | [stdout](testdata/test-upgrade-20260906/boundary-BenchmarkSignCanonicalBodyMaximum.stdout.txt) | [CPU](testdata/test-upgrade-20260906/boundary-BenchmarkSignCanonicalBodyMaximum.cpu.pprof) | [memory](testdata/test-upgrade-20260906/boundary-BenchmarkSignCanonicalBodyMaximum.mem.pprof) | [binary](testdata/test-upgrade-20260906/boundary-BenchmarkSignCanonicalBodyMaximum.test) | [record](testdata/test-upgrade-20260906/boundary-BenchmarkSignCanonicalBodyMaximum.json) |
| VerifyCanonicalBody64KiB | [stdout](testdata/test-upgrade-20260906/boundary-BenchmarkVerifyCanonicalBody64KiB.stdout.txt) | [CPU](testdata/test-upgrade-20260906/boundary-BenchmarkVerifyCanonicalBody64KiB.cpu.pprof) | [memory](testdata/test-upgrade-20260906/boundary-BenchmarkVerifyCanonicalBody64KiB.mem.pprof) | [binary](testdata/test-upgrade-20260906/boundary-BenchmarkVerifyCanonicalBody64KiB.test) | [record](testdata/test-upgrade-20260906/boundary-BenchmarkVerifyCanonicalBody64KiB.json) |
| VerifyCanonicalBodyMaximum | [stdout](testdata/test-upgrade-20260906/boundary-BenchmarkVerifyCanonicalBodyMaximum.stdout.txt) | [CPU](testdata/test-upgrade-20260906/boundary-BenchmarkVerifyCanonicalBodyMaximum.cpu.pprof) | [memory](testdata/test-upgrade-20260906/boundary-BenchmarkVerifyCanonicalBodyMaximum.mem.pprof) | [binary](testdata/test-upgrade-20260906/boundary-BenchmarkVerifyCanonicalBodyMaximum.test) | [record](testdata/test-upgrade-20260906/boundary-BenchmarkVerifyCanonicalBodyMaximum.json) |
| EnvelopeMarshalJSON | [stdout](testdata/test-upgrade-20260906/boundary-BenchmarkEnvelopeMarshalJSON.stdout.txt) | [CPU](testdata/test-upgrade-20260906/boundary-BenchmarkEnvelopeMarshalJSON.cpu.pprof) | [memory](testdata/test-upgrade-20260906/boundary-BenchmarkEnvelopeMarshalJSON.mem.pprof) | [binary](testdata/test-upgrade-20260906/boundary-BenchmarkEnvelopeMarshalJSON.test) | [record](testdata/test-upgrade-20260906/boundary-BenchmarkEnvelopeMarshalJSON.json) |
| EnvelopeUnmarshalJSON | [stdout](testdata/test-upgrade-20260906/boundary-BenchmarkEnvelopeUnmarshalJSON.stdout.txt) | [CPU](testdata/test-upgrade-20260906/boundary-BenchmarkEnvelopeUnmarshalJSON.cpu.pprof) | [memory](testdata/test-upgrade-20260906/boundary-BenchmarkEnvelopeUnmarshalJSON.mem.pprof) | [binary](testdata/test-upgrade-20260906/boundary-BenchmarkEnvelopeUnmarshalJSON.test) | [record](testdata/test-upgrade-20260906/boundary-BenchmarkEnvelopeUnmarshalJSON.json) |

The [final manifest](testdata/test-upgrade-20260906/manifest.json) covers the
complete local bundle, including raw stdout/stderr, profiles, matching binaries,
source snapshots and execution records. The bundle is retained on disk and
ignored by Git; the package reports and tests are committed.

## Earlier attempts retained

`candidate` preceded the detailed row/oracle review. `reviewed` followed that
review but preceded the final signature-whitespace and destination-prefix
boundary additions. Both passes are retained; the table above selects the
final source phase, not the fastest attempt. Benchmark workload code and
production execution code are identical across these three post-upgrade passes.

| Phase | Workload | Iterations | ns/op | B/op | allocs/op | Process seconds | Record |
| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |
| candidate | CanonicalObjectReusedScalarBuffer | 126943118 | 258.6 | 0 | 0 | 35.54 | [record](testdata/test-upgrade-20260906/candidate-BenchmarkCanonicalObjectReusedScalarBuffer.json) |
| candidate | SignCanonicalBody64KiB | 155025 | 199274 | 1053 | 18 | 32.76 | [record](testdata/test-upgrade-20260906/candidate-BenchmarkSignCanonicalBody64KiB.json) |
| candidate | SignCanonicalBodyMaximum | 27790 | 2743808 | 1055 | 18 | 77.97 | [record](testdata/test-upgrade-20260906/candidate-BenchmarkSignCanonicalBodyMaximum.json) |
| candidate | VerifyCanonicalBody64KiB | 59088 | 1165873 | 425 | 10 | 71.28 | [record](testdata/test-upgrade-20260906/candidate-BenchmarkVerifyCanonicalBody64KiB.json) |
| candidate | VerifyCanonicalBodyMaximum | 2677 | 16913105 | 434 | 10 | 54.22 | [record](testdata/test-upgrade-20260906/candidate-BenchmarkVerifyCanonicalBodyMaximum.json) |
| candidate | EnvelopeMarshalJSON | 2538355 | 23046 | 1793 | 30 | 62.77 | [record](testdata/test-upgrade-20260906/candidate-BenchmarkEnvelopeMarshalJSON.json) |
| candidate | EnvelopeUnmarshalJSON | 749150 | 43679 | 3726 | 48 | 37.39 | [record](testdata/test-upgrade-20260906/candidate-BenchmarkEnvelopeUnmarshalJSON.json) |
| reviewed | CanonicalObjectReusedScalarBuffer | 252387134 | 139.1 | 0 | 0 | 36.23 | [record](testdata/test-upgrade-20260906/reviewed-BenchmarkCanonicalObjectReusedScalarBuffer.json) |
| reviewed | SignCanonicalBody64KiB | 313674 | 114117 | 1053 | 18 | 36.59 | [record](testdata/test-upgrade-20260906/reviewed-BenchmarkSignCanonicalBody64KiB.json) |
| reviewed | SignCanonicalBodyMaximum | 65994 | 545348 | 1054 | 18 | 36.78 | [record](testdata/test-upgrade-20260906/reviewed-BenchmarkSignCanonicalBodyMaximum.json) |
| reviewed | VerifyCanonicalBody64KiB | 480975 | 74260 | 424 | 10 | 36.52 | [record](testdata/test-upgrade-20260906/reviewed-BenchmarkVerifyCanonicalBody64KiB.json) |
| reviewed | VerifyCanonicalBodyMaximum | 69337 | 508530 | 425 | 10 | 36.06 | [record](testdata/test-upgrade-20260906/reviewed-BenchmarkVerifyCanonicalBodyMaximum.json) |
| reviewed | EnvelopeMarshalJSON | 16695332 | 2140 | 1794 | 30 | 36.52 | [record](testdata/test-upgrade-20260906/reviewed-BenchmarkEnvelopeMarshalJSON.json) |
| reviewed | EnvelopeUnmarshalJSON | 8350554 | 4332 | 3731 | 48 | 37.97 | [record](testdata/test-upgrade-20260906/reviewed-BenchmarkEnvelopeUnmarshalJSON.json) |
