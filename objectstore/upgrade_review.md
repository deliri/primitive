# Objectstore upgrade — reviewed v2026.1.25 checkpoint

Base: published `v2026.1.24`, commit `07e59437fc990fcd16d247f9a9c788cc3db214bc`. The user approved publication as v2026.1.25 after the external review reported no issues. This review records the concrete fixes and their evidence; it does not claim exhaustive correctness or full-module gate approval.

The local testing protocol was read in full (2,214 lines), SHA-256 `dd83cd7f62c172092546dab6b7c7d5c59753b5e8ae784631a94cff4d6d48126c`. Evidence records identify each exact dirty source snapshot, complete command, Go toolchain, environment, cache state, exit status, and retained artifacts. Test counts below include parent/subtest events; they are not independent defect counts.

## Confirmed changes

- ExactReader delegates reads directly to its caller-owned `io.Reader`. The former extra 32 KiB `bufio.Reader` obscured a final `(n, io.EOF)` and could panic before the outer invalid-count check. Native count/error pairs now reach the mechanical guard directly. Invalid counts refuse; returned bytes accompanying native errors are counted; zero-length destinations do not consume the source-progress budget. Terminal failure is sticky.
- Inspection checks context after the final source read and final extent observation. A source that cancels the operation while returning the final bytes cannot produce a completed inspection.
- ExactReader and Inspection follow Go's documented unwrapped-EOF rule. Joining EOF with cancellation or a native failure cannot certify completion or erase that failure. Two narrow Witness sentinel-comparison waivers document the Go contract, Objectstore ownership, and a 2026-12-08 review date; the same EOF exception already exists in Filestore.
- Upload, Download, and Inspection validation refuse typed-nil streams. The writer-presence rule has one Core owner, used by Objectstore and Exchange; Exchange's duplicate response-writer reflection helper was removed. Core's consumer-ownership ratchet was kept intact.
- Browser upload projection construction uses the existing signing-header validator. A body exceeding the provider's raw-upload ceiling is rejected before the projection is returned, rather than being admitted and failing during encoding. This removes a duplicate provider-shape validator.
- The BLAKE3 and upload/download commitment JSON decoders enforce Core's document ceiling before decoding. Oversize, absent, and malformed inputs preserve the prior typed identity.
- Transfer and received evidence now enforce the owning provider/direction extent ceiling. Confirmed empty-object integrity must carry the actual empty SHA-256 and zero CRC32C; one owning `Integrity.Validate` rule closes requests, transfers, and received evidence.
- ExactReader refuses a source whose successful Stat returns no FileInfo, preserving typed refusal instead of panicking.
- Architecture tests read compiler-embedded Go sources. All 63 named production types have compiler-bound roles, including defined projections and consumer interfaces. The public JSON/text decoder inventory binds seven method doors to actual fuzz functions and detects generic receivers too. `ParseSignedURL` and its campaign are bound separately; automatic discovery of future free parser functions is not claimed.

## Test evidence so far

The untouched starting Objectstore race suite passed 1,435 test events with zero failures/skips and 86.7% statement coverage. The strengthened benchmark harness was then captured before production edits.

Recorded red states:

| Run | Result and meaning |
| --- | --- |
| `stream-admission-red` | 11 failing leaves plus three parents: native reader result handling, empty destinations, and final cancellation |
| `nil-stream-admission-red` | One failing row exercised three validators admitting typed-nil streams |
| `projection-extent-red` | S3 and GCS each admitted one byte beyond their provider upload ceiling |
| `scalar-json-extent-red` | One oversize row exposed all three unbounded scalar decoders |
| `eof-identity-red` | Three wrapped/joined EOF rows exposed false completion and swallowed source causes |
| `mutation-writer-presence-red` | Deliberately restoring interface-only nil detection failed five typed-nil kinds |
| `mutation-exact-byte-red` | Deliberate byte corruption failed both the exact-byte table and fuzz seeds |
| `transfer-extent-red` | Five provider/direction leaves admitted oversized confirmed transfer/evidence facts |
| `empty-integrity-red` | Three false empty-content digest combinations passed four validation/ingress doors |
| `exact-metadata-red` | Two nil-FileInfo leaves panicked, for empty and nonempty sources |
| `provider-handoff-checksum-mutation-red` | Removing the provider CRC comparison failed all four CRC-only provider/extent leaves |
| `inventory-type-mutation-red` | Adding an actual production defined projection without a role failed the inventory |
| `inventory-decoder-mutation-red` | Adding an actual public JSON decoder without a fuzz binding failed the ingress inventory |
| `progress-accounting-mutation-red` | Discarding accepted progress accounting failed eight sequence leaves across both directions |

`current-package-race` passed Objectstore with 87.1% statement coverage. The combined command failed Core's consumer-ownership check because the new writer helper initially had only Objectstore as a consumer. After consolidating Exchange's identical rule, `shared-writer-integration-green` passed 174 selected Core/Exchange events with race detection, including that ownership check and socket admission. The later complete `checkpoint-package-race-confirmed` passed 3,413 Objectstore/Core events with zero failures/skips: Objectstore coverage 87.1%, Core 85.6%. The final EOF comparison factoring/waivers also passed its targeted race regression. Earlier failures remain recorded rather than being relabeled successful.

Scoped vet and staticcheck passed for Objectstore/Core/Exchange. Complexity passed for all Objectstore production and the changed Core/Exchange files. Objectstore Witness passes with the two documented EOF waivers; Core Witness passes with one existing waiver. Exchange Witness exits internally with `invalid doctrine report`, including when invoked alone; it is unresolved tool evidence, not a successful gate. A multi-package Witness attempt and an invalid `go run` attempt outside the selected module are retained. The first new-test Witness run found five inadequate failure messages; those messages were improved and its corrected run passed. Full-module gates were not run.

The former 40-row ExactReader extent table sampled many arbitrary lengths while checking counts. Its replacement checks exact byte order, chunk boundaries, withheld final chunks, short-stream prefixes, source consumption, and immutable terminal outcomes. `FuzzExactReaderExtentAndBytes` varies payload, declared extent, and destination extent and completed a requested 30-second campaign with 1,003,193 executions, four workers, and no failure. This is one bounded campaign, not exhaustive proof.

The baseline harness edit had an initial compilation failure caused by an overly broad text replacement in a test file. It was corrected before baseline benchmarks. Both attempts remain in the evidence manifest.

## Performance evidence

See [the captured baseline](upgrade_benchmark_baseline.md). All six starting runs requested 30 seconds and explicitly supplied CPU profiles, memory profiles, and matching test-binary paths. The three benchmark source files and initial harness are retained locally for comparison.

The 1 KiB upload allocation profile attributed 81.46% of allocated bytes to `bufio.NewReaderSize`, supporting removal of the redundant buffer. Updated upload samples, captured before the final EOF correction, use byte-identical benchmark source files and payloads:

| Operation | Baseline ns/op → checkpoint ns/op | Baseline B/op → checkpoint B/op | allocs/op |
| --- | ---: | ---: | ---: |
| GCS upload 1 KiB | 16,813 → 10,018 | 40,330 → 7,284 | 99 → 97 |
| GCS upload 10 MiB | 6,478,261 → 6,310,210 | 40,157 → 7,261 | 99 → 97 |

Both requested 30 seconds and retained CPU/memory profiles and matching binaries; neither source changed during its run. These are intermediate single samples, not final-source results or statistical trends. A separate Go race command in another workspace was observed after the pair. Its overlap with each timed interval was not established, so machine isolation is not claimed. The first process-list capture failed under the sandbox and saw a simultaneous test-source edit; it is explicitly unusable as source-stable verification. An approved read-only process-list observation was then retained separately. The final six-workload comparison is now captured separately in [upgrade_benchmark_after.md](upgrade_benchmark_after.md).

## Final validation and review boundaries

The combined Objectstore/Core race command passed **4,026 test events**, zero failures and zero skips. The last test-only boundary cleanup added below-limit cases and passed a fresh Objectstore race run: **2,081 test events**, zero failures and zero skips (`review-boundaries-race`). Objectstore statement coverage is **87.9%**, Core **85.6%**. The command uses `-json -race -count=1` and retains its coverage profile. This is measured coverage, not a claim of exhaustive correctness.

Final-source vet, staticcheck, and Objectstore Witness pass. Witness retains only the two documented unwrapped-EOF exceptions. Production complexity is at most ten; Objectstore errcheck and nilaway pass. Linux/amd64 and Windows/amd64 test binaries also compiled on the final reviewed test source (`review-linux-compile`, `review-windows-compile`); they were not executed on those operating systems. The earlier affected Core/Exchange race run passed. Exchange's standalone Witness internal failure remains unavailable validation; full-module gates remain deferred.

The provider handoff table exercises the real Upload → Evidence → JSON ingress → VerifyProviderUpload chain for S3/GCS and empty/nonempty bodies. It exhausts **144 realizable combinations of six selected observation-field fault dimensions**, giving 576 leaves. Per provider/body group, exactly one primary class is assigned: one exact boundary, seven contradictions, 64 typed refusals, 72 absent-evidence neutral cases. These are exhaustive combinations of the declared fault model, not every possible provider response or a claim of 576 separate regressions. Every successful decoded baseline is compared with independently observed bytes, digests, version, provider, and direction. Refusals preserve absence of sealed proof; a succeeding baseline remains unchanged after each fault.

The evidence decoder table previously used helpers that claimed to reorder/remove one fact while replacing several unrelated values, and then checked only Validate. Those helpers were removed. Expected issuer values are now compared exactly, member reordering uses Go's JSON canonicalizer, omitted versions preserve all other fields, and every refusal mutation must change its source document. The uppercase-digest row now changes case without changing width. Empty fixtures now use actual empty digests. Progress tests replace arbitrary buffer-size samples with cumulative and transactional sequences; inspection uses nonuniform byte patterns so reordered content cannot hide behind repeated identical bytes.

The final checks initially found an unused fixture helper, a mutable package-level inventory slice, and two vague failure messages. Their corrected source passed the checks. A download fuzz formatting oracle also incorrectly treated empty or coincidentally redaction-shaped header values as leaks; it now checks exact permitted output. Upload fuzz refusal checks the full capability commitment, and signed-URL fuzz has an acceptance oracle using the Core endpoint contract. Failed attempts and corrected attempts remain distinct records.

All six final benchmark captures passed with CPU/memory profiles, matching binaries, and no source drift or missing requested artifact. All eleven requested-30-second fuzz campaigns passed, four workers each. Two separately labeled diagnostic campaigns also passed. Production and benchmark sources stayed unchanged throughout the measurements. The last table-only cleanup occurred afterward; its fresh race/staticcheck/Witness and platform compilation evidence identifies that newer test snapshot. The measurement records remain bound to their original complete source snapshots, not silently reassigned to the later tests.

The inspection and provider-response campaigns stopped reporting additional executions after roughly six seconds, although they exited successfully at their requested duration. The follow-up campaigns used `GODEBUG=fuzzdebug=1 -fuzzminimizetime=1s`, retaining the 30-second total and four workers. Both maintained progress; debug logs identify minimization of coverage-expanding inputs. Go's local fuzz coordinator/worker source was retained for reference: reported counts advance when worker results reach the coordinator, and minimization is separately budgeted. These observations are consistent with minimization consuming the earlier run budget; they do not retrospectively prove every worker's state in those earlier runs. The diagnostic runs have different settings and cache contents and are not substituted for the standard runs.

The user explicitly approved retaining the existing `github.com/zeebo/blake3` dependency on 2026-09-08. Inspection's published BLAKE3 contract remains in place as that narrow dependency exception.

Objectstore continues to execute HTTP through Exchange and Go's transport. These changes add no retry policy, product state machine, workflow engine, goroutine pool, or alternative runtime. Streaming reads remain bounded; inspection memory is independent of payload size. This does not mean constant time for hashing or transferring an arbitrarily large object.

Raw logs, profiles, binaries, source snapshots, and mutation overlays remain in ignored `testdata/test-upgrade-20260908/`. [The evidence manifest](upgrade_evidence.json) preserves their coordinates and hashes; a hash in a report does not replace unavailable raw bytes.

## Timed fuzz evidence

Each row retains its complete command, initial seed/cache snapshot, ending corpus snapshot, elapsed time, raw output, and exit status in the manifest. Counts are Go-reported callback executions, not unique semantic cases.

| Run | Reported executions | Result |
| --- | ---: | --- |
| `final-fuzz-BLAKE3DigestTextSemanticClosure` | 879,625 | pass |
| `final-fuzz-BLAKE3DigestJSONSemanticClosure` | 637,762 | pass |
| `final-fuzz-TransferEvidenceJSONSemanticClosure` | 469,230 | pass |
| `final-fuzz-DownloadCapabilityJSONSemanticClosure` | 611,490 | pass |
| `final-fuzz-UploadCapabilityCommitmentJSONSemanticClosure` | 828,032 | pass |
| `final-fuzz-DownloadCapabilityCommitmentJSONSemanticClosure` | 892,944 | pass |
| `final-fuzz-ParseSignedURL` | 905,793 | pass |
| `final-fuzz-UploadCapabilityAdmitsOnlyTransferableCapabilities` | 419,988 | pass |
| `final-fuzz-InspectSemanticIntegrityAndSizeClosure` | 30,593 | pass |
| `final-fuzz-GoogleCloudStorageDownloadCRC32CProviderValuesSemanticClosure` | 10,056 | pass |
| `final-fuzz-ExactReaderExtentAndBytes` | 740,288 | pass |
| `diagnostic-fuzz-minimization-InspectSemanticIntegrityAndSizeClosure` | 316,796 | pass |
| `diagnostic-fuzz-minimization-GoogleCloudStorageDownloadCRC32CProviderValuesSemanticClosure` | 113,766 | pass |

## Review surface

- Objectstore production: `exact.go`, `inspection.go`, `values.go`, `client.go`, `transfer_evidence.go`, `upload_http_projection.go`, and the three scalar JSON decoder files (`blake3_digest.go`, `upload_capability.go`, `download_capability.go`). Existing provider APIs and wire shapes remain; invalid extents, false empty-content declarations, and absent streams are refused earlier.
- Shared ownership: `core/io_contracts.go` owns writer presence; Exchange's socket admission consumes it and its duplicate reflection helper was deleted. The former duplicate browser-provider validator and redundant ExactReader buffer were removed. No compatibility path was added.
- Tests: the new boundary/metadata/stream/empty-integrity/extent tables, provider handoff matrix, decoder/type inventory mutations, and ExactReader fuzz campaign; strengthened existing evidence, progress, inspection, and capability fuzz oracles. Benchmark source changes predate the baseline and stay byte-identical across the measured pairs.
- Evidence: this review, baseline and candidate benchmark reports, and the compact manifest. Raw source snapshots, outputs, binaries, profiles, overlays, and fuzz cache snapshots stay ignored and local. Manifest references were checked for resolvable source sets and retained files; every measured Objectstore production file still matches the reviewed production source.
- Not performed: full-module gates, execution on Linux/Windows hosts, live-provider interoperability, statistical benchmark replication, publication, version bump, or commit of this slice. Exchange Witness remains an internal tool failure as documented above. No data migration or new runtime service is introduced.

## Publication approval

The user supplied [the external review](../_docs/objectstore_review_20260908.md) and explicitly instructed bump, commit, and push. Its Issues section is empty; no further production patch was warranted. Package-run records above retain their original dirty-source coordinates. Publication adds the release coordinate and reports, followed by fresh Compass/Version coordinate checks. Historical statements about uncommitted work describe the review state, not a new validation run.
