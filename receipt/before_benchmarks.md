# Receipt before — boundary audit, 2026-09-10

The baseline measures the actual existing production with checked benchmark
results. The older two KeepAlive-only measurements remain historical below.

The executable hostile-red run reproduced three failure classes:
- All six structured JSON doors rejected valid large whitespace because of
  package byte quotas: EvidenceBody, Header, EvidencePayload, EvidenceDocument,
  Scope and Watermark.
- EvidencePayload.WriteCanonical panicked for a typed-nil destination.
- Direct Generation.UnmarshalJSON rejected otherwise valid JSON whitespace
  around a canonical positive integer.

The later module build identified Submission's decision quota as a dependent
use of the deleted Receipt quota. Its necessary repair is included in this
slice, without representing Submission as fully upgraded.

| Case | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| BenchmarkVerifyEvidence-8 | 104284.0 | 3030 | 62 |
| BenchmarkAdvanceWatermark-8 | 231.6 | 0 | 0 |
| BenchmarkReceiptBoundary/issue-8 | 205323.0 | 3989 | 74 |
| BenchmarkReceiptBoundary/encode_document-8 | 39275.0 | 5409 | 80 |
| BenchmarkReceiptBoundary/decode_document-8 | 91420.0 | 11906 | 251 |
| BenchmarkReceiptBoundary/write_canonical-8 | 18681.0 | 2322 | 48 |
| BenchmarkReceiptBoundary/decode_scope-8 | 6723.0 | 849 | 21 |
| BenchmarkReceiptBoundary/decode_watermark-8 | 24451.0 | 3181 | 74 |

Evidence on Furnace: /home/d/engineering-evidence/primitive/receipt-upgrade-20260910/.
Base: 28950f769830d5fadeee121cd8f8322a25796335 (v2026.1.42).
This slice is uncommitted for user review. Each run records complete argv, base
commit, dirty source hashes, retained content snapshots, toolchain, selected
environment, stdout/stderr, exit, elapsed time and source stability.
manifest.json hashes retained profiles, binaries, snapshots and raw records.
These are author verification records, not an independent acceptance receipt.

Go 1.27.1; Linux amd64; AMD EPYC 7282; GOWORK=off; GOMAXPROCS=8.
Machine and selected Go environment are retained. Shared host and uncontrolled
power posture; one sample per case per pass. Timings are observations, not a
statistical performance guarantee. Eight cases per pass ran serially with
30 seconds configured for each, -count 1 -benchmem and explicit CPU/memory
profile and retained test-binary paths. All cases exceeded 30 seconds of
estimated timed work; counts and estimates are in benchmark-comparison.json.
Profiles report cumulative CPU/allocation samples, not peak or retained heap.

R=/home/d/engineering-evidence/primitive/receipt-upgrade-20260910
Commands:
```sh
go test ./receipt -run '^$' -bench . -benchtime 30s -count 1 -benchmem -cpuprofile "$R/baseline-profiles/cpu.pprof" -memprofile "$R/baseline-profiles/mem.pprof" -o "$R/baseline-profiles/receipt.test"
go test ./receipt -run '^$' -bench . -benchtime 30s -count 1 -benchmem -cpuprofile "$R/after-profiles/cpu.pprof" -memprofile "$R/after-profiles/mem.pprof" -o "$R/after-profiles/receipt.test"
```

Fixtures and expected canonical bytes are built before timing. The existing
verification and watermark benchmarks now check errors and exact retained
results after their deterministic loops; they no longer only KeepAlive an
unchecked error. Six added workloads measure issuance, document encoding and
decoding, canonical payload output, scope decoding and watermark decoding.
The canonical sink counts acknowledged bytes without retaining them.
These checked benchmarks were installed before changing production and kept
unchanged between passes. There is no live provider traffic in these cases.

Command elapsed seconds: baseline-profiles=295.14, after-profiles=288.126.

The previous note remains historical below.

---

# receipt before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. VerifyEvidence / AdvanceWatermark on authenticated facts.
