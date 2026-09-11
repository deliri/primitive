# Upgrade review — v2026.1.61

Upgrade now rejects oversized selector and trial metadata while reading into
fixed nominal storage. Previously, both readers accumulated the entire file in
a bytes.Buffer before applying the existing canonical-document ceiling. An
actual 16 MiB malformed selector allocated 33,558,832 bytes before the repair
and 53,896 bytes afterward in the same exploratory one-operation benchmark.
These are allocation observations, not an accepted performance trend.

BootstrapRequest.Validate now rejects typed-nil io.Reader values using Core's
existing reader contract, before creating any installation files. The public
Bootstrap test pins typed refusal, zero Primary and absence of a slot.

The package remains a mechanical two-slot installer. Product code supplies the
trial outcome; Primitive neither runs the product test nor decides what passing
means. No new scheduler, provider coupling, compatibility layer or production
policy was added. metadataBuffer is classified in the production struct inventory.

## Boundary inventory and evidence

| Boundary | Proof |
|---|---|
| Bootstrap / BootstrapRequest.Validate | Real signed and verified Release manifest, exact installed bytes, typed-nil refusal, foreign/truncated/oversized stream refusal, zero result and empty installation on rejection |
| Stage / StageRequest / DownloadSource | Real Release EvaluateInstalled and Prepare handoff, signed artifacts, local HTTP transport through Objectstore, exact staged executable and receipt; canceled stage, foreign payload, selector conflict, replay and unchanged primary |
| CompleteTrial / TrialReport / Promotion | Exact observed candidate binding, exhaustive outcome enum checks, failed trial yields no promotion; public lifecycle supplies product-owned pass/fail observations |
| Promote / DiscardTrial | Real Bootstrap → Stage → CompleteTrial → Promote/Discard chain; mutated persisted receipt cannot change candidate/prior authority; selection remains installed on refusal; successful promotion removes only the former slot |
| ResolvePrimary / ResolveRequest | Real persisted selector plus independently checked executable identity; malformed/foreign selector yields typed refusal and zero Primary, reads preserve bytes and create no scratch state |
| Slot.UnmarshalJSON | Both production-encoded nominal seeds, standard JSON string agreement, canonical round trip, preserved populated receiver on refusal |
| Selection/trial JSON and revisions | Canonical fixed point, validation, typed errors and zero documents; existing hostile member/revision/order/extent cases and semantic fuzzing retained |
| Metadata read/write storage | Empty, exact, below/above and extreme extents; all-or-nothing oversized writes, preservation after capacity refusal, actual canonical/absent/empty/oversized disk reads |
| Capacity and artifact verification | Existing checked overflow/reserve cases, extent/SHA256/CRC32C contradictions, absence versus unknown-read outcomes and safe reclaim |
| Recovery and cleanup | Existing interrupted temporary writes, stale receipts, unowned-file preservation, cancellation, post-commit cleanup failures and exact error phases |
| Architecture/data flow | Exact import inventory, production struct roles, no hidden runtime/world model, common live/synthetic forbidden-shape matcher, fixed metadata storage guard |

The HTTP transport in tests is local and synthetic. Release signing,
authentication and preparation, Upgrade entry points, Objectstore transfer,
Filestore persistence, artifact verification and selector transitions are real.
No live provider or product executable is contacted or run.

Seven semantic fuzz targets cover the external ingress:
FuzzSelectionDocumentJSON, FuzzTrialDocumentJSON, FuzzSlotJSONSemanticClosure,
FuzzResolvePrimaryPersistedSelection, FuzzPublicBootstrapStream,
FuzzStageDownloadedBytes and FuzzTrialReceiptTerminalOperations. Accepted
executable streams must equal the genuinely signed seed; rejected streams or
receipts cannot produce a primary, trial target or promotion. The terminal fuzz
oracle checks the receipt emitted by Stage before applying the mutation and
proves the selector and retained receipt after the operation.

Filesystem helpers now receive t.TempDir from the owning parallel test or
subtest. Cleanup failures are checked. The existing serial exceptions and
closed-domain exhaustive enum tests remain. No synthetic 50-row classification
matrix is claimed: this package consumes authenticated Release authority and
returns installation effects, not a product evidence classification. Evidence
manifest/reporter/ledger layers are not implemented here; executable/manifest
identity consistency is exercised through Release and real disk verification.

## Red/green and execution accounting

Base revision: b1d7f8b6176d0778b471e68ac9f2a69703c8d3aa.
Evidence root: /work/engineering-evidence/primitive/upgrade-upgrade-20260910.

metadata-red fails the original whole-file reader storage invariant.
metadata-capacity-mutation disables the new capacity rejection and fails the
behavioral boundary test; it was restored immediately after capture.
shared-matcher-mutation disables goroutine recognition and fails the shared
synthetic matcher test, then restores it.
bootstrap-nil-red-2 demonstrates the original typed-nil admission defect.
Compile errors while developing the tests and the lint diagnostic correction
are retained as failed attempts, never presented as behavioral red proof.

The final committed phase runs cache-disabled race tests, witness-lint, go vet,
staticcheck, strict errcheck and a full-module build. The unchanged
BenchmarkResolvePrimaryFourKiB workload runs for 30 seconds on the same
host/toolchain/CPU setting before and after, with CPU/memory profiles and binary
retention. Each fuzz target runs once for 10 seconds with one worker after the
benchmark. Short allocation exploration is explicitly separate.

Every run retains source revision, dirty/clean facts, full argv, toolchain and
machine, source digests, stdout/stderr hashes and bytes, emitted counts, exit
status and attempt identity. The evidence manifest and disk verifier check all
retained files. Historical undiscovered not-run/unavailable denominators remain
unknown. Local verification does not issue independent acceptance. Full-module
build success is not full-module test acceptance; unrelated previously reported
Core/AWSidentity/Deploy test failures retain their separate status.

## Memory and remaining work

The selectors and trial receipts are bounded canonical nominal documents, with
existing 8 KiB and 16 KiB ceilings. Their bytes are materialized within fixed
storage; this is not a general incremental JSON decoder. The new reader enforces
those existing nominal bounds before file-sized allocation. Executable transfer
and verification retain fixed streaming buffers and no new transfer quota.
Nested Release/Core nominal decoding retains those owners' contracts.

Independent acceptance and consumer migration remain separate. Deploy is next
in the package queue. Release tags are derived from compass/config.json, the
repository's authoritative version declaration.
