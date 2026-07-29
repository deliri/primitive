# Attest package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole recon report for archived package `attest`. The archive,
Primitive-internal demand, and current Kernel, Witness, Bug, and Peachfuzz
signing engines are integrated below. The package is a strong 2026 ownership
candidate, but the archived implementation is not admissible unchanged.

## Evidence boundary


- Primitive archive HEAD:
  `d046f7b675fcb797398d7cdc87b5504f43978056` (`Harden capability
  inventory evidence`, 2026-07-27).
- Archived `attest` tree:
  `1c0dae431ea17b84cb0cbf2a104edad7f46c9546`.
- Last commit that changed `attest`:
  `d259789e87bcadb829c5ffac72c6c91ccc604098`
  (`centralize constants and close capabilities`, 2026-07-25).
- Package introduction:
  `743e86b6f3b1b4cbfc327577ac8d155ee3af1886`
  (`add typed attestation primitive`, 2026-07-24).
- Kernel HEAD:
  `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`; committed Primitive pin
  `0df2954a2d911a5d7d775691d023d569affa2c20`; dirty working pin
  `e8b7172161a4994efcb7f092113e23c28928da43`.
- Witness HEAD:
  `b9629af57b7058b68982be5d3b282be440b1e76e`; Primitive pin
  `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9`.
- Bug HEAD:
  `39ce96242240d7174d562c90bb255860946595dc`; Primitive pin
  `388e593231a28434f6faae9f0ab9dffcf332dfc3`.
- Peachfuzz HEAD:
  `2b2d080c455edaadf88502c1c253845605a4336a`; Primitive pin
  `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75`.

The committed Kernel pin and all three tool pins predate `attest`; none contains
`attest/SPEC.md`. Kernel's dirty pin contains the exact archived `attest` tree,
but current Kernel production has no direct import. Witness, Bug, and Peachfuzz
also have no direct production import. Copied `protocol/_foundation_source`
trees and vendored Primitive code were inspected as historical evidence but
are not independent capability owners.

The archive itself had one unrelated untracked file,
`core/api_http_boundary_hostile_test.go`, during this read-only interview. It
does not alter the committed `attest` tree or the findings below.

## Capability ownership

`attest` owns one narrow cryptographic protocol: seal and verify a bounded
typed canonical fact with Ed25519. It owns:

- a consumer-defined, self-reconstructing signing domain;
- a typed canonical-body streaming contract;
- exact body extent and SHA-256 computation;
- a length-delimited, generation-separated fixed signing frame;
- Ed25519 signing and mandatory post-sign self-verification;
- a bounded immutable caller-selected trust set;
- strict canonical detached-envelope JSON; and
- an unforgeable proof-carrying verification result.

It does not own body meaning, product schemas, trust-anchor selection, key
generation or persistence, rotation, revocation, time, certificate chains,
transparency, transport, filesystem durability, or commercial acceptance
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:6`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:17`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:234`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:350`).

This is a real cross-package primitive, not a product-named wrapper. The body
owner supplies `CanonicalBody[D]`; the domain owner supplies a closed `D`; and
the composition root supplies a concrete signing key or trusted public keys.
The compiler prevents a body from being signed under a second independently
chosen string domain
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/contracts.go:10`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/contracts.go:23`).

## Archive evidence

### Archived architecture worth preserving

### Minimal intent surface

The public operations are only `NewTrustedKeys`, `Sign`, and `Verify`.
The only exported supporting types are `SigningDomain`, `CanonicalBody`,
`SignRequest`, `VerifyRequest`, `Envelope`, `TrustedKeys`, and `Verified`.
There is no raw-byte signing verb, map payload, loose domain string, network
client, clock, filesystem helper, callback signer, or compatibility alias
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/public.go:1`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/architecture_test.go:20`).

That surface is materially stronger than the local generic signers found
downstream: the caller cannot forget domain separation, body extent, body
digest, signer binding, authority membership, or post-sign verification.

### Compiler-owned domain and body binding

`SigningDomain[D]` is self-referential. Its canonical text must round-trip
through `ParseCanonicalText`; `attest` passes a bounded copy to the parser and
then independently reprojects the returned value to the original token. The
internal fixed `domainToken` rejects nonzero tail bytes and impossible lengths,
so forged in-memory storage cannot manufacture an alternate wire spelling
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/domain.go:11`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/domain.go:36`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/domain.go:82`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/domain.go:100`).

`CanonicalBody[D]` provides `Validate`, `AttestationDomain`, and
`WriteCanonical`. Canonical bytes stream once through a hard-limit SHA-256
writer. The package retains only domain, byte count, and digest; it does not
retain or duplicate a body proportional to its size
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/canonical.go:20`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/canonical.go:73`).

Hostile body and domain callbacks are panic-contained. Callback errors are
collapsed into Core-owned identities rather than preserving attacker-controlled
`Error`, `Is`, `As`, or `Unwrap` behavior
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/canonical.go:103`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/domain.go:84`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/guard.go:10`).

### Exact cryptographic frame

The signed message is SHA-256 over:

1. `foundation-attestation-2026`;
2. one zero separator;
3. a big-endian `uint16` domain length and domain bytes;
4. the raw 32-byte signer public key;
5. a big-endian `uint64` body length; and
6. the raw 32-byte body digest.

Plain Ed25519 signs that typed 32-byte frame digest. Variable-length fields have
explicit lengths, and signer, body extent, domain, and generation are all
cryptographically bound
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:169`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/frame.go:32`).

`Sign` derives the public key, canonicalizes once, signs, verifies the new
signature against the derived public key, and only then returns a validated
envelope. `Verify` validates authority before canonicalizing the body, compares
domain/length/digest, reconstructs the same frame, and returns private
proof-carrying state only after signature success
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/operations.go:20`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/operations.go:50`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/operations.go:85`).

### Closed authority and proof capabilities

`TrustedKeys` uses fixed private storage for at most 16 distinct validated keys,
copies caller input, and validates the unused tail. There is no mutable slice or
unbounded map
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/trusted.go:9`).

`Verified[D]` has private state, rejects its zero value, and returns only a copy
of the verified envelope. A consumer cannot construct a successful proof using
a struct literal
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/envelope.go:44`).

### Dependency and complexity closure

Production depends only on the standard library and `core`. It contains no
product import, sibling Primitive capability, I/O owner, network, clock,
process, cache, retry, or background goroutine. Body processing is O(n) time
and O(1) auxiliary memory; the trust scan is O(k) with `k <= 16`; frame work is
fixed-size
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:17`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:289`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/architecture_test.go:72`).

Core currently owns the shared frame constants, field-name constants, limits,
and stable error hierarchy
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/attest_constants.go:3`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:28`). It also owns the opaque private-key value and the
low-level `SignSHA256` execution. An archive-wide ratchet proves
`attest/operations.go` is the sole Primitive production caller
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/ed25519_signing_key.go:12`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/ed25519_signing_key.go:56`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/architecture_test.go:112`).

### Primitive-internal demand

The archived package is already the shared signature engine for ten independent
Primitive capabilities:

| Consumer | Role | Representative call sites |
| --- | --- | --- |
| `callbudget` | sign and verify budget documents | `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/document.go:170`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/verified.go:37` |
| `controlstate` | sign and verify aggregate snapshots | `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:626`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:68` |
| `gate` | verify signed gate documents | `archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/verified.go:33` |
| `lease` | verify signed lease documents | `archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/verified.go:33` |
| `rate` | sign receipts; verify and retain signer authority | `archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/receipt.go:295`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/receipt.go:354`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/record.go:151` |
| `receipt` | sign and verify page/evidence documents | `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/page_document.go:115`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/page_document.go:214`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/documents.go:270`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/documents.go:421` |
| `register` | sign device proof; verify declaration and protocol facts | `archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/values.go:459`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/values.go:481`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/protocol.go:584` |
| `release` | sign and verify manifest and Latest documents | `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/manifest.go:276`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/manifest.go:307`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/latest.go:190`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/latest.go:230` |
| `status` | sign and verify status documents | `archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/document.go:36`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/verified.go:47` |
| `submission` | carries typed trust through the upload composition | `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/client.go:1` |

Thirty archived production files import `attest`. `update` and `upgrade`
do not directly import it in production; they consume already verified release
facts. This dependency direction is correct: protocol packages own their
domains and canonical bodies, while `attest` owns only the common sealing and
verification mechanics.

## Consumer evidence

### Kernel

Current Kernel does not import Primitive `attest`. Its dirty Primitive pin
does contain the exact archived package, so Kernel is a pending adopter rather
than an independent implementation user.

Kernel's local `professor` package is a broad crypto isolation boundary. It
provides raw `[]byte` Ed25519 `Signer` and `Verifier` interfaces, accepts a seed
or private key, validates the derived public half, and self-validates on each
operation
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:professor/professor.go:1`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:professor/professor.go:112`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:professor/professor.go:148`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:professor/professor.go:170`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:professor/professor.go:224`).
That isolation boundary is useful, but it does not provide typed domains,
canonical-body closure, fixed framing, trust sets, or proof-carrying results.
Its `PrivateBytes` method also exposes a copy of raw private material
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:professor/professor.go:197`), which must not be promoted into
Primitive `attest`.

No current Kernel production call site uses `professor.NewSigner` or
`professor.NewVerifier`. The reusable gem is the architectural demand for one
auditable crypto boundary, not the raw-message API or ceremony metadata.

### Witness

Witness owns a live product-specific machine-attestation system in
`internal/attest`. Its valuable mechanics are:

- a `TrustBackedSigner` callback capable of wrapping an external signer;
- configuration-time challenge signing to prove callback/public-key agreement;
- verification of every runtime signature before it escapes;
- an explicit trust-mode policy that forbids a repository-local anchor for
  external claims;
- a compiler-pinned tool-bundle anchor catalog; and
- bounded OpenSSH/custom-PEM Ed25519 key parsing.

Evidence:
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/attest.go:43`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/attest.go:82`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/attest.go:226`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/attest.go:285`;
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/trust_mode_policy.go:147`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/trust_mode_policy.go:175`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/trust_mode_policy.go:213`;
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/toolbundle_anchor.go:10`; and
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/key.go:53`.

The machine-attestation claim, trust-mode choice, project-local parent walk,
tool-bundle catalog, signer name, and binding sidecar remain Witness product
facts. The external signer callback is a genuine capability gap to preserve at
composition: archived `attest` deliberately accepts only a concrete in-memory
`core.Ed25519SigningKey`. It should not be widened casually into an arbitrary
callback. Primitive 2026 needs an explicit decision whether external
KMS/HSM-style signing belongs in a separate key-provider capability whose
output is still constrained to the exact `attest` frame.

### Bug

Bug has a certified writer-proof chain rather than a generic attestation
primitive. A locally generated writer seed signs an operation digest; a
server-signed writer certificate binds device and writer identity; verification
checks repository, operation, device, writer, certified time interval, and both
signature levels
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/writer_key.go:17`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/writer_key.go:66`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/writer_key.go:82`;
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/writer_attestation.go:44`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/writer_attestation.go:71`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/writer_attestation.go:94`).

`WriterProofClaim` binds bug ID, operation, commit, proof hash, and occurrence
time. `WriterProofArtifact` binds the claim digest to the writer attestation and
certificate digest. Paths derive from typed identities, and persistence is
create-only or exact-byte idempotent
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/writer_proof.go:74`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/writer_proof.go:114`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/writer_proof.go:153`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/writer_proof.go:214`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/writer_proof.go:242`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/writer_proof.go:318`).

Those certificate, revocation, repository, operation, and time-window facts
remain Bug/license policy. The reusable generic signing, canonical framing,
trust membership, and proof result should move to Primitive. The product gem
to preserve is the explicit certificate -> writer attestation -> immutable
proof chain and its exact-content collision refusal, not Bug's raw Ed25519 seed
and legacy `core.Signed` mechanics.

### Peachfuzz

Peachfuzz currently builds `SignedRunEvidence` on the older generic
`core.Signed[RunEvidence]`. It additionally derives `MachineID` from the signing
public key and rejects evidence whose claimed machine namespace does not match
that signer
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/signed_run_evidence.go:10`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/signed_run_evidence.go:24`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/signed_run_evidence.go:42`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/signed_run_evidence.go:60`).

Its archive layer signs the typed run evidence, emits exact validated JSON,
addresses it by digest, and on decode jointly proves strict JSON, signature,
canonical bytes, and object digest
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/run_evidence.go:15`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/run_evidence.go:57`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/run_evidence.go:76`).

Machine namespace derivation, run-evidence schema, upload descriptor, and
archive custody stay Peachfuzz facts. The portable gem is the combined
signature/canonical-byte/content-address proof at archive ingress. Migration
must preserve those exact durable bytes deliberately; the new detached
`attest.Envelope` is not wire-compatible with the old embedded
`key_id/signature/body` record by accident.

## Strong mechanics and proof

### Testing-protocol and hostile proof

The archive read the project-local `_docs/testing_protocol.md`. Relevant
requirements are behavioral evidence, retained red/green or a named ratchet
reason, hostile boundaries, layer-triad coverage, strongest typed error
assertions, valid structural ratchets, and semantic fuzz oracles
(`foundation@working-tree:_docs/testing_protocol.md:149`, `foundation@working-tree:_docs/testing_protocol.md:194`, `foundation@working-tree:_docs/testing_protocol.md:466`, `foundation@working-tree:_docs/testing_protocol.md:518`,
`foundation@working-tree:_docs/testing_protocol.md:644`, `foundation@working-tree:_docs/testing_protocol.md:1008`, `foundation@working-tree:_docs/testing_protocol.md:1210`).

The test suite is unusually strong:

- a deterministic real Ed25519/SHA-256 vector pins signer, body length, body
  digest, frame digest, and signature
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/vector_test.go:5`);
- the canonical-body table covers zero, one, irregular chunks, byte-at-a-time,
  maximum-minus-one, maximum, maximum-plus-one, ignored writer errors, callback
  errors/panics, and a nil body
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/canonical_hostile_test.go:11`);
- domain tests cover alphabet and size extremes plus validation and marshal
  panics; retained writers are proven closed
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/canonical_hostile_test.go:84`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/canonical_hostile_test.go:107`);
- verification tables tamper body, domain, extent, digest, signature, signer,
  authority, and forged trust storage using typed `errors.Is` assertions
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/verification_hostile_test.go:10`);
- trust tests cover cardinality boundaries, duplicates, zero keys, and caller
  slice mutation
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/trusted_hostile_test.go:10`);
- JSON tests cover strict round trips, bounds, receiver preservation, missing,
  unknown, duplicate, reordered, escaped, wrong-type, trailing, null, and
  impossible domain-storage inputs
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json_hostile_test.go:14`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json_hostile_test.go:115`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json_hostile_test.go:195`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json_hostile_test.go:439`);
- public API, struct inventory, dependency direction, and raw-signing ownership
  have AST/source ratchets
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/architecture_test.go:20`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/architecture_test.go:49`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/architecture_test.go:72`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/architecture_test.go:112`);
- the package has an explicit positive/negative/neutral layer triad
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/layer_triad_test.go:10`); and
- two fuzzers check canonical JSON semantic closure and body-mutation rejection
  with real production cryptography
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/fuzz_test.go:12`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/fuzz_test.go:70`).

Fresh run-time gate evidence collected read-only on 2026-07-27:

- `go test ./attest`: green;
- `go test -race ./attest`: green;
- `go test -shuffle=on ./attest`: green;
- statement coverage: 90.0 percent;
- `go vet ./attest`: green;
- `staticcheck ./attest`: green;
- `gosec ./attest`: zero issues;
- production `gocyclo -over 10`: no findings; and
- pure-Go Linux/amd64 and Windows/amd64 test-binary cross-builds: green.

These gates establish broad behavioral and platform closure. They do not
override the reviewed wire-contract defect below.

## Defects and blockers

### 1. Canonical envelope order contradicts the reviewed specification

The reviewed specification requires:

`signer`, `body_sha256`, `signature`, `body_length_bytes`, `domain`

(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:195`).

The actual custom encoder emits:

`domain`, `signer`, `body_sha256`, `signature`, `body_length_bytes`

(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json.go:104`, especially `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json.go:128-135`).

This is not cosmetic. Accepted JSON must re-encode byte-exact, and downstream
packages persist envelopes. The implementation therefore publishes a different
canonical protocol from its reviewed contract.

### 2. The field-order ratchet proves the inactive representation

`TestEnvelopeWireFieldOrderRatchet` inspects the declaration order of
`envelopeWire` and expects the specification's signer-first sequence
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/architecture_test.go:153`). But production bypasses
`encoding/json` field emission and hand-writes order in `marshalEnvelopeWire`.
The ratchet never inspects or executes that owner.

The specification is internally contradictory as well. It says
`encoding/json` plus declaration order owns the wire and directs the ratchet to
that inactive struct (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:201`), then correctly says
one canonical encoder owns field names and order and that memory layout is not
the protocol (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:219`). The clean implementation must delete the
false declaration-order ownership claim, not merely choose a new order.

Worse, behavioral tests explicitly expect domain-first bytes
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json_hostile_test.go:59`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json_hostile_test.go:195`). The suite is green
because its structural ratchet and behavioral oracle disagree with each other
while each checks a different representation. This violates the testing
protocol's requirement that structural tests ratchet the real compiler-visible
contract.

The fix must choose one reviewed order, update the specification and all
consumers deliberately if that order changes, and make a single behavioral
byte-exact ratchet observe the actual encoder. A dormant struct-order check
must not claim wire ownership.

### 3. The required semantic fuzzer does not independently mutate every signed field

The specification requires a semantic envelope fuzzer with independently
selected mutations of every signed field
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:342`). `FuzzEnvelopeJSONSemanticClosure`
mutates arbitrary input bytes and checks any accepted nonbaseline envelope
cannot verify; its seeds include the baseline, malformed/truncated inputs, and
one unknown field, but not independent canonical mutations for domain, signer,
body digest, signature, and body length
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/fuzz_test.go:12`). The deterministic verification table
covers those fields, but it is not the required stateful semantic fuzz oracle.

Add a typed mutation selector and rebuild canonical candidate envelopes for
each signed field, while retaining byte-exact decode/re-encode and receiver
non-mutation assertions.

### 4. No downstream wire-equivalence retirement proof exists yet

The prior comparative audit correctly requires exact downstream wire and
durable-byte equality before copied signing/parsing code is deleted
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/protocol_capability_comparative_audit.md:21`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/protocol_capability_comparative_audit.md:41`). Current Witness, Bug, and Peachfuzz still use different product envelopes
or legacy `core.Signed`; none imports `attest`. Kernel has the package only in
its dirty Primitive pin and has no production call site.

This does not disprove the shared capability. It blocks claiming downstream
retirement or wire-compatible adoption. Each product needs a typed migration
record distinguishing:

- bytes that must remain identical;
- schemas that intentionally move to a new version;
- product certificate/trust facts that remain outside `attest`; and
- deletion of the old raw signing path only after both representations are
  independently verified.

The freeze board still lists `attest` as `Open` because exact current signing
engines had not been indexed
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:510`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:532`). This interview supplies that index but does not manufacture the missing
migration proof.

## Primitive 2026 ownership and DAG

Keep the capability, but redraw execution ownership explicitly:

- `core` owns universal nominal public values (`SHA256Hex`,
  `Ed25519PublicKeyHex`, `Ed25519SignatureHex`, `ByteCount`), shared stable
  error identities, shared limits, and field/frame constants needed across
  packages.
- `attest` owns domain/body contracts, canonical streaming, the fixed frame,
  detached envelope encoding, trust-set membership, Ed25519 sign/verify
  execution for that frame, and `Verified[D]`.
- `keygen` owns entropy, key generation, operator import/export formats, and
  construction of the opaque signing capability.
- a separate key-provider/composition boundary, if approved, owns KMS/HSM or
  process-backed signing. It must expose a typed capability that signs exactly
  one attestation frame digest, with public-key binding and post-sign verification;
  it must not turn `attest` into a raw callback-signing API.
- consumer protocol packages own their closed signing-domain enums, canonical
  bodies, total signed DTOs, semantic validation, and version migrations.
- Witness owns machine-attestation trust modes, operator/local-anchor policy,
  tool-bundle anchors, and binding sidecars.
- Bug/license owns writer certificates, revocations, certified time intervals,
  repository/operation binding, and writer-proof persistence.
- Peachfuzz owns machine namespace derivation, run-evidence schema,
  content-addressed archive custody, and upload policy.
- Kernel owns application crypto composition and ceremony descriptions, not a
  second generic Ed25519 message protocol.

One design issue must be resolved during the clean 2026 implementation:
`core.Ed25519SigningKey.SignSHA256` currently performs cryptographic execution
inside Core even though `attest` is its sole permitted caller
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/ed25519_signing_key.go:56`). The ratchet makes the
coupling explicit, but it does not make execution a universal value contract.
Prefer an opaque construction path in which the package that owns signing
execution also owns or seals the execution capability. Do not expose private
bytes, reintroduce a generic `Sign([]byte)`, or add compatibility wrappers.

No package may copy the frame generation, field names/order, domain grammar,
trust-set maximum, error identities, or signing algorithm. Product facts must
not move into Core merely because several signed documents carry them.

## Decision rationale and conditions

`attest` is a justified Primitive 2026 capability. Its compiler-owned domain
binding, bounded streaming, fixed framing, immutable trust capability,
proof-carrying result, hostile containment, and internal use across ten
protocol packages are materially stronger than the generic and product-local
signing paths downstream.

Finding: **keep and cleanly reimplement; reject the archived package unchanged**.

Admission requires:

1. one reviewed canonical envelope field order shared by specification,
   production encoder, decoder closure, fixed bytes, and the architecture
   ratchet, with the contradictory declaration-order claim deleted;
2. an independent typed semantic fuzzer for every signed field;
3. an explicit Core/Attest/key-provider execution boundary that keeps secrets
   opaque and raw signing unavailable;
4. byte-exact or explicitly versioned migration proof for Witness, Bug, and
   Peachfuzz, plus a real Kernel adoption call site if Kernel needs the
   capability; and
5. deletion of old signing engines only after their retained product facts and
   stronger composition mechanics have named owners.

Until those conditions are met, green package gates are evidence of a strong
prototype, not evidence that its current wire contract is safe to publish.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
