# Chitauth upgrade — v2026.1.57

RequestDocument.Validate now binds the query envelope signer to the installation
certificate's nominated DeviceKey, alongside the existing exact build and scope
bindings. Assemble and UnmarshalJSON previously admitted a same-build foreign
device query that Verify could never authenticate. Refusal now occurs at the
typed wall, preserves Core's binding identity and releases no document.
ControlRoute now validates the complete credential before returning a route.
ControlNonce and ControlRevision remain direct projections of signed fields.
Verify returns a zero proof on any final validation failure.

The outer RequestDocumentJSONMaximumBytes ceiling and its compatibility
assumptions are retired. Strict JSON uses Core's extensible document extent;
duplicates, unknown members, invalid types and invalid nominal documents remain
refused. A real signed query surrounded by 1 MiB of legal whitespace decodes to
the exact original credential and authenticates through the real verifier.
No new runtime, policy, persistence, transport or dependency was added.

## Public boundary and data-flow inventory

| Surface | Evidence |
|---|---|
| RequestDocument/RequestAssembly.Validate, Assemble | exact nominated signer, build and scope; foreign-device/absent-certificate zero refusal |
| ControlRoute/ControlNonce/ControlRevision | complete credential gate before route; signed-field projections remain direct |
| RequestDocument.MarshalJSON/UnmarshalJSON | local JSON triad, strict malformed cases, receiver preservation, canonical round trip and second identical encoding; real typed production seeds |
| Verification.Validate, Verify, Verified.Validate/Payload | authority certificate verification followed by nominated-device query verification; exact payload or typed refusal with zero proof |
| IssueResponse/ResponseIssuance.Validate | real Controlplane issuance, exact catalog projection, invalid signer/authority, every sibling family and zero-input refusal |
| VerifyResponse/ResponseVerification.Validate | real signed sibling-family refusals, header expectation substitutions, foreign authority and zero-proof behavior; response authority fuzzing |
| Six production structs | existing compiler-visible data-flow inventory, with checked AST assertions; no new production carrier |

Three fuzz targets cover the local doors: credential JSON decoding with
independent certificate/query verification; targeted signed nonce/selection/digest
mutations forced through structural decode and the real verifier; and response
JSON plus trusted-client/nonce/account/family mutations. An admitted response
must expose the exact signed header and catalog seed and pass Chit's independent
catalog verifier against the original query and pinned authority key. Raw JSON
and signed semantic mutations are separate proof surfaces.

Padded success rows that only varied opaque key/offering markers are removed.
The request admissions retain the two load-bearing selection variants. Each
response success path proves its exact catalog once. Obsolete whitespace-ceiling
rows and helper are retired; hostile grammar and closed-family rejection cases
remain. This is a typed authentication handoff, not a producer/classifier
classification system. No synthetic 50-case classifier matrix is claimed.

## Retained execution and mutations

Base commit: 0be87c57ba3c07d41cc8a4a4c848722f3ef7e4a1.
Evidence root on furnace:
`/work/engineering-evidence/primitive/chitauth-upgrade-20260910`.
Each execution preserves revision, dirty source snapshots and hashes, complete
argv, toolchain/environment, stdout/stderr digests, emitted counts and exit.
Cache reuse is disabled for tests, benchmarks and fuzz campaigns. Historical
missing not-run/unavailable denominators remain unknown, not passed.

binding-red demonstrates foreign-device admission, missing-certificate route
admission and the outer whitespace quota against original production. The first
binding-green run exposed a test fixture attempting to marshal a zero certificate;
the corrected missing-field fixture uses a typed request-only envelope, and the
failed attempt remains retained. The lint wording correction is retained too.
green-2 and strengthened-tests pass race tests. receiver-mutation-red compiles
but discards an accepted document and fails the JSON fuzz callback.
family-mutation-red bypasses the Chitauth family gate and is killed by real,
authentically signed sibling-family responses. Both mutations are discarded.

The final committed phase runs package/release-coordinate race tests, lint,
vet, staticcheck, strict errcheck and full module build. The unchanged Assemble
workload runs once for 30 seconds before and after with CPU/memory profiles,
matching binaries, fixed fixture, host/toolchain and single-CPU setting. Each of
three fuzz targets then runs once with one worker for 10 seconds. Final outcomes
and exact revision bindings live in the external records. One benchmark
observation per phase is not a performance trend.

## Memory and scope limits

Chitauth owns complete typed credentials and caller-owned JSON byte slices.
Removing the outer quota does not make the composed JSON decoder an O(1) stream.
Core's strict decoder and the nested Chit/Controlplane decoders still own their
materialized values, depth/member rules and nominal constraints; their separate
streaming audits are not closed by this wrapper change. Authentication does not
load object payloads or perform network I/O.

No live service or consumer migration was performed. Full-module build success
is not full-module test acceptance; previously reported unrelated
Core/AWSidentity/Deploy failures remain separate. Local manifest verification and
author execution do not constitute independent acceptance.
