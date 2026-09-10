# Googleidentity before/after — 2026-09-10

| Work per iteration | Before ns/op | After ns/op | Change | Before → after B/op | Before → after allocs/op |
|---|---:|---:|---:|---:|---:|
| AccessResponseDecode | 6414 | 6449 | +0.55% | 890 → 891 | 20 → 20 |
| MetadataIdentityAcquisition | 222572 | 222976 | +0.18% | 10782 → 10775 | 121 → 121 |
| MetadataAccessAcquisition | 229365 | 231688 | +1.01% | 10412 → 10412 | 128 → 128 |
| VerifyCachedCertificate | 99246 | 100477 | +1.24% | 4855 → 4857 | 58 → 58 |
| AudienceParseBatch | 37855 | 38075 | +0.58% | 0 → 0 | 0 → 0 |
| AudienceStringBatch | 37499 | 131.7 | -99.65% | 0 → 0 | 0 → 0 |
| CommandTokenDisclosure | 15049 | 14862 | -1.24% | 8976 → 8976 | 3 → 3 |
| ServiceAccountAcquisition | 4.06665e+06 | 3.71937e+06 | -8.54% | 111058 → 93033 | 414 → 353 |

Audience batches perform 64 checked operations on the same 512-byte UTF-8 value. Command disclosure parses 4 KiB plus CRLF and checks the exact bearer. Metadata uses real local HTTP. Verification uses a warmed certificate cache and real SDK signature verification. Service-account acquisition includes Filestore reads, SDK signing and real local HTTP; the repaired path additionally enforces Exchange boundaries.

These are single paired observations, not statistical significance claims. Small timing/byte differences are reported without attributing them to improvements. All three benchmark source files are byte-identical across phases; fixture helper changes add checked setup and cleanup outside timed work. No IAM throughput claim is made.

The baseline CPU profile attributed 33.09 cumulative seconds to Audience.String and repeated UTF-8 validation. String now returns its immutable admitted value directly. Parsing still scans input. Allocation counts for the original seven cases remain unchanged. Service-account acquisition drops 414 to 353 allocations; the after CPU profile remains dominated by Go RSA signing (74.18% cumulative). Its allocation profile points to io.copyBuffer, JSON and crypto ownership. No custom crypto or buffer runtime was added.

~~~text
after-bench: go test ./googleidentity -run=^$ -bench=^(BenchmarkAccessResponseDecode|BenchmarkMetadataIdentityAcquisition|BenchmarkMetadataAccessAcquisition|BenchmarkVerifyCachedCertificate|BenchmarkAudienceParseBatch|BenchmarkAudienceStringBatch|BenchmarkCommandTokenDisclosure)$ -benchmem -benchtime=30s -count=1 -cpuprofile=/home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910/after-bench/cpu.pprof -memprofile=/home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910/after-bench/mem.pprof -o /home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910/after-bench/googleidentity.test
after-service-bench: go test ./googleidentity -run=^$ -bench=^BenchmarkServiceAccountAcquisition$ -benchmem -benchtime=30s -count=1 -cpuprofile=/home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910/after-service-bench/cpu.pprof -memprofile=/home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910/after-service-bench/mem.pprof -o /home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910/after-service-bench/googleidentity.test
~~~

Raw evidence: /home/d/engineering-evidence/primitive/googleidentity-upgrade-20260910. Profiles, pprof summaries, binaries, complete commands and exact source snapshots are retained outside Git. Each case ran once per phase for 30 configured seconds.
