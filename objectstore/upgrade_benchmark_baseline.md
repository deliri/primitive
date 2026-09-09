# Objectstore benchmark baseline — 2026-09-08

Production base: `07e59437fc990fcd16d247f9a9c788cc3db214bc` (`v2026.1.24`). Go 1.27.1, Darwin/arm64, Apple M1 Max. The benchmark harness was strengthened before capturing these measurements; it checks exact digest/checksum/extent results and exact transport framing. It is preserved under ignored `testdata/test-upgrade-20260908/baseline-harness/`.

| Operation | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| GCS CRC32C response projection | 168.1 | 16 | 2 |
| Inspect 1 KiB, three digests | 7,065 | 43,992 | 9 |
| Inspect 1 MiB, three digests | 2,319,725 | 44,005 | 9 |
| Inspect 16 MiB, three digests | 36,317,001 | 44,203 | 9 |
| GCS upload 1 KiB | 16,813 | 40,330 | 99 |
| GCS upload 10 MiB | 6,478,261 | 40,157 | 99 |

Each measurement is one serial requested-30-second sample, `-count=1 -benchmem`, with `-cpuprofile`, `-memprofile`, and `-o` explicitly naming unique retained artifacts. The complete commands, timestamps, power/environment observations, source hashes, and matching binary/profile hashes are in [upgrade_evidence.json](upgrade_evidence.json). CPU and allocation top reports were generated with the matching binary. No statistical confidence interval or performance trend is implied by a single sample.

Upload uses a deterministic in-process HTTP RoundTripper, draining and checking the exact body through the real Exchange/Objectstore path. These figures measure stream/framing/integrity overhead and local hashing, not network throughput, provider latency, or durability. Inspection computes SHA-256, CRC32C, and the existing BLAKE3 contract. The essentially fixed allocation size across inspection payload sizes is consistent with its bounded buffer; it is not an assertion that every caller-owned reader uses bounded memory.

The 1 KiB upload allocation profile attributes 81.46% of allocated bytes directly to `bufio.NewReaderSize`; NewExactReader accounts for 81.69% cumulatively. That measured redundant allocation informs the direct-reader change. The 1 KiB inspection profile attributes about 74.44% to Inspect's buffer and 24.78% to BLAKE3 construction. No pooling or custom cryptographic implementation was introduced to hide those costs.

Historical `before_benchmarks.md` and `after_benchmarks.md` predate this source-bound campaign and are not substituted for these captured measurements.
