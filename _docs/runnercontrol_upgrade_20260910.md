# Runnercontrol Go output metadata repair — 2026-09-10

This is a narrow ingestion repair, not completion of the runnercontrol package sweep.
Release: v2026.1.51. Base revision: 0094a70583d0e0f464cc578739e4d375f4ce05d5.

The installed Go 1.27.1 test2json producer emits OutputType on output events.
Runnercontrol's strict wire struct omitted it, turning real test output into an
unavailable observation. Existing handwritten event fixtures missed the defect.
The wire decoder now admits the four upstream states (ordinary/empty, frame,
error, error-continue), refuses unknown metadata and metadata attached to a
non-output action, and leaves package terminal events in charge of accounting.
Primitive observes the mechanical Go wire agreement; it does not infer product
completion, acceptance, or authority from diagnostic content.

## Evidence

Server: furnace, Linux amd64, Go 1.27.1. Repository: /work/code/primitive.
Append-only local review evidence:
/work/engineering-evidence/primitive/runnercontrol-upgrade-20260910.
Each run retains its argument vector, revision, dirty state, source snapshots,
environment, toolchain, stdout/stderr hashes and sizes, exit, and source stability.
These are author-run facts, not independent acceptance receipts.

The baseline package run passed 68 top-level tests and 550 subtests. The new
real-process regression executed the installed Go toolchain against a temporary
module with an intentionally failing test, required actual frame/error metadata,
then streamed those emitted bytes through Write/Seal and required one failed
package rather than unavailable accounting. The pre-fix run output-metadata-red
failed specifically on the unknown OutputType member. Its failures are retained.
The metadata domain table exhausts known values plus unknown and wrong-action
refusals, and proves ordinary diagnostic metadata does not manufacture benchmark
measurements or a successful package outcome.

Full package tests passed after the fix. Race/shuffle/count=2 passed 140 top-level
tests and 1,112 subtests. Go vet, staticcheck, witness-lint, and module-wide go build
passed. The existing Go observation semantic fuzz target ran once for 30 seconds
with four workers and passed. Its scope is parser/accounting closure, not complete
producer-to-classifier or artifact/manifest integration proof. No benchmark or
performance improvement is claimed by this metadata-only repair.

## Remaining runnercontrol surfaces, in order

1. Go observation streaming: whole-line accumulation and arbitrary event extent
   limits remain. Remove total-extent quotas without replacing them with unbounded
   allocation; prove fixed working windows, chunk invariance, cancellation,
   native failures, and exact terminal accounting. Build-output/build-fail events
   and ImportPath also need real-toolchain fixtures and explicit dependency versus
   selected-package accounting. This repair does not claim to support that door.
2. JUnit, coverage, and profile streaming doors: inventory materialized tokens,
   whole documents, caller-owned collections, invented limits, and lifecycle joins.
3. Producer/accounting handoffs: audit contradictory process/terminal observations,
   duplicate benchmark metrics, mutable observation ownership, and seal lifecycle.
   Preserve exact facts; leave product acceptance policy with the product.
4. Complete the remaining capability/authentication/execution/resource-plan doors,
   their ingress fuzz inventory, and local layer proofs before advancing packages.

The design target is the blind typed Unix-style socket: direct standard-library
I/O, fixed buffers, backpressure, and O(1) working memory in total stream extent
where possible. A bounded aggregate owned by a caller must be explicit; it must
not be advertised as constant-memory streaming. No new transfer quota, workflow,
compatibility adapter, product state, or global mutable state is introduced here.

## Strict errcheck limitation

`errcheck -blank -asserts ./runnercontrol` failed with 19 findings in five
unchanged test files: capability_document_layer_triad_external_test.go,
execution_budget_hostile_external_test.go,
experiment_completion_layer_triad_external_test.go,
go_profile_hostile_external_test.go, and
socket_state_authentication_layer_triad_external_test.go. A diff against the base
revision confirmed these files were unchanged. The failed execution remains in
metadata-errcheck; this is an explicit existing-source baseline, not a claim that
strict errcheck is clean. Retiring these unchecked setup errors and assertions is
required before the package sweep closes.
