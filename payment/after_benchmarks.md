# Payment upgrade — 2026-09-10

Review slice on Furnace; not committed. Release base: `727ad6e807d4131b429545bc3d35f78e8624fdd5` (`v2026.1.43`).
Exact modified sources, complete commands, outputs, toolchain/environment, source patches and content-addressed snapshots are retained under:
`/home/d/engineering-evidence/primitive/payment-upgrade-20260910`.
The evidence manifest is `manifest.json` in that directory. Raw binaries/profiles remain outside Git.

## Production changes

- All six Payment structured JSON decoders use Core's strict extensible document contract. Deleted the four Payment byte-quota constants and their encode/decode checks.
- The real dependent PaymentAuth request decoder referenced the removed query quota. Its corresponding quota and encode/decode checks are removed too; this is a necessary request-boundary correction, not a complete PaymentAuth upgrade.
- All three canonical writers refuse typed-nil destinations through Core and preserve native writer errors and short-write identity.
- Catalog issuance validates the payload and copies its entries before calling the standard `crypto.Signer` capability. Mutations through caller storage during `Public`, during `Sign`, or after return cannot alter the returned signed facts.
- Canonical JSON and cryptography still belong to Core/encoding/json/v2 and Attest/crypto. No product policy, runtime, state machine, custom JSON grammar, or transport was added.

## Resource contract

The existing Core-owned catalog page window and caller-requested limit remain: they bound an individual page, not the complete catalog. Continuation has no total catalog-count ceiling.
These JSON APIs accept caller-owned complete byte slices; allocation scales with the current document. They are **not O(1)-memory stream decoders**, and removing byte quotas does not change that API fact.
The returned catalog owns one page's entries; copies have a measured cost. Fixed identity/domain grammar, strict fields, ordering, scope, query commitments, signing and requested-page validation remain in force.

## Hostile evidence and layer coverage

Before production edits, `hostile-red` failed for all three writer panics, the six JSON quota paths, the dependent PaymentAuth quota, and all three catalog aliasing paths. `hostile-green` then passed.

- Schema/decoding: each structured door receives canonical data, large valid prefix/suffix/interior whitespace, malformed data after that whitespace, trailing objects, and no-value input. Rejection preserves exact receivers. All ten public JSON receivers also reject nil receivers and preserve unset values without emitting documents.
- Canonical output: each writer checks full output, zero progress, partial progress, every strict prefix with a native failure, full output accompanied by failure, nil/typed-nil writers, and invalid payloads that perform no writes.
- Issuance: exact signed output, caller ownership, signer callback mutation, native signer failure, typed-nil signing capability, and invalid payloads that perform no signing.
- Verification/projection: authenticated specific selections reject wrong identities, multiple receipts and continuation; empty selection returns an authenticated empty page. Input and accessor mutations cannot alter retained verified facts.
- Service periods: absent, reversed and zero-duration intervals return no bounds; positive, pre-epoch and cross-epoch intervals preserve exact typed endpoints.
- Existing source/fuzz inventories remain compiler checked; ingress source inspection now uses the already-owned embedded sources.
- Removed inert/padded rows, made reordered fixtures actually reorder members, and strengthened query constructor checks to compare every returned fact. Changed the over-window catalog case to use otherwise-valid ordered receipts instead of a zero-filled slice.
- Generic fuzz receivers now use compile-time pointer constraints. Known valid and generated valid probes reject always-refuse implementations; signed seed verification may not silently fail. UUID text uses the owning ID parser as oracle and signing domains use the explicit closed token set.
- Nine compiled mutations were killed: byte quota, typed nil, signing ownership, short write, native error identity, verification ownership, specific selection, reject-all JSON, and dependent caller quota. Exact commands and failures are in `mutation-results.json` and each run directory.

The untouched downstream transport, storage, billing policy and PaymentAuth response paths are outside this slice; no new claims are made about them. Payment catalog authentication proves the authority-signed page; it does not add a new independent trust policy for each embedded receipt.

## Validation

Scoped `go test -race -count=1` for Payment and PaymentAuth, `go vet`, `go fix -diff`, staticcheck, errcheck and witness-lint pass. The fix diff is empty. Full production `go build ./...` passes. Touched production is at gocyclo ten or below.
macOS/arm64 and Windows/amd64 test binaries compile for both packages; runtime testing was Linux/amd64 only.
Full-module tests and broader global gates were deferred, as requested.

The first Witness run found missing allocation reporting on parent benchmark functions and insufficient failure context. These were fixed and Witness rerun successfully. Both the failed run and successful correction remain in evidence. The benchmark corrections affect parent setup/failure diagnostics, not timed workloads. The query fixture now explicitly retains its existing signing key for failure tests; fixture creation remains outside timing.

## Benchmarks

The first correct candidate repeated payload validation after the ownership copy. Its allocation profile attributed 117.01 MB of cumulative sampled allocation to the extra issuance validation path, including repeated entry validation and identity formatting. This is cumulative allocation across the run, not retained heap. The final implementation keeps validation before copying and delegates signer validation directly to Attest, removing that duplicate pass. The complete scoped checks, platform compilation, benchmarks and fuzz targets were repeated on the final source.

The initial suite measured eight substantive workloads plus the original tiny domain parser. The domain parser hit Go's one-billion-iteration ceiling after about ten timed seconds despite `-benchtime=30s`; that result is retained as exploratory and is not acceptance evidence.
A distinct fixed batch of sixteen domain parses and one/full-window catalog issuance workloads were then captured before production changes. The final optimized phases use the matching eight main workloads and these three additional workloads. The earlier after phases are retained as a separate intermediate candidate, not silently overwritten. Each accepted case is configured for thirty seconds; effective timed durations estimated from the displayed iteration count and rounded ns/op are in `benchmark-comparison.json`. The batch reports sixteen parses/op; its ns/op is per batch, not per parse.

CPU and memory profiles and the exact test binary were passed explicitly and retained for every benchmark phase. The original main profile includes the short exploratory parser; aggregate CPU percentages across differing phase mixes are not a direct performance comparison. Only matching benchmark workloads appear below.
The shared host and uncontrolled power posture limit timing conclusions. These are single-pass measurements, not distributions or proof of a speed improvement. Byte and allocation counts, absolute timings, and slower results are all retained.


| Workload | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
|---|---:|---:|---:|---:|---:|---:|
| PaymentIssue | 186182.0 | 187777.0 | 2914 | 2914 | 63 | 63 |
| PaymentVerify | 103457.0 | 98747.0 | 2068 | 2068 | 54 | 54 |
| PaymentCommitQuery | 27183.0 | 27269.0 | 2467 | 2467 | 60 | 60 |
| PaymentCatalogVerify/one | 204239.0 | 232011.0 | 13241 | 13242 | 220 | 220 |
| PaymentCatalogVerify/full_window | 4671743.0 | 4642315.0 | 881580 | 881544 | 8267 | 8267 |
| PaymentCatalogDecode | 10836813.0 | 10563872.0 | 1336890 | 1336467 | 29343 | 29339 |
| PaymentCatalogCanonical | 4090125.0 | 4027109.0 | 823524 | 823302 | 7587 | 7587 |
| ParsePaymentID | 146.4 | 144.9 | 0 | 0 | 0 | 0 |
| PaymentCatalogIssue/one | 294943.0 | 300757.0 | 11088 | 10905 | 168 | 165 |
| PaymentCatalogIssue/full_window | 4180539.0 | 4910658.0 | 849589 | 877978 | 8221 | 8218 |
| ParseSigningDomainBatch | 130.4 | 119.2 | 0 | 0 | 0 | 0 |

## Fuzz

Five unique targets ran serially after benchmarks, each with a configured thirty-second budget and four workers: four Payment targets plus the affected credentialed PaymentAuth request target. Both the initial correct candidate and final simplified candidate were exercised: ten runs, 300 configured fuzz seconds. Total reported executions: 6,053,179. Targets, actual durations and counts are in `phase-plan.json`, `optimized-phase-plan.json`, both fuzz-results files and `fuzz-summary.json`. Default Go fuzz cache state was retained; runs are not claimed to start with an empty cache. Normal tests used count=1. Mutation runs, benchmark test filters and cross-compilation remain explicitly distinct evidence.

## Source and evidence binding

Every executed run records its base commit, full modified-source hashes, exact arguments, environment, elapsed time, exit status, stdout/stderr and whether sources stayed unchanged. Reports and final manifests are written after execution; they do not re-label historical evidence as a committed Payment release. The release remains subject to user review.

---

## Historical notes retained

# payment after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./payment`

```
BenchmarkParseSigningDomain-10    	211384524	         5.616 ns/op	       0 B/op	       0 allocs/op
BenchmarkParsePaymentID-10        	10098781	       115.3 ns/op	      48 B/op	       1 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
