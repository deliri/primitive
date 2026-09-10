# Compass benchmark comparison — 2026-09-10

| Benchmark | Before ns/op | After ns/op | Before → after B/op | Before → after allocs/op |
|---|---:|---:|---:|---:|
| BenchmarkCurrent-8 | 22213 | 22409 | 6261 → 6261 | 54 → 54 |
| BenchmarkDecodeConfiguration/contiguous-8 | 20929 | 19957 | 6200 → 6200 | 52 → 52 |
| BenchmarkDecodeConfiguration/one_byte_reads-8 | 22352 | 22382 | 6196 → 6196 | 52 → 52 |
| BenchmarkParseProjectNameBatch-8 | 3356 | 2607 | 0 → 0 | 0 → 0 |
| BenchmarkProjectNameStringBatch-8 | 13351 | 168.5 | 0 → 0 | 0 → 0 |

The parse batch contains 16 calls; the String batch contains 64 calls. These are batch times, including exact-result checks and loop overhead.

Five cases ran once each before and after, 30 seconds per case with explicit CPU/memory profiles and retained executables. Baseline comprised four original cases plus a separately added getter case before any production edit. The after pass ran all five serially after checks, followed by two serial 30-second fuzz targets with four workers.

The getter baseline attributes 98.16% of sampled CPU time to ProjectName.Validate. String now returns the admitted private string directly. Admission, explicit Validate and JSON boundaries still validate. No claim of faster JSON decoding follows from single timing samples; decoder allocation counts are unchanged. Core.readStrictJSONDocument dominates baseline allocated bytes, so this slice does not disguise its complete-document buffering as O(1) streaming.

These are single before/after observations, not distributions or a statistically established speed trend. Raw iterations, absolute times, run durations, source hashes and artifact digests remain available in /home/d/engineering-evidence/primitive/compass-upgrade-20260910. The baseline and after benchmark source files match exactly; production changes and all other test changes remain in per-run source snapshots. No favorable attempt was selected or failed attempt discarded.

Full run commands: baseline-bench/result.json, baseline-string-bench/result.json, after-bench/result.json. Each benchmark uses -run=^$ -benchmem -benchtime=30s -count=1 and explicit -cpuprofile, -memprofile, -o; baseline filters are recorded, after uses -bench=.. All execute go test ./compass. Evidence manifest: /home/d/engineering-evidence/primitive/compass-upgrade-20260910/manifest.json.
