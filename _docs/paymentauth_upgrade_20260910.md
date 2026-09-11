# Paymentauth upgrade — v2026.1.59

RequestDocument.Validate now requires the payment query envelope's signer to
match the installation certificate's nominated DeviceKey, alongside the existing
build and receipt-scope bindings. Previously Assemble admitted a same-build
foreign-device request that Verify could not authenticate. Admission now refuses
that contradiction with Core's typed response-binding identity and zero output.
ControlRoute validates the credential before returning a route. Verify returns a
zero proof if final proof validation fails.

This is a mechanical agreement between the certificate and query, with no
payment policy or provider knowledge. No production struct, compatibility layer,
dependency or alternate representation was added.

## History and public-boundary inventory

The v2026.1.44 Payment change (67875b5) already removed Paymentauth's outer JSON
quota and added its large-whitespace extent test. This review preserves that
work. Earlier generic benchmark reports are historical, not proof of this sweep.

| Surface | Local proof |
|---|---|
| RequestDocument/RequestAssembly.Validate, Assemble | nominated signer, exact build and scope; foreign key, account, offering and absent-certificate refusals; zero output |
| ControlRoute, ControlNonce, ControlRevision | complete credential gate before route; direct signed nonce/revision projections |
| RequestDocument.MarshalJSON/UnmarshalJSON | typed production seed, strict hostile grammar, populated/zero receiver preservation, real member reordering, canonical round trip and identical second encoding |
| Verification/Verify/Verified.Validate/Payload | authority certificate and nominated-device query authentication; exact payload or typed refusal and zero proof |
| IssueResponse/ResponseIssuance.Validate | real signed catalog projection, every sibling family, invalid authority/body/assessment and zero issuance |
| VerifyResponse/ResponseVerification.Validate | real signed sibling-family refusal, expected nonce/account/installation/offering/family substitutions, time rollback, foreign authority and zero-proof behavior |
| Catalog and receipt handoff | outer proof exposes exact catalog, Payment.VerifyCatalog authenticates against original query, Payment.Verify authenticates the exact settled receipt |
| Six production structs | existing compiler-visible data-flow inventory, checked AST assertions; no new production carrier |

The large-whitespace triad retains a real authenticated query surrounded by
2 MiB of whitespace, trailing/truncated hostile data and whitespace-only refusal.
JSON member-order fixtures assert the wire actually changes. Opaque identity
marker success rows are removed; request admission keeps the two distinct
all/specific selection arms. Response issuance and verification each prove the
exact settled receipt instead of repeating synthetic key-marker permutations.

Three fuzz targets cover the public external agreement:

- FuzzCredentialedPaymentQueryJSONSemanticAndAuthorityClosure: hostile JSON,
  receiver preservation, canonical closure, real certificate/device verification
  and independently checked exact query payload.
- FuzzPaymentCredentialSignedFacts: a production-encoded signed seed and forced
  nonce, selection, page limit, signer, length and digest changes. Each mutation
  must change a consumed typed fact; structural admission either rejects the
  nomination or preserves the changed document for real signature refusal.
  The raw-input arm authenticates only the genuine seed.
- FuzzPaymentResponseAuthorityClosure: real signed catalog seed plus raw JSON
  and forced nonce/account/family/trusted-client mutations. Refused JSON preserves
  the seeded receiver; refused authentication exposes neither header nor body;
  accepted output passes the independent catalog and settled-receipt verifiers.

This wrapper is not a producer/classifier system, so no synthetic 50-case matrix
is claimed. It does not own durable evidence files, ledger replay, manifests,
CLI execution or reporting. Those local triads remain in their owning packages.
No live payment provider, network service or money movement is involved.

## Retained evidence

Base revision: f6eedfe254ed888996ff1a8639898fd6938099cd.
Evidence root on furnace:
`/work/engineering-evidence/primitive/paymentauth-upgrade-20260910`.

The binding-red run against original production fails foreign-device admission
and absent-certificate routing. A deliberate family-gate bypass fails authentic
sibling response tests; a receiver-discard mutation fails the JSON fuzz oracle.
Both mutations are restored. The initial sentinel-comparison lint finding and
its corrected run remain separate retained attempts.

The committed phase runs uncached race tests for Paymentauth and release
coordinates, the doctrine linter, vet, staticcheck, strict errcheck and full
module build. The unchanged Assemble benchmark runs once per before/after phase
for 30 seconds, with the same fixture, host, toolchain and CPU setting, retaining
CPU/memory profiles and binaries. Each fuzz target runs once for 10 seconds with
one worker after the benchmark. One sample per phase is not a performance trend.
Exact results, commands, revisions, clean/dirty facts, source snapshots, counts,
exit status and output/artifact hashes are retained externally.

Historical missing not-run/unavailable denominators remain explicitly unknown,
never passed. Local manifest/disk verification validates the retained bundle;
author-run evidence is not independent acceptance.

## Memory and remaining scope

Paymentauth consumes complete typed credentials and caller-owned JSON slices.
Its strict composed decoder is not an O(1) streaming parser. Core, Payment and
Controlplane own nested materialization and nominal limits. The existing outer
extent repair does not close those owners' separate streaming work. Paymentauth
adds no whole-payload transport or persistent product state.

Consumer migration and independent acceptance remain separate. Full-module build
success is not full-module test acceptance; known unrelated Core/AWSidentity/Deploy
test failures retain their separate status. Retrievalauth is next in the queue.
