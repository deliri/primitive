Review follow-up: see [the resolved findings and current evidence](distribution_review_20260909.md). The original report below is retained as historical evidence for its original source.

# Distribution upgrade — ready for review — 2026-09-09

Base: **v2026.1.34**, commit **0c20d585dcf1912af14e8eab44811542761ce92a**.
The ID checkpoint was committed and pushed before this slice. Distribution and
the necessary Deploy handoff repair are uncommitted and await user review.

Furnace: Go 1.27.1 linux/amd64, AMD EPYC 7282, GOWORK=off, GOMAXPROCS=8.
The complete 2,214-line project-local testing protocol was read before editing
tests; its SHA-256 remains
dd83cd7f62c172092546dab6b7c7d5c59753b5e8ae784631a94cff4d6d48126c.
This is author-produced execution evidence, not an independent Anvil acceptance
receipt or a guarantee that no defect remains.

## Production changes and ownership

The previous Distribution before/after notes covered the bulk heap-reservation
audit and one enum benchmark; they did not record a complete package upgrade.

- **Bind upload evidence to the signed grant.** Completion verification previously
  accepted evidence for identical content uploaded under a different granted
  capability. Every one of the eight evidence slots must now carry the exact
  corresponding grant commitment. A separately signed replacement grant with a
  different destination cannot reuse the original completion.
- **Preserve that commitment through the actual producer.** Deploy previously
  unpacked an UploadCapability and called Objectstore.UploadGCS with a raw target.
  That dropped the commitment from TransferEvidence. It now calls the existing
  Objectstore.Upload capability API. Receipt progress, provider, content and
  transport ownership remain with their existing packages. Deploy's complete
  package sweep remains in the saved queue.
- **Reject absent interface values at admission.** All seven canonical writers
  use Core.WriterIsNil; publication sources use Core.ReaderIsNil. Typed nils
  cannot pass validation and panic or reach the effect layer. Short, negative
  and impossible writer counts preserve io.ErrShortWrite; returned provider
  errors preserve their native identity.
- **Close request commitments.** RequestCommitment.Validate accepts only the
  three request domains. CommitRequest has a compiler-visible RequestPayload
  constraint containing PublicationRequestPayload, UpdateRequestPayload and
  UpgradeRequestPayload. Arbitrary canonical callbacks, response bodies and
  typed-nil pointer bodies cannot enter this request boundary.
- **Bind staging roots before projection.** UpgradeStageRequest.Validate delegates
  root/directory identity to Filestore. Absent, closed and foreign roots fail
  before staging. PrepareUpgradeStage returns a zero StageRequest with the
  Distribution identity if downstream validation refuses it.
- **Bound the domain decoder before decoding.** SigningDomainJSONMaximumBytes
  derives from the longest closed canonical token. Oversized input is refused
  before Core decodes or copies it. Core still owns JSON grammar. Once a valid
  closed token is decoded, direct canonical byte checks replace redundant JSON
  re-encoding. SigningDomain parsing now constructs its fixed token table once,
  instead of repeatedly validating and reconstructing it while searching.

The existing payload ceilings are now public compiler-owned constants:
RequestPayloadJSONMaximumBytes (96 KiB), ResponsePayloadJSONMaximumBytes and
PublicationCompletionPayloadJSONMaximumBytes (128 KiB). Request documents remain
128 KiB and response/grant documents 256 KiB. The duplicate private grant limit
was removed; no compatibility alias was added.

No production structs, registries, mutable product state, goroutines, worker
pools, caches, dependencies or competing OS/network runtime were introduced.
The new interface is a closed generic type constraint, not a runtime dispatch
layer. Go's JSON, SHA-256 and Ed25519 implementations remain authoritative through
Core/Attest. Exchange, Objectstore and Filestore perform their owned effects;
Temporal supplies the existing observation contracts. Canonical documents remain
bounded materializations, and hashing streams those bounded bytes into Core's
digest writer. This is not a claim that JSON encoding is zero-allocation or
constant-time in admitted document size.

## Contract and migration impact

Call CommitRequest with one of the three concrete request payload values.
Interface-typed custom canonical bodies and response payloads are deliberately
no longer accepted; there is no alternate or compatibility entry point.
A compiler probe rejects UpdateResponsePayload, while all three request types
compile. Existing repository call sites exercised by Distribution and Deploy
compile unchanged.

Completion verification now refuses historical transfer evidence without its
upload-capability commitment. Producers need the repaired Deploy handoff (or
Objectstore's capability API) to produce evidence that closes the grant.
No persisted-data rewrite, product policy, or consumer version update is performed
in this slice. Consumers remain responsible for their own release rollout.

## Boundary inventory and hostile proof

| Boundary | Local proof |
| --- | --- |
| Closed signing domains | All 256 underlying enum values; all seven canonical tokens; wrong case, escaping, whitespace, null and trailing documents; direct owner behavior distinguished from Go JSON framing |
| Request commitments | Three accepted request domains; all response/unknown domains refused; unset/all-zero digest refused; independent SHA-256 frame oracle; compiler negative and positive probes |
| All 16 JSON owners | Typed ingress inventory paired with 16 external fuzz targets; strict field presence, duplicate/unknown fields, neutral null, wrong kinds, malformed and trailing documents; fresh and populated receivers; nil receivers |
| Document bounds | Below/at/above each public byte ceiling using actual bounded documents padded with whitespace; fixed-size array cardinality one short/one long |
| JSON facts | Canonical accepted facts compared independently using Go jsontext; raw integer tokens are preserved, so large integer differences cannot disappear through float64 conversion |
| Seven canonical writers | Exact full output and call count; zero/short/negative/overshoot counts; native errors before/after output; nil and typed-nil destinations |
| Issuance | All seven issuers reject absent/refusing signers with zero output and exact native identity/call count; valid issuance is exercised through typed fixtures and verifier paths |
| Publication plan | Exact grant/manifest and every required source; no source read during preflight; absent grant, foreign manifest and each missing source refuse before effects |
| Deploy to completion | Full eight-object path plus every incomplete prefix; exact request/receipt counts; retained evidence matches each capability; absent plan has no effect; every foreign grant slot refuses with zero proof |
| Update/upgrade | Exact signed request/build/nonce, signer and signature substitutions, trust refusal, lifetime edges, candidate/installed manifest relationships; positive/negative/absent response triad |
| Upgrade stage | Owned root succeeds; foreign, closed and absent roots refuse with zero stage and no transport call; every subtest owns its temporary root |
| Architecture | Compiler-embedded production inventory, no source reads from a mutable checkout; ingress scanner has synthetic positive/negative/non-ingress fixtures |
| Refusal allocation | Serial Core/testserial runtime-allocation declarations; oversized 1 KiB and 1 MiB domain JSON stays within six allocations |

Seven deliberate production mutations compiled and caused behavioral test
failures: capability binding, typed-nil writer, typed-nil reader, predecode bound,
request-only domain validation, strict document byte ceiling, and commitment
frame separator. Every mutation was restored. The original Deploy path separately
failed the new capability evidence table before repair. Failure event totals
include parent tests and are not advertised as separate bugs.

The injected HTTP transport exercises real Distribution → Deploy → Objectstore →
Exchange code, but its provider response is local test data, not a live GCS
receipt. Deploy's existing loopback transport tests also run in the scoped suite.
No credentials or live provider service are required or claimed.

## Attempts retained

The manifest includes all original, red, intermediate, mutation and final runs.
Initial benchmark/test builds used invalid comparisons on non-comparable verified
grant structs; the corrected benchmark observes exact request projections and
valid proof. One JSON pressure test draft misused a Go jsontext.Token after the
decoder advanced; copying its name before ReadValue fixed the test harness.
One test build incorrectly passed both results of Temporal.Instant.Time;
it was corrected before the final checks.

The first candidate completion run exposed the necessary Deploy producer repair.
Those failing runs remain as evidence rather than being folded into success.
Witness found a missing parent benchmark ReportAllocs call; the parent now declares
it outside timed work. No build failure or harness panic is counted as a production
mutation kill.

## Validation

Original Distribution race run: 309 passing events (including parent/package
events), 32 top-level functions, no failures/skips, 80.1% statement coverage.
Final Distribution race run: **1,230 passing events**, **63 top-level functions**,
no failures/skips, **89.9% statement coverage**. The separately scoped Deploy run
has 51 passing events, 13 top-level functions, no failures/skips and 87.0% coverage.

Final go fix -diff, go vet, Staticcheck, Errcheck, Witness and production
gocyclo <=10 pass on the declared slice. go fix emitted no suggested changes.
Distribution and Deploy test binaries compile for macOS arm64 and Windows amd64;
these binaries were not run on those operating systems. Exact commands, tool
binary hashes/build information, filters and all run source manifests are retained.

## Benchmarks and profiles

Both baseline and candidate use twelve checked workloads, one pass each, serially:

    go test -run=^$ -bench=. -benchmem -benchtime=30s -count=1 -timeout=20m
      -cpuprofile=<phase>/cpu.pprof -memprofile=<phase>/mem.pprof
      -o <phase>/distribution.test ./distribution

The timed benchmark source is identical. Removing only the candidate's untimed
parent ReportAllocs declaration reconstructs the baseline benchmark file byte for
byte. Fixture constructors were changed to use Objectstore's owned typed
constructors outside timing. The completion fixture now contains the previously
missing eight capability commitments: its wire payload and validation work have
intentionally changed, so its latency is not presented as a code-only comparison.

| Workload | Baseline ns/op | Candidate ns/op | B/op, before → after | Allocs/op, before → after | Effective seconds, before → after |
| --- | ---: | ---: | ---: | ---: | ---: |
| BenchmarkParseSigningDomain | 63.03 | 14.83 | 0 → 0 | 0 → 0 | 36.007 → 14.830 |
| BenchmarkCommitPublicationRequest | 1,615,162.00 | 1,640,097.00 | 204,528 → 202,115 | 3557 → 3555 | 35.755 → 35.830 |
| BenchmarkVerifyPublicationRequest | 5,698,247.00 | 5,810,280.00 | 679,832 → 679,765 | 12402 → 12402 | 35.027 → 35.605 |
| BenchmarkVerifyPublicationGrant | 4,524,219.00 | 4,544,515.00 | 582,718 → 578,105 | 9861 → 9859 | 35.343 → 35.375 |
| BenchmarkVerifyUpdateRequest | 87,768.00 | 87,668.00 | 2,210 → 2,210 | 50 → 50 | 36.995 → 33.997 |
| BenchmarkVerifyUpdateResponse | 14,227,690.00 | 14,665,406.00 | 1,814,271 → 1,814,013 | 32602 → 32599 | 34.787 → 35.461 |
| BenchmarkVerifyUpgradeRequest | 295,311.00 | 295,412.00 | 27,694 → 27,689 | 602 → 602 | 33.647 → 34.743 |
| BenchmarkVerifyUpgradeGrant | 436,763.00 | 446,576.00 | 50,011 → 49,121 | 942 → 940 | 35.062 → 35.721 |
| BenchmarkVerifyPublicationCompletion | 2,973,593.00 | 3,054,354.00 | 401,046 → 403,993 | 7363 → 7434 | 35.867 → 30.544 |
| BenchmarkUpdateResponseJSON/encode | 4,334,702.00 | 4,365,372.00 | 690,627 → 690,730 | 10875 → 10875 | 33.360 → 32.797 |
| BenchmarkUpdateResponseJSON/decode | 4,901,268.00 | 4,916,574.00 | 758,457 → 758,202 | 14630 → 14626 | 34.363 → 34.878 |
| BenchmarkSigningDomainJSONRefusal | 410,732.00 | 596.70 | 262,533 → 208 | 11 → 6 | 36.553 → 35.304 |

Process wall: 424.402s baseline, 396.701s candidate.
Wall/30s configuration ratios: 14.15× and 13.22×.
Wall/360s nominal phase budget: 1.18× and 1.10×.


The nominal phase budget is 360 seconds (twelve 30-second configurations).
Effective timed durations are estimated as iterations × reported ns/op and are
recorded per row. Go may stop a fast duration-configured benchmark at its iteration
ceiling; a 30s flag is not proof of thirty effective seconds for that row.

Furnace was shared, without isolated affinity or a controlled power posture.
Candidate host observations and tool identities are retained. No baseline-start
or continuous governor parity was captured. One sample per workload and revision
is not a distribution, trend, overall speedup, or statistical performance
acceptance. All samples, including slower ones, remain visible.

The profiled changes are narrow:

- ParseSigningDomain: **63.03 → 14.83 ns/op**, still **0 B/op and 0 allocs/op**.
  The original CPU profile attributes 16.94s flat to SigningDomain.Validate
  and 31.16s cumulative to SigningDomain.String. Both disappear from the
  candidate's top-30 list after removing the repeated validation/table work.
  This does not mean those methods consumed zero CPU.
- A 256 KiB invalid domain JSON token: **410,732 → 596.7 ns/op**,
  **262,533 → 208 B/op**, **11 → 6 allocations/op**.
  Refusal now precedes token decoding; the remaining six allocations construct
  typed error context. Independent allocation tests cover 1 KiB and 1 MiB inputs.
- The aggregate baseline alloc_space profile shows 21.77GB in JSON string
  allocation (36.21% of 60.12GB sampled allocation). That node drops out of the
  candidate top-30 list. Candidate sampled allocation totals 49.12GB; bytes.Clone
  accounts for 13.88GB (28.25%), while errors.Join accounts for 7.09GB (14.44%).
  The now-fast refusal benchmark performs 59,165,154 iterations, making error
  construction more prominent in the aggregate profile.
- Release artifactDigest and manifestFactDigest remain large cumulative
  allocation sites: candidate 16.41GB (33.41%) and 17.92GB (36.49%).
  Candidate CPU still contains Go JSON machinery, Ed25519 field multiplication
  (23.57s flat, 5.39%) and SHA-256. These are measured costs of existing canonical
  and cryptographic checks, not an excuse to bypass them.

The full-document timings are mixed. For example, VerifyUpdateResponse records
14,227,690 → 14,665,406 ns/op while allocation counts fall from 32,602 to 32,599.
VerifyPublicationCompletion records 2,973,593 → 3,054,354 ns/op and 7,363 → 7,434
allocations because the fixture now carries and verifies its capability evidence.
Both observations are retained without claiming that all operations got faster.

The candidate domain parse reached Go testing's 1,000,000,000-iteration prediction
ceiling, yielding about **14.83 effective timed seconds despite -benchtime=30s**.
That row is explicitly exploratory timing evidence, not a 30-effective-second
acceptance sample. The installed Go testing/benchmark.go is retained as
stdlib-benchmark.go, including maxBenchPredictIters and predictN. Other rows'
effective durations and sample counts remain in the table and machine manifest.


Aggregate alloc_space profiles describe cumulative sampled allocation, not peak
RSS or retained heap. Faster workloads can execute more operations in the same
time budget and accumulate more total allocation. Aggregate percentages cannot
explain every individual benchmark delta. No nested Release validation or
cryptographic check was removed to improve these numbers.

## Semantic fuzz campaigns

Seventeen targets run serially after scoped checks and benchmarks: sixteen
external decoder owners plus the independent request-commitment frame oracle.
Each uses -run=^$, an exact target filter, -fuzztime=30s,
-fuzzminimizetime=1s, -parallel=4 and -timeout=2m. Persistent Go fuzz caches remain
enabled; baseline gathering/progress output is retained. Normal test checks use
-count=1. Fuzz runs intentionally filter out ordinary tests.

| Target | Executions | Process wall |
| --- | ---: | ---: |
| FuzzPublicationCompletionDocumentExternalDecoder | 311,827 | 37.674s |
| FuzzPublicationCompletionPayloadExternalDecoder | 286,821 | 31.178s |
| FuzzPublicationGrantDocumentExternalDecoder | 78,541 | 31.889s |
| FuzzPublicationGrantPayloadExternalDecoder | 393,743 | 30.986s |
| FuzzPublicationRequestDocumentExternalDecoder | 385,115 | 31.816s |
| FuzzPublicationRequestPayloadExternalDecoder | 226,104 | 31.002s |
| FuzzRequestCommitmentCanonicalFrame | 376,111 | 31.654s |
| FuzzRequestCommitmentExternalDecoder | 567,769 | 31.283s |
| FuzzSigningDomainExternalDecoders | 682,757 | 30.848s |
| FuzzUpdateRequestDocumentExternalDecoder | 403,242 | 31.506s |
| FuzzUpdateRequestPayloadExternalDecoder | 538,516 | 30.930s |
| FuzzUpdateResponseDocumentExternalDecoder | 134,406 | 31.598s |
| FuzzUpdateResponsePayloadExternalDecoder | 276,940 | 31.432s |
| FuzzUpgradeGrantDocumentExternalDecoder | 247,054 | 31.457s |
| FuzzUpgradeGrantPayloadExternalDecoder | 527,131 | 31.672s |
| FuzzUpgradeRequestDocumentExternalDecoder | 296,114 | 31.803s |
| FuzzUpgradeRequestPayloadExternalDecoder | 256,151 | 31.326s |

All 17 targets passed, totaling 5,988,342 executions.
Nominal budget: 510s; process wall: 540.054s.
Wall/per-target flag: 18.00×; wall/phase budget: 1.06×.
No failing campaign or newly generated failing corpus was observed.


## Evidence and review

Go/module-input digest (all tracked/untracked Go files plus go.mod and go.sum,
sorted path/hash JSON): **5fd8df5face57c548747bf28fc09af7937cd5d700e7c0197ee81728aa054bd23**.
Final scoped checks, candidate benchmarks and all fuzz campaigns match this digest.
The containing Git HEAD is the published ID base; the candidate is a dirty source
snapshot, not a claimed committed revision.

- [Machine evidence](distribution_upgrade_20260909_evidence.json) records every
  command, result, environment, source binding, benchmark sample and artifact hash.
- Raw root: /home/d/engineering-evidence/primitive/distribution-upgrade-20260909.
- 4,307 distinct content-addressed source snapshots were rehashed.
  Recorded run artifacts and original files were also checked.
- Large binaries, profiles, source snapshots, logs and historical attempts remain
  outside Git. Small review reports and the manifest are the commit surface.
- The manifest builder verifies content integrity, not independent acceptance.

Distribution is ready for user review. Its changes and the small Deploy handoff
repair remain uncommitted. Lineio is next after approval; Deploy's full package
sweep remains queued. Full-module gates, live-provider tests, consumer migrations,
and native macOS/Windows execution were not run.
