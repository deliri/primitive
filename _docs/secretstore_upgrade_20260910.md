# Secretstore upgrade — v2026.1.55

The official SDK now owns protobuf receive configuration. Primitive's invented
4 KiB envelope allowance rejected an otherwise valid response containing
128 KiB of unknown protobuf metadata. The actual provider-owned 64 KiB secret
payload limit remains enforced after SDK decoding. No compatibility constants
or alternate client DTOs remain. The obsolete quota architecture test is
retired; the local SDK transport test exercises the shared constructor itself.

Provider payload custody is registered before projection validation so direct
projection refusal also clears already-owned bytes. Public Access validates its
request before contacting the provider: this second regression is a direct
projection boundary test, not a claim that the public RPC admitted bad requests.
Destroy errors remain joined to validation errors; fmt.Formatter acknowledges
destination failure through its no-error-return interface.

## Boundary inventory and proof

| Boundary | Proof |
|---|---|
| ParseGoogleProjectID / Validate / String | hostile grammar table and FuzzParseGoogleProjectIDSemanticClosure; independent regular-expression grammar |
| ParseGoogleSecretID / Validate / String | hostile grammar table and FuzzParseGoogleSecretIDSemanticClosure; independent regular-expression grammar |
| Positive project/version numeric constructors and closed latest selector | exhaustive zero/nonzero and representability-boundary classes, request field/enum validation |
| NewValue, CopyBytes, Text, Validate, Destroy, Format | TestNewValueLayerTriad, ownership and redaction tests, FuzzNewValueSemanticClosure; exact bytes, UTF-8 identity and shared destruction |
| Provider resolved name | FuzzResolvedGoogleReferenceSemanticClosure; canonical numeric identity and typed zero refusal |
| Provider response projection | TestOfficialGoogleResponseLayerTriad and FuzzOfficialGoogleResponseSemanticClosure; independent size/CRC facts, exact request/reference/material, zero result and cleared provider bytes on refusal |
| GoogleReader construction / Access / Close | TestGoogleSDKTransportLayerTriad, cancellation/close lifecycle test, FuzzGoogleSDKResponseSemanticClosure; real official SDK over local gRPC, exact typed result or preserved native status |
| All eight production structs | TestProductionStructDataFlowInventory; no new production carrier |

The SDK fuzz callback emits typed protobuf through a local provider for every
iteration and verifies the real SDK/custody path. Single-fact mutations cover
missing checksum, wrong checksum, zero version, absent payload and provider
refusal; baseline, empty and exact/over-limit payloads pin admission. The direct
projection fuzz oracle no longer invokes NewValue to decide acceptance or leaves
an extra oracle Value alive. Response tables no longer count byte-spelling and
irrelevant version permutations as distinct boundaries. This package has no
producer/classifier split, ledger, manifest, durable writer, or product policy.
No classifier quota or evidence-file triad is claimed for it.

## Red state and retained execution

Evidence root on furnace:
`/work/engineering-evidence/primitive/secretstore-upgrade-20260910`.
Each result.json binds argv, revision, dirty source hashes/content, toolchain,
environment, cache posture, stdout/stderr digests, exit and emitted counts.
Attempts are append-only. The initial transport-red captures both regressions
against b8d474a9e63a690e8fa927391a71574a5b141b47 plus the behavior-preserving
constructor seam. transport-green was inadvertently run before the sandbox-
blocked upload completed; its failure remains retained. transport-green-2 and
strengthened-tests pass. The checksum-mutation-red disables only CRC comparison
and fails the real SDK fuzz seed; mutations.py restores the original bytes.
Initial lint diagnostic failures and strict errcheck findings remain retained.
Final lint, vet, staticcheck, strict errcheck and full module build pass.

The final committed phase records cache-bypassed package race tests and release
coordinate tests, then unchanged 30-second Value benchmarks with CPU/memory
profiles and binaries, then each of six fuzz targets once for a 10-second budget.
These final phase outcomes are in the external receipts, not inferred from this
plan. The original and candidate Value workloads are identical; Value allocation
behavior did not change and no performance improvement is claimed.

## Scope limits

SDK protobuf decoding owns a complete message; Value owns exactly the admitted
payload. This is explicit materialized ownership, not O(1) network parsing.
Authentication, project-ID/number translation and native retry policy remain
Google SDK/provider responsibilities. CRC32C is not authentication. Tests disable
SDK retries to prove one attempt and use synthetic secrets, not live credentials.
The integration-tag test compiles but live Secret Manager access is not run.
The module build is not a full-module test acceptance claim; previously reported
unrelated core/awsidentity/deploy test failures were not reclassified or erased.
Historical missing test denominators stay unknown rather than passed.
The author workstation's execution and local manifest verification do not
constitute independent acceptance. Consumer migration remains separate.
