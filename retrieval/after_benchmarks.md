# Retrieval after — boundary audit, 2026-09-10

Retrieval now validates and replaces Selection atomically at its own JSON door.
Unknown fields, contradictory or missing union arms, null input, and failed
decoding cannot publish partial fields or retain a stale previous arm.
Both canonical writers and DownloadCallRequest reject typed-nil destinations.
Every canonical short prefix, native writer failure, impossible count, and
invalid body's absence of writes is covered.

The four Retrieval JSON quota constants and the dependent Retrievalauth
credentialed-document quota were removed, with the real in-Primitive caller
updated and no aliases. Strict grammar, closed fields, typed validation,
authentication, exact signed extents and hashes remain. Large valid whitespace
crosses these doors; trailing values and truncated structures still fail without
mutating an existing receiver.

File downloads retain Objectstore -> Exchange -> Go transport and Filestore ->
Go filesystem execution. Tests now obtain filesystem capabilities through
Filestore. Empty authenticated objects, multiple copy windows, truncation,
extra bytes, altered hashes, provider refusal, cancellation, observer refusal,
response-close failure, create conflict, occupied staging names and pre-effect
refusal check the real transfer/stage/activation path. A pre-existing stage is
preserved; failure preserves the prior target. Correctly signed but contradictory
traversal grants are attacked at both issuance and verification.
Duplicated size/timestamp padding rows were removed and reordered fixtures
actually reverse their members.

| Case | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| BenchmarkRetrievalDocuments/request-8 | 88411.0 | 98055.0 | 10735 | 11418 | 241 | 255 |
| BenchmarkRetrievalDocuments/request_whitespace_32KiB-8 | 245771.0 | 253800.0 | 141194 | 141886 | 250 | 264 |
| BenchmarkRetrievalDocuments/grant-8 | 297720.0 | 300885.0 | 42483 | 42484 | 709 | 709 |
| BenchmarkRetrievalDocuments/request_canonical-8 | 24646.0 | 24907.0 | 2450 | 2450 | 62 | 62 |
| BenchmarkRetrievalDocuments/grant_canonical-8 | 69305.0 | 70636.0 | 10238 | 10239 | 145 | 145 |
| BenchmarkRetrievalDownloadFile/32KiB-8 | 5686799.0 | 5797311.0 | 94747 | 94631 | 732 | 732 |
| BenchmarkRetrievalDownloadFile/4MiB-8 | 15818457.0 | 15957800.0 | 94826 | 94828 | 732 | 732 |
| BenchmarkParseSigningDomain-8 | 9.897 | 9.478 | 0 | 0 | 0 | 0 |

Evidence on Furnace: /home/d/engineering-evidence/primitive/retrieval-upgrade-20260910/.
The base commit is 6adae2ce6c9907d19ec55e922c76e70670a303a4 (v2026.1.40).
Measurements were captured from the uncommitted source approved by the user
for v2026.1.41. Each run records the complete command,
base commit, dirty source hashes and retained content snapshots, environment,
stdout/stderr, exit, elapsed time and whether source changed during execution.
manifest.json covers raw runs, profiles, binaries, source snapshots and summaries.

Go 1.27.1; Linux amd64; AMD EPYC 7282; GOWORK=off; GOMAXPROCS=8.
Machine and selected Go environment details are retained. Shared host and
uncontrolled power posture; one sample per case in each pass. These are absolute
samples, not distributions or statistically established performance trends.
Eight cases run serially with -benchtime 30s -count 1 -benchmem and explicit
-cpuprofile/-memprofile/-o paths, after the relevant tests/checks.
The domain parser reaches Go testing's native one-billion-iteration ceiling:
its estimated timed work is 9.897 seconds before and 9.478 seconds after,
despite a 30-second configured target. It is not a full 30-second timed sample.
Other cases exceed 30 seconds of estimated timed work. Per-case iterations
and estimates are in benchmark-comparison.json; exact argv is in each run record.

Benchmark command elapsed time: 258.757 seconds before; 258.119 seconds after.

Request decoding now performs Selection's missing validation. The sample cost
increased from 88,411 to 98,055 ns/op and from 241 to 255 allocations/op.
This is a measured correctness cost, not a speed improvement. Allocation
profiles are dominated by Go JSON decoder fetches, bytes.Clone and Core strict
validation; cumulative allocated bytes are not retained heap size.
No replacement parser, runtime, workflow engine or product state was added.

Both transfer sizes report 732 allocations/op and approximately 95 KB/op.
This supports steady allocation across the measured 128x size increase.
It does not establish O(1) time or prove exabyte execution. The response test
adapter acquired close-error injection between passes; treat file timings as
whole-slice observations, not isolated production-only optimization evidence.
Parent ReportAllocs declarations and post-loop failure context were corrected
after baseline without changing timed operations or input extents.

Boundary of this slice:
- Download object bytes stream; authenticated length is exact identity, not a
  product-size quota. Streaming memory is independent of total file size.
- JSON methods still accept or return whole byte slices. Callers own those
  allocations; no O(1) whole-document decoder is claimed.
- Nested provider, Receipt and Controlplane contracts retain their own existing
  validation/limits. This is not certification that every nested metadata door
  or every Primitive package has completed the wider streaming audit.
- This is a Retrieval upgrade plus its necessary Retrievalauth caller correction,
  not a full Retrievalauth or dependency-package audit.

Verification: scoped Retrieval and Retrievalauth tests/race, go fix -diff (empty),
vet, staticcheck, errcheck, witness-lint and production gocyclo <= 10 passed.
All Primitive production packages build. Retrieval test binaries compile for
macOS arm64 and Windows amd64; execution is Linux-only. Full-module test gates
and live cloud-provider integration were not run. Failed lint, build, oracle
and red attempts remain in the evidence rather than being folded into success.

Four compiled mutations were killed and restored: short-write acceptance,
premature selection receiver mutation, traversal-check bypass, and reinstated
credentialed-document quota. Nine Retrieval fuzz targets plus the affected
Retrievalauth target each ran once for 30 seconds with four workers, targets
serially after profiles. Configured aggregate fuzz budget: 300 seconds.
Fuzz seed/cache progress and execution counts remain visible in raw output and
fuzz-results.json; no filtered run is described as a full-module test pass.

The latest test-only refinements were verified by review-checks, review-witness
and review-race before after-profiles. Earlier static and cross-platform checks
have their own exact source snapshots; production is identical across those
checks and the final profiles. Post-run repository edits are these notes and the approved release coordinate.
This author verification was approved by the user for v2026.1.41; it is not an independent receipt.


---

# retrieval after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./retrieval`

```
BenchmarkParseSigningDomain-10    	215200395	         5.576 ns/op	       0 B/op	       0 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
