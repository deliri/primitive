# Distributionauth upgrade — v2026.1.58

All four received credentials now bind the request/completion envelope signer to
its installation certificate's DeviceKey, in addition to the existing exact
build binding. Assembly and JSON admission previously accepted same-build foreign
signers that the authentication path could not verify. These contradictions now
return Core's typed response-binding refusal without a document. ControlRoute
validates the complete credential before projecting a route; final verification
failure returns a zero proof.

The issue-only publication completion projection enforces the same nomination.
Distribution exposes its structurally validated projection Signer as a nominal
public key so the wrapper can compare the compiler-owned agreement directly.
The accessor does not authenticate that key. No policy, runtime, provider
coupling, compatibility adapter or new production struct was introduced.

Outer RequestDocumentJSONMaximumBytes and
PublicationCompletionDocumentJSONMaximumBytes quotas are retired. Core's strict
extensible JSON admission retains grammar and nominal validation. A real typed
credential with 1 MiB of legal outer whitespace remains admissible. Nested
Distribution, Release, Objectstore and Controlplane contracts retain ownership
of their own bounds and representation rules.

## Public boundary and data-flow inventory

| Local surface | Proof |
|---|---|
| Update, Upgrade, Publication and PublicationCompletion document/assembly Validate and Assemble | nominated signer and build, foreign signer/build refusal, absent certificate, exact output or zero refusal |
| Four ControlRoute methods; ControlNonce and ControlRevision | whole credential gate before route, direct signed nonce/revision projections |
| Four received document MarshalJSON/UnmarshalJSON doors | strict hostile grammar, transactional receivers, canonical round trip, genuinely reordered members, large legal whitespace, typed production fuzz seeds |
| Four Verification/Verify and verified payload/grant accessors | real authority certificate plus nominated signer authentication; retained inner request/completion/grant binding tests; zero refusal |
| PublicationCompletionProjectionAssembly, projection MarshalJSON, Distribution projection Signer | real deploy/release completion issuance through a local provider transport, nominal signer lookup, wrong device and missing certificate refusal, exact received completion |
| Material, Publication, PublicationCompletion, Update and Upgrade response issuance/verification | each real typed body crosses its public wrapper and Controlplane signing/verification; exact signed header and body, wrong nonce, authentic sibling-family rejection, absent-input rejection |
| Existing production structs | package AST inventory retained and checked assertions added; no new carrier |

Nine fuzz targets cover the four credential JSON doors and five response sockets.
Credential callbacks preserve populated receivers on refusal and run the real
certificate/device verifier after structural admission. Response callbacks run
public family-specific verifiers and compare authenticated headers and body facts
against the issuer's typed seed. Receive-only grants are compared through exact
payload/attestation and capability commitments, without adding serialization
shims. Rejected response JSON must leave the previously admitted seed verifiable;
unauthenticated input must not expose a valid proof. Material body custody is
destroyed after each decoded body is observed.

The response harness proves the outer authentication socket. Its Update body is
an independently signed, structurally valid document, not an installed-build
selection scenario. Inner release selection/download execution remains owned by
Distribution's tests. The existing publication response test additionally runs
VerifyPublicationGrant on the authenticated body. No live provider or network
service is used; the completion fixture drives the real writer/issuer through an
isolated local HTTP transport that consumes the upload streams.

Opaque key/offering marker rows and obsolete whitespace-ceiling rows were
removed. Member-order fixtures now assert that the mutation actually changes the
wire. This is an authentication agreement, not a producer/classifier system; no
padded 50-case classifier matrix is claimed. The package does not own a durable
evidence writer, manifest, ledger or reporter, so those layer triads remain with
their owning packages.

## Execution evidence

Base revision: 15c3f2da5777143462a39b8e3f747e724b4603e3.
Evidence root on furnace:
`/work/engineering-evidence/primitive/distributionauth-upgrade-20260910`.

The original-production binding-red and projection-red runs demonstrate the
credential and issue-only nomination defects. Five separate deliberate mutations
bypassing each response family's verifier gate fail on an authentically signed
sibling response. A projection signer mutation fails real completion assembly;
a receiver-discard mutation fails the semantic credential fuzz seed. Mutations
are restored. Compilation/fixture mistakes, the non-vacuous reorder failure and
lint failures remain append-only attempts, followed by their corrected results.

The committed phase runs race tests for Distributionauth, Distribution and the
release coordinate; the doctrine linter, vet, staticcheck, strict errcheck and
full module build; the unchanged AssembleUpdate benchmark once for 30 seconds
with CPU/memory profiles and retained binary; then each fuzz target once for
10 seconds with one worker. The before benchmark uses the same identity,
workload, host, toolchain and CPU setting. One sample is not a performance trend.
Final outcomes and committed revision bindings are in the external run records.

Each run records complete argv, toolchain, scope, cache posture, source hashes,
stdout/stderr byte counts and digests, emitted test counts and exit. Test cache
reuse is disabled. Historical missing not-run/unavailable denominators remain
unknown, never passed. A local manifest/disk check validates retained evidence;
this author-run evidence does not establish independent acceptance.

## Memory and remaining scope

These wrappers own complete typed credentials and caller-supplied JSON slices.
Removing an outer quota does not turn composed JSON decoding into an O(1)
stream. Core and nested protocol owners still materialize their nominal values;
their separate streaming work is not closed by this wrapper review. Object
payload execution continues through the standard streaming interfaces.

Consumer migration and independent acceptance remain separate. Full-module build
success is not full-module test acceptance; previously reported unrelated
Core/AWSidentity/Deploy test failures retain their separate status. Paymentauth
is next in the package queue.
