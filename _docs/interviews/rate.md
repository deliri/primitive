# Rate package recon

Status: `COMPLETE` | Decision: `DEFER`

This report is the sole recon record for archived package `rate`. Here, `rate`
means the one-time customer-rating protocol, not request rate limiting, Web
Vitals classification, security severity, or test grading. Those colliding
domains are deliberately separated below.

## Evidence boundary

The following repositories and exact revisions were inspected read-only:

| Repository | Revision or Primitive pin | Relevant state |
| --- | --- | --- |
| `foundation_back_up_july_27th_2026` | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` | Full archived package |
| archived Rate tree | `a44e52e37aaacb047c28e5d7074ba9c2ec28ae35` | `HEAD:rate` tree |
| initial Rate specification | `0ab797959f8bee442c3a3940b4f85d4d01a129bb` | One-time rating contract introduced |
| Rate implementation | `251429ce3de8230eaa4195cec2c03bb5736bad61` | Permanent protocol implementation introduced |
| latest package-affecting change | `40ded9c104a99cbc4b0b672cd7392901b468d1eb` | Comparative-contract hardening |
| archived comparison | `e8b7172161a4` | Same Rate tree and Rate-owned Core contracts as archive HEAD |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | Committed pin `0df2954a2d91`; dirty worktree pin `e8b7172161a4` |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` | Pin `773add8ba0fc` |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` | Pin `388e593231a2` |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` | Pin `3f74d8fc35b4` |

The archive worktree has an unrelated untracked Core test. No archived source
was changed during this interview.

The focused package gates were rerun:

- `go test ./rate` completed successfully.
- `go test -race -shuffle=on -count=2 ./rate` completed successfully.
- Production-only `gocyclo -over 10` reported no findings.

The green result proves the archived package on the current host. It does not
prove a production authority adapter, consumer integration, multi-platform
execution, or suitability for Primitive 2026.

### Consumer pin facts

The committed pins used by Kernel, Witness, Bug, and Peachfuzz do not contain a
`rate` directory. Kernel's dirty worktree pin `e8b7172161a4` does contain Rate,
but Kernel has no Rate import or call site. Witness and Bug vendor trees do not
contain Rate. Peachfuzz has no vendored Rate package.

An exact import scan found:

- no `github.com/deliri/primitive/v2026/rate` import in Kernel;
- no such import in Witness;
- no such import in Bug;
- no such import in Peachfuzz.

Therefore none of the four current consumers exercises this package. Similar
words such as `rating`, `score`, `rate`, and `receipt` are not evidence of this
protocol.

## Capability ownership

The archive defines a permanent one-time customer opinion for exactly one
account/offering pair (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/SPEC.md:6-26`). Its intended transaction is:

1. construct a closed rating;
2. derive stable identity, digest, and submit idempotency;
3. submit through a shared transport;
4. reconcile ambiguous completion;
5. verify an authority-signed receipt; and
6. retain that receipt as the local completion fact.

The package correctly claims ownership of:

- a closed one-through-five `Score`;
- an optional bounded exact `Comment`;
- a nominal `RatingIdentity` derived from account and offering;
- a distinct nominal `RatingDigest` derived from complete rating facts;
- exact submit and reconcile wire structures;
- typed submit/reconcile outcomes;
- an authority-signed rating receipt;
- the two-phase reserved-to-receipted record transition;
- create-or-confirm-identical local receipt retention.

The package explicitly does not own prompting, UI, eligibility, throttling,
product identity, routes, credentials, product policy, moderation, analytics,
aggregation, editing, or public display (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/SPEC.md:807-822`).

That separation is essential. Primitive may own a product-neutral one-time
rating protocol. The composition root must own:

- whether and when a customer may rate;
- which offering is being rated;
- prompt copy and rendering;
- moderation and support workflow;
- authority endpoints and credentials;
- database uniqueness/CAS transactions;
- signer-key retention operations;
- notifications, analytics, exports, and public display.

## Archive evidence

### Closed score and exact comment

`Score` is a closed `uint8` enum with only values one through five valid
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/enums.go:110-160`). Its JSON grammar accepts exactly one bare byte from
`1` through `5`; it rejects quoted numbers, signs, leading zeroes, fractions,
exponents, whitespace, and trailing payload.

`Comment` privately stores state and exact text (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/rating.go:12-29`).
Validation rejects empty present values, invalid UTF-8, values over 140 Unicode
code points, controls, and Unicode line/paragraph separators
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/rating.go:31-60`). It does not normalize or trim accepted text. JSON is
strict, bounded, canonical, and receiver-preserving
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/rating.go:63-106`).

The rune limit and decoded byte preflight derive from one compiler-owned rule:
`RateCommentMaximumUTF8Bytes = RateCommentMaximumRunes * utf8.UTFMax`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/rate_constants.go:14-15`). This avoids a second independent length rule.

### Nominal identity and digest

`RatingIdentity` and `RatingDigest` use distinct private phantom domains over
fixed 32-byte storage (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/identity.go:15-21`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/identity.go:69-72`). They cannot be assigned to each other despite their
common cryptographic width.

Identity, rating digest, and submit idempotency use a length-prefixed,
domain-separated canonical frame (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/identity.go:120-177`). Identity derives
from account and offering (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/identity.go:180-196`); digest derives from all
rating facts (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/identity.go:198-213`); the submit idempotency key derives
from identity, digest, and the submit operation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/identity.go:215-226`).

This is a valuable mechanism. It avoids `fmt`, concatenation ambiguity, Go
layout, JSON field order, and caller-supplied idempotency.

### Immutable rating reconstruction

`NewRating` accepts source facts and derives revision, identity, and digest
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/rating.go:108-166`). `Rating.Validate` recomputes both derivations
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/rating.go:176-217`). Strict JSON decode reconstructs from account,
offering, score, and comment, then requires the canonical encoded derivations to
match (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/rating.go:220-291`).

No caller can construct a partially closed `Rating`, and no timestamp is part
of the customer's opinion. The authority separately owns acceptance time.

### Signed proof of completion

`Receipt` binds revision, identity, digest, account, offering, score, comment,
and authoritative `temporal.Instant` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/receipt.go:14-65`). Its canonical
signing frame is length-delimited and domain-separated
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/receipt.go:68-88`).

`IssueReceipt` accepts only a validated reserved `Record` and a signing key
whose public identity matches the reservation (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/receipt.go:265-306`).
`VerifyReceipt` returns a privately constructible proof-carrying
`VerifiedReceipt` only after attestation verification against caller-supplied
trusted keys (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/receipt.go:309-366`).

The signer appears once in the attestation envelope. The body does not copy it.
That is a good single-source-of-truth decision.

### Authority record transition

`Record` has exactly two phases: reserved and receipted
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/record.go:12-65`). Reservation fixes the rating, acceptance instant, and
signer (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/record.go:67-106`). A receipted record validates every receipt
fact and its signature (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/record.go:117-170`).

`AdvanceRecord` applies one typed receipt event. Reserved advances once;
receipted accepts byte-identical replay; any differing event is conflict
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/record.go:172-316`). Strict record JSON preserves the same invariant
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/record.go:318-388`).

This is a useful provider-neutral state machine. The actual unique database
transaction remains outside Primitive, as it should.

### Submit/reconcile asymmetry

Submit uses idempotent POST with the compiler-derived key
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/client.go:267-301`). Reconcile uses single-attempt POST with no
idempotency key (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/client.go:304-336`).

The rationale is strong: caching an early `unknown` reconcile result under the
lifetime identity could replay that stale result after acceptance forever
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/SPEC.md:127-149`). A later explicit reconcile must observe current
authoritative state.

Headers cannot override idempotency, content type, or content length
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/client.go:138-148`). The package delegates HTTP, TLS, retry, backoff,
redirect, status, and body-limit behavior to `exchange`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/SPEC.md:360-375`).

### Typed closed outcomes

Submit has accepted, already-rated, rejected, retry-later, and indeterminate
variants (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/result.go:13-83`). Reconcile has no-record, accepted,
retry-later, and indeterminate (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/result.go:85-150`). Private unions ensure
only the payload appropriate to a disposition is present
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/result.go:200-335`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/result.go:455-672`).

Accepted bytes are not trusted directly. The client verifies the receipt and
its expected binding before projecting a proof-carrying result
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/client.go:381-490`).

### Bounded local completion

`Store` derives its directory and target from a validated root and complete
account/offering binding (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/store.go:17-104`). Callers cannot supply an
independent target.

`Load` reads one bounded receipt, checks owner-only mode when the platform
reports it, verifies the signature, and checks the binding
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/store.go:161-260`). `Retain` accepts only a verified matching receipt and
uses `filestore.InstallCreate` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/store.go:262-323`). If create fails
because another process won, it loads and accepts only byte-identical existing
content (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/store.go:335-360`).

The process test launches five real child processes with different valid
receipts for one binding and proves one winner, four conflicts, and unchanged
winner bytes (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/process_hostile_test.go:36-179`). This is meaningful
concurrency evidence, not a mock.

### Architecture and hostile proof

The archive ratchets:

- the production import allowlist and absence of loose maps
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/architecture_test.go:34-88`);
- the package-level public function list
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/architecture_test.go:90-124`);
- create-only storage and absence of `InstallReplace`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/architecture_test.go:126-158`);
- at-most-three-parameter production operations
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/architecture_test.go:160-290`);
- reconcile's lack of idempotency
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/architecture_test.go:292-328`);
- exhaustive inventory of every local `uint8` enum
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/architecture_test.go:330-372`).

All 256 values of every closed enum are exercised
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/enum_hostile_test.go:24`). Score, comment, rating, receipt, record,
transport, ambiguity, error sanitization, store, and process concurrency have
dedicated hostile tests. One semantic JSON fuzzer covers eight public boundary
families (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/fuzz_test.go:14-141`).

### Archived Primitive dependents

Only `controlstate` imports Rate in production.

`controlstate.RatingState` is a closed union:

- unavailable;
- eligible; or
- completed with a `rate.ReceiptDocument`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/unions.go:12-67`).

Completion can be constructed only from `rate.VerifiedReceipt`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/unions.go:69-91`). Validation rejects a receipt on non-completed
states and requires a valid document for completion
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/unions.go:92-110`). Canonical JSON emits the receipt only for the
completed variant (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/unions.go:112-171`).

Control-state revision matching explicitly includes Rate
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/revisions.go:61-68`). Aggregate verification accepts Rate trust
keys only when the snapshot is completed
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:15-46`) and independently verifies the nested rating
receipt (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:97-145`). The proof-carrying aggregate then
exposes the verified receipt (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:148-215`).

This is structurally coherent, but it is archive-only composition. It is not
evidence of a live rating authority, prompt flow, or any current consumer.

## Consumer evidence

### Kernel

Kernel has no Primitive Rate import or one-time customer-rating flow. Its
committed Primitive pin `0df2954a2d91` predates the package. The dirty
worktree pin `e8b7172161a4` contains Rate, but no Kernel call site consumes it.

Kernel does have a distinct `ClientMetricRating` for Web Vitals:

- the domain is good, needs-improvement, or poor
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/client_telemetry.go:22-61`);
- `telemetry/photon.DeriveRating` classifies metric thresholds
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:telemetry/photon/photon.go:21-54`);
- the package explicitly describes anonymous performance telemetry, not a
  customer opinion (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:telemetry/photon/photon.go:1-12`).

This is a naming collision, not a reusable implementation. It must not import
or alias `rate.Score`.

Kernel's useful lesson is ownership separation: metric classification stays
with telemetry. If Kernel later gains a customer rating UI, prompt eligibility,
routes, credentials, and commercial policy must remain Kernel-owned while it
consumes a Primitive rating protocol.

### Witness

Witness has no Primitive Rate import. Its pin `773add8ba0fc` and vendor tree
predate Rate.

Witness owns a separate quality-grade domain. `GradeRecord` and `GradeBand`
live in `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/types.go:3182` and `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/types.go:3440`;
validation lives at `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/types.go:3611`; the default score of 100 is
owned at `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/constants.go:82`. Folding computes and saturates that
evidence grade in `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/fold/fold.go:1433-1473`.

That is not a one-to-five customer opinion. The local gem is the same one Rate
needs: closed typed scores, owning validation, saturation, and hostile
boundaries. No code should be shared merely because both domains use the word
the word `score`.

### Bug

Bug has no Primitive Rate import. Its pin `388e593231a2` and vendor tree
predate Rate.

Bug's nearby ordinal domain is issue `Severity`, a closed enum at
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/types.go:2398` with validation in
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/enum.go:230` and compiler-owned Core token projection in
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/types.go:4103-4147`. Field-binding tests ratchet the Core tokens
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/field_binding_test.go:143-198`).

Severity is neither a customer score nor a rating disposition. The useful gem
is the explicit typed mapping between owner-local enum and shared compiler
tokens. There is no Rate capability to copy from Bug.

### Peachfuzz

Peachfuzz has no Primitive Rate import. Its pin `3f74d8fc35b4` predates Rate.

Its relevant pattern is immutable per-run evidence: `RunEvidence` is one
machine's one fuzz-slice atom, validates all constituent types, and refuses
aggregate mutation (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/run_evidence.go:29-69`). Canonical emission is
structure-to-structure (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/run_evidence.go:72-95`).

That pattern supports Rate's principle of one immutable accepted fact, but the
domains and schemas are unrelated. Peachfuzz should not gain a Rate dependency.

## Strong mechanics and proof

The strongest reusable mechanics are the proof-carrying accepted receipt,
strict canonical closure, typed reconciliation result, and monotonic store
transition described above. The archive tests exercise those mechanics
directly, while the consumer evidence establishes only adjacent patterns and
does not establish a current production workflow for the complete package.

## Defects and blockers

### 1. There is no current product demand or integration owner

No current consumer imports Rate, and all committed consumer pins predate it.
The only dependent is archived `controlstate`. A 6,381-line Go package/test
slice (7,203 lines including its SPEC)
with no live client or authority is speculative infrastructure, even when its
local tests are strong.

Primitive 2026 must not admit it until a named consumer supplies:

- the real account/offering meaning;
- prompt eligibility and lifecycle;
- authority endpoints and credentials;
- a database uniqueness transaction;
- signer-key retention and rotation operations;
- a full ambiguous-submit/reconcile recovery path.

### 2. Outcome identity is duplicated across result and error

Authority rejection constructs `SubmitResult{Disposition: SubmitRejected}` and
also returns `core.ErrRateRejected`; retry-later does the same with
`core.ErrRateRetryable`; indeterminate does the same with
`core.ErrRateIndeterminate` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/client.go:403-424`).

Reconcile retry and indeterminate repeat the pattern
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/client.go:456-466`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/client.go:493-501`). Transport ambiguity also
returns a non-zero indeterminate result plus a non-nil error
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/client.go:291-300`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/client.go:326-335`).

This is two sources of truth for one outcome. A caller can branch on the error
and ignore the result, or branch on the disposition and ignore the error.
Primitive 2026 must choose one compiler-owned channel:

- expected authoritative outcomes as a validated result union, with `error`
  reserved for invalid/verification/execution failure; or
- typed outcome errors carrying the complete structured payload, with a zero
  result.

It must not return the same classification in both.

### 3. Rate-local contracts are over-centralized in Core

`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/rate_constants.go:8-55` owns Rate revision, operation, state, disposition,
reason, record, store-directory, and filename tokens. Lines 57-226 own every
Rate-specific wire-size derivation.

Core should retain facts shared by multiple packages: stable error identities,
generic JSON field names, common account/offering identities, common digest
widths, and shared HTTP contracts. Rate-only tokens, frame domains, size
derivations, and local store paths should be compiler-visible constants in
Rate. Moving everything into Core creates reverse conceptual ownership even
without an import cycle.

### 4. The public surface is too broad to admit without consumers

The archive exposes 29 package-level functions
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/architecture_test.go:93-103`) spanning values, client transport,
authority responses, receipt issuance/verification, authoritative record
transition, and local filesystem retention.

The specification calls this one package, but those are four distinct
capability groups:

- rating value and identity;
- client/authority wire protocol;
- authority receipt/record transition;
- local completion store.

A real consumer interview must show whether they change together. If not, the
2026 design should split them rather than admit a monolith solely to preserve
the archive.

### 5. Authoritative durability is specified but unproved

The archive correctly says `IssueReceipt` can accept only a reserved typed
record but cannot prove an external database acknowledged persistence
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/SPEC.md:297-311`). The actual unique create/readback/CAS adapter is
outside Primitive (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/SPEC.md:455-482`).

No inspected consumer implements that adapter. Package tests construct records
in memory. Therefore durable once-only acceptance is not proved across the
complete execution path.
The issuer integration must prove persistence-before-signing, crash repair,
conflicting proposal handling, and signer-key retention against its real
database.

### 6. The negative `InstallReplace` proof required by the spec is absent

The production and AST ratchets correctly pin `filestore.InstallCreate`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/store.go:304-314`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/architecture_test.go:126-158`), and a real
multi-process create-only race exists
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/process_hostile_test.go:36-179`).

However, the specification also requires the same race under
`filestore.InstallReplace` to fail, proving the positive test detects the exact
last-writer-wins defect (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/SPEC.md:758-765`). No such negative-control test
exists. The ratchet proves source selection; it does not demonstrate the
behavioral red state promised by the spec.

### 7. The attestation fuzzer lacks a cryptographic oracle

The sole fuzzer dispatches among eight JSON types and accepts success when
decode followed by re-encode is byte-identical
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/fuzz_test.go:71-85`). For `ReceiptDocument`, it unmarshals and marshals
without calling `VerifyReceipt` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/fuzz_test.go:102-107`).

`ReceiptDocument.Validate` validates body and envelope structure but does not
cryptographically verify the signature (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/receipt.go:200-211`).
Cryptographic verification happens only in `verifyReceipt`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/receipt.go:350-365`).

The specification requires a signature/binding oracle at attestation trust
boundaries (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/SPEC.md:779-782`). The current fuzzer proves canonical JSON,
not cryptographic receipt acceptance.

### 8. There is no explicit no-alias ratchet

The architecture suite inventories `uint8` declarations by inspecting
`TypeSpec` underlying identifiers (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/architecture_test.go:330-372`), but it
does not reject `TypeSpec.Assign.IsValid()`. A compatibility alias could enter
without violating the exact public function list or enum inventory.

Primitive 2026 requires an explicit no-alias/no-shim architecture ratchet.

### 9. Cross-platform claims exceed retained evidence

The specification promises native Darwin, Linux, and Windows tests for absent,
create, replay, conflict, read-only, interrupted, and concurrent retention
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/SPEC.md:661-672`). This interview verified the current host only. No
retained multi-platform run evidence accompanies the archived package.

Cross-build success is not native filesystem semantics. Admission needs
retained native evidence on every claimed platform or a narrower platform
contract.

### 10. Prompt suppression and local-loss recovery remain composition claims

The specification says the authentic CLI loads before prompting and reconciles
after local loss (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/SPEC.md:541-544`). The package has a useful loopback
test for ambiguous submit, reconcile, and retain
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/client_hostile_test.go:386-495`), plus a file-absence unit test
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/hardening_hostile_test.go:182-220`).

There is no authentic consumer CLI. Those tests prove Rate mechanisms, not that
a product cannot prompt twice. The real composition must prove:

- valid local receipt suppresses the prompt;
- corrupt receipt fails loudly;
- missing receipt reconciles before prompting;
- accepted reconciliation restores the receipt;
- unknown reconciliation alone authorizes a new submit.

### 11. Derived facts are duplicated on the wire

`ratingWire` serializes account, offering, score, comment, plus the identity and
digest derived from those source facts (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/rating.go:220-228`). Decode
reconstructs and checks them (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/rating.go:230-239`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:rate/rating.go:271-290`).

The mismatch check is safe, but the wire carries three representations of the
same rating meaning: source fields, derived identity, and derived digest. A
2026 protocol review must justify each derived field as a required
cryptographic/interoperability commitment. Any field not independently needed
should be omitted and recomputed by the owner.

## Primitive 2026 ownership and DAG

### Candidate minimum Rate domain

If a real consumer appears, the smallest defensible package owns:

- `Score`;
- `Comment`;
- `Rating`;
- nominal `RatingIdentity` and `RatingDigest`;
- canonical derivation and strict bounded encoding;
- package-local domain tokens and bounds;
- stable Rate errors shared through Core only where cross-package matching is
  required.

This leaf should depend only on Core and the standard library.

### Candidate signed-protocol layer

Receipt issuance and verification may remain with Rate if client and authority
must share exactly one body and signing domain. That raises dependencies to:

- Core;
- Rate domain;
- Temporal;
- Attest.

It should not depend on exchange or filestore.

### Candidate transport layer

Submit/reconcile wire types and execution require:

- Core;
- Contextstate;
- Rate domain/receipt;
- Attest trust;
- Exchange.

Transport must not own prompt policy, endpoints, credentials, or product
eligibility. The result/error dual channel must be resolved before admission.

### Candidate local completion layer

Local receipt retention requires:

- Core typed paths;
- Contextstate;
- Rate verified receipt;
- Filestore.

It does not need Exchange and should not force authority record machinery into a
client that only loads and retains receipts.

### Candidate authority transition

Reserved-to-receipted transition requires:

- Rate domain/receipt;
- Temporal;
- Attest key identity.

The database adapter and key-retention operations remain downstream. Their
integration proof cannot be replaced by a Primitive in-memory state machine.

### Resulting DAG

Admitting the archived monolith would require Primitive to admit and stabilize:

`core -> contextstate -> temporal -> attest -> exchange -> filestore -> rate -> controlstate`

The exact dependency order among the middle leaves can be parallelized, but
Rate cannot precede any direct dependency, and controlstate cannot precede Rate
if it retains the completed-rating union.

No current consumer needs that closure. Building it now would violate the
consumer-driven rebuild boundary.

## Decision rationale and conditions

Do not copy or implement the archived Rate package in the initial Primitive
2026 rebuild.

The archive contains excellent mechanisms worth preserving as design evidence:

- nominal, domain-separated identity and digest derivation;
- a closed one-to-five score;
- exact bounded comment handling;
- immutable rating reconstruction;
- compiler-derived submit idempotency;
- uncached reconciliation;
- signed proof-carrying completion;
- reserved-to-receipted deterministic repair;
- create-or-confirm-identical receipt retention;
- real multi-process create-only convergence;
- strong architecture and hostile boundary ratchets.

Admission requires all of the following:

1. A named current consumer with a real customer-rating use case.
2. A real authority adapter proving unique persistence before signing.
3. A real client composition proving prompt suppression and local-loss
   reconciliation.
4. A reviewed package split or explicit justification for the monolith.
5. One outcome channel instead of duplicated disposition-plus-error identity.
6. Rate-local constants moved out of Core unless genuinely cross-package.
7. Review and minimization of derived identity/digest wire duplication.
8. A cryptographic receipt fuzz oracle.
9. The promised negative `InstallReplace` behavioral control.
10. An explicit no-alias/no-shim ratchet.
11. Retained native evidence for every claimed platform.
12. Atomic consumer pin, source, test, and vendor migration only after the
    complete pushed Primitive dependency closure exists.

Until then, `controlstate` must not force Rate into the new DAG. Its rating
union should also be deferred, or specified only when the real control-plane
consumer is ready to supply the complete protocol closure.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
