> Release checkpoint: the user reviewed these changes and explicitly approved
> **v2026.1.31**, commit, and push. See [release notes](release_v2026.1.31.md).
> The uncommitted status and source coordinates below describe the historical
> validation runs; the release evidence binds the unchanged reviewed package.

# Controlplane review follow-up — 2026-09-09

The review identified two real production gaps. The initial report's writer
identity claim was too broad: its test checked only `ErrControlPlaneContract`.
The issuance fix also left received check-in documents structurally open to a
foreign device signer. Both gaps are now fixed; the stronger fuzz oracle also
exposed a missing check-in wrapper on the existing domain-consistency refusal.
This candidate remains uncommitted and awaits user review.

## Exact changes

- `writeCanonical` now returns mechanical failures: the existing Core contract
  error for a nil destination, the original writer error, or `io.ErrShortWrite`.
  It no longer adds registration identity. The two registration writers add
  `ErrControlPlaneRegistration`; check-in payloads add `ErrControlPlaneCheckIn`;
  check-in responses add `ErrControlPlaneCheckInResponse`; response commitments
  add `ErrControlPlaneResponseDocument`. Existing lower-level causes remain
  discoverable. The helper does not strip identities supplied by a caller's
  writer; it simply stops manufacturing registration identity.
- `CheckInRequest.Validate` now compares the admitted envelope signer with the
  certificate's device key after validating their structure. A mismatch returns
  check-in plus installation-binding identity. All public admission/emission
  paths use that rule: `Validate`, `UnmarshalJSON`, `MarshalJSON`, issuance, and
  verification. The issuance-only copy of the comparison was removed. Issuance
  still returns a zero request on refusal.
- The shared document-validation helper can return a consistency error for a
  mismatched signing domain. `CheckInRequest.Validate` now wraps that error with
  check-in identity instead of exposing only the lower-level refusal. Signature
  authentication remains Attest's job: matching signer identity with an invalid
  signature is structurally admissible and still fails real verification.

No public method signatures, wire fields, dependencies, or runtime machinery
were added. These are local mechanical checks and error ownership corrections.

## Hostile proof

The full 2,214-line local testing protocol was reread before edits. Its content
hash remains the one recorded in the initial evidence manifest.

`TestCanonicalWriterLayerTriadPreservesExactEffects` now tests the four exact
writer-owner error identities for every non-successful I/O row, requiring the
owning identity and excluding the other three. It still checks exact offered
canonical bytes, call count, nil-destination refusal, zero effects on invalid
input, all short/invalid counts, and preserved cancellation or writer failure.

`TestReceivedCheckInSignerBindingLayerTriad` covers a correctly certified
signature, an independently valid foreign signature, a signer-only mutation,
a corrupted signature with the correct signer, a foreign signing domain, and
an absent attestation. It exercises direct validation, projection, fresh and
populated receiver decoding, and real verification. Refused receivers remain
zero or preserve their exact original canonical document; verification refuses
without exposing a request through a proof.

The check-in fuzzer no longer asks the production marshaler to emit a document
whose signer the current contract refuses. It replaces the exact typed envelope
inside a genuinely emitted valid request, and includes both a signer-only
mutation and a real signature from a different key. The callback explicitly
checks signer/certificate equality for every admitted value, exact check-in and
JSON error identities for rejection, receiver preservation, canonical stability,
and the existing independent authentication and usage-commit oracles.

`review2-red-boundaries` fails against the pre-review production paths for both
reported bugs. `review2-green-boundaries` passes after their correction. The
subsequent `review2-race` fails on the newly required check-in identity for an
existing signing-domain seed; that refusal is now wrapped at its owner. These
are production regression observations. The initial `review2-red` compile
failure, the `review2-package` obsolete seed-construction failure, and the
`review2-witness` diagnostic-context finding are retained separately as test or
tooling development failures. None is counted as a production bug.

## Suggestions considered

**Rehash on every `VerifiedResponse.Body()` call:** not added. The owning
`ResponseDocument` validates the exact body length and SHA-256 commitment before
verification. Its retained body bytes are private, independently allocated, and
never exposed directly. Every consumer decoder receives a clone. Replacing a
source document assigns a new buffer; it does not modify a buffer already held
by a proof. Existing hostile tests mutate returned nested bodies and use a
consumer decoder that erases its input, then recheck original and sibling
proofs. There is no public retained-byte mutation path requiring another hash
on each accessor call. This decision depends on preserving that ownership
boundary, not on treating mutable data as authenticated.

**Checking the private key before signing:** not added. Attest owns complete
private-key admission, including key length and consistency between seed and
public half. Its current signing API returns the validated signer identity in
the envelope. Controlplane now applies the one request-owned binding rule to
that result. A separate raw-public-half precheck could misclassify a malformed
private key as an installation mismatch; duplicating full validation would add
work and a second owner. An earlier check is a possible measured optimization
through an explicitly designed Attest capability, not a correctness fix needed
for this release. No such extra capability was introduced here.

## Evidence scope

The base commit remains `b25bfdc8b82c0c47425927b6055762dd11ea2ac4`. Per-run source
snapshots identify the actual dirty candidate. The new follow-up manifest
supersedes the initial evidence for the changed source; the original runs remain
historical. No commit, version bump, push, full-module gate, or consumer upgrade
was performed. Linux runs execute on Furnace; macOS arm64 and Windows amd64 are
compilation checks only.

The fresh benchmark pass keeps the same ten workload identities and timed
operations, with one 30-second configuration per case, allocation reporting,
CPU/memory profiles, and the matching binary. This is a new candidate measurement,
not a retry selected for faster numbers. The original baseline and first
candidate samples remain visible. Shared-host timing and the enum parser's
one-billion-iteration cap retain their previously documented limitations.

The full package race run executes all test and fuzz-seed functions. Only
check-in request ingress semantics changed in this review, so the fresh mutation
campaign is `FuzzCheckInRequestDecodeAndVerify`, 30 seconds, four workers, one
second per minimization. The earlier 21-target campaigns remain historical;
they are not relabeled as executions on the new source. Writer identity changes
are covered through the full public writer table and real signed paths.

## Final follow-up results

The uncached full-package race run passed, with zero failures or skips. Go vet, Staticcheck, Errcheck, Witness lint, production complexity at or below ten, and macOS/Windows compilation passed. The changed check-in fuzz campaign passed: `fuzz: elapsed: 30s, execs: 128787 (0/sec), new interesting: 111 (total: 298)`. Exact run commands, source hashes, all attempt outcomes, profile/binary hashes, and limits are retained in [the follow-up manifest](controlplane_upgrade_20260909_review_followup.json).

| Workload | Original ns/op | First candidate ns/op | Current ns/op | Current B/op | Current allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| ParseProductStatus | 12.76 | 12.7 | 12.59 | 0 | 0 |
| RegistrationCodec/marshal | 160232 | 191825 | 192381 | 26898 | 329 |
| RegistrationCodec/unmarshal | 372431 | 458655 | 453718 | 64329 | 1091 |
| VerifyRegistration | 446004 | 530609 | 561699 | 29814 | 502 |
| VerifyCheckIn | 206422 | 237861 | 233524 | 9934 | 195 |
| CommitCheckIn | 26539 | 38161 | 38049 | 3139 | 87 |
| AuthenticatedResponse/verify | 204166 | 194047 | 193626 | 6712 | 122 |
| AuthenticatedResponse/body | 8578 | 453090 | 444651 | 69132 | 1118 |
| AdvanceUsageWatermark/one_class | 5427 | 7811 | 7442 | 912 | 24 |
| AdvanceUsageWatermark/maximum_classes | 29161 | 35386 | 35049 | 2817 | 86 |

The fresh benchmark pass took 332.60s wall time (11.09× the per-case 30s flag, explicitly above the protocol’s 10× reporting threshold). The ten-case nominal budget was 300s. The tiny parser again hit Go’s iteration ceiling; its shorter result is informative only. CPU, cumulative-allocation, and in-use sampled-memory summaries use the exact retained current binary and profiles. No baseline or first-candidate sample was discarded.

Both reviewer-reported bugs and the additional missing check-in error context are closed on the current measured source. The two suggestions remain intentionally unchanged for the ownership reasons above. Review is still required before versioning or committing.

The current CPU profile remains dominated by standard-library Ed25519 field multiplication (8.29% flat) and Go JSON struct decoding (32.98% cumulative). Sampled allocation space attributes 24.09% flat to `bytes.Clone`. All ten measured allocation counts match the first candidate; the prior body-extraction correctness cost remains. These are aggregate profiles and single workload observations, not peak-memory measurements or statistical speedup claims.

---

## Historical initial review candidate

The following original note and measurements predate the review fixes above. Its writer identity claim and issuance-only signer closure were incomplete, as corrected above.

# Controlplane upgrade — review candidate, 2026-09-09

This slice closes authenticated-response ownership, canonical-writer completion,
registration-secret cleanup, check-in issuance binding, response commitment
bounds, and an impossible conflict disposition. It also replaces padded tests
with boundary and transition proofs and expands measured workloads from one enum
parser to ten cases. No version bump, commit, or push is included in this slice.

## Source and ownership

Base: `b25bfdc8b82c0c47425927b6055762dd11ea2ac4` (v2026.1.29). The candidate is
an uncommitted working tree on Furnace (`d@192.168.1.81`,
`/home/d/code/primitive`). The evidence manifest binds each run to its complete
source inventory and preserves content-addressed source snapshots. A base SHA
alone is not proof of the modified code. Reports are written after measurements;
the final package-source digest identifies the measured Go files.

Controlplane continues to perform typed document, signature, binding, and
watermark mechanics. It does not choose product policy, persist usage, retry
requests, or build a mutable workflow engine. Attest owns signature mechanics;
Core owns strict Go JSON and stable errors. Existing public types, method
signatures, and wire shapes remain. Private generic response storage now retains
bounded authenticated bytes instead of a mutable product graph. Each `Body()`
call decodes an independent typed result with the consumer's existing decoder.
That is a correctness cost, explicitly measured below. This is bounded document
processing: time and retained bytes grow with document size up to the existing
Core limit; it is not O(1) time or a whole-stream O(1)-memory claim.

## Production defects and proofs

| Boundary | Original failure | Current behavior | Regression proof |
| --- | --- | --- | --- |
| Five canonical writers | A writer could return fewer bytes and no error, and Primitive reported success. The response-commitment writer also panicked on a nil destination and lost package error identity. | Exact write count is required; short counts preserve `io.ErrShortWrite`; underlying cancellation/errors remain discoverable. Nil destinations refuse. | `writer_boundary_test.go`, `red-boundaries` |
| Registration authority | A zero authority returned before destroying the supplied registration token. | Cleanup is owned before validating the authority; cleanup failure joins the primary error and prevents a proof escaping. | `issuance_boundary_test.go`, `red-boundaries` |
| Check-in issuance | A foreign signer could issue under another device certificate. A structurally refused request could escape beside its error. | The actual signer must match the certificate device key; failed construction returns a zero request. | `issuance_boundary_test.go`, `red-boundaries` |
| Check-in conflict response | A signed conflict could retain the exact predecessor, although the producer would accept that predecessor. | Conflict refuses both predecessor and exact successor; a distinct current watermark remains a conflict. | `conflict_boundary_test.go`, `red-boundaries` |
| Generic verified response | Mutating a returned nested certificate changed authenticated facts or invalidated sibling proofs. A consumer decoder could overwrite the bytes later used to form the commitment. | Retained wire bytes never reach the consumer decoder directly; each access returns independently decoded data. | `response_ownership_test.go`, `decoder_ownership_test.go`, `red-boundaries`, `red-original-decoder` |
| Response commitment | An impossible body extent and a bodyless commitment with a nonempty digest validated. | The existing document ceiling and exact empty-body SHA-256 apply at the owning type. | `commitment_boundary_test.go`, `red-commitment` |

Failed leaf-row counts are not independent bug counts. The original body-mutation
row was renamed because its altered build was not actually the same encoded
length; the ownership defect did not depend on equal length. Earlier decoder
fixture failures and compile failures are retained as unsuccessful development
attempts, not counted as production regressions. The corrected
`red-original-decoder` fails at the intended receiving `UnmarshalJSON` call
against the original production file; the corrected candidate passes.

## Protocol application

The entire 2,214-line project-local `_docs/testing_protocol.md` was read before
test edits. Protocol SHA-256:
`dd83cd7f62c172092546dab6b7c7d5c59753b5e8ae784631a94cff4d6d48126c`.

- Canonical writer tables prove exact attempted bytes, call count, typed refusal,
  and zero effects on invalid input, across all five canonical body owners.
- Thirteen document decoders receive valid canonical documents padded with legal
  JSON whitespace to ceiling minus one, ceiling, and ceiling plus one. The first
  two must succeed; the last must refuse without replacing a populated receiver.
  The deleted unknown-member padding could reject for the wrong reason.
- The check-in producer/comparison triad executes real marshal, decode, signature
  verification, commit classification, and exact retained-watermark observation.
  Its exclusive primary classes cover predecessor, successor, and neither, with
  separate window/chain/generation mutations and failed/absent producer proof.
  This uses the protocol's exhaustive-small-domain replacement for the comparison
  relation, not a claim that all cryptographic byte strings are enumerated.
  Other admission and authority-binding tables remain separately in the package.
  Opaque offering permutations that merely repeated those states were removed.
- The existing struct-role inventory now parses compiled embedded sources. The
  new ingress inventory binds every one of the 20 JSON decoder owners to an
  actual semantic fuzz function through compiler references. Its AST scope is
  JSON decoder ownership, not every public method. Enum campaigns also exercise
  canonical text/JSON doors and the UsageClass/OutcomeClass constructors; the
  separate usage-watermark campaign checks generation/arithmetic behavior.
- Usage-window fuzzing uses typed valid seeds, exact ceiling seeds, uint64
  extremes, independent `math/big` totals, exact accepted round trips, and full
  populated-receiver preservation on rejection. Generic-response fuzzing now
  proves retained authenticated header/body facts after a refused replacement.
- Four deliberate mutations were killed: widening the usage-window ceiling by
  one byte; removing total consistency; adding an unfuzzed JSON decoder; and
  adding an unclassified production struct. Original files were restored before
  the final validation and benchmark/fuzz runs. Mutation sources and failing
  test identities are in the manifest and raw run records.
- Golden JSON and inventory sources use Go `embed.FS`, removing direct test
  filesystem reads. Production has no new filesystem, HTTP, clock, or global
  effect. Tests remain parallel where their ownership permits it.

## Measurement interpretation

Before and after use Go 1.27.1 linux/amd64 on the same Furnace server, AMD EPYC
7282 (16 cores / 32 logical CPUs), GOWORK=off, GOMAXPROCS=8. It is a shared server
without CPU affinity or controlled power isolation. A later metadata snapshot
reports the schedutil governor; it is not proof that governor state was sampled
at both benchmark start times. Each workload has one before and one after
sample: these are observations, not statistically established latency trends.

The baseline harness was added before changing production. Between measurement
passes, parent benchmark `ReportAllocs` calls were added to satisfy Witness, and
fixture reads changed to compiled embedding of the same bytes. Timed operation
bodies, fixture contents, benchmark identities, and input sizes did not change.
Setup/signing is excluded from timing. Verification benchmarks execute real
Ed25519 verification. Both CPU and sampled memory profiles and their matching
binaries are retained, with hashes. Whole-suite profile totals combine workloads
with different iteration counts; they are not per-operation attribution or peak
resident-memory measurements.

Before allocation profiling identified repeated `VerifiedResponse.Validate`
body-copy/validation allocations (12.28% flat, 24.47% cumulative in the aggregate
allocation profile). The new immutable representation removes that work from
proof validation. Body extraction pays for independent decoding rather than
returning an alias. No reflection copier, caching layer, custom JSON runtime,
or speculative hash optimization was added. Standard-library JSON, SHA-256,
Ed25519 arithmetic and memory copying remain visible in the profiles.

After CPU profiling still shows standard-library work prominently: Ed25519
field multiplication is 8.47% flat and the Go JSON struct decoder is 33.39%
cumulative in the aggregate profile. After allocation profiling shows
`bytes.Clone` at 24.72% flat; the former verified-response validation copy is
absent from the top allocation sites. The sampled end-of-run live heap contains
1,539 KiB attributed to runtime thread allocation. This snapshot is neither a
peak-memory bound nor evidence that the package allocates nothing while working.

## Review and limits

Only the Controlplane slice is under review. Linux tests/race/fuzz execute on
Furnace; macOS arm64 and Windows amd64 checks are compilation only. Full-module
gates and consumer upgrades were not run. No external enrollment, persistence,
or live provider effect is claimed by this pure-package validation. Historical
benchmark reports remain historical; this report and its manifest describe the
new candidate. Shutdown remains next after this slice is reviewed and released.

## Fuzz minimization follow-up

Five signed-document campaigns reported only 20–30 executions and one payload campaign reported 1,114. Go source inspection showed that the default 60-second minimization allowance also applies to coverage-preserving input minimization. Six additional 30-second campaigns set the standard `-fuzzminimizetime=1s` flag. This preserves minimization while letting mutation work advance within a short campaign. No Go source or package source was changed; the exact Go source excerpts and hashes are retained. Corpus caches were retained, so execution-count comparisons are operational observations, not controlled throughput benchmarks.

| Target | Initial final progress | Bounded-minimization final progress |
| --- | --- | --- |
| FuzzInstallationCertificateDocumentDecodeAndVerify | fuzz: elapsed: 31s, execs: 23 (0/sec), new interesting: 0 (total: 14) | fuzz: elapsed: 31s, execs: 214927 (0/sec), new interesting: 218 (total: 232) |
| FuzzRegistrationDocumentDecodeAndVerify | fuzz: elapsed: 31s, execs: 30 (0/sec), new interesting: 0 (total: 16) | fuzz: elapsed: 31s, execs: 52021 (0/sec), new interesting: 186 (total: 202) |
| FuzzCheckInRequestDecodeAndVerify | fuzz: elapsed: 31s, execs: 24 (0/sec), new interesting: 0 (total: 15) | fuzz: elapsed: 31s, execs: 62556 (0/sec), new interesting: 171 (total: 186) |
| FuzzCheckInResponseDocumentDecodeAndVerify | fuzz: elapsed: 31s, execs: 24 (0/sec), new interesting: 0 (total: 15) | fuzz: elapsed: 31s, execs: 92621 (0/sec), new interesting: 199 (total: 214) |
| FuzzRegistrationPayloadExternalDecoder | fuzz: elapsed: 31s, execs: 1114 (0/sec), new interesting: 7 (total: 16) | fuzz: elapsed: 30s, execs: 161929 (0/sec), new interesting: 218 (total: 234) |
| FuzzUpgradeRequiredResponseSemanticClosure | fuzz: elapsed: 31s, execs: 20 (0/sec), new interesting: 0 (total: 11) | fuzz: elapsed: 30s, execs: 22658 (0/sec), new interesting: 119 (total: 130) |

All original and supplemental outcomes remain in the manifest. The supplementary phase addresses ineffective campaign allocation, not a newly claimed production defect.

## Validation and measured results

The final uncached Linux race run passed with 85.3% statement coverage. There were 0 skipped test events and 0 failures. Scoped Go vet, Staticcheck, Errcheck, Witness lint, production `gocyclo -over 10`, and Darwin/Windows test-binary compilation passed. All 21 fuzz targets completed an initial serial 30-second campaign each, with four workers per target. Six low-progress targets also completed explicitly recorded supplemental campaigns with one-second minimization bounds. Exact commands, exits, effective duration, final execution counters and artifact hashes are in [the evidence manifest](controlplane_upgrade_20260909_evidence.json).

The commands used `GOWORK=off GOMAXPROCS=8`. Tests used `go test -json -race -count=1 -coverprofile=… ./controlplane`; benchmarks used `go test -json -run=^$ -bench=. -benchtime=30s -benchmem -count=1 -cpuprofile=… -memprofile=… -o … ./controlplane`; each fuzz run used `go test -json -run=^$ -fuzz=^TARGET$ -fuzztime=30s -parallel=4 -timeout=2m ./controlplane`. Full argument arrays and absolute artifact destinations are retained.

| Workload | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| ParseProductStatus | 12.76 | 12.7 | 0 | 0 | 0 | 0 |
| RegistrationCodec/marshal | 160232 | 191825 | 26907 | 26915 | 329 | 329 |
| RegistrationCodec/unmarshal | 372431 | 458655 | 64305 | 64327 | 1091 | 1091 |
| VerifyRegistration | 446004 | 530609 | 29820 | 29823 | 502 | 502 |
| VerifyCheckIn | 206422 | 237861 | 9933 | 9935 | 195 | 195 |
| CommitCheckIn | 26539 | 38161 | 3138 | 3139 | 87 | 87 |
| AuthenticatedResponse/verify | 204166 | 194047 | 11227 | 6712 | 174 | 122 |
| AuthenticatedResponse/body | 8578 | 453090 | 2289 | 69143 | 27 | 1118 |
| AdvanceUsageWatermark/one_class | 5427 | 7811 | 912 | 912 | 24 | 24 |
| AdvanceUsageWatermark/maximum_classes | 29161 | 35386 | 2817 | 2817 | 86 | 86 |

`AuthenticatedResponse/body` now measures independent decoding: 8,578 → 453,090 ns/op, 2,289 → 69,143 B/op, and 27 → 1,118 allocations. This is a substantial measured regression in accessor cost in exchange for eliminating mutable authenticated aliases. A caller should extract its typed body once when consuming the proof; repeated extraction creates repeated independent objects. `AuthenticatedResponse/verify` fell from 11,227 to 6,712 B/op and from 174 to 122 allocations. These absolute observations do not establish a latency trend. Unchanged paths also moved in wall time; attribution to machine contention is plausible but unproven.

Each pass configured 30 seconds per measured case, ten cases, one pass per revision (300 seconds nominal). Go stopped the enum parser at its one-billion-iteration cap after approximately 12.7 timed seconds; that row is exploratory, not 30-second acceptance evidence. All other cases ran at least 30 timed seconds. before-benchmarks: 344.95 seconds total wall time, 11.50× the per-case flag. after-benchmarks: 336.60 seconds total wall time, 11.22× the per-case flag. These ratios exceed the protocol’s 10× visibility threshold once setup/calibration are included. The ten-case multiplier was explicit in the measured harness, and both passes are retained. Fuzzing configured 30 seconds per target × 21 targets = 630 nominal serial seconds: its 21× multiplier likewise exceeds 10× and is explicitly reported, with four workers inside each target. Six supplemental 30-second campaigns added a declared 180 seconds after low-progress runs were identified, giving 810 nominal seconds across 27 campaigns. They used `-fuzzminimizetime=1s`; original attempts remain visible. This is an explicit additional phase, not a post-hoc budget relabeling.

All raw stdout/stderr, source snapshots, CPU/memory profiles, and matching binaries remain outside Git at `/home/d/engineering-evidence/primitive/controlplane-upgrade-20260909` on Furnace. The manifest verifies their byte lengths and SHA-256 digests. Source files were unchanged throughout every final run. The initial Witness benchmark-parent findings and unused-fixture Staticcheck finding were fixed; those failures, the compile failures, and invalid decoder-fixture attempts remain in the evidence instead of being relabeled as successful verification.
