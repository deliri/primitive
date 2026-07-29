# Receipt package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole recon report for archived package `receipt`. It integrates the
archived implementation and specification, every Primitive package that depends
on Receipt, and current Kernel, Witness, Bug, and Peachfuzz receipt-like
capabilities. The capability has a justified place in Primitive 2026, but the
archived package is not admissible unchanged.

## Evidence boundary


| Source | Exact revision or pin | Receipt availability and relevance |
| --- | --- | --- |
| Primitive archive | `d046f7b675fcb797398d7cdc87b5504f43978056` (`Harden capability inventory evidence`, 2026-07-27) | Archived Receipt tree `dd9682428ffdb60e2c4557e521347ad4c95985cc`. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | Committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins Primitive `0df2954a2d911a5d7d775691d023d569affa2c20`, before Receipt. The pre-existing dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43`, whose Receipt tree is exactly `dd9682428ffdb60e2c4557e521347ad4c95985cc`. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins Primitive `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9`, before Receipt. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins Primitive `388e593231a28434f6faae9f0ab9dffcf332dfc3`, before Receipt. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins Primitive `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75`, before Receipt. |

Witness and Bug have only an untracked `.ledger_pending.md`; Peachfuzz has only
a modified `.ledger_pending.md`. Their production source and module files are
clean. Kernel's committed and dirty pin states are separated in the table.

The package history is:

- `bcadec7dadf5ac99671ad8cef1cd6a13e5094b6f`, specify the signed
  receipt primitive;
- `4ce34f1f0a7de3a5d59cd50c12fdd9f9f79f95f9`, establish the
  commercial Core contracts;
- `7d375270bc574e18f9aac18a06c9795dbb725e46`, complete Filestore
  lifecycle prerequisites;
- `73b161140fcc99b026defd9449136f5681fc0f4f`, add the hardened
  protocol and durable Store;
- `bfc888f8af2cee8e43928f07a7f1a961888bfc76`, add the Watermark
  contract; and
- `40ded9c104a99cbc4b0b672cd7392901b468d1eb`, apply comparative
  hardening.

The committed Kernel pin and the Witness, Bug, and Peachfuzz pins do not contain
`receipt/SPEC.md`. Kernel's dirty pin contains the exact archive tree, but current
Kernel production has no direct Receipt import. No current Witness, Bug, or
Peachfuzz production file imports Primitive Receipt. Their local types below are
independent product evidence, not adoption proof.

## Capability ownership

Receipt owns one narrow authority-fact capability:

> authenticate, retain, and page immutable facts that an authoritative service
> has accepted.

The initial total fact bodies are accepted evidence and accepted payment. They
share a signed header and verification engine but remain separate concrete
types. There is no optional-field union, generic payload, `any`, raw-message
dispatch, callback registry, global Store, default path, or product entry point
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/SPEC.md:6`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/SPEC.md:90`).

The owned operations are:

- issue and verify signed `EvidenceDocument` and `PaymentDocument`;
- issue, verify, and monotonically advance a bounded signed remote `Page`;
- derive and monotonically advance an unsigned structural `Watermark` intended
  to live inside a separately signed control snapshot; and
- retain verified concrete documents in a caller-rooted, append-only,
  crash-recoverable local Store.

The package does not decide that evidence or a payment should be accepted. It
does not submit an object, authorize an upload, call a provider, inspect Stripe,
run checkout, choose a plan, issue account or object identities, decide a lease
or gate, rate a product, render output, or call a network
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/SPEC.md:8-20`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/SPEC.md:552-564`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/SPEC.md:639-650`).

That distinction is essential. A Receipt `PaymentBody` may faithfully record a
negative, zero, or positive authoritative amount; business acceptance belongs to
the issuer. An evidence receipt proves an authority accepted the named object
digest and extent; it is not upload authorization and does not prove an
unmodified client.

## Archive evidence

### Concrete proof-carrying documents

`EvidenceBody` closes submission identity, object identity, provider version,
byte extent, SHA-256, and CRC32C. Its zero-extent case requires the canonical
empty SHA-256 and CRC32C values
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/evidence.go:13-79`).

`PaymentBody` closes an opaque commercial reference, exact `currency.Amount`,
plan identity, and a strictly ordered covered interval
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/payment.go:13-67`). It contains no provider object,
webhook, invoice, URL, email, or rendered description.

Issuance derives the body digest, constructs the typed header, signs the complete
canonical evidence or payment payload through Attest, and returns a concrete
document. Verification accepts caller-owned trusted keys and returns a private
proof-carrying `VerifiedEvidence` or `VerifiedPayment`; zero values cannot
masquerade as verified facts
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/documents.go:252-311`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/documents.go:314-435`).

Three digest roles are deliberately different:

1. `Header.BodySHA256` identifies the kind-specific body;
2. `attest.Envelope.BodySHA256` identifies the complete signed receipt payload;
3. the Store document digest identifies the complete canonical persisted
   document, including the envelope.

The hostile suite explicitly proves those roles cannot be confused
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/document_hostile_test.go:290`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/protocol_hostile_test.go:15`).

### Bounded authenticated remote paging

A Page is a fixed-capacity value containing at most 64 summaries. Construction
copies the caller slice into private fixed storage; access returns a copy. Every
summary must match page account and offering, identities must be unique, and
ordering is total by `(OccurredAt, ReceiptIdentity)`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/page.go:200-292`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/page.go:341-368`).

The signed page binds logical page identity, account, offering, revision,
generation, exact request position, issue and validity instants, summaries, and
continuation. `VerifyPage` requires the caller's expected identity, scope,
position, and observation time; a valid signature alone is insufficient
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/page_document.go:129-238`).

`AdvancePage` gives the monotonic rule an executable owner. Lower generation is
rollback. Equal generation requires byte-identical canonical Page payload.
Higher generation must preserve identity, scope, revision, and request position;
must not regress issue/validity; and may only extend the summary prefix and
continuation consistently
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/advance.go:46-139`).

This is stronger than making every consumer compare two signed pages itself.
Remote Page and local `StorePage` are also nominally distinct, as are their
cursors. A local disk cursor cannot compile as an authority cursor.

### Authority Watermark without raw-cursor leakage

`Watermark` binds a derived identity to revision, account, offering, positive
generation, a nominal digest of the authority's opaque cursor, and a nominal
accepted-chain hash. The cursor digest and chain hash share SHA-256
representation but are not interchangeable types. Identity is re-derived during
every validation from a length-framed domain/revision/account/offering tuple
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/watermark.go:190-249`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/watermark.go:350-382`).

Advance is explicit: foreign identity conflicts; lower generation rolls back;
equal generation must be identical; higher generation requires both closures to
change. A half-updated pair conflicts
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/watermark_advance.go:41-95`).

Watermark intentionally is not self-authenticating. Its trust comes from the
signed `controlstate` snapshot that embeds it. Receipt owns its structure and
monotonic comparison, while Controlstate owns aggregate snapshot signing and
freshness.

### Crash-recoverable bounded Store

`StoreRequest` accepts an absolute caller root, an immutable trusted-key
capability, and exact compiler-owned modes and size ceilings. It rejects
alternative modes or caller-selected unbounded ceilings
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/store_types.go:16-40`).

The durable layout has four typed facts:

1. a create-only canonical document at a deterministic identity path;
2. bounded append-only, length-framed, hash-chained index segments;
3. a replace-durable checkpoint naming the committed segment, byte offset,
   sequence, and last record digest; and
4. one pending intent naming the exact document digest, record digest, target
   segment, starting offset, and sequence.

Retention writes and syncs pending intent, activates the create-only document,
appends and syncs the index suffix, replaces the checkpoint, and finally clears
pending intent. It accepts only proof-carrying verified documents and
re-authenticates on retention, replay, lookup, and paging
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/SPEC.md:380-445`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/store_retain.go:13-120`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/store_retain.go:143-168`).

The pending record is the important gem. After a crash, recovery names exactly
one in-flight identity without enumerating the document directory. An absent
document can be cleared only at the expected tail. A matching document resumes
an empty or exact-prefix record by appending only the missing suffix. Divergent,
overlong, wrongly positioned, or mismatched state is typed corruption
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/store_recovery.go:14-176`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/store_recovery.go:244-265`).

Rollover opens and locks the deterministic successor while retaining the old
segment lock; it syncs the new segment and checkpoint before closing the old
writer. This closes the otherwise subtle second-writer entry window
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/store_recovery.go:272-315`).

Committed replay streams bounded frames and one bounded document at a time. Page
collection uses a fixed maximum. The Store does not build a directory or
lifetime-sized in-memory index. Local hash-chain verification detects
inconsistency in retained state, but the specification correctly admits that a
machine owner can substitute an entire new local history without an independent
signed Page or Controlstate anchor
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/SPEC.md:478-489`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/SPEC.md:552-561`).

### Compiler-owned boundaries

Core owns the stable Receipt error hierarchy, protocol tokens, domain strings,
field names, path components, modes, frame widths, and size ceilings
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:93`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/receipt_constants.go:5-99`). Receipt wraps local context while preserving
typed identities for `errors.Is`; signing-domain mismatch has one owner and
retains both Receipt verification and Attest verification identities
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/errors.go:18-64`).

Production imports only standard library plus `core`, `attest`, `temporal`,
`currency`, and `filestore`. An AST ratchet rejects other dependencies, maps,
production goroutines, and directory globbing
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/architecture_test.go:124-174`).

### Complete Primitive dependency and demand graph

The production import graph, recomputed with `go list`, has exactly two direct
Receipt dependents and one transitive dependent:

```text
core / attest / temporal / currency / filestore
                         |
                       receipt
                      /       \
              submission    controlstate
                                |
                             register
```

No other Primitive production package reaches Receipt.

### `submission`: accepted-evidence consumer

Submission is the strongest adoption evidence:

- its authoritative record and result types carry concrete
  `receipt.EvidenceDocument` and proof-carrying `receipt.VerifiedEvidence`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/record.go:69`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/record.go:247`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/record.go:353`,
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/results.go:11`);
- declaration, completion, and reconciliation paths call
  `receipt.VerifyEvidence`, then independently bind the verified Receipt body to
  the corresponding Submission facts
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/client.go:574-631`);
- `Journal.Accept` first checks that the verified document matches the current
  completion, retains it through `Store.RetainEvidence`, then advances the
  authoritative record and durably writes the Journal
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/journal.go:237-296`); and
- acknowledgement compares the exact Receipt identity before removing the
  Journal file (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/journal.go:313-345`).

That sequencing is sound ownership: Submission decides lifecycle consistency;
Receipt authenticates and retains the accepted evidence fact.

### `controlstate`: Watermark consumer

Controlstate embeds Receipt's `Watermark` in a closed `ReceiptState`, validates
the current/empty union, exposes it only through typed accessors, and delegates
monotonic comparison to `receipt.AdvanceWatermark`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/unions.go:221-257`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/advance.go:350-360`). It also ratchets Controlstate and Receipt
protocol revisions together (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/revisions.go:70`).

This is the intended trust composition: Controlstate authenticates the signed
aggregate; Receipt owns the nested Watermark's derivation and advance rule.

### `register`: transitive only

Register imports Controlstate in its client, protocol, and store. It does not
import Receipt or manipulate a Watermark directly
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/client.go:13`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/protocol.go:9`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:14`). It is a transitive consumer of
the aggregate snapshot contract, not a second Receipt owner.

### Unadopted archived surface

No Primitive production package outside Receipt calls `IssueEvidence`,
`IssuePayment`, `VerifyPayment`, `IssuePage`, `VerifyPage`, `AdvancePage`,
`NewWatermark`, or `OpenStore` directly. Submission does exercise Store through
an injected `*receipt.Store`, while Controlstate receives and advances decoded
Watermarks. Payment documents and remote Pages therefore have strong specified
design but no independent archived production call site.

`rate` has its own rating receipt and does not import this package. Its receipt
proves a rating lifecycle fact and must remain distinct rather than being forced
into evidence/payment.

## Consumer evidence

### Kernel

Kernel currently owns the authoritative payment decision and product side
effects. `fulfillParams` carries facts from an already verified provider event
and validates provider, provider object, currency, amount, and optional email
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:payapi/fulfill.go:20-67`).

`fulfillPayment` performs entitlement/product mutation, payment-ledger append,
order transition, and outbox enqueue inside one `Atom.Do`; duplicate operation is
idempotent success (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:payapi/fulfill.go:88-143`). The audit ledger
has explicit Receipt and parent-Receipt correlation identities and closed
payment event kinds (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/ledger/event.go:19-35`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/ledger/event.go:117-123`).
Its chain hashes the previous digest plus canonical fields without an
intermediate map (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/ledger/chain.go:40-64`).

Reusable gems:

- issue a Payment Receipt only from the already verified, atomically committed
  authoritative payment fact;
- retain idempotent operation identity and the tamper-evident audit chain;
- keep entitlement, order, provider, email, tax, refund policy, and outbox
  execution in Kernel/composition; and
- do not confuse Kernel's product audit event or correlation `ReceiptID` with a
  Primitive `PaymentDocument`.

Kernel supplies a credible future Payment Receipt issuer/consumer, but it is not
current adoption proof.

### Witness

Witness's `internal/core/receipt.go` is a different domain: RFC 3161 timestamp
and artifact/run evidence. Its kinds are RFC3161 and Artifact, not accepted
evidence and payment (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/receipt.go:15-24`).

The local model has valuable mechanics:

- explicit `not_requested`, `captured`, `verified`, `unverified`, `failed`, and
  `fallback_exhausted` statuses (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/receipt.go:55-100`);
- typed per-endpoint attempt evidence with latency, HTTP status, byte extent,
  truncation, and attempt number (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/receipt.go:103-140`);
- a closed failure-kind taxonomy that keeps operator prose separate from
  machine dispatch (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/receipt.go:143-202`);
- a typed by-artifact-path row projection rather than a loose map contract
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/receipt.go:372-429`);
- deep-clone sealing so post-seal pointer/slice mutation cannot alter evidence
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/receipt.go:489-522`); and
- an explicit `OnlyRFC3161` fixed-point projection that drops receipts whose
  content would recursively depend on rendered artifact bytes (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/receipt.go:524-538`).

These are evidence-model gems for Timeproof, Witness, and future archive
packages. They must not be renamed into Primitive Receipt merely because both
use the English word `receipt`.

### Bug

Bug's release protocol has product-specific signed acknowledgements:

- `UpdateDiagnosticReceipt` binds diagnostic identity, disposition, and recorded
  time, then verifies the exact diagnostic identity
  (`bug@39ce96242240d7174d562c90bb255860946595dc:protocol/release/update_contract.go:711-766`);
- `UploadReceipt` binds product/version/release/commit, manifest digest, upload
  attempt, uploaded artifact set, and totals
  (`bug@39ce96242240d7174d562c90bb255860946595dc:protocol/release/models.go:343-417`); and
- deploy finalization verifies nested manifest, upload receipt, and download
  index signatures and cross-binds them to the request
  (`bug@39ce96242240d7174d562c90bb255860946595dc:protocol/release/deploy_transport.go:259-345`).

The most portable store gem is compare-and-delete: after a signed diagnostic
receipt is verified, Bug removes only the pending diagnostic with the exact
identity; a concurrent or substituted value is preserved
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/store.go:86-102`).

These receipts remain Release-owned product facts. Their typed cross-binding and
compare-and-delete mechanics should be preserved in the Release interview; they
are not new kinds for this package.

### Peachfuzz

Peachfuzz's `PackReceipt` is a local operation result naming an immutable pack
and manifest plus object count. `PublishPack` streams and validates the pack,
uses create-if-absent publication, verifies pre-existing bytes on collision,
publishes pack and manifest, and returns the receipt only after both immutable
writes succeed
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/publish.go:40-62`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/publish.go:89-140`).

Lifecycle attaches the resulting archive reference to findings only after
successful publication (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/lifecycle.go:238-255`).
That publish-before-attach rule is a useful analogue for Receipt's
retain-before-Journal-advance sequencing.

Peachfuzz also demonstrates the missing inventory shape in the archive: every
Archive production struct is paired with an explicit compiler-owned role such
as protocol fact, internal flow, or wire adapter
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/data_flow_contracts.go:5-42`).

`PackReceipt` remains Archive-owned. It is neither an authoritative signed
evidence/payment fact nor a candidate Receipt kind.

## Strong mechanics and proof

The project-local `_docs/testing_protocol.md` was read completely before this
interview. Receipt is a trust-boundary package for protocol emission/parsing,
cryptographic verification, durable writing, and replay, so the full protocol
applies, including hostile behavioral evidence, local layer triads, typed error
identity, semantic fuzzing, real filesystem paths, and classified data-flow
inventory.

Fresh archive gates on Darwin/arm64:

- `go test ./receipt`: green;
- `go test -race ./receipt`: green;
- `go test -shuffle=on ./receipt`: green;
- `go test -cover ./receipt`: green, 74.8 percent statement coverage;
- `go vet ./receipt`: green;
- `staticcheck ./receipt`: green;
- `gosec ./receipt`: zero issues across 26 production files and 5,958 lines;
- production-only `gocyclo -over 10`: no findings;
- pure-Go Linux/amd64 and Windows/amd64 test-binary cross-builds: green; and
- each of the four semantic fuzz targets ran for two seconds without failure:
  evidence document, signed Page, Watermark closure, and index-record frame.

The suite has real strengths:

- every `uint8` enum value is scanned;
- evidence, payment, identity, occurrence, digest-role, signing-domain, strict
  JSON, Page ordering/freshness/succession, Watermark identity/advance, error
  identity, Store duplication, corruption, cursor forgery, recovery, and
  rollover have direct hostile matrices;
- the Store layer triad uses a real temporary directory and the real Filestore
  path for retain, idempotent replay, absent lookup, local paging, close, reopen,
  and exact replay (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/store_hostile_test.go:10-104`);
- pending recovery covers no document, zero/partial/exact/overlong/divergent
  tails, mismatched documents, already-checkpointed state, recovery-position
  classification, exclusive-open collision, cancellation, and rollover
  boundaries
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/store_recovery_hostile_test.go:84-453`);
- the four fuzzers require accepted inputs to validate and re-encode
  byte-exactly; Evidence and Page go through real Ed25519 verification,
  Watermark identity is independently re-derived, and an index frame is decoded
  with exact digest and extent
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/receipt_fuzz_test.go:123-350`); and
- public functions, exported types, Store methods, dependency closure,
  no-map/no-goroutine/no-enumeration rules, and the four-parameter request-struct
  threshold are ratcheted with production ASTs
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/architecture_test.go:124-249`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/architecture_test.go:281-343`).

The gates establish a strong prototype. They do not close the blockers below.

## Defects and blockers

### 1. Persisted Receipt documents inherit Attest's contradictory canonical order

The reviewed Attest specification requires envelope fields in this order:

```text
signer, body_sha256, signature, body_length_bytes, domain
```

(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:191-203`).

The actual custom encoder emits:

```text
domain, signer, body_sha256, signature, body_length_bytes
```

(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json.go:104-135`).

Every Evidence, Payment, and Page document embeds that envelope
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/documents.go:46-51`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/documents.go:125-130`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/page_document.go:44-49`). The Store then hashes and persists the complete
canonical document bytes (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/store_document.go:118-175`).
The mismatch therefore reaches Receipt's wire protocol, document identity,
create-only files, pending intent, index records, checkpoint chain, replay, and
all downstream durable comparisons.

This is not a cryptographic forgery bug: signing and verification agree on the
signed body. It is a publication blocker because the custom encoder/decoder and
behavioral tests agree on domain-first bytes while the reviewed specification
requires signer-first.

The existing green architecture ratchet is vacuous: it inspects the signer-first
field order of the inactive `envelopeWire` struct
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/architecture_test.go:154-169`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json.go:16-22`), while `MarshalJSON` bypasses that representation and
calls the domain-first custom encoder (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json.go:45-59`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json.go:130-134`).
The specification compounds the defect by claiming declaration order plus that
ratchet owns the wire (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:201-205`).

Attest must choose one reviewed order and make the specification, actual encoder,
decoder closure, fixed-byte behavioral oracle, and a non-vacuous architecture
ratchet observe that same owner before any Receipt wire is admitted.

### 2. The production struct inventory is exhaustive by name but unclassified

`receiptDataFlowInventory` contains one field for every production struct, and
`TestProductionStructDataFlowInventoryIsExhaustive` compares only the reflected
type names to parsed production struct names
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/architecture_test.go:15-122`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/architecture_test.go:266-279`).

No entry states whether the struct is a protocol fact, sealed projection,
internal flow carrier, capability wrapper, or wire adapter. The current testing
protocol requires every production struct in an eligible trust-boundary package
to have an intentional role and specifically distinguishes wire structs from
internal flow (`foundation@working-tree:_docs/testing_protocol.md:1046-1092`).

This matters here: `EvidenceDocument`, `VerifiedEvidence`,
`evidenceDocumentWire`, `pendingIntent`, `indexRecord`, `Store`, and
`boundedFileReadRequest` have radically different trust roles despite all being
listed in the same undifferentiated aggregate.

The Receipt specification also requires exact product-neutrality, field
classification, wire-field ownership, and no-world ratchets
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/SPEC.md:629-633`). The architecture suite has an import
allowlist, public-surface test, struct-name inventory, and parameter AST scan,
but no role classification, field-ownership ratchet, explicit product-token
ratchet, or no-world data-flow ratchet. The archive does not satisfy its own
required proof.

### 3. Mandatory durable-failure proof is absent

Receipt's specification requires real Filestore-backed coverage for:

- ENOSPC/full disk;
- partial and short writes;
- fsync failure;
- directory-sync failure;
- checkpoint ambiguity;
- symlink/reparse attacks; and
- every crash-reconciliation boundary

(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/SPEC.md:566-575`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/SPEC.md:623-626`).

The current tests cover cancellation, exact-prefix partial append recovery,
corrupt tails, exclusive open, rollover positioning, activation, sync on the
success path, checkpoint replay, and many crash states. They contain no ENOSPC,
short-write, fsync-failure, directory-sync-failure, or Filestore fault-harness
case. An exact-prefix tail manufactured before reopen proves recovery logic; it
does not prove how the Store behaves when the underlying write or durability
operation fails at runtime.

Because Store claims crash durability rather than merely serialization, these
are admission blockers, not optional coverage improvements. The clean
implementation needs a real reviewed Filestore fault harness or native
filesystem mechanism that preserves typed OS error identity and observes every
ordered durability boundary without replacing production with a fake Store.

### 4. Cross-build closure is not the required native filesystem matrix

Linux/amd64 and Windows/amd64 cross-builds pass. The fresh real filesystem suite
ran only on Darwin/arm64. Receipt itself states that cross-builds prove only
compilation closure and that native Darwin, Linux, and Windows filesystem
matrices are required before Store is permanent
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/SPEC.md:568-575`).

No evidence in the archive proves native Linux behavior for advisory locks,
rename/replace durability, directory sync, or symlink defense, or native Windows
behavior for sharing modes, reparse points, replace semantics, and lock
lifetime. Store cannot be marked permanent until those matrices are green.

### 5. Red/green provenance and full adoption remain unproved

Commit `73b161140fcc99b026defd9449136f5681fc0f4f` introduced the durable
implementation and its hostile tests together: 45 files and roughly 9,000 added
lines. Commit history therefore does not provide an auditable fail-first
red/green sequence for the new behavior. The suite is meaningful after the fact,
but green HEAD is not evidence of the development sequence required by the
testing protocol. A 2026 implementation must retain the actual failing test or
ratchet reason for each slice before production is added.

Primitive has a real Evidence consumer and Watermark consumer. It has no
independent production issuer for Evidence, no Payment document consumer, and no
remote Page consumer. Kernel supplies credible payment demand, but has not
adopted the contract. This does not invalidate the ownership model; it blocks
claiming that every archived public operation has crossed a real integration
boundary.

Any future reconsideration should require an explicit call-site contract for
each proposed surface. Do not copy all 36 exported functions and 54 exported types
merely because the archive's exact-public-surface test is green
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/architecture_test.go:176-249`).

## Primitive 2026 ownership and DAG

The corrected candidate does not contain a Receipt node. The following
archive-derived ownership sketch is evidence for a future topology review, not
the current graph:

- `core` owns cross-package nominal identities, hashes, byte extents, exact
  common field/path constants, stable error identities, and shared hard limits.
  It does not own Receipt bodies, Store state, or cryptographic execution.
- `attest` owns canonical signed-envelope framing, trust-set membership,
  signing/verification execution, and proof-carrying verified attestation.
- `currency` owns exact signed amount representation and currency validation.
- `temporal` owns exact instants and comparison.
- `filestore` owns reviewed path-safe durable file capabilities, locking,
  append, create/replace installation, sync, and OS-specific error identity.
- `receipt` owns the total Evidence and Payment authority facts, their headers
  and domains, verified projections, remote receipt Page protocol, Watermark
  derivation/advance, and the append-only verified-document Store.
- `submission` owns evidence submission, provider transfer, declaration and
  completion consistency, Journal lifecycle, and when an accepted Evidence
  Receipt is expected.
- `controlstate` owns signed aggregate snapshots, freshness, and embedding the
  Receipt Watermark.
- Kernel/composition owns provider verification, payment acceptance, entitlement,
  order/outbox atomicity, and the moment an authoritative Payment Receipt may be
  issued.
- Witness/Timeproof owns RFC3161, machine-attestation, artifact-attempt,
  verification-outcome, and fixed-point evidence.
- Release owns upload, diagnostic, manifest, publication, download-index, and
  deployment receipts.
- Peachfuzz Archive owns pack publication results, content-addressed custody,
  lifecycle attachment, and hydration.

A future topology review must derive the smallest acyclic dependency closure
from current consumers. This report does not reserve Receipt, Submission,
Controlstate, or Register nodes or edges.

No copied domain token, path component, field name, digest-role convention,
generation rule, or error string may survive in consumers. Conversely, no
product fact should be moved into Core merely because multiple documents carry
it.

### Future reconsideration sequence

The corrected `v2026.0.0` topology defers a standalone Receipt package. New
current demand would have to justify it in bounded slices:

1. repair and independently approve Attest's canonical envelope before defining
   Receipt durable bytes;
2. define the minimal Evidence/Header/Verified contract from Submission's real
   producer and consumer boundary, with red/green proof;
3. restore the Watermark contract from Controlstate's real signed-snapshot
   boundary;
4. implement Store only after Filestore fault injection and native
   Darwin/Linux/Windows matrices exist;
5. add Payment only with a reviewed Kernel issuer/consumer composition that
   preserves atomic business acceptance outside Receipt;
6. add remote Page only with a concrete authority/client integration and
   rollback/conflict semantics agreed at that boundary;
7. classify every production struct and ratchet actual wire owners, product
   neutrality, field ownership, and no-world behavior; and
8. retire copied/local mechanisms only after byte-exact or explicitly versioned
   migration proof and durable replay evidence.

There should be no aliases, shims, compatibility decoders, dual writers, or
wrapper functions preserving the archived API. Clean callers move to the
reviewed 2026 contract. Witness receipts, Bug Release receipts, Peachfuzz
`PackReceipt`, Kernel ledger events, and Rate receipts are not retirement
targets; they own different facts. Only duplicated generic signing, accepted
evidence/payment documents, Watermark comparison, or verified-document storage
become candidates after equivalence is proved.

## Decision rationale and conditions

Receipt contains strong redesign evidence: concrete total documents, three
distinct digest roles, proof-carrying verification, bounded signed Page,
nominal Watermark closures, caller-rooted verified-document Store,
pending-intent crash recovery, hash-linked replay, and an explicit threat
boundary. Current demand does not justify a `v2026.0.0` package. The archived
package must not be copied.

Future reconsideration requires:

1. one reviewed Attest envelope wire order shared by specification, production,
   behavioral bytes, decoder closure, and structural ratchet;
2. a role-classified production struct inventory plus actual field-ownership,
   product-neutrality, and no-world ratchets;
3. real typed ENOSPC, short-write, fsync, directory-sync, checkpoint, and
   symlink/reparse failure proof;
4. native Darwin, Linux, and Windows Store matrices;
5. auditable red/green proof for each clean slice;
6. real Evidence, Payment, Page, and Watermark integration contracts before
   their respective surfaces are admitted; and
7. explicit migration/retirement evidence that preserves stronger product-local
   mechanics and never claims unrelated receipts as duplicates.

Until those conditions are met, the green package gates establish an unusually
strong prototype, not a publishable permanent protocol.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
