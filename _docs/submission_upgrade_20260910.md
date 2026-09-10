# Submission upgrade — reviewed, 2026-09-10

Approved Payment was published as v2026.1.44, commit `67875b51b4bee0a41f7c0a2dba0551b3655745fa`. The subsequent Submission slice was reviewed and approved for release v2026.1.45.

## Production changes

- Remove six Submission JSON document/payload byte ceilings and the two dependent Submissionauth envelope ceilings. Decode through Core's extensible strict JSON contract; unknown/duplicate/malformed values remain invalid and rejection preserves the receiver.
- Reject typed-nil canonical destinations across all four signed payload forms through Core.WriterIsNil. Preserve native write errors and exact written prefixes.
- Reject typed-nil upload sources at UploadCallRequest.Validate through Core.ReaderIsNil. Validation and projection perform no source reads.
- UploadDecision returns an exact zero on refusal.
- VerifyCompletion authenticates grant and device signatures before classifying cross-document binding mismatches. Structural validation still runs first. Authentic mismatches and unauthenticated input have exclusive tested identities.
- Remove duplicate completion-projection construction from IssueCompletion, informed by the baseline CPU and allocation profiles.

The public removal is eight obsolete size constants; real dependent references inside Primitive are updated. No shims, new runtime, transport, workflow engine or product policy was added. No consumer repository or Witness source was changed. Submissionauth changes are limited to the coupled envelopes, their tests and inherited completion error precedence; this is not a claim of a full Submissionauth upgrade.

## Hostile proof

`_docs/testing_protocol.md` was read in full before test work; its SHA-256 is `5eaec400ce3eda457f3683fdacac8ec8f4f0f51ee229643b0f7f62ea420e6ec7`, unchanged from the previous slice's preserved source.

New local table triads cover JSON framing at seven Submission and two credentialed doors; absent/typed-nil/canonical/invalid-body writers; source admission without reads; zero rejected projections; authentication-before-binding mutation pairs; signer errors/cancellation/zero effects; and receipt extent precision, Go's exact signed maximum and typed overflow.

The original production failed the targeted regression tables in `red-boundaries`. Nine deliberate mutations were killed: typed-nil writer admission, typed-nil reader admission, short-write detection, JSON quotas, rejected projections, receiver preservation, capability binding, reject-all domain parsing, and completion authentication order. Mutations were reverted individually; exact mutation sources and failures remain in evidence.

Fuzzing now uses compiler-constrained receivers, exact typed round-trips, generated valid whitespace probes, valid-seed acceptance checks, signed-fact refusal oracles, both decision arms, independent UUIDv7/domain parsing oracles, and an embedded-source ingress inventory. All six targets passed once at 30 seconds with four workers: **1,655,466 executions**, 180 configured seconds. Corpus growth and actual elapsed durations are retained, not hidden as a cache-free run.

## Validation and measurement

- Scoped race tests passed: Submission 87.2% statement coverage, Submissionauth 86.6%.
- go fix, go vet, staticcheck, errcheck, witness-lint, production gocyclo <=10 and goconst (minimum 4 characters, 3 uses, no tests) passed.
- Full-module production `go build ./...` passed. Full-module tests and remaining global gates were not run.
- Linux runtime tests passed; macOS/arm64 and Windows/amd64 Submission test binaries compile. This is not native runtime evidence for those systems.
- Ten benchmark cases ran serially before and after, 30 seconds each, with CPU/memory profiles and retained executables. Fuzz targets followed benchmarks serially. See [the complete comparison](../submission/after_benchmarks.md).

Initial harness compile/import mistakes, a Witness diagnostic, the incorrectly assumed uint64 extent cases, missing goconst launches and its failed offline installation remain explicitly recorded. The pinned goconst v1.7.0 installation and final successful checks supersede them; they are not erased or called successful.

## Evidence and limits

Raw evidence: `/home/d/engineering-evidence/primitive/submission-upgrade-20260910`. Per-run records contain the full base commit, actual command, toolchain, selected environment, source hashes, source patch, output, status and duration. Content snapshots preserve uncommitted source. Final manifest: `/home/d/engineering-evidence/primitive/submission-upgrade-20260910/manifest.json`.

The evidence names the reviewed working tree over the Payment commit; it does not retrospectively claim those runs used the subsequent release commit. No statistical speed claim, total-input throughput claim, O(1) JSON-document claim, exhaustive correctness claim or native macOS/Windows pass is made. Whole-document JSON APIs retain their actual allocation cost; Objectstore owns streaming object content. Nested Core, Controlplane, Attest and Objectstore contracts remain with their existing owners. This slice removes eight local JSON document ceilings; it does not claim that every nested type elsewhere in Primitive has already lost every byte limit. Go's signed extent representation is unchanged.
