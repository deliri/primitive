# Manual before — boundary audit, 2026-09-10

The prior benchmark measured only ParseLine. This baseline adds book validation,
projection, index/help/full text, a longer line and complete machine JSON output.

The executable hostile-red run reproduced arbitrary page/item/text quotas,
typed-nil writer panics, unvalidated schema decoding, admitted Unicode line and
paragraph separators, canonical self/duplicate relation aliases, foreign typed
identity borrowing a documented name, and PageReport's missing self-relation
check. The later writer tables also attack impossible native write counts.

Before profiles identified io.WriteString conversions and output fragments as
allocation sources. Text emission wrapped every fragment in an io.Writer-only
adapter, forcing string-to-byte conversions rather than using Go's WriteString
buffer path. Book validation and line scanning occupied much of CPU time.

| Case | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| BenchmarkParseLine-8 | 56.71 | 0 | 0 |
| BenchmarkManualBook/validate-8 | 5387.0 | 256 | 2 |
| BenchmarkManualBook/project-8 | 7867.0 | 1040 | 19 |
| BenchmarkManualText/index-8 | 8747.0 | 752 | 34 |
| BenchmarkManualText/help-8 | 9795.0 | 880 | 46 |
| BenchmarkManualText/manual-8 | 16127.0 | 1960 | 120 |
| BenchmarkManualText/manual_1KiB_line-8 | 18656.0 | 2960 | 120 |
| BenchmarkManualJSON-8 | 12272.0 | 512 | 8 |

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

# manual before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Validate/report walk caller-owned book slices (cap = len).
