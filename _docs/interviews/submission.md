# Submission package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole reconstruction report for archived package `submission`.
The archive is evidence, not authority. No archived production or test source
was copied.

The capability is real and commercially necessary. Witness, Bug, and Peachfuzz
already implement three specialized forms of the same transaction:

1. declare an exact immutable evidence or artifact set;
2. obtain a short-lived storage capability;
3. stream bytes directly to storage;
4. report immutable provider facts;
5. accept only a response bound to the original declaration; and
6. preserve custody across ambiguous outcomes.

The archived package is a strong prototype of that common lifecycle. It has a
product-neutral declaration, nominal declaration digest, non-persistable upload
grant, one-shot O(1) transfer, closed result variants, reconciliation, verified
receipt acceptance, and a durable journal.

It is not admissible unchanged. Five defects are blockers:

- completion and reconciliation do not verify an accepted receipt against the
  complete declaration even though the specification requires that binding;
- a locally initialized journal record is called authoritative even though no
  control response carries the generation the record claims;
- transfer expiry has two independent time owners;
- direct declared-to-rejected and declared-to-conflict transitions manufacture
  the presence of a zero authorization in accessors and JSON; and
- lower-generation rollback is collapsed into ordinary conflict.

The current doctrine also requires a cleaner compiler-owned implementation:
operation-specific key-material structs instead of variadic strings and byte
blobs, schema-derived JSON bounds, package-local implementation ratchets, and
Core ownership only for genuinely shared identities and invariants.

The 2026 direction is therefore a clean reconstruction informed by the archive
and all four consumers. It is not a copy operation.

## Evidence boundary


### Source revisions and pins

| Source | Exact revision or Primitive pin | `submission` availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`, `Harden capability inventory evidence`) | Present. Specification introduced by `881ec505eb175fb3da44e7f1edb780a0ca0a2793`; implementation introduced by `8e7aa9a170d5b63e0f6a0664ab6670b97cd253c8`. | One unrelated untracked file, `core/api_http_boundary_hostile_test.go`; the inspected Submission and Core files are clean against HEAD. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | Committed `go.mod` pins `0df2954a2d911a5d7d775691d023d569affa2c20`, where `submission` is absent. Dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` (`2026-07-27T00:33`, `2026-07-27T00:47-04`, `2026-07-27T00:00`), where `submission` is present. Current Kernel production does not import it. | Broad pre-existing dirty migration. The committed and dirty pins are distinct facts. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9`, where `submission` is absent. | Only the pre-existing untracked ledger was observed. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3`, where `submission` is absent. | Only the pre-existing untracked ledger was observed. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75`, where `submission` is absent. | Only the pre-existing modified ledger was observed. |

The archive repository contains the consumer pins, so `git cat-file` can
qualify package availability even where the consumer repository does not retain
the Primitive Git object. The dirty Kernel pin contains `submission`; the
other four recorded pins do not. A repository-wide production import scan
found no current Kernel import of `foundation/v2026/submission`.

The material archive history is:

- `881ec505eb175fb3da44e7f1edb780a0ca0a2793` specifies the evidence
  submission primitive;
- `8e7aa9a170d5b63e0f6a0664ab6670b97cd253c8` implements the hardened
  submission protocol;
- `6906facd7c6e67754455185891cfcb336101eb61` hardens policy boundaries and
  lifecycle recovery; and
- `40ded9c104a99cbc4b0b672cd7392901b468d1eb` hardens comparative contracts.

At archive HEAD, the package contains:

- 4,337 production Go lines;
- 3,049 test and fuzz Go lines;
- 7,386 total Go lines; and
- an 817-line permanent-contract specification.

Archive status records contradict each other. The specification index still
describes Submission as specified with implementation pending and its contract
as a draft
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:38`). The pending ledger calls it a reviewed
production implementation (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_pending.md:46-50`). The completed
ledger calls the specification and implementation complete and records user
approval plus a future landing gate
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1521-1577`).

Those claims describe archive history. They are not 2026 admission evidence and
do not override the defects below.

## Capability ownership

The reusable capability is one recoverable, provider-neutral evidence
acceptance transaction:

```text
declaration
    -> authorization or existing accepted receipt
    -> one create-only direct storage transfer
    -> completion
    -> verification or reconciliation
    -> verified accepted receipt
```

The archive describes this boundary directly
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:6-20`) and excludes evidence generation, product
meaning, pricing, entitlement policy, storage-provider policy, retention
policy, compression, packaging, and UI
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:22-62`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:801-817`).

The reconstructed package should own:

- one immutable typed `Declaration`;
- a declaration digest nominally distinct from an evidence-byte digest;
- authority-issued positive authorization attempts;
- a short-lived exact create-only `UploadGrant`;
- closed authorization, completion, and reconciliation results;
- exactly one streamed transfer attempt through `objectstore`;
- a typed completion report over observed provider facts;
- exact receipt expectation and verification;
- reconciliation after every potentially committed ambiguous outcome;
- a local durable journal that never persists bearer material; and
- stable Core-owned error identities for contract, verification, expiry,
  rollback, conflict, rejection, retryable, indeterminate, and persistence
  outcomes.

It should not own:

- product-specific evidence kinds or schemas;
- account entitlement, quota, lease, or gate decisions;
- provider credential issuance;
- source-file opening, filesystem traversal, packing, or compression;
- receipt retention or commercial retention policy;
- a clock read duplicated from the actual transfer owner;
- a server database adapter;
- local invention of authority generations; or
- consumer-specific retry budgets and operator policy.

## Archive evidence

### Product-neutral transaction boundary

The archive has a clear declaration-to-receipt lifecycle and deliberately sends
artifact bytes directly to storage rather than through the control plane
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:6-20`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:301-365`). Production imports only the
standard library and the narrow Primitive packages `attest`, `contextstate`,
`core`, `exchange`, `filestore`, `objectstore`, `receipt`, `temporal`, and
`timeproof`.

That boundary is more reusable than any one consumer protocol. It belongs
above storage, receipt, time-proof, transport, and persistence primitives, and
below the product composition roots.

### Immutable declaration and nominal digest

`Declaration` privately retains revision, submission, account, offering,
device, evidence-kind, exact object integrity, exact time proof, and its derived
digest (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/declaration.go:12-53`). `Validate` recomputes the
digest, and an authoritative time proof must bind the declared evidence SHA-256
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/declaration.go:56-87`).

`DeclarationDigest` is nominally distinct from `core.SHA256Hex`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/identity.go:153-210`). That distinction prevents the
evidence-byte digest and declaration-facts digest from being substituted merely
because both are 64-character SHA-256 encodings. The specification explains
the compiler-level reason precisely
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:174-187`).

This is a genuine reusable gem.

### Bearer non-disclosure

`UploadGrant` keeps the signed target private, exposes no target accessor, and
implements neither JSON, text, nor string rendering
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/grant.go:9-92`). Only the private authorization wire owns
the encoded bearer (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/result_wire.go:13-34`).

The specification closes the permitted carrier inventory and explicitly
forbids the URL and signed headers from journals, records, receipts,
completion, diagnostics, and errors
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:224-271`). An AST ratchet checks that inventory
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/enum_architecture_hostile_test.go:145-215`).

This is stronger than the current specialized consumers, where signed upload
URLs remain ordinary exported fields.

### One-shot streaming with indeterminate outcome

`Transfer` accepts an `io.Reader`, calls `objectstore.Upload` exactly once, and
retains only typed transaction identities when the provider may have committed
without returning trustworthy version facts
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/transfer.go:22-50`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/transfer.go:129-205`).

The confirmed result contains provider, immutable provider version, exact
extent, SHA-256, and CRC32C. The indeterminate result cannot fabricate those
provider facts (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/transfer.go:52-127`).

This is the correct O(1) and no-duplicate-upload foundation. An ambiguous
single-attempt transfer must reconcile; it must not restart a consumed stream.

### Closed total result variants

Authorization, completion, and reconciliation expose closed dispositions with
private variant storage and typed accessors
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/results.go:9-330`). Strict result-wire decoding admits
only the variant matching the discriminant
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/result_wire.go:36-230`).

This is materially safer than an exported optional-field response where
disposition and payload can disagree.

### Receipt verification precedes acceptance

All accepted paths cryptographically verify a `receipt.EvidenceDocument`
before projecting accepted state
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/client.go:574-631`). The journal retains the verified
receipt before persisting the accepted record
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/journal.go:259-296`), and acknowledgment removes the
journal only after matching the exact accepted receipt identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/journal.go:313-349`).

The ordering is correct: accepted state is not durable until the evidence receipt
is durable.

### Explicit transition table and replay semantics

The record state machine has an exhaustive compiler-visible transition table
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/record.go:560-583`). Equal-generation identical events are
replays; equal-generation divergence and skipped generations are rejected
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/record.go:450-487`).

The approach is worth preserving after authority ownership and phase-presence
bugs are corrected.

### Durable exclusive journal

The journal derives a typed private path, acquires exclusive `filestore`
ownership, validates every snapshot, serializes access with a mutex, and uses
bounded durable reads and writes
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/journal.go:18-165`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/journal.go:167-235`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/journal.go:375-420`).

No upload target or bearer is persisted. Receipt-first acceptance and explicit
acknowledgment make the crash boundary understandable.

### Stable error identity

Core owns stable Submission error identities
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:112-119`), and package errors retain those
identities for `errors.Is`/`errors.As`. Rendered wrapper errors expose a bounded
label rather than joining arbitrary provider text
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/errors.go:5-50`).

The missing rollback identity is a correction, not a reason to discard the
error graph.

### Primitive dependency graph

At archive HEAD, `go list` proves one direct package node:

```text
submission -> attest
submission -> contextstate
submission -> core
submission -> exchange
submission -> filestore
submission -> objectstore
submission -> receipt
submission -> temporal
submission -> timeproof
```

No other archived Primitive production package imports `submission`. A
repository-wide import scan finds only the package itself. It is therefore a
leaf orchestration capability, not a prerequisite of lower primitives.

The 2026 DAG should preserve that direction:

- `core` owns only cross-package identities, stable error identities, shared
  protocol values, and genuinely shared bounds;
- `temporal`, `attest`, `exchange`, `filestore`, `objectstore`, `receipt`, and
  `timeproof` remain independent lower primitives;
- `submission` composes those primitives without any lower package importing
  it; and
- Kernel, Witness, Bug, and Peachfuzz import `submission` only through
  product-owned adapters.

Receipt binding creates one deliberate cross-package invariant. If the accepted
receipt must prove the exact declaration digest, the nominal digest type cannot
remain private to `submission` while `receipt` must carry it. The 2026 design
must choose one compiler-visible owner, preferably a Core-owned nominal
`SubmissionDeclarationDigest` used by both packages. Duplicating two wrappers
or comparing raw SHA strings is forbidden.

Submission-local wire tokens, frame domains, journal names, labels, and
architecture counts should not be in Core merely because Submission uses them.
They remain package constants unless another package truly shares the contract.
Stable error identities and a cross-package declaration digest remain Core
owned.

## Consumer evidence

### Kernel

Kernel has no current production use of the archived package. Its committed
Primitive pin predates Submission, and its dirty pin contains Submission
without any import. Kernel nevertheless supplies two useful lower-level gems.

First, Firestore evidence idempotency binds the caller retry identity to the
complete typed evidence document. Exact retries converge, while changed
evidence cannot be suppressed by reusing the same request identity
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/client_evidence_identity.go:13-35`).

That is the right semantic rule for a submission declaration: idempotency must
cover all immutable declaration facts, not merely a submission ID or caller
header.

Second, Exodus models evidence provenance explicitly. `Envelope` carries an
evidence kind, exact byte-range or row-index evidence, source identity, a
deterministic record identity, and a tamper-evident envelope digest
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/envelope.go:16-60`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/envelope.go:107-150`). Its evidence-kind validation
requires exactly one matching evidence projection
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/envelope.go:222-267`), and its idempotency derivation includes
content so changed records cannot collide
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/envelope.go:289-366`).

The gem is the requirement, not the implementation shape. Exodus still uses
many raw strings and manual byte framing
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/envelope.go:269-310`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/envelope.go:369-410`), so it must not be copied into
the 2026 compiler-owned Submission contract.

Kernel conclusion:

- no cutover evidence exists;
- complete-declaration idempotency is proven necessary;
- typed provenance should enter through a product-owned evidence descriptor or
  schema identity; and
- raw Exodus framing is archaeological evidence, not a Primitive template.

### Witness

Witness has the strongest independent use case. Its custody protocol is already
the same transaction at bundle scale: open a session, receive signed direct
storage targets, stream each artifact, finalize exact provider objects, and
verify a signed receipt. Its package documentation states that artifact bytes
never cross the control plane and Offgrid verifies storage before signing
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/doc.go:1-5`).

The open request binds release, lease, customer, bundle root, artifact
descriptors, exact total bytes, exact count, and schema
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/models.go:43-79`). The response is a closed choice
between an upload grant and an existing verified receipt, both bound to customer
and bundle root (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/models.go:81-170`).

Each upload target is validated against the exact customer, bundle root,
artifact, object identity, provider, method, and headers
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/models.go:172-227`). Upload streams in O(1) memory,
hashes on the wire, proves exact extent, captures the provider generation, and
constructs an immutable `UploadedObject`
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/client.go:193-271`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/client.go:298-327`).

Finalize and the signed receipt bind the exact session, customer, bundle root,
and complete uploaded-object set
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/client.go:150-190`). The receipt adds retention,
timestamp proof, ledger sequence, and chain hash and validates their ordering
and bundle binding
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/models.go:294-435`).

Witness gems to admit:

- exact set cardinality and total-byte validation;
- grant-to-request and object-path binding;
- simultaneous SHA-256, BLAKE3, extent, and provider-generation proof;
- exact request-to-receipt object-set equality;
- signed existing-receipt reuse instead of issuing another capability; and
- receipt timestamp, ledger, and retention facts remaining receipt-owned.

Witness also reveals what not to centralize. A Witness bundle has multiple
artifacts, release and customer identities, BLAKE3 bundle roots, retention
classes, and RFC 3161 custody policy. Those remain in Witness/Custody. Generic
Submission should accept one typed evidence object per transaction or a
consumer-owned bundle object; it must not absorb Witness product policy.

### Bug

Bug has a production release-deployment transaction that mirrors Submission:
check whether the exact publication already exists, obtain a signed deployment
plan, upload exact release artifacts, finalize, and verify the response
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:143-213`).

The deploy input verifies the signed manifest and binds it to the release plan
before execution (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:55-91`). Immediately before
upload, Bug re-derives every local artifact and rejects drift against the signed
manifest (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:215-252`). Files are opened beneath an
`os.Root`, required to be regular and exact-size, streamed through a SHA-256
hasher, and compared to the manifest digest before an uploaded artifact fact is
returned (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:278-388`).

Bug gems to admit:

- lookup of an already-published authoritative result before new upload;
- signed prepare and signed finalize plans;
- request-to-response verification at both control crossings;
- pre-transfer source drift rejection;
- rooted file access at the consumer boundary;
- on-wire hashing and exact extent; and
- upload output retaining the exact attempt and binding supplied by authority.

Bug also shows the boundary Submission must improve. Its raw uploader returns
an ordinary transport error after `HTTP.Do`
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:350-376`); it does not itself represent a
possibly committed upload or force reconciliation. The Primitive primitive
must preserve the archive's indeterminate outcome rather than copying this
weaker behavior.

### Peachfuzz

Peachfuzz has the smallest and most direct evidence-submission use case. It
derives an immutable descriptor from a verified signed `RunEvidence` atom:
typed content-addressed object name, SHA-256 digest, and exact size
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/run_evidence_upload.go:133-235`).

The authorization request carries the signed evidence itself and validates the
descriptor deterministically
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/run_evidence_upload.go:250-273`). The grant binds the exact
descriptor, signed URL, bounded headers, expiry, provider, method, and schema
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/run_evidence_upload.go:275-315`). The response is a closed
choice between upload-required and already-present, and `ValidateRequest`
proves the response descriptor equals the request descriptor
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/run_evidence_upload.go:317-366`).

The live publisher derives idempotency from the evidence digest, uses typed
Exchange semantics, and validates the authorization response against the exact
request (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/evidence_http.go:76-125`). The transfer is a
single-attempt signed PUT through Exchange
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/evidence_http.go:137-164`).

Peachfuzz gems to admit:

- content-addressed typed object naming;
- descriptor derivation from the verified canonical evidence atom;
- digest-derived semantic idempotency;
- exact grant/request binding; and
- an explicit present-versus-upload-required result.

Two Peachfuzz behaviors must not be generalized:

- `already-present` is treated as success without an accepted receipt
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/evidence_http.go:55-66`), whereas generic Submission
  correctly distinguishes provider object existence from authoritative
  acceptance; and
- retryable upload failure causes a fresh authorization and another upload
  attempt (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/evidence_http.go:55-73`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/evidence_http.go:167-176`).
  Generic Submission must reconcile a possibly committed upload and never
  replay a consumed source implicitly.

## Strong mechanics and proof

### Cross-consumer synthesis

The four repositories support one narrow shared primitive:

| Requirement | Kernel | Witness | Bug | Peachfuzz | Archive |
| --- | --- | --- | --- | --- | --- |
| Exact typed evidence declaration | Proven by evidence identity and provenance | Bundle root plus exact artifact set | Signed manifest plus release plan | Signed evidence descriptor | `Declaration` |
| Semantic idempotency covers immutable facts | Proven | Bundle-root key | Signed request identity | Evidence-digest key | Derived operation keys |
| Direct bounded/streamed transfer | Indirect migration evidence | Proven O(1) | Proven file stream | Small in-memory atom | Proven O(1) |
| Grant bound to exact request | Not cut over | Proven | Signed deploy plan | Proven | Proven |
| Provider immutable version/generation | Not cut over | Proven GCS generation | Attempt/binding retained | Not retained | Proven typed provider version |
| Accepted receipt bound to request | Not cut over | Strongly proven | Signed finalize response | Missing | Intended, incomplete |
| Ambiguous outcome reconciliation | Not cut over | Control retry types exist | Missing in uploader | Incorrect second-upload path | Strongly intended |
| Durable local custody | Migration-specific | Bundle custody | Release files remain rooted | Archive/worktree custody | Journal |

The shared Primitive value is not an uploader. It is the acceptance
transaction that prevents an uploader from lying about whether bytes, authority
state, and receipt state agree.

## Defects and blockers

### 1. Completion does not verify the complete declaration binding

The specification requires accepted completion to apply the same receipt
verification and exact declaration binding as authorization
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:395-398`). Authorization does compare receipt
account, offering, submission, extent, SHA-256, and CRC32C to the full
declaration (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/client.go:574-595`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/client.go:634-643`).

Completion cannot do the same. `Completion` contains the declaration digest,
submission, authorization, object, provider version, extent, and digests, but
not account, offering, device, evidence kind, or time proof
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/completion.go:12-24`). `verifyCompletionReceipt` checks
only submission, object, provider version, extent, SHA-256, and CRC32C
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/client.go:598-613`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/client.go:645-653`).

An authentic accepted receipt for the same bytes, object, and submission but a
crossed account or offering can therefore pass completion verification. The
completed-ledger claim that every completion field is bound
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1564-1568`) is narrower than the specification's
declaration-binding requirement.

Required correction:

- the accepted receipt must carry a compiler-owned nominal declaration digest,
  or
- the signed receipt and local request must carry a typed expectation that
  covers every immutable declaration fact.

The preferred design is a shared Core-owned nominal declaration digest carried
by both Submission and Receipt, plus the object/version/integrity facts fixed
at completion.

### 2. Reconciliation receipt verification is weaker still

`ReconcileRequest` contains a declaration digest
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/client.go:117-134`), but
`verifyReconcileReceipt` ignores it. It accepts any valid receipt whose body has
the requested submission identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/client.go:616-631`).

That does not prove account, offering, declaration digest, object, integrity,
or provider version. A valid crossed receipt can close the wrong local
transaction.

Required correction:

- reconciliation acceptance must use the same single typed
  `ReceiptExpectation` owner as declaration and completion acceptance; and
- hostile tests must mutate every expectation field independently for all
  accepted paths.

### 3. `AuthoritativeRecord` is locally invented

The specification explicitly says the authority-issued attempt ordinal must
never be guessed (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:140-145`). Its record-generation
contract separately requires monotonicity, byte-identical equal generation,
and lower-generation rollback (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:447-466`), but does not name
the generation issuer.

`OpenJournal`, however, creates `BeginRecord` locally before any control
response and persists generation one in phase declared
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/journal.go:146-165`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/record.go:78-95`). Public `NewRecordGeneration` accepts any positive
integer (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/values.go:79-109`).

No authorization, completion, or reconciliation result carries a record
generation. The caller must therefore invent the generation supplied to
`RecordEvent` and the journal. The implementation cannot prove that its
the generation or phase came from the authority.

Required correction:

- separate `AuthoritySnapshot` from `LocalJournalSnapshot`;
- only a verified control response may construct or advance an authority
  generation;
- local journal sequence must have a separate type and owner; and
- if the server does not expose authoritative generation, remove that claim
  and model only locally observed transaction phase.

One type must not represent both truths.

### 4. Transfer expiry has two independent clock owners

`TransferRequest` requires caller-supplied `ObservedAt` and rejects an expired
grant before calling Objectstore
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/transfer.go:22-49`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/transfer.go:129-145`).

`objectstore.Upload` then reads its own client clock at the real upload boundary
and performs the same expiry decision before touching the source
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/client.go:163-180`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/client.go:321-333`).

The observations can disagree. Submission can reject a grant Objectstore would
accept, or accept preflight and have Objectstore reject at execution. This is
duplicated time truth.

Required correction:

- Objectstore remains the sole request-start expiry owner; or
- one typed observation is captured once and passed through to the actual
  execution owner without another clock read.

The first design is simpler and matches ownership.

### 5. Direct terminal transitions manufacture authorization presence

The transition table permits declared-to-rejected and declared-to-conflict
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/record.go:564-568`). Those records validate without an
authorization because `recordRequiresAuthorization` covers only authorized
through accepted (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/record.go:117-129`).

But `Authorization()` reports presence whenever the numeric phase is at least
`RecordAuthorized`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/record.go:280-285`). Rejected and conflict are numerically
later, so a directly rejected declaration returns a zero `AuthorizationFacts`
with `present=true`.

`MarshalJSON` uses the same numeric comparison and serializes a zero
authorization for those direct terminal transitions
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/record.go:644-675`).

Required correction:

- phase history/presence must be explicit and closed;
- accessors must use validated presence, not enum ordering;
- wire projection must use the same presence owner; and
- hostile tests must exercise declared-to-rejected and
  declared-to-conflict accessors and canonical persistence.

### 6. Rollback is mislabeled as conflict

The specification distinguishes lower-generation rollback from immutable
same-generation conflict
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:460-466`).

`AdvanceRecord` returns `ErrSubmissionConflict` for a lower generation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/record.go:450-480`). Core has no
`ErrSubmissionRollback`; it has contract, verification, expired, conflict,
rejected, retryable, indeterminate, and persistence identities only
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:112-119`).

Required correction:

- add a Core-owned `ErrSubmissionRollback`;
- preserve it through wrapping;
- reserve conflict for immutable fact divergence; and
- assert `errors.Is`/`errors.As`, never rendered text.

### 7. Canonical derivation erases typed structure

The public declaration is typed, but digest derivation collapses fields into a
slice of byte slices before hashing
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/declaration.go:91-125`). The shared helper accepts a raw
domain string and variadic byte blobs
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/identity.go:231-267`).

Idempotency derivation is weaker: it converts operation-specific typed values to
strings, passes a raw domain string and variadic strings, then converts them to
`[][]byte`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/client.go:298-355`).

That protocol is not compiler-visible. Reordering, omitting, or adding fields
still compiles.

Required correction:

- define `DeclarationDigestMaterial`,
  `AuthorizationIdempotencyMaterial`,
  `CompletionIdempotencyMaterial`, and
  `ReconciliationIdempotencyMaterial`;
- each fixed struct validates and owns its exact canonical projection;
- use typed domain enums or package constants selected by the material type;
- forbid variadic string/byte field protocols; and
- pin independent known-answer vectors for every material type.

The final hashing loop may stream bytes, but the semantic input must remain a
typed structure.

### 8. JSON ceilings are generic rather than schema-derived

Declaration, Completion, AuthoritativeRecord, result decoding, and journal
storage all use the broad `core.StrictJSONMaxBytes`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/declaration.go:184-192`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/completion.go:125-133`; `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/record.go:721-725`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/result_wire.go:274-276`; `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/journal.go:68-72`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/journal.go:375-405`).

This proves a global ceiling, not the maximum legal encoding of each schema.
It permits large invalid inputs to enter deeper parsing and makes schema growth
invisible.

Required correction:

- derive a maximum canonical encoded size for each wire type from the exact
  field maxima;
- keep the constants with the owning package/type;
- ratchet encoded boundary fixtures at maximum-minus-one, maximum, and
  maximum-plus-one; and
- use bounded readers before allocation.

### 9. Core contains Submission-local implementation details

`core/submission_constants.go` contains all Submission wire tokens, local frame
domains, journal directory names, error labels, and implementation inventory
counts
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/submission_constants.go:5-77`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_constants.go:186-205`).

The last three values are especially local:

- `SubmissionProductionStructCount = 61`;
- `SubmissionProductionFunctionMinimumCount = 300`; and
- `SubmissionBearerCarrierCount = 7`.

They drive Submission architecture tests
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/enum_architecture_hostile_test.go:203-211`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/enum_architecture_hostile_test.go:288-329`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/enum_architecture_hostile_test.go:369-376`). A minimum function count is also a weak ratchet: arbitrary
function growth cannot fail it.

Required correction:

- keep cross-package error identities and a truly shared declaration-digest
  type in Core;
- move package-local tokens, labels, paths, frame domains, and ratchet counts
  to Submission;
- use an exact compiler-visible inventory rather than a global minimum count;
  and
- ensure other packages never import Submission merely to share a Core
  contract.

### 10. No consumer cutover is proved

Only Kernel's dirty Primitive pin contains the package, and Kernel does not
import it. Witness, Bug, and Peachfuzz pins predate it. No archived Primitive
package depends on it.

The specialized consumers prove demand and supply design gems, but they do not
prove API compatibility, persisted-state migration, or production operation of
the archive.

Required correction:

- build typed adapters for at least Witness custody, Bug release deployment,
  and Peachfuzz evidence publication;
- prove each adapter without shims or duplicate lifecycle paths;
- delete the replaced consumer protocols only after clean cutover; and
- keep product-specific receipt, bundle, release, and retry policy downstream.

### 11. Live provider proof was not reproduced

The specification requires real files, loopback sockets, cryptographic evidence,
and live GCS/S3 resources when credentials are present
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:753-799`).

The package suite contains loopback GCS-shaped tests but no direct Submission
test that names a live provider environment or integration entry point. The
current interview environment did not supply cloud credentials, so no live
GCS/S3 transfer was proven.

Required correction:

- keep deterministic real-socket hostile tests in the default gate;
- add explicit opt-in live GCS and S3 tests with typed configuration;
- publish exact provider revision, object version/generation, and cleanup
  evidence; and
- never report a skipped credential-dependent test as a pass.

### Mechanical gate evidence at the inspected archive

The following read-only gates were run against archive HEAD:

| Gate | Result |
| --- | --- |
| `go test ./submission` | PASS |
| `go test -race -shuffle=on -count=2 ./submission` | PASS |
| `go vet ./submission` | PASS |
| `staticcheck ./submission` | PASS |
| `gocyclo -over 10` over production files only | PASS; no production function above 10 |

Running `gocyclo` across production and tests reports multiple hostile tests
above 10. That is not a production failure; the production-only invocation is
the applicable doctrine gate.

These gates prove that the inspected archived implementation is mechanically
clean under its own suite. They do not disprove the semantic defects above,
prove live cloud behavior, prove a consumer cutover, or authorize copying.

## Primitive 2026 ownership and DAG

### Potential future Submission ownership

The corrected `v2026.0.0` topology defers a standalone Submission package. If
new current demand reopens the boundary, a reconstructed package would own:

- typed immutable declaration;
- shared nominal declaration digest;
- typed authority attempt;
- exact ephemeral upload grant with no rendering or persistence;
- closed authorization/completion/reconciliation outcomes;
- one-shot streamed transfer through Objectstore;
- typed provider observation;
- one exact receipt expectation used by every accepted path;
- reconciliation-only recovery after ambiguous commit;
- local journal state with explicit local sequence; and
- package-local canonical wire, paths, tokens, and architecture ratchets.

### Shared contracts if the boundary is reopened

Core should own only the contracts crossing package boundaries:

- `SubmissionIdentity`;
- the nominal declaration digest if Receipt must carry it;
- stable Submission error identities, including rollback;
- shared HTTP/object/provider identities already consumed by lower packages;
- shared validation interfaces; and
- any cross-package schema or path value demonstrably imported by more than one
  owner.

### Preserve in lower Primitive packages

- `objectstore`: request-start clock observation, exact source extent and
  digest verification, create-only provider semantics, provider version
  capture, and ambiguous transfer classification.
- `exchange`: bounded control-plane transport, replay semantics, retry
  classification, and response bounds.
- `receipt`: signed accepted evidence, trust verification, retention, ledger,
  and durable receipt store.
- `timeproof`: official/unofficial evidence-time proof.
- `filestore`: exclusive durable journal ownership and bounded atomic I/O.
- `temporal`: instants, comparison, arithmetic, and explicit observation types.
- `attest`: proof-carrying signature verification.

### Keep in consumers

- Kernel: migration provenance, byte/row evidence identities, ingestion policy,
  and source schemas.
- Witness: bundles, artifact sets, BLAKE3 root, customer/release/lease
  identities, retention, timestamping, custody ledger, and multi-object policy.
- Bug: release plan, signed manifest, rooted artifact opening, publication
  lookup, deployment policy, and release-specific finalize receipt.
- Peachfuzz: run-evidence schema, content-addressed object naming, machine and
  project identity, archive scheduling, and backlog policy.

### Required DAG

```text
core -> temporal
core -> attest
core -> exchange
core -> filestore
core -> objectstore
core -> receipt
core -> timeproof
temporal + attest + exchange + filestore + objectstore + receipt + timeproof
  -> submission
submission -> witness
submission -> bug
submission -> peachfuzz
bug -> kernel adapters
```

The drawing shows dependency direction conceptually, not that every lower
package imports every sibling. `submission` may import the lower primitives;
lower primitives must not import `submission`; consumers may import
`submission`; and consumers must not import one another merely to share the
lifecycle.

## Decision rationale and conditions

### Required implementation proof

Before implementation is presented for user review:

1. define the exact package/Core ownership table;
2. define typed fixed-field canonical material for every digest and idempotency
   key;
3. make the accepted receipt carry or prove the exact declaration digest;
4. use one `ReceiptExpectation` path for authorization, completion,
   reconciliation, and journal acceptance;
5. separate server-authoritative generation from local journal sequence;
6. remove Submission's duplicate transfer clock observation;
7. replace enum-order presence checks with compiler-visible variant presence;
8. add `core.ErrSubmissionRollback`;
9. derive per-schema ingress and persistence bounds;
10. keep all bearer material outside JSON-capable and persistence-reachable
    public values;
11. preserve one-shot O(1) transfer and indeterminate reconciliation;
12. prove declared-to-rejected and declared-to-conflict projections;
13. prove every accepted path rejects every crossed declaration field;
14. prove equal replay, lower rollback, skipped generation, terminal mutation,
    and immutable drift with typed errors;
15. build real adapters for Witness, Bug, and Peachfuzz without compatibility
    shims;
16. prove default real-file and real-socket gates;
17. separately prove live GCS and S3 behavior when typed credentials are
    supplied;
18. run package and full-tree test, race, shuffle, vet, staticcheck, nilaway,
    errcheck, gosec, govulncheck, deadcode, gocyclo, goconst, fieldalignment,
    sentinel, and witness gates required by the repository; and
19. obtain fresh independent review after the implementation and consumer
    adapters exist.

### Current rationale

The capability belongs in Primitive 2026 because:

- all three evidence-producing consumers implement overlapping forms of it;
- Kernel independently proves full-evidence idempotency and typed provenance
  requirements;
- the archive has unusually strong non-disclosure, streaming, result-variant,
  reconciliation, and receipt-first journal mechanics; and
- centralizing the correct lifecycle eliminates dangerous duplicate upload and
  false-acceptance behavior from consumers.

Reject archive-as-is because:

- two accepted paths do not prove their advertised declaration binding;
- the client locally invents state labeled authoritative;
- time ownership is duplicated;
- terminal phase projection can claim a nonexistent authorization;
- rollback identity is missing;
- digest/key protocols erase typed structure;
- JSON bounds and implementation ratchets are not correctly owned; and
- no production consumer cutover or live cloud proof is established.

Do not copy the directory or start a Submission specification. Preserve the
proven mechanics and consumer evidence in this report. A new current consumer,
authority boundary, and explicit topology review are required before any later
reconsideration. No commit or push is authorized by this report.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
