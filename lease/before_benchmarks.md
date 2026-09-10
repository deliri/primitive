# Lease before — JSON extent audit, 2026-09-10

Base production: b7526585cb764450d57487d5d56d9736134b18e3 (v2026.1.39).
The baseline benchmark fixtures were strengthened before measurement: results
are checked, decoding compares the exact signed fixture, and canonical writing
counts every emitted byte. Production was unchanged. The later parent benchmark
ReportAllocs addition only declares allocation reporting for its subbenchmarks;
timed operations and inputs are identical.

Lease imposed JSON whitespace quotas on twelve public decoder doors. Its
Decision decoder and Attest envelope decoder copied already-owned input through
Core's reader buffer. One purported size-boundary test supplied NUL bytes, so it
could pass without exercising a valid oversized representation.

| Case | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| BenchmarkEvaluate-8 | 3278 | 128 | 4 |
| BenchmarkVerify-8 | 108640 | 2381 | 68 |
| BenchmarkDecisionCanonicalJSON-8 | 19901 | 1626 | 53 |
| BenchmarkDocumentJSONDecode/canonical-8 | 95974 | 16589 | 243 |
| BenchmarkDocumentJSONDecode/whitespace_256-8 | 94043 | 16589 | 243 |
| BenchmarkDecisionCanonicalWriter-8 | 18843 | 1625 | 53 |

Evidence on Furnace: /home/d/engineering-evidence/primitive/lease-upgrade-20260910/.
Each run retains complete argv, base commit, dirty source hashes and snapshots,
environment, stdout/stderr, exit and elapsed time. Profiles and matching test
binaries are in baseline-profiles and after-profiles; manifest.json covers them.
Go 1.27.1, Linux amd64, AMD EPYC 7282, GOWORK=off, GOMAXPROCS=8.
Six benchmark cases each received 30 seconds, serially after tests/checks;
shared host, one sample per case, power posture uncontrolled. These are absolute
before/after samples, not distributions or an established performance trend.

Effective benchmark command durations: 218.277 seconds before; 229.255 seconds after.

The older note below remains historical.

# lease before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Evaluate/Verify are bounded document operations.
