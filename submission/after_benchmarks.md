# Submission benchmark comparison — 2026-09-10

Same benchmark source, fixtures, Go 1.27.1, machine, GOMAXPROCS=8 and 30-second-per-case configuration as [the baseline](before_benchmarks.md). This is one profiled pass per phase, not a statistical confidence interval or a claim that every path became faster.

| Work | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs | After allocs |
|---|---:|---:|---:|---:|---:|---:|
| BenchmarkParseSigningDomainBatch | 112.7 | 117.6 | 0 | 0 | 0 | 0 |
| RequestCommitment | 34091 | 36745 | 4256 | 4256 | 91 | 91 |
| IssueRequest | 237045 | 234968 | 6366 | 6365 | 126 | 126 |
| VerifyRequest | 136533 | 136163 | 5387 | 5387 | 117 | 117 |
| IssueGrant | 453058 | 452053 | 29013 | 29005 | 369 | 369 |
| VerifyGrant | 254467 | 257039 | 19811 | 19811 | 287 | 287 |
| IssueCompletion | 638587 | 410012 | 57683 | 31711 | 766 | 431 |
| VerifyCompletion | 306525 | 303597 | 21104 | 21103 | 330 | 330 |
| DecodeCompletion | 109219 | 110506 | 14735 | 14735 | 284 | 284 |
| UploadCall | 207610 | 210338 | 33468 | 33468 | 425 | 425 |

The before CPU profile attributes 16.66 of IssueCompletion's 34.69 sampled CPU seconds to completionProjection; CompletionIssuance.Validate accounts for 11.29 cumulative seconds, including its first projection. The allocation profile attributes 1.35 GB of that case's 2.93 GB cumulative allocation to CompletionIssuance.Validate. These are cumulative allocations across all benchmark iterations, not live memory.

IssueCompletion now constructs that projection once and delegates signer admission and signing to Attest. The public CompletionIssuance.Validate method remains usable and owns the same admission rule. Tests retain exact signatures, native signer errors, cancellation, invalid-input zero results and zero signing effects.

Other fixes remove document quotas, reject typed-nil stream endpoints, return a zero rejected decision and authenticate completion signatures before classifying binding mismatches. Signing and verification still use Primitive Attest and Go Ed25519; no crypto or transport replacement was introduced.

Command:
```sh
go test ./submission -run=^$ -bench=. -benchmem -benchtime=30s -count=1 -cpuprofile=/home/d/engineering-evidence/primitive/submission-upgrade-20260910/after-bench/cpu.pprof -memprofile=/home/d/engineering-evidence/primitive/submission-upgrade-20260910/after-bench/mem.pprof -o /home/d/engineering-evidence/primitive/submission-upgrade-20260910/after-bench/submission.test
```

Raw runs, before/after profiles and retained binaries: `/home/d/engineering-evidence/primitive/submission-upgrade-20260910`. Profiles share the exact source snapshot of their own benchmark pass. The final artifact manifest covers both passes and all failures.

Submission describes an object's extent; Objectstore performs the content stream. These measurements are metadata costs, not simulated terabyte transfers. ByteLength follows Go's signed size domain. JSON UnmarshalJSON accepts a caller-owned complete byte slice: removing quotas does not make that API O(1)-memory document ingestion. Narrow field grammars and inherited provider contracts remain owned by their existing types.
