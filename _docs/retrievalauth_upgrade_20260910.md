# Retrievalauth upgrade — v2026.1.60

RequestDocument.Validate now binds the retrieval request envelope signer to the
installation certificate's nominated DeviceKey, alongside the existing build
and scope bindings. Assemble previously admitted a same-build foreign-device
request that Verify could not authenticate. Admission now returns Core's typed
retrieval-binding refusal and no document. ControlRoute validates the complete
credential before projecting a route. Final verification failure returns zero
proof.

The agreement remains a mechanical certificate/request binding. No provider
policy, product state, runtime, dependency, compatibility shim or production
struct was introduced.

## History and public-boundary inventory

The v2026.1.41 Retrieval repair (b459236) already removed outer document quotas
and installed request semantic fuzzing. This review preserves those changes.
Historical generic benchmark reports do not establish this package's closure.

| Surface | Evidence |
|---|---|
| RequestDocument/RequestAssembly.Validate, Assemble | exact nominated signer/build/scope, absent and foreign facts, exact document or typed refusal and zero output |
| ControlRoute/ControlNonce/ControlRevision | whole credential validation before route; signed nonce and revision projections |
| RequestDocument.MarshalJSON/UnmarshalJSON | strict hostile grammar, receiver preservation, canonical fixed point, large outer/nested whitespace, genuine member reordering |
| Verification/Verify/Verified.Validate/Document | independent authority certificate and device request authentication, signed substitutions, exact document or zero proof |
| IssueResponse/ResponseIssuance.Validate | real retrieval grant projection and signed response; every sibling family refuses issuance; existing zero-input validation/issuance tests retained |
| VerifyResponse/ResponseVerification.Validate | real response authority verification, exact typed grant, every authentically signed sibling family refused, nonce/offering/trust substitutions and absent proof |
| Response → retrieval grant handoff | real receipt issuance/verification, manifest accumulation and membership seal, chit issuance/verification, grant issuance and independent VerifyGrant after response authentication |
| Existing production structs | compiler-visible data-flow inventory retained with checked AST assertions; no new production carrier |

The response fixture uses a fixed nonempty evidence payload. Receipt facts,
manifest membership, chit and grant are produced by the real typed production
APIs. An inert signed URL supplies the capability representation; no object-store
request or download is performed. The outer response proves exact payload,
attestation and capability commitment, then independently verifies the grant
against the original request, chit and manifest membership. It does not claim
provider transport or object-download coverage.

Two fuzz targets cover the external doors:

- FuzzRequestDocumentExternalDecoderAndVerifier: production-encoded credential,
  canonical closure and receiver preservation, real verifier, plus typed nonce,
  signer, signature, digest and certificate substitutions. Each mutation must
  actually change the signed facts. Only the genuinely signed seed may verify.
- FuzzRetrievalResponseSemanticAuthorityClosure: real signed grant response,
  raw JSON and forced nonce/family/offering/trusted-client substitutions. Refused
  JSON preserves the authenticated seed; refused verification exposes neither
  header nor grant; accepted output must match the signed seed and pass the
  independent grant verifier.

Opaque offering/key-marker repetitions are removed from the assembly and
verification positive cases. The reordered-JSON fixture previously emitted the
canonical order; it now changes the order and asserts that change. Whitespace
padding fails if it cannot enlarge its input instead of silently returning an
unchanged fixture. Duplicate empty fuzz seeds are removed.

This is a typed authentication wrapper, not a producer/classifier system; no
synthetic 50-case classification matrix is claimed. It owns no durable evidence
writer, manifest file, ledger replay, CLI or reporter. Their local behavioral
triads remain with the packages that implement those effects.

## Execution evidence

Base revision: 65ed20b14cc7c1451c3a6790d2875f978ea115fd.
Evidence root on furnace:
`/work/engineering-evidence/primitive/retrievalauth-upgrade-20260910`.

The original-production binding-red run demonstrates foreign-device assembly
and absent-certificate route admission. A deliberate response family-gate
bypass fails the authentic sibling-family test. A receiver-discard mutation
fails the request JSON fuzz oracle. Mutations are restored and all attempts
remain retained.

The committed phase runs cache-disabled race tests for Retrievalauth and release
coordinates, the doctrine linter, vet, staticcheck, strict errcheck and full
module build. The unchanged Assemble benchmark runs once per before/after phase
for 30 seconds on the same host/toolchain/CPU setting, retaining CPU/memory
profiles and binaries. Each fuzz target runs once for 10 seconds with one worker,
serially after the benchmark. One benchmark sample per phase is not a trend.

Exact commands, revisions, clean/dirty facts, source snapshots, output hashes,
byte counts, emitted test counts and exits are retained externally. Historical
missing not-run/unavailable denominators remain unknown rather than passed.
Local manifest/disk verification checks the retained evidence; it does not issue
independent acceptance.

## Memory and remaining scope

Retrievalauth consumes complete typed credentials and caller-owned JSON slices.
Its composed decoder is not an O(1) streaming parser. Core, Retrieval and
Controlplane own the nested nominal values and their materialization rules;
removing an outer quota does not close those owners' separate streaming work.
The wrapper introduces no object-payload buffering or product state.

Independent acceptance and consumer migration remain separate. Full-module build
success is not full-module test acceptance; previously reported unrelated
Core/AWSidentity/Deploy failures retain their separate status. Upgrade is next in
the package queue.
