# Lease after — JSON extent audit, 2026-09-10

Lease's byte-slice JSON doors no longer impose arbitrary whitespace/document
quotas. Core owns strict grammar and Go owns JSON decoding. Decision and nested
Attest envelope decoding now use Core's byte-slice API, removing redundant reader
copies. Attest's envelope input quota was removed to close the actual nested
Lease document path; other Attest capabilities were not swept again.

Canonical sizes still describe the exact fixed-shape signed agreement. This
slice does not add a streaming JSON API: callers own the complete input slice,
and decoding allocations can follow that representation. No exabyte execution
or O(1) whole-JSON decoder is claimed. Lease's canonical writer emits its fixed
typed record with direct synchronous backpressure.

Typed-nil destinations now fail before a write call. Decision JSON cross-field
refusals retain both Lease and JSON error identities. Arbitrary document quota
constants and quota-only tests were removed, without aliases.

| Case | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| BenchmarkEvaluate-8 | 3278 | 3037 | 128 | 128 | 4 | 4 |
| BenchmarkVerify-8 | 108640 | 110223 | 2381 | 2382 | 68 | 68 |
| BenchmarkDecisionCanonicalJSON-8 | 19901 | 18370 | 1626 | 1626 | 53 | 53 |
| BenchmarkDocumentJSONDecode/canonical-8 | 95974 | 88386 | 16589 | 10842 | 243 | 239 |
| BenchmarkDocumentJSONDecode/whitespace_256-8 | 94043 | 87304 | 16589 | 10851 | 243 | 239 |
| BenchmarkDecisionCanonicalWriter-8 | 18843 | 18330 | 1625 | 1625 | 53 | 53 |

Evidence on Furnace: /home/d/engineering-evidence/primitive/lease-upgrade-20260910/.
Each run retains complete argv, base commit, dirty source hashes and snapshots,
environment, stdout/stderr, exit and elapsed time. Profiles and matching test
binaries are in baseline-profiles and after-profiles; manifest.json covers them.
Go 1.27.1, Linux amd64, AMD EPYC 7282, GOWORK=off, GOMAXPROCS=8.
Six benchmark cases each received 30 seconds, serially after tests/checks;
shared host, one sample per case, power posture uncontrolled. These are absolute
before/after samples, not distributions or an established performance trend.

Effective benchmark command durations: 218.277 seconds before; 229.255 seconds after.

The baseline allocation profile identifies Core's redundant reader buffer as
a leading allocation source. The after measurements use the same workloads;
no alternative JSON parser, transport, clock, or retained product state was added.

Hostile proof:
- Twelve JSON doors accept canonical input and below/at/above their former
  quotas, plus 1 MiB + 1 byte of whitespace. Large trailing values and NULs remain
  invalid; populated receivers remain unchanged on refusal.
- Nested whitespace inside the signed Lease document reaches both Lease and
  Attest, then passes the real signature verifier. Padded truncation is refused.
- Every canonical write prefix from zero through the full body, plus impossible
  counts on both sides, is tested with nil and native errors. The exact emitted
  prefix and one-call backpressure are checked.
- Typed-nil writer panic, valid-whitespace refusal and missing JSON identity
  were observed as executable reds. Compiled short-write and signing-domain
  mutations were killed and restored.
- Six Lease fuzz targets and Attest's envelope semantic target each ran once
  for 30 seconds after profiles. Exact attempts remain in the manifest.

Scoped Lease and Attest race tests, go fix -diff (empty), vet, staticcheck,
errcheck, witness-lint and production gocyclo <= 10 passed. The first witness
attempt exposed a missing parent ReportAllocs declaration; it is retained and
the correction passed. All Primitive production packages build. Lease test
binaries compile for macOS arm64 and Windows amd64; execution was Linux-only.
Full-module test gates were not run. These are author-run facts ready for review,
not an independent acceptance receipt.

Scope: Lease JSON ingress/canonical output streaming audit and the required
nested Attest decoder correction. Existing assessment/advance policy remains
unchanged. No claim is made that every other Primitive package is streaming.
Final post-run edits affect these benchmark notes only.

The older report below remains historical.

# lease after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./lease`

```
BenchmarkEvaluate-10                 	  955819	      1124 ns/op	     128 B/op	       4 allocs/op
BenchmarkVerify-10                   	   20282	     59720 ns/op	    2377 B/op	      68 allocs/op
BenchmarkDecisionCanonicalJSON-10    	  307743	      4150 ns/op	    1625 B/op	      53 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
