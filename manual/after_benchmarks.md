# Manual after — boundary audit, 2026-09-10

## Review follow-up — 2026-09-10

The external review at /tmp/manual_review_20260910_findings.md found no production
bug. Its findings are retained as review-findings.md in the evidence directory.
Package documentation now states caller ownership, item-count-dependent
validation indexes, copied projection slices, native text/JSON buffering, and
the need to Validate a Report after generic JSON decoding. Empty/invalid-UTF-8
diagnostics no longer describe a removed extent quota.

Scoped race tests, vet and witness-lint passed for this follow-up. Tests,
acceptance rules, output bytes and successful benchmark paths were unchanged.
The historical CPU/memory profiles were not rerun for documentation and error
wording changes; their exact source snapshots remain in the original run records.
No extra decoder, buffer layer or lookup abstraction was introduced.


## Contract and implementation

Removed MaximumPages, MaximumSectionItems, MaximumLineBytes and MaximumTopicBytes.
There are no replacement total-extent quotas or compatibility aliases. Nonempty
required sections, valid UTF-8, canonical topic grammar and exact duplicate and
relation checks remain. Line rejects U+2028 and U+2029 as line-breaking content.

Related commands bind both the canonical topic name and the exact comparable
product-owned topic value. Aliases cannot create self relations or duplicate
canonical relations. PageReport owns its own self-relation refusal.

Schema is now a closed uint8 enum with SchemaUnknown and SchemaV1. NewSchema
parses the compiler-owned SchemaV1Token; JSON decoding validates before replacing
the receiver and preserves Core JSON/manual error identities. The published
wire token is unchanged. This is a clean Go contract change from the former
string representation, with no alias. No production in-Primitive consumers
required a migration.

WriteText delegates buffering to bufio.NewWriter and emission to Go's
io.WriteString. It performs all request validation before any output. Both text
and JSON reject typed-nil destinations. The small writer adapter preserves
native error identities and refuses short or impossible counts. It does not
retry, invent a runtime, manage product state, or implement JSON escaping.

## Measured costs

| Case | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| BenchmarkParseLine-8 | 56.71 | 69.0 | 0 | 0 | 0 | 0 |
| BenchmarkManualBook/validate-8 | 5387.0 | 5782.0 | 256 | 256 | 2 | 2 |
| BenchmarkManualBook/project-8 | 7867.0 | 8259.0 | 1040 | 1040 | 19 | 19 |
| BenchmarkManualText/index-8 | 8747.0 | 9504.0 | 752 | 4432 | 34 | 5 |
| BenchmarkManualText/help-8 | 9795.0 | 9571.0 | 880 | 4432 | 46 | 5 |
| BenchmarkManualText/manual-8 | 16127.0 | 10479.0 | 1960 | 4432 | 120 | 5 |
| BenchmarkManualText/manual_1KiB_line-8 | 18656.0 | 13912.0 | 2960 | 4432 | 120 | 5 |
| BenchmarkManualJSON-8 | 12272.0 | 14233.0 | 512 | 536 | 8 | 11 |

Full manual text measured 16,127 -> 10,479 ns/op and 120 -> 5 allocations/op.
That is accompanied by 1,960 -> 4,432 B/op for Go's fixed buffer. Index text
measured 8,747 -> 9,504 ns/op despite fewer allocations. ParseLine measured
56.71 -> 69.00 ns/op; this slice adds missing separator checks. JSON measured
12,272 -> 14,233 ns/op and 8 -> 11 allocations/op with validated schema encoding.
These are disclosed costs, not an across-the-board performance improvement.

Before allocation samples attributed about 28.8% to io.WriteString and 25.7%
to output. After samples attribute about 83.3% to bufio.NewWriterSize. Allocation
has shifted from many fragments to one fixed buffer. No pool or custom buffer
lifetime was added to suppress those measured bytes.

After-only extent samples:

| Case | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| BenchmarkManualExtent/text_1KiB-8 | 9337.0 | 4576 | 5 |
| BenchmarkManualExtent/text_1MiB-8 | 1753603.0 | 4592 | 5 |

A 1,024x line-size increase retains 5 allocations/op and approximately 4.6 KB/op.
This supports fixed text-rendering working allocation over the measured range;
it does not claim constant time or prove petabyte execution.

Book and Report are explicit caller-owned values. Exact duplicate validation
uses metadata proportional to item count, Project allocates independent output
slices, and Go's JSON encoder may buffer an individual string token. This is
not an O(1) whole-document JSON parser or an O(1) materialized book. The text
renderer streams through the native buffer without aggregating emitted text.

## Hostile proof

Tests bind to the production structs and typed identities. They cover required
fields, unknown enums, typed identity collisions, canonical aliases, invalid
machine reports, strict scalar grammar, nil writers, exact text layout and
declaration order, absent selected topics, every short output prefix, native
failure identities, impossible counts, and failures across multiple buffer
flushes. Every projected slice is attacked for aliasing in both directions.
Invalid requests and late invalid line content emit zero bytes.

The three small enum domains are exhaustively checked across all 256
representations. Protocol/projection structs have a compiler-bound inventory.
Old Contains-only rendering and decoder-detached ownership tests were removed
in favor of exact layout and direct ownership tables. Cases follow actual
Manual failure dimensions; there is no producer-to-classifier handoff here,
and no synthetic 50-row classifier matrix or duplicated numeric-quota padding
is claimed.

Six compiled mutations were killed and restored: foreign typed relation
acceptance, short-write acceptance, aliased projected slices, line-separator
acceptance, and rejection of the canonical View and SelectionMode fuzz seeds.
The final two demonstrate that the strengthened fuzz oracles detect
reject-everything behavior as well as invalid admission.

Scoped race tests, go fix -diff (empty), vet, staticcheck, errcheck, witness-lint
and production gocyclo <= 10 pass. All Primitive production packages build.
Final Manual test binaries compile for macOS arm64 and Windows amd64;
runtime execution is Linux-only. Full-module test gates were not run.

Six fuzz targets ran serially after benchmark/profile collection for 30 seconds
each with four workers. The two pre-existing enum targets were then strengthened
with independent native-token admission oracles and each rerun for 30 seconds.
Configured total: 240 seconds across eight target runs, six unique targets.
Raw output and fuzz-results.json retain seed/cache progress and reported
execution counts; repeated runs are visible. All targets passed.

Initial red failures, staticcheck raw-regexp findings and Witness's rejection
of a generic deep comparison remain recorded. Final tests compare typed fields.
Final test-only refinements occurred after profiles; production and comparable
timed workloads are unchanged. Final scoped checks and cross-compilation bind
to the final test sources. Documentation was written after execution.
Evidence on Furnace: /home/d/engineering-evidence/primitive/manual-upgrade-20260910/.
Base commit: b459236e8f7a75239a77afb17a327eb50d2023d5 (v2026.1.41).
Manual changes are uncommitted for user review. Per-run result.json records the
complete argv, toolchain, selected environment, base commit, dirty source hashes,
retained content snapshots, stdout/stderr, exit, elapsed time and source stability.
manifest.json hashes retained profiles, test binaries, run records and source
snapshots outside Git. These notes are summaries, not independent receipts.

Go 1.27.1; Linux amd64; AMD EPYC 7282; GOWORK=off; GOMAXPROCS=8.
Machine and selected Go environment are retained. Shared host, uncontrolled power
posture, one sample per case per pass: these are observations, not statistical
performance guarantees. Eight comparable cases ran serially with 30 seconds
configured per case, -count 1 -benchmem and explicit CPU/memory profile and binary
paths. The after-only extent pass used two further 30-second cases.
All reported cases exceeded 30 seconds estimated timed work; iterations and
estimates are in benchmark-comparison.json. Profiles report cumulative CPU and
allocation samples; cumulative allocated bytes are not retained heap size.

Commands (R=/home/d/engineering-evidence/primitive/manual-upgrade-20260910):
```sh
go test ./manual -run '^$' -bench . -benchtime 30s -count 1 -benchmem -cpuprofile "$R/baseline-profiles/cpu.pprof" -memprofile "$R/baseline-profiles/mem.pprof" -o "$R/baseline-profiles/manual.test"
go test ./manual -run '^$' -bench '^(BenchmarkParseLine|BenchmarkManualBook|BenchmarkManualText|BenchmarkManualJSON)$' -benchtime 30s -count 1 -benchmem -cpuprofile "$R/after-profiles/cpu.pprof" -memprofile "$R/after-profiles/mem.pprof" -o "$R/after-profiles/manual.test"
go test ./manual -run '^$' -bench '^BenchmarkManualExtent$' -benchtime 30s -count 1 -benchmem -cpuprofile "$R/extent-profiles/cpu.pprof" -memprofile "$R/extent-profiles/mem.pprof" -o "$R/extent-profiles/manual.test"
```

The baseline added seven workloads to the existing ParseLine benchmark before
changing production. The comparable eight timed workloads and fixtures were
preserved. Extent benchmarks were added later and excluded from the comparable
after pass. Fixture construction and input strings are outside timed loops.
Sinks drain bytes and check exact counts; they do not retain rendered documents.
There are no live providers in these benchmarks.

Command elapsed seconds: baseline-profiles=286.963, after-profiles=288.813, extent-profiles=72.578.

The prior note is preserved as historical evidence below.

---

# manual after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./manual`

```
BenchmarkParseLine-10    	30580324	        33.42 ns/op	       0 B/op	       0 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
