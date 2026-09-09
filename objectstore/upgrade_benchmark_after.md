# Objectstore benchmark comparison — 2026-09-08

The same three benchmark source files and six workloads were used before and after production changes. Every run requested `-benchtime=30s -count=1 -benchmem`, explicitly supplied CPU/memory profile paths and a matching retained test-binary path, and ran serially. Go 1.27.1, Darwin/arm64, Apple M1 Max. Full dirty-source snapshots, raw commands, iteration counts, timing, power observations, and artifact hashes are recorded in [upgrade_evidence.json](upgrade_evidence.json).

| Operation | Baseline ns/op | Candidate ns/op | Baseline B/op | Candidate B/op | Baseline → candidate allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| GCS CRC32C response projection | 168.1 | 154.0 | 16 | 16 | 2 → 2 |
| Inspect 1 KiB | 7,065 | 6,854 | 43,992 | 43,992 | 9 → 9 |
| Inspect 1 MiB | 2,319,725 | 2,246,782 | 44,005 | 44,006 | 9 → 9 |
| Inspect 16 MiB | 36,317,001 | 35,842,760 | 44,203 | 44,197 | 9 → 9 |
| GCS upload 1 KiB | 16,813 | 11,032 | 40,330 | 7,284 | 99 → 97 |
| GCS upload 10 MiB | 6,478,261 | 6,448,883 | 40,157 | 7,265 | 99 → 97 |

These are six single baseline/candidate pairs, not distributions or statistically proven timing trends. Host isolation was not established; a separate workspace's Go race run was observed during this upgrade. No favorable sample was substituted for an unfavorable one. The two intermediate upload samples remain in [the review report](upgrade_review.md) and the manifest; they predate the final production source and are not additional final-source samples.

The baseline 1 KiB upload allocation profile attributed 81.46% of allocated bytes to `bufio.NewReaderSize`. That redundant 32 KiB reader buffer is absent from the candidate path and profiles. Both upload sizes now use approximately 7.3 KiB per operation with two fewer allocations. The 10 MiB CPU profile is dominated by Go's SHA-256 and CRC32C work: object bytes still pass through the standard readers, writers, and hashing implementations. No pool or custom cryptography was added.

Inspection retains its 32 KiB working buffer and BLAKE3 hasher, approximately 44 KiB per operation across the measured payload sizes. The 16 MiB CPU profile attributes approximately 64% cumulatively to the existing BLAKE3 compression implementation. BLAKE3 remains the user-approved dependency. This profile is evidence of where work occurs, not permission to replace a cryptographic implementation or broaden package scope.

Memory profiles include setup, runtime, and profile-capture allocations outside benchmark timing. In the 16 MiB inspection and 10 MiB upload runs the fixed fixture payload is visible in `alloc_space`; it must not be mistaken for per-operation growth. `B/op` is the benchmark's timed allocation counter. Small B/op differences in the inspection pair and large upload pair include runtime amortization. Profiles use the matching retained binary, not a subsequently rebuilt executable.

Upload benchmarks use a deterministic in-process HTTP transport that consumes and verifies body bytes and declared framing through real Objectstore/Exchange execution. They measure local transfer machinery and hashing, not provider/network latency or remote durability. Benchmarks check exact results on every operation. Raw profiles, binaries, and logs are local ignored artifacts; reports preserve their coordinates and hashes without committing their bytes.
