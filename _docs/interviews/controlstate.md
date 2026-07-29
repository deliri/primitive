# Controlstate package interview

Status: `COMPLETE` | Decision: `DEFER`

This is the sole reconstruction report for archived package `controlstate`.
The archive is evidence, not authority. No archived source was copied.

The package contains several strong compiler-owned mechanics, but the evidence
does not support admitting the archived aggregate unchanged. No inspected
consumer revision contains the package, the only Primitive dependent is the
later archived `register` package, and two contract defects sit directly on the
trust boundary:

1. the document derives a 30-minute aggregate expiry but exposes no typed
   operation that assesses or rejects an expired aggregate; and
2. `Issue` can sign a completed rating reconstructed from an unverified raw
   `rate.ReceiptDocument`, despite the specification requiring a
   proof-carrying `rate.VerifiedReceipt`.

The 2026 decision should preserve the narrow proof, closure, and advance
mechanics while deferring the total commercial-lifecycle aggregate until a real
consumer proves which facts must be atomically bound.

## Evidence boundary


### Source revisions and pins

| Source | Exact revision or Primitive pin | `controlstate` availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`) | Present. Specification introduced by `c34125b3f0e5c5e24cd86eac5389cbd31c0b9cb9`; implementation introduced by `7cec8d33faefa21bc48659d456aa68ebb02bc33d` on 2026-07-26. | One unrelated untracked file, `core/api_http_boundary_hostile_test.go`; `controlstate` and its Core contract files are clean against HEAD. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | Committed `go.mod` pins `0df2954a2d911a5d7d775691d023d569affa2c20`; dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43`. Neither Primitive revision contains `controlstate`. | Broad pre-existing dirty migration. The committed and dirty pins are reported separately. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9`. That revision does not contain `controlstate`. | Only the pre-existing untracked ledger was observed. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3`. That revision does not contain `controlstate`. | Only the pre-existing untracked ledger was observed. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75`. That revision does not contain `controlstate`. | Only the pre-existing modified ledger was observed. |

The exact absence result matters. A direct-import scan alone can miss a package
present in a pin but not yet adopted; here `git cat-file` against all five
consumer pins proves the package directory itself is absent. There is no
downstream cutover evidence to interpret.

The material archive history is:

- `c34125b3f0e5c5e24cd86eac5389cbd31c0b9cb9` specifies aggregate lifecycle
  control state;
- `4ce34f1f0a7de3a5d59cd50c12fdd9f9f79f95f9` establishes commercial Core
  contracts;
- `bfc888f8af2cee8e43928f07a7f1a961888bfc76` implements the Receipt watermark
  prerequisite;
- `7cec8d33faefa21bc48659d456aa68ebb02bc33d` implements the complete aggregate;
- `385669e7af53a252673ad38a6a061688aa67a900` adds the sole production
  dependent, `register`; and
- `6906facd7c6e67754455185891cfcb336101eb61` hardens policy boundaries and
  recovery.

The implementation commit added approximately 8,157 lines across
`controlstate` and its Core contracts in one change. At archive HEAD,
`controlstate` contains 3,181 production Go lines, 4,768 test/fuzz Go lines, and
a 683-line specification. Size is not itself a defect, but it raises the proof
bar for a package with no external adopter.

## Capability ownership

The specification describes one pure, product-neutral, signed lifecycle
aggregate. It binds one account/offering/device relationship to:

- a catalog plan;
- device-registration state;
- a verified lease;
- a verified action gate;
- a verified call budget;
- optional completed rating state;
- an accepted-receipt watermark;
- independently signed service status; and
- independently signed latest-release state.

The outer signature is not meant to replace the nested authorities. Its
specific job is to prevent a caller or intermediary from splicing individually
valid narrow documents from different bindings or aggregate generations
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:6-26`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:290-322`).

The package is deliberately pure. It owns no transport, clock read, storage,
server, client, scheduler, database, cloud SDK, terminal, or product hook
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:16-26`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:45-52`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:666-683`).

Its stable package operations are:

```go
NewBinding(BindingRequest) (Binding, error)
NewPlan(PlanRequest) (Plan, error)
Issue(IssueRequest) (Document, error)
Verify(VerifyRequest) (Verified, error)
Advance(AdvanceRequest) (AdvanceResult, error)
```

Those five operations are isolated in `public.go`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:73-95`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/public.go:3-15`). Constructors and parsers for their owned scalar
and union types remain beside those types.

## Archive evidence

### The aggregate does not erase narrow authority

`IssueRequest` accepts proof-carrying `lease.Verified`, `gate.Verified`,
`callbudget.Verified`, `status.Verified`, and `release.VerifiedLatest` values
rather than caller-assembled nested documents
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:17-45`).

`closeVerifiedComponents` extracts the exact authenticated documents from those
proof values and closes them into fixed private storage
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/components.go:15-84`). On ingress, `Verify` authenticates
the outer envelope and delegates each nested document back to its owning
package. It retains the resulting proof-carrying values instead of reducing
them to booleans or prose
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:49-168`).

This is the correct direction for an aggregate proof: structure-to-structure,
with nested stable error identities preserved through wrapping.

### Identity is derived, not caller selected

`Binding` contains nominal Core account, offering, and device identities plus
the independently nominal Lease and Gate subjects
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/values.go:136-197`). `Identity` is a private SHA-256 value
derived from a domain-separated, revisioned, length-prefixed frame over that
binding; the caller cannot supply it to `Issue`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:97-124`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/values.go:273-297`).

The architecture test also ratchets `IssueRequest` against gaining caller-owned
identity or expiry fields
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/architecture_hostile_test.go:122-155`).

### Closed values and fixed-capacity unions

Revision, signing domain, billing cadence, term unit, capability,
registration state, rating state, receipt state, and advance state are closed
typed enums with invalid zero values. Generation and plan generation are
positive private `uint64` values serialized as canonical decimal strings
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/enums.go:5-280`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/values.go:80-134`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/plan.go:14-71`).

The rating and receipt states use private closed unions rather than nullable
bags. `CapabilitySet` copies caller input into fixed storage, validates padding,
and has a compile-visible capacity tied to the single current capability
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/unions.go:12-316`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/plan.go:165-262`).

These are good compiler-owned patterns. The particular commercial members are
not yet proven generic; the representation mechanics are.

### Canonical, bounded encoding

The fifteen-field snapshot uses a bounded canonical writer. Maximum document
size is derived from the exact nested maxima, and compile-time assertions prove
that both snapshot and document fit the shared strict-JSON ceiling
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/controlstate_constants.go:71-205`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:388-414`).

JSON decoders reject unknown, duplicate, reordered, non-canonical, trailing,
and oversized input and preserve the receiver on failure. Canonical encoding
is deterministic and bounded by the compiler-owned schema
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:434-624`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/document.go:24-80`).

### Aggregate cross-invariants are explicit

Validation derives identity, checks all scope bindings, closes nominal revision
mappings, derives aggregate expiry, rejects nested issue instants later than
the aggregate issue, and enforces the registration/Gate lattice
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:103-201`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:322-386`).

The revision bridge uses typed exhaustive switches for each nominal nested
revision rather than comparing coincidentally equal text or integers
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/revisions.go:12-77`).

### Advance is total over the schema

`Advance` distinguishes rollback, replay, equal-generation conflict, and higher
generation. A fixed classification table assigns every one of the fifteen
snapshot fields to immutable, monotonic, transition, delegated, derived, or
dispatch handling
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/advance.go:97-220`).

Narrow state progression delegates to the owning package:

- `lease.Advance`;
- `gate.Advance`;
- `callbudget.Advance`;
- `status.Advance`; and
- `release.AdvanceLatest`.

The aggregate does not recode those rules
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/advance.go:358-397`). An AST ratchet requires every
snapshot field exactly once in the table
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/architecture_hostile_test.go:15-120`).

### Stable error identity

Core owns `ErrControlStateContract`, `ErrControlStateVerification`,
`ErrControlStateExpired`, `ErrControlStateRollback`, and
`ErrControlStateConflict`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:107-111`). Local errors wrap those identities
and preserve nested causes for `errors.Is`/`errors.As`; their rendered form is a
bounded label and does not render hostile nested errors
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/errors.go:5-51`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/errors_hostile_test.go:16-46`).

### The implementation is bounded and mechanically clean

Production has no maps, goroutines, network, filesystem, clock read, or
unbounded collection. Work is constant in the closed schema and O(n) only in
the compiler-bounded canonical byte extent
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:560-577`).

At the inspected archive tree:

- `go test ./controlstate ./register` passed;
- `go vet ./controlstate` passed;
- `staticcheck ./controlstate` passed;
- `gocyclo -over 10 controlstate` reported only test/fuzz functions, so
  production remains at or below ten; and
- 48 top-level test/fuzz entry points exist, including five semantic fuzzers.

Those results prove substantial local engineering. They do not supply the
missing contract semantics or downstream adoption proof.

### Primitive dependency graph

The archive's direct production dependencies are:

```text
core / temporal / currency / attest / lease / gate / callbudget / rate /
receipt / status / release
    |
    v
controlstate --> register
```

More precisely, `controlstate` imports standard library plus `core`,
`temporal`, `currency`, `attest`, `lease`, `gate`, `callbudget`, `rate`,
`receipt`, `status`, and `release`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:28-52`; verified by `go list` at archive HEAD).

### All Primitive dependents

There is exactly one direct production dependent: `register`.

- The installed enrollment response carries one `controlstate.Document`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/client.go:294-324`).
- The registration receipt binds the exact document digest and generation;
  the client closes account/offering/device, digest, generation, and permitted
  registration state before accepting the response
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/client.go:460-510`,
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/protocol.go:595-600`).
- The client then calls `controlstate.Verify` with the complete typed trust
  capability set (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/client.go:513-524`).
- Complete durable records retain the document, not a fabricated summary, and
  re-verify the receipt and aggregate on load before returning proof-carrying
  values (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:446-510`).

No other Primitive production package imports `controlstate`. No package
imports `register`, so there is no indirect Primitive production dependent
beyond that leaf. Register's tests also import `controlstate` for fixtures and
hostile response/store proof, but that does not add an independent use case.

Register is useful evidence for how a verified aggregate could cross transport
and persistence. It is not independent market proof: it was implemented after
`controlstate`, in the same archive, specifically around the aggregate
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:23-55`).

## Consumer evidence

### Kernel

Kernel has no `controlstate` import and neither its committed nor dirty
Primitive pin contains the package.

Its relevant lifecycle behavior is split among real owners rather than one
total snapshot:

- `core.Plan` is a Kernel-owned closed enum and currently contains only
  `PlanFree` (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/plan.go:10-68`).
- Browser device hints are explicitly low-trust evidence until the server
  validates and signs what it accepts
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/device_snapshot.go:13-22`).
- Device identity excludes location and IP and uses a domain prefix plus
  length-prefixed fields to avoid delimiter collisions
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/device_snapshot.go:125-188`).
- Provider plus provider-payment-object identity owns subscription/payment
  lifecycle lookup; neither provider nor identity is inferred from a string
  prefix (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/provider_payment_contracts.go:25-43`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/provider_payment_contracts.go:85-115`;
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/order_ports.go:9-30`).
- Payment cancellation is a distinct typed ledger event, including
  subscription deletion, refund, or dispute
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/ledger/event.go:117-143`).

Kernel's gems are the trust distinction between hints and accepted facts,
injective identity framing, explicit provider scoping, and atomic transition
ports. Kernel does not prove that plan, registration, rating, receipt,
service-status, release, lease, gate, and call budget must be delivered in one
signed aggregate. Its plan vocabulary also demonstrates that archived
Controlstate commercial members are not automatically universal.

### Witness

Witness has no `controlstate` import and its Primitive pin does not contain
the package. It uses the older `foundation/v2026/license` protocol and owns
consumer-local durable state.

The strongest Witness evidence is:

- one state owner retains the independently signed server-time commitment,
  last check-in, durable clock floor, and the lease identity/generation
  high-water pair; it deliberately carries no usage tally or teaching flag
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:12-26`);
- persistence-boundary validation requires the lease identity and generation
  floors to appear together and validates every retained typed fact
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:63-90`);
- accepted check-in state merges concurrent clock observations and ratchets
  progression without lowering either
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:126-187`);
- trusted time selects the maximum authentic floor when the wall clock has
  implausibly regressed (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:190-207`);
- stored grants must verify against the pinned authority, match the device and
  server-time commitment, survive the progression floor, and remain writable
  at the trusted time before becoming trusted
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/gate.go:35-105`); and
- the check-in path sends the prior lease identity/generation, a fresh nonce,
  device fingerprint, exact binary identity, and platform, then commits only
  the verified nonce-bound response under store ownership
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:239-377`).

Witness's gems support nominal binding, proof-carrying ingress, anti-rollback
high water, trusted time, and re-verification at persistence. They also show an
important ownership boundary: mutable local progression, check-in cadence, and
clock floors do not belong inside an authority-signed aggregate.

Most importantly, Witness is designed for offline operation across the signed
lease window. It does not refresh a total lifecycle snapshot every 30 minutes.

### Bug

Bug has no `controlstate` import and its Primitive pin does not contain the
package. It also uses the older `foundation/v2026/license` protocol.

Bug contributes the richest product-specific counterexample to a universal
aggregate:

- its local state includes a signed time commitment, clock and lease
  progression floors, product command usage, last check-in, and a one-time
  teaching flag (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:12-24`);
- usage is a typed command-by-command structure, not a loose map
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:26-77`);
- failed transmission does not lose counts: a report closes a copy of the
  current window, and only authenticated acceptance subtracts the reported
  snapshot from concurrently accumulated usage
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:194-255`);
- offline lease import advances clock and progression only; it does not erase
  usage or pretend a check-in occurred
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:270-298`);
- trust requires a pinned signature, device fingerprint and writer binding,
  trusted clock floor, writable lease, and a signed writer-revocation set
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/gate.go:35-139`).

The Bug gems are concurrency-safe subtractive acceptance, separate offline
import semantics, writer-key binding, and independently authenticated
revocation. Its usage and teaching facts are intentionally consumer-owned.
They must not be added to a generic Controlstate merely to make one document
feel complete.

Bug also makes the 30-minute question unavoidable: a valid offline commercial
lease must continue to authorize local work without an aggregate refresh every
30 minutes.

### Peachfuzz

Peachfuzz has no commercial Controlstate or license consumer. Its Primitive
pin does not contain `controlstate`; its production Primitive imports concern
typed Core values, context, exchange, durability, shutdown, workload identity,
host resources, and fuzz evidence.

Current Peachfuzz uses the word `lease` for resource-custody
mechanics, not commercial lifecycle state:

- the scheduler owner returns a private typed lease that binds target context,
  scheduler entry, and slot and validates all three together
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/schedstate.go:53-75`);
- one owner grants and releases slots, validates release payload identity, and
  checks global slot accounting before persistence
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/schedstate.go:535-579`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/schedstate.go:581-679`);
- worktree in-memory lease state is authoritative while durable markers exist
  only for crash evidence and startup salvage
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/worktree/worktree.go:117-152`); and
- release takes a typed evidence-custody disposition and is idempotent
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/worktree/worktree.go:461-508`).

The Peachfuzz gem is ownership clarity: a lease is a typed grant from one state
owner, with explicit release/custody semantics. It is not evidence for the
commercial aggregate. Adding a `controlstate` dependency to Peachfuzz would be
world building without a use case.

## Strong mechanics and proof

### Cross-consumer conclusion

The consumers independently prove these reusable needs:

| Need | Kernel | Witness | Bug | Peachfuzz | Likely owner |
| --- | --- | --- | --- | --- | --- |
| Nominal device identity and injective binding | Yes | Yes | Yes | Target/run identity analogue | Core identity primitive only after its own admission proof |
| Signed narrow authority facts | Accepted auth snapshot | Lease/time commitments | Lease/time/revocation | Signed run evidence | `attest` plus each narrow protocol owner |
| Monotonic generation/high-water rejection | Atomic order transitions | Lease ID/generation floor | Lease ID/generation floor | Sequence/slot owner | Narrow owner plus consumer persistence |
| Trusted-time rollback resistance | Not a Controlstate use | Strong | Strong | Scheduler time is unrelated | `temporal`/`timeproof`, Lease, and consumer state |
| Offline commercial execution | No current paid CLI evidence | Yes | Yes | Not commercial | Lease/Gate composition |
| Product usage accounting | Payment/ledger facts | Explicitly absent | Strong and Bug-specific | Run evidence, unrelated | Consumer |
| Rating state | No product-rating use found | None | None | None | Not admitted from this evidence |
| Receipt watermark | No matching aggregate watermark | None | None | Archive receipts are evidence custody, not commerce | `receipt` only if separately admitted |
| Full nine-domain atomic snapshot | No | No | No | No | Unproven |

The aggregate's anti-splice problem is coherent, but the selected total schema
is not independently demanded. The consumer evidence favors composing verified
narrow facts at the real operation boundary and persisting only the high-water
facts that operation needs.

## Defects and blockers

### 1. Aggregate expiry is compiler-visible but not enforceable

The snapshot derives `ExpiresAt` as the earliest of the 30-minute outer
ceiling, Lease grace end, Gate validity, Callbudget expiry, Status validity,
and Release Latest validity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:203-320`). The public surface exposes
`Snapshot.ExpiresAt` but has no `Assess`, `Evaluate`, `Freshness`, or
observed/trusted-time request
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/public.go:3-15`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:416-432`).

`VerifyRequest` contains trust keys, the document, and expected binding only
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:15-46`). `Verify` authenticates signatures
and cross-invariants but never compares a trusted observation with aggregate
expiry (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:61-168`).

`core.ErrControlStateExpired` is reached only while deriving expiry when
`ExpiresAt` is not strictly after `IssuedAt`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:221-244`). A correctly issued document
remains verifiable indefinitely after its signed expiry.

This creates an informal protocol the compiler cannot enforce:

- if callers manually compare `Snapshot.ExpiresAt`, every caller can drift;
- if callers treat nested Gate/Lease assessment as sufficient, aggregate
  expiry has no operational meaning; and
- if aggregate expiry is a refresh hint rather than a validity boundary, its
  name and error identity are wrong and need a closed typed freshness contract.

This is an admission blocker.

### 2. The 30-minute ceiling conflicts with demonstrated offline operation

Core fixes both `CapabilityOfflineOperation` and a 30-minute maximum snapshot
lifetime (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/controlstate_constants.go:39-55`). If the missing
expiry enforcement is added literally, an otherwise valid offline Lease/Gate
composition becomes unusable after 30 minutes without a new total snapshot.

Core's own compile-time coupling note says this ceiling is dominated by
Callbudget's rolling interval and statically asserts that relationship
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/controlstate_constants.go:206-214`). The short lifetime is
therefore deliberate and structurally tied to Callbudget, not an incidental
default; it cannot silently double as a long offline authorization window.

Witness and Bug both preserve offline execution against signed lease windows
and durable trusted-time floors. Bug's offline import explicitly advances the
authentic lease progression without pretending an online check-in occurred
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:270-298`), while Witness rate-limits online
refresh so an offline machine does not dial on every command
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:106-124`).

The contract must decide, in types, whether aggregate expiry:

- denies use;
- requires refresh but permits narrow offline authority;
- expires only some projections; or
- should not exist above the narrow documents.

The archive leaves that decision implicit.

### 3. `Issue` can sign an unverified completed rating

The specification says completed `RatingState` carries exactly one
`rate.VerifiedReceipt`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:236-255`). The intended constructor enforces
that proof and extracts its document
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/unions.go:69-91`).

However, public JSON decoding reconstructs the same `RatingState` directly
from a structurally valid raw `rate.ReceiptDocument`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/unions.go:112-120`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/unions.go:153-171`). `RatingState.Validate`
checks only `ReceiptDocument.Validate`, not its signature
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/unions.go:92-106`).

`IssueRequest` accepts `RatingState`, validates it structurally, and signs it
without a Rate trust capability or retained `rate.VerifiedReceipt`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:17-45`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:66-100`). The nested Rate
signature is checked only later by a separate consumer's `Verify`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:137-145`).

Therefore an issuer boundary can outer-sign an invalid nested rating after
decoding it from JSON. The outer signature does not make that inner claim
authentic. Existing hostile proof checks wrong Rate keys during `Verify` and
round-trips completed `RatingState` JSON, but does not feed a decoded
wrong-signer completed state into `Issue`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/aggregate_hostile_test.go:760-803`).

The 2026 issuance type must make the distinction structural. Viable clean-cut
directions include a separate issuer-only input union whose completed variant
contains `rate.VerifiedReceipt`, or an `IssueRequest` field that carries the
verified receipt independently and derives the wire union. Do not add a
boolean, wrapper shim, or trust-by-convention comment.

### 4. Future-valid components are not rejected at issuance

The specification states that already expired or not-yet-valid components
prevent issuance (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:353-366`).

Implementation checks that every nested issue instant is no later than the
aggregate issue and derives the earliest end
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:322-386`). It does not compare aggregate
issue with:

- `lease.Fact.NotBefore`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/fact.go:148-168`); or
- `release.LatestFact.ValidFrom`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/latest.go:100-102`).

The hostile test named `TestSnapshotRefusesNestedIssueAfterAggregateIssue`
covers nested issue instants, not these separate validity starts
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/aggregate_boundary_hostile_test.go:638-686`).

Either issuance must reject those future-start facts, or the specification
must define a typed pending-validity aggregate state. Current code and contract
disagree.

### 5. The total schema creates unproved availability coupling

Every snapshot requires Lease, Gate, Callbudget, Status, and Release Latest,
even when a lifecycle operation changes only one narrow fact
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:17-31`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:48-64`). Every authoritative
lifecycle operation is then required to return a new complete Controlstate
document rather than a delta
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/SPEC.md:502-522`).

That design:

- requires every issuer path to obtain all five mandatory documents;
- makes the shortest unrelated validity end the aggregate refresh boundary;
- makes plan/rating/receipt/status/release deployment availability part of
  lease/gate publication; and
- forces all consumers to trust every nested authority even when they use only
  one projection.

The outer anti-splice signature may justify that coupling, but no inspected
consumer implements or requests it. Witness and Bug instead authenticate and
persist the narrow facts needed for their operation. This is an architectural
proof gap, not permission to add optional fields or maps.

### 6. Commercial vocabulary is consumer policy, not Primitive Core

The archive places billing cadence, commercial term, plan display, price,
offline capability, registration, rating, receipt, and their maxima into one
generic package. Core then owns 149 `ControlState` references, including
package-private Go symbol names and call counts as well as wire values and
bounds (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/controlstate_constants.go:5-217`).

Consumer evidence settles ownership:

- Kernel owns a one-member `PlanFree` domain
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/plan.go:10-68`);
- Witness intentionally carries no usage or teaching state
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:12-26`);
- Bug owns command usage, teaching state, and local check-in policy
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:12-24`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:164-176`); and
- Peachfuzz has no commercial lifecycle use.

Plan members, offer members, prices, display text, billing choices, term
limits, offline entitlement, usage, rating policy, and receipt policy remain
downstream. Controlstate-private AST names, implementation function names,
file names, package names, and call counts belong in package tests as local
ratchets if a smaller aggregate is later admitted. They do not become shared
contracts merely because tests inspect them.

The 30 minute lifetime is rejected with the deferred aggregate rather than
retained as a generic default. If a smaller Controlstate package is admitted
later, it may own only its noncommercial schema discriminants and
package-local bounds. Stable cross-package error identities and independently
proven shared facts remain Core candidates. See `_docs/interviews/core.md`,
section `Controlstate commercial constants`, for the matching Core
disposition.

### 7. Archive review status is internally contradictory

The package index describes `controlstate` as specified with prerequisites
pending
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:28`). The same index says Receipt and Rate
implementations are pending (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:37-39`), even though
the source tree contains them.

The pending ledger says `controlstate`, `rate`, and `receipt` have reviewed
production implementations
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_pending.md:41-50`), and the completed ledger claims the
Controlstate implementation and all gates were reviewed and green
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1334-1387`).

This does not invalidate passing code, but it prevents the archive's status
labels from serving as trustworthy admission evidence. The new report and
future package ledger must have one source of truth.

### 8. No real consumer cutover or persistence migration was proved

No consumer pin contains the package. The sole archive dependent, Register,
persists one initial document and re-verifies it on load, but no implementation
shows:

- repeated aggregate refresh;
- durable aggregate anti-rollback across process restarts;
- recovery after an ambiguous refresh;
- how a stale aggregate coexists with a newer independent Status document;
- a 30-minute offline boundary;
- trust-key rotation across all nested authorities; or
- migration from the current Witness/Bug lease state.

`Advance` is a strong pure comparator, but it is not durable high-water
ownership. A production composition root still needs a typed persistence
transaction and crash/replay proof.

## Primitive 2026 ownership and DAG

### Preserve in narrow owners

- `attest` owns signing domains, envelopes, trusted-key capabilities, and
  proof-carrying verification.
- `temporal` and `timeproof` own typed instants, duration arithmetic, and
  trustworthy observation/floor mechanics.
- `lease` owns commercial validity windows, trusted-time lease assessment, and
  lease progression.
- `gate` owns per-action permission and recovery decisions.
- `callbudget` owns covered lifecycle-call admission and reservation.
- `status` owns independently deployed service-status truth.
- `release` owns Latest and release assessment.
- `rate` owns rating acceptance and its verified receipt.
- `receipt` owns accepted-receipt watermark progression.
- consumer stores own durable clock/generation high water and crash-safe
  mutation.

Controlstate must never copy those rules into parallel fields.

### Core candidates

Only genuinely shared facts should survive in `core`:

- stable `ErrControlState*` identities if the package is admitted;
- independently admitted account and device nominal identities;
- JSON field names or size relationships shared by more than one package;
- exact cross-package compile assertions that cannot be owned by either side.

Offering and plan identities remain outside Core because current consumers do
not demonstrate one shared opaque contract for either domain. Keep
Controlstate-only enum members, implementation symbol names, file names, AST
call counts, and commercial policy out of Core. Tests can own private typed
expectation tables without turning implementation names into global API.

### Candidate aggregate boundary

If real consumer evidence ultimately requires an anti-splice aggregate, the
minimum acceptable DAG is:

```text
core / temporal / attest
          |
          v
lease gate callbudget status release rate receipt
          |
          v
      controlstate
          |
          v
 register and consumer composition roots
```

No narrow package may import `controlstate`. `controlstate` must not import
transport, persistence, clock observation, product policy, or consumer code.

Before freezing the schema, the design must answer:

1. Which exact operation requires all selected narrow facts atomically?
2. Which facts are mandatory for every consumer?
3. What does aggregate expiry do at a typed execution boundary?
4. How does offline authority survive the aggregate refresh ceiling?
5. Which validity-start and expiry rules belong to Issue, Verify, or Assess?
6. How is durable aggregate generation high water committed?
7. Why are plan, rating, and receipt state generic Primitive facts rather than
   OGS/consumer projections?

Convenience of a complete document is not sufficient evidence.

## Decision rationale and conditions

### Required admission proof

Before any 2026 production implementation begins:

1. obtain one source-cited consumer operation that needs anti-splice closure
   across at least the proposed mandatory components;
2. reduce the snapshot to that proven minimum rather than copying all fifteen
   archived fields;
3. define a typed freshness/assessment result and make stale, not-yet-valid,
   refresh-required, offline-permitted, and terminal states exhaustive;
4. make completed-rating issuance structurally require
   `rate.VerifiedReceipt`;
5. prove every validity start and end at one nanosecond before, equal, and
   after the observation/issue boundary;
6. preserve downstream ownership of plan, billing, rating, registration, and
   receipt policy;
7. preserve nested proof-carrying values and require each nested signature
   exactly once at aggregate ingress;
8. retain a total compiler-owned field-classification ratchet for Advance;
9. prove no stale outer document can be treated as current merely because its
   nested Gate or Lease remains valid;
10. prove no valid offline Lease/Gate authority is accidentally reduced to a
    30-minute online requirement;
11. prove durable generation high water, equal replay, equal fork, rollback,
    crash-before-commit, crash-after-commit, and ambiguous refresh;
12. prove authority-key rotation and independent nested-key failure;
13. keep encoding strict, canonical, bounded, receiver-preserving, and O(1) in
    the closed document extent;
14. keep production `gocyclo <= 10` and preserve typed error identity through
    `errors.Is`/`errors.As`;
15. run native-platform build proof and the full canonical gate; and
16. migrate one real consumer before calling the package permanent.

Tests must follow `foundation@working-tree:_docs/testing_protocol.md:1-12` and must begin with
red behavioral proof for the expiry, future-validity, and decoded-rating
defects. AST ratchets come after the behavior is correct.

### Current rationale

**The archived `controlstate` package remains outside the current import graph
until a real consumer proves the minimum atomic aggregate.**

Preserve as design evidence:

- derived nominal aggregate identity;
- private immutable document and proof-carrying `Verified`;
- strict bounded canonical wire closure;
- nested verification delegated to each owning package;
- total field-classified Advance;
- stable typed error wrapping; and
- fixed-capacity, no-map, no-I/O implementation discipline.

Required corrections before possible specification:

- unenforced aggregate expiry;
- the ambiguous 30-minute/offline contract;
- raw decoded completed-rating issuance;
- missed future `Lease.NotBefore` and `Release.ValidFrom` boundaries;
- an unproved mandatory nine-domain snapshot;
- unproved generic commercial vocabulary;
- Controlstate-private implementation names in Core; and
- archive status claims as a substitute for consumer cutover evidence.

The strongest current 2026 direction is to finish and admit the narrow
packages independently, preserve consumer-owned durable high-water state, and
revisit an aggregate only when one concrete operation demonstrates that
separate verified documents permit a splice the composition root cannot
otherwise close. That produces a smaller compiler-owned contract and avoids
building a lifecycle world model merely because the archive already contains
one.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
