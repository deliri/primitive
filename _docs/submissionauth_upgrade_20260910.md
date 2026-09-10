# Submissionauth upgrade — v2026.1.49, 2026-09-10

Base revision: b5f6154e00c9326beda706cf54832ec489af787c (v2026.1.48). The reviewed implementation was prepared in /work/code/primitive on Furnace. Release closeout bumps compass/config.json to v2026.1.49.

## Production

RequestDocument and CompletionDocument now bind the request/completion envelope signer to the installation certificate's nominated DeviceKey, in addition to the existing exact build binding. Previously, Assemble and the JSON receivers accepted an otherwise valid credential that Verify could never authenticate. The new rejection preserves Core's control-plane and response-binding identities and returns no document.

The issue-only CompletionProjection assembly enforces the same nomination rule. Submission now exposes CompletionProjection.Signer(), a validated typed projection of its existing envelope key, alongside Build(). The outer credential does not encode and decode its own issue-only document to discover the signer. Signature authentication remains in Attest, Submission and Controlplane; this getter does not claim authenticity.

Both ControlRoute methods now validate the owning credential before returning a route. Previously, a request or completion with a missing certificate could still produce a successful route. ControlRevision and ControlNonce remain exact field projections; they do not fabricate defaults.

These are mechanical binding checks. No product policy, state machine, retry loop, runtime, transfer quota or new dependency was added. Response wrappers and reconciliation remain compositions of their existing owning capabilities. The only production change outside Submissionauth is the small typed Signer accessor in Submission.

## Hostile proof

The entire local testing protocol was read before editing tests. Its SHA-256 is 5eaec400ce3eda457f3683fdacac8ec8f4f0f51ee229643b0f7f62ea420e6ec7.

The recorded red-nomination run fails on the former request, completion and projection nomination gaps and invalid route admission. Final tables prove exact nominated identity, same-build foreign identity, absent credentials, zero output on refusal, retained receivers, exact route/revision facts, and the Signer accessor's zero-value contract.

Existing authenticated request, provider-upload, grant, completion and receipt integration tests remain. The local provider fixture uses Exchange and Objectstore; no live provider is contacted. A successful reconciliation must retain the exact authority receipt identity, object/submission identities, provider observation time, integrity and manifest entry.

Padded projection rows that only varied opaque markers were replaced with earned canonical/foreign/truncated/duplicate/absent cases. The repeated ten-marker reconciliation success loop was reduced to its actual invariant. The obsolete padding helper was deleted. AST architecture checks use embedded source rather than direct filesystem reads, and JSON ingress/fuzz functions have compiler-bound inventories.

Both response families have local triads. An authentic sibling-family response is produced through Controlplane and refused by the actual Submissionauth wrapper. A counting signer proves wrong-family and absent-authority issuance performs no signing. Completion responses carry an actual issued chit built from a reconciled receipt and manifest summary. Independent verification checks exact signed headers and bodies/grant facts.

Nine deliberate semantic regressions were killed by tests and restored: request nomination, completion nomination, projection nomination, request route admission, completion route admission, request receiver preservation, completion receiver preservation, response-family wiring and provider content-type binding. Failures are real test failures, not compilation failures.

There is no producer/classifier classification split in this package. Its output is an authenticated agreement or typed refusal; no synthetic 50-row classification matrix is claimed. Unreachable defensive failures after already-validated sealed values are not forced through forged private graphs merely to raise coverage.

Five final fuzz campaigns passed, each configured for 30 seconds with four workers: request 68 executions, completion 24,918, projection 344,697, submission response 3,593, and completion response 105. These are individual campaign observations, not comparable throughput measurements. Several campaigns spent substantial time without advancing the reported execution counter; no broader exploration claim is made. Go's existing fuzz cache was reused. Earlier pre-optimization campaigns are retained separately and are not added to final counts. Accepted documents must survive exact canonical round trips and assembly; refusals preserve typed error identity and receiver state.

## Validation and measurements

Final-source checks passed: go fix and go vet for Submissionauth and Submission, Staticcheck, strict Errcheck for Submissionauth, Witness lint, production complexity at most ten, and Goconst with minimum four characters and three uses excluding tests. Race tests passed for both packages; statement coverage was 90.1% for Submissionauth and 86.8% for Submission. The full module builds. macOS/arm64 and Windows/amd64 compile checks passed; native runtime testing was Linux only. Full-module tests and the remaining global gates were not run for this slice. A final uncached JSON test run and its separate package/test/subtest accounting are retained in review-tests-json and execution-accounting.json.

Seven workloads have baseline, initial binding-fix candidate, and final profile-guided candidate observations, CPU/memory profiles and matching retained binaries: assembly, request verification, request JSON round trip, completion verification, completion JSON round trip, projection JSON and receipt reconciliation. Each of those three explicit phases ran once at 30 configured seconds per case. Setup and the provider fixture are outside timing. Failure diagnostics improved after baseline; successful workloads, names, arguments and budgets did not change. Full observations and limitations are in [before benchmarks](../submissionauth/before_benchmarks.md) and [after benchmarks](../submissionauth/after_benchmarks.md).

The initial binding-fix candidate added one projection allocation (205 to 206). Allocation profiles identified redundant projection validation; removing that repeated validation preserved both build and signer checks and restored 205 allocations. All seven final allocation counts match baseline. Final completion verification measured 648,259 ns/op versus 595,840 before (+8.8%); projection encoding measured 110,640 versus 108,305 (+2.2%). These single observations do not establish statistical significance or an overall speedup. CPU and cumulative allocation profiles for all three phases remain available; allocation-space profiles are not peak-memory measurements.

Exploratory failures are retained: the red nomination tests, a fuzz fixture that attempted to emit an internally invalid certificate, an attempted use of MarshalJSON on a receive-only DecisionDocument, unused test inventory/padding symbols, and Witness's diagnostic-context findings. The fixture and test issues were corrected rather than waived. The submitted source does not retain them.

## Memory and remaining ownership

Submissionauth adds no package-local extent ceiling. Its APIs carry complete typed credentials and caller-owned JSON byte slices; this is not an O(1)-memory JSON transport claim. Existing extent tables exercise valid documents surrounded by 1 MiB of whitespace and malformed counterparts. Authentication and reconciliation retain fixed typed facts; they do not load object payloads.

Inherited Core, Controlplane and Chit JSON/field limits remain owned by those packages and need their separate streaming audits. This slice does not claim that composed response/chit paths accept unlimited JSON or that a terabyte transfer was executed. No consumer or Witness source was modified.

Raw evidence: /home/d/engineering-evidence/primitive/submissionauth-upgrade-20260910. Per-run records retain the complete command, exact base commit plus source hashes/content snapshots, Go toolchain, relevant environment, output, exit, duration and source-stability result. measurements.json, mutations.json, manifest.json and audit.json cover the review tree and artifacts. Profiles, binaries and snapshots stay outside Git. Local verification is review evidence, not an independently issued acceptance receipt.

## Release review — 2026-09-10

Reviewed /tmp/submissionauth_upgrade_20260910_findings.md against the complete uncommitted slice. No additional production fix was needed. ControlRevision and ControlNonce retain their documented field-projection contracts; routing callers validate ControlRoute before consuming these facts. The remaining Build/Signer validation duplication and the nomination table's validation-only zero-output placeholder do not change correctness.

Independent closeout checks passed: uncached Submission and Submissionauth tests, race tests with shuffle enabled, go vet for both packages, gofmt inspection, and git diff --check. Compass tests passed after the version bump.

The full-module test run was attempted but is not green. The first run exhausted the remote /tmp user quota. A rerun with TMPDIR and GOTMPDIR on /work passed both changed packages and exposed existing failures in Core (package catalog, clean-upgrade/effect scans, validation witness inventory and export ownership), AWS identity (response extent assertions), and Deploy (unset objectstore policy). The same failures were reproduced from an untouched git archive of base b5f6154e00c9326beda706cf54832ec489af787c. Release artifact tests initially exhausted /tmp through nested build processes because their exact environment dropped TMPDIR and GOTMPDIR. The release test helper now preserves those two caller settings. The final uncached release package tests and go vet passed with temporary storage on /work. The reproduced Core, AWS identity and Deploy failures predate this slice; no full-gate success is claimed.

Closeout logs on Furnace are preserved under /work/engineering-evidence/primitive/submissionauth-release-20260910, including the initial and rerun full-module logs, baseline failures, race tests, config-tests.log and release-tests.log.
