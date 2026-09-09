# Attest upgrade — review candidate 2026-09-09

Base: `b1fae1821e258675f3435657226314846e7234f4` (`v2026.1.27`). Work runs on `d@192.168.1.81:/home/d/code/primitive`. This candidate is uncommitted and awaits user review; no version bump, commit, push, or consumer migration is included.

## Production scope

One correctness defect family was reproduced through both public operations. If a canonical body exhausted its byte budget and then returned cancellation, deadline, or another producer error, `canonicalDigestWriter.close` returned only the writer refusal and lost the producer's error identity. It now uses Go's `errors.Join` to retain both. Ten failing rows across Sign and Verify reproduced this same defect; they are not ten independent bugs.

The new 14-row stream table executes through both Sign and Verify. It covers empty input; minimum, ceiling-minus-one, and exact-ceiling accepted bodies; ignored overflow; cancellation before bytes, after partial bytes, and after a complete maximum body; and overflow combined with cancellation, deadline, wrapping, a producer error, or joined failures. Rows observe accepted byte counts, callback counts, signing suppression, zero refusal outputs, original writer/callback error identities, writer closure after return, and exact successful body facts. Successful digests are checked independently through Go SHA-256. Existing tests retain independent Ed25519 framing and fixed-vector proof.

Two profile-informed allocation improvements preserve existing wire and ownership contracts:

- Signature decoding uses `[ed25519.SignatureSize]byte` instead of a dynamic temporary slice.
- Envelope JSON fields point into the method's existing value copy rather than four separately allocated local copies. The wire still uses the same typed pointer fields, required-member checks, Core encoder, member order, and detached signature.

No public API, domain, shared Core contract, dependency, crypto implementation, parser, transport, filesystem implementation, clock, policy, workflow, or runtime was added. Production changes are confined to three Attest files and reduce source size. All production functions remain at cyclomatic complexity ten or below. Streaming hashing retains bounded memory; reading/hashing N bytes still requires O(N) work. No constant-time processing of an arbitrary stream is claimed.

## Test and fuzz scope

All 2,214 lines of the project-local testing protocol were read before editing tests. Its SHA-256 remains `dd83cd7f62c172092546dab6b7c7d5c59753b5e8ae784631a94cff4d6d48126c`.

Attest's existing ingress inventory, hostile parser/ownership/signature tables, layer triads, and semantic fuzz targets remain. The new `FuzzAttestStreamFailureClosure` varies body extent and callback failure, proves refusal conservation and closure, and checks accepted extent, SHA-256, domain, signer, and an independent Ed25519 frame. No producer-to-classifier classification lattice exists in this package, so the 50-case classifier matrix rule is not applicable.

Architecture inventories now inspect embedded source and `testing/fstest` fixtures. They no longer read repository files or write synthetic files through `os`; the existing hostile synthetic matcher cases still execute the same AST matchers. The JSON-door scan shares source discovery with that inventory. Arbitrary page/prime body sizes in the verification table were replaced by SHA-256 block-minus-one, exact-block, and block-plus-one inputs.

Allocation ratchets cap canonical envelope encoding at 27 allocations (previously 30) and decoding at 47 (previously 48) for the declared fixture on ordinary builds. Reverting each production allocation change independently failed its intended allocation row. The stream defect has an actual pre-fix failing run; the allocation changes have two independently failing reversion runs. Every temporary production reversion was restored.

## Execution accounting

The initial final race run failed because ordinary-build allocation budgets had been applied to race-instrumented code: encoding measured 33 and decoding 54 allocations. This was a test-configuration failure, not a race report. The allocation test now has an explicit `!race` constraint and a typed runtime-allocation isolation declaration. Its parent and two rows run in the separate complete ordinary-build suite. That exclusion remains visible rather than being hidden by looser budgets or folded into skipped/passing race counts.

Final ordinary-build package tests passed: **685 passing test events, zero failures, zero skips**. The corrected unfiltered Linux race run passed: **682 passing test events, zero failures, zero skips**, **93.0% statement coverage**. These counts include parent tests and fuzz seed executions, not independent invariant counts. The three-event difference is exactly the ordinary-build allocation test and its two rows.

Package-scoped vet, Staticcheck, errcheck, Witness lint, and production complexity checks passed. Darwin/arm64 and Windows/amd64 test compilation passed; native execution was Linux/amd64 only. Those successful analyzer/cross-compilation records preceded only the allocation test's build-constraint/comment addition and retain their exact source hashes. Full-module gates were not run.

Build and fuzz caches were retained. Package test runs use `-count=1` to bypass cached test results. Red runs, both deliberate failing allocation reversions, the initial race-configuration failure, and the corrected runs remain separately recorded. Raw commands, toolchain, environment, hashes, outputs, exit codes, and source stability are retained in the companion evidence JSON and raw run records.

## Fuzz campaigns

All eleven public-boundary targets completed one requested 30-second campaign with four workers, serially after the final benchmark pass. Aggregate requested fuzz time was 330 seconds. Existing fuzz caches were retained; seed-baseline counts, execution counts, and all progress reports remain in raw stdout.

| Fuzz target | Executions reported | Last elapsed report (s) | Exit |
| --- | ---: | ---: | ---: |
| `FuzzAttestStreamFailureClosure` | 424,668 | 30 | 0 |
| `FuzzCanonicalObjectEmitsStrictlyDecodableAndStableDocuments` | 1,110,607 | 31 | 0 |
| `FuzzCanonicalObjectNestedMember` | 596,830 | 31 | 0 |
| `FuzzEnvelopeJSONSemanticClosure` | 180,120 | 31 | 0 |
| `FuzzSignCanonicalBodyStreaming` | 149,046 | 31 | 0 |
| `FuzzSignExternalSignerResponse` | 1,378,630 | 30 | 0 |
| `FuzzSignLocalPrivateKey` | 1,456,267 | 30 | 0 |
| `FuzzSignatureExternalJSONDoor` | 33,161 | 32 | 0 |
| `FuzzSigningDomainAdmission` | 767,385 | 30 | 0 |
| `FuzzTrustedKeysExternalAdmission` | 1,617,624 | 30 | 0 |
| `FuzzVerifyRejectsEveryIndependentlyMutatedSignedField` | 278,010 | 30 | 0 |

## Measurements

Both phases use the same ten benchmark identities, benchmark source, fixture helpers, workloads, Go 1.27.1 toolchain, Linux/amd64 machine, GOMAXPROCS=8, and GOWORK=off. Each benchmark requested 30 seconds, serially after its package checks. CPU and memory profiles and the matching test binary were passed as explicit output arguments and retained for each phase. Test-only files were added/refactored between phases; complete benchmark/fixture source hashes are compared in the evidence manifest.

The harness now includes minimum-body Sign/Verify and maximum-cardinality trust verification, alongside 64 KiB/maximum bodies, canonical object scalars, and envelope JSON. Signing additionally checks the final signature against an independently assembled Ed25519 frame outside timing. Verification checks the entire retained envelope. Workloads and oracles are identical across the comparison.

This is one sample per benchmark per phase on a shared EPYC server without CPU isolation or controlled power posture. Timings are observations, not baseline/candidate distributions or a proven speed trend. Allocation budgets have deterministic normal-build ratchets. Total allocated space in the combined profile is cumulative churn across ten workloads; it is not peak resident memory or a per-operation body buffer.


| Benchmark | Before ns/op | After ns/op | B/op before → after | Allocations before → after |
| --- | ---: | ---: | ---: | ---: |
| `BenchmarkCanonicalObjectReusedScalarBuffer-8` | 237.5 | 227.1 | 0 → 0 | 0 → 0 |
| `BenchmarkSignCanonicalBody64KiB-8` | 174229 | 173446 | 1053 → 1053 | 18 → 18 |
| `BenchmarkSignCanonicalBodyMaximum-8` | 794912 | 796416 | 1059 → 1060 | 18 → 18 |
| `BenchmarkVerifyCanonicalBody64KiB-8` | 113581 | 113296 | 424 → 424 | 10 → 10 |
| `BenchmarkVerifyCanonicalBodyMaximum-8` | 739481 | 739223 | 430 → 431 | 10 → 10 |
| `BenchmarkEnvelopeMarshalJSON-8` | 10178 | 10246 | 1793 → 1769 | 30 → 27 |
| `BenchmarkEnvelopeUnmarshalJSON-8` | 18752 | 19550 | 3746 → 3681 | 48 → 47 |
| `BenchmarkSignCanonicalBodyMinimum-8` | 129418 | 126220 | 1058 → 1059 | 18 → 18 |
| `BenchmarkVerifyCanonicalBodyMinimum-8` | 69454 | 69505 | 424 → 424 | 10 → 10 |
| `BenchmarkVerifyCanonicalBodyMaximumTrust-8` | 72510 | 73528 | 905 → 905 | 25 → 25 |


The JSON encode allocation count is 30 → 27 and the decode count 48 → 47. Encoding measured 1,793 → 1,769 B/op; decoding measured 3,746 → 3,681 B/op. JSON timing samples are explicitly slower in this pair (10,178 → 10,246 ns/op encoding; 18,752 → 19,550 ns/op decoding). This release candidate claims fewer allocations, not improved latency or a proven timing regression. Larger-body signing and verification keep essentially flat allocation counts as body size grows; the maximum trust set has its separately measured bounded cost.

The baseline allocation profile identified Attest's envelope wire projection (6.84% flat) and signature decoder (0.86% flat), which led to the two local changes. The final combined allocation profile shows wire projection at 5.85% flat and removes the signature decoder from the top twenty entries. These percentages span different iteration counts across ten workloads and are supporting diagnostics, not per-call savings. Per-operation allocation results and mutation-proven allocation budgets are the direct evidence.

Both CPU profiles remain dominated by Go's SHA-256 and Ed25519 machinery. Remaining allocation costs are mainly the existing Core strict-JSON reader, byte cloning, hexadecimal encoding, and Go JSON decoding. The final sampled in-use profile shows 513 kB attributed to runtime worker allocation; that post-GC sample is not a peak-memory measurement.

## Retained evidence and review boundary

The [machine-readable execution manifest](attest_upgrade_20260909_evidence.json) binds all 33 recorded runs to their exact base revision, source inventory, full argument vector, relevant environment, raw artifact hashes, and exit. The audit verified all 142 referenced run artifacts and 4197 unique source snapshots. A single run's raw result contains its full source inventory; these are dirty-tree execution facts rather than independent acceptance receipts for a future commit.

Raw benchmark stdout/stderr, CPU and memory profiles, matching test binaries, fuzz logs, mutation snapshots, and full source snapshots remain outside Git at `/home/d/engineering-evidence/primitive/attest-upgrade-20260909`. Only this report and its compact evidence manifest are proposed for commit. The earlier September 6 Attest report remains historical and is not used as the baseline for this Linux comparison.

The user must review this coherent slice before commit. No new work in another package has begun. The next saved priority after Attest is Controlplane.
