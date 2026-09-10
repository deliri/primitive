# Retrieval before — boundary audit, 2026-09-10

The previous benchmark measured only signing-domain parsing. This baseline adds
request/grant decoding, canonical output and complete 32 KiB/4 MiB file downloads.
Production was unchanged for the baseline. Fixtures, signing and input creation
are outside timed loops. Downloads use deterministic HTTP responses through
Exchange and real Filestore staging and replacement on Furnace, without live
provider credentials. File fixture bytes are allocated before timing.

The executable hostile run reproduced nil-destination panics in both canonical
writers, admission of typed-nil DownloadCallRequest destinations, four local
JSON quota refusals for valid large whitespace, and direct Selection decoding
that admitted invalid fields or retained a previous arm. Go's outer JSON decoder
may reject syntax before invoking a custom method; one initial test incorrectly
required Primitive error identity there, and was corrected to exercise the
public Primitive decoder directly. Both failed attempts remain recorded.

| Case | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| BenchmarkRetrievalDocuments/request-8 | 88411.0 | 10735 | 241 |
| BenchmarkRetrievalDocuments/request_whitespace_32KiB-8 | 245771.0 | 141194 | 250 |
| BenchmarkRetrievalDocuments/grant-8 | 297720.0 | 42483 | 709 |
| BenchmarkRetrievalDocuments/request_canonical-8 | 24646.0 | 2450 | 62 |
| BenchmarkRetrievalDocuments/grant_canonical-8 | 69305.0 | 10238 | 145 |
| BenchmarkRetrievalDownloadFile/32KiB-8 | 5686799.0 | 94747 | 732 |
| BenchmarkRetrievalDownloadFile/4MiB-8 | 15818457.0 | 94826 | 732 |
| BenchmarkParseSigningDomain-8 | 9.897 | 0 | 0 |

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

The previous note remains historical below.


---

# retrieval before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Hot path is a typed parse/assemble door.

## Measured

ParseSigningDomain 5.655 ns/op 0 B/op 0 allocs
