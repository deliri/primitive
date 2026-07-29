# Lease package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole reconstruction report for archived package `lease`.
The archive is evidence, not authority. No archived source was copied.

The commercial-lease capability is real. Witness and Bug both implement
device-bound signed lease consumption, offline operation, expiry/grace
decisions, renewal check-ins, and durable anti-rollback state. The archived
Primitive package also contains strong reusable mechanics: one narrow signed
timeline, nominal identity and subject types, exact boundary classification,
proof-carrying verification, and a total generation-advance lattice.

The archive must not be copied unchanged. Its central rollback claim is stronger
than its types and implementation:

- `Evaluate` computes `max(ObservedAt, signed issuance-time floor)`;
- after a client has observed a later time, a rollback to any instant still
  above that old signed floor can move an expired lease back into active or
  grace; and
- Witness and Bug prevent exactly that attack by merging signed server time,
  durable state high water, and durable device high water before evaluating a
  lease.

The archive also does not make expected-subject binding part of `Verify` or
`Evaluate`, uses guessed rather than schema-derived JSON ceilings, contradicts
itself about a seven-day grace minimum, and describes a fixed-capacity
canonical writer that production does not implement.

The 2026 direction is therefore a clean admission of the narrow capability
after those contracts are corrected. Consumer-owned check-in, persistence,
usage, writer attestation, revocation, billing policy, and product prose must
remain outside `lease`.

## Evidence boundary


### Source revisions and pins

| Source | Exact revision or Primitive pin | `lease` availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`) | Present. Introduced by `99fee07e02aa534bd2cd5ab683bc9945e5473c78` on 2026-07-25. | One unrelated untracked file, `core/api_http_boundary_hostile_test.go`; `lease` and inspected Lease/Core contract files are clean against HEAD. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | Committed `go.mod` pins `0df2954a2d911a5d7d775691d023d569affa2c20`; dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43`. Neither revision contains `lease`. | Broad pre-existing dirty migration. The committed and dirty pins are reported separately. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9`. That revision does not contain `lease`; current code consumes the older `foundation/v2026/license` protocol. | Only the pre-existing untracked ledger was observed. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3`. That revision does not contain `lease`; current code consumes the older `foundation/v2026/license` protocol. | Only the pre-existing untracked ledger was observed. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75`. That revision does not contain `lease`. | Only the pre-existing modified ledger was observed. |

`git cat-file` against all five consumer Primitive pins, including Kernel's
dirty pin, proves that the `lease` directory is absent at each revision. No
consumer could have imported the archived replacement package at its recorded
pin. Witness and Bug are nevertheless strong independent capability evidence
because they consume the older live license protocol and own the surrounding
production state.

The material archive history is:

- `99fee07e02aa534bd2cd5ab683bc9945e5473c78`: add the signed Lease primitive;
- `d259789e87bcadb829c5ffac72c6c91ccc604098`: centralize constants and close
  capabilities;
- `4ce34f1f0a7de3a5d59cd50c12fdd9f9f79f95f9`: establish later commercial Core
  contracts;
- `6906facd7c6e67754455185891cfcb336101eb61`: harden policy boundaries and
  lifecycle recovery; and
- `6f35a55050caea6dac7b630f278d76aa6f58ceb5`: consolidate Temporal operating
  primitives used by the package.

At archive HEAD, `lease` contains 1,363 production Go lines, 2,415 test/fuzz Go
lines, and a 375-line specification. `archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:35` calls it a
reviewed implementation and reviewed permanent contract. The completed ledger
also records user approval and green hostile, analysis, cross-build, and lint
proof (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1191-1224`).

Those status claims describe the archive's review state. They do not override
the defects proved below or constitute 2026 admission.

## Capability ownership

`lease` is a pure product-neutral signed commercial timeline. It owns:

- one continuing opaque Lease identity;
- one opaque commercial subject;
- one protocol revision and monotonic generation;
- issue, not-before, renewal-open, paid-through, term-end, grace-end, and
  trusted-floor instants;
- the Lease attestation domain and canonical document;
- proof-carrying verification;
- exact timeline and renewal classification; and
- same-generation conflict, lower-generation rollback, immutable identity
  conflict, and monotonic-boundary advance.

It deliberately does not own a clock read, payment provider, plan, price,
account model, action grant, filesystem, persistence transaction, network,
retry, product copy, or executable
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:6-18`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:363-375`).

Its stable intent operations are:

```go
NewFact(FactRequest) (Fact, error)
Verify(VerifyRequest) (Verified, error)
Advance(AdvanceRequest) (AdvanceResult, error)
Evaluate(EvaluateRequest) (Assessment, error)
```

Five scalar constructors close Lease identity, subject, and generation ingress
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:40-67`; `archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/public.go:5-48`).

The signed `Fact` has exactly eleven typed fields:

1. Lease identity;
2. subject identity;
3. protocol revision;
4. generation;
5. issue instant;
6. not-before instant;
7. renewal-open instant;
8. paid-through instant;
9. term-end instant;
10. grace-end instant; and
11. trusted-time-floor instant.

The schema and invariants are explicit in
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:97-151` and `archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/fact.go:12-122`.

## Archive evidence

### Archived strengths worth preserving

### Narrow, product-neutral authority

Production imports only the standard library, `core`, `attest`, and
`temporal`. An architecture ratchet forbids product names, maps, reflection,
clock reads, process/filesystem/network access, and undeclared Primitive
siblings (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/architecture_test.go:66-105`).

This is a materially cleaner boundary than the older live license protocol.
The old `SeatLeaseBody` embeds Bug writer identity, developer key, device
fingerprint, plan, billing period, prepaid years, collection windows, and write
grace in the same signed body
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/lease.go:25-89`). The old
`SubscriptionLeaseBody` similarly embeds Witness plan and billing policy
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/lease.go:210-266`).

The archive correctly extracts the reusable signed timeline and leaves product
and billing vocabulary with the issuer and composition root.

### Nominal immutable values

`Identity` and `Subject` are separate private fixed-size types, not aliases.
They close exactly 128 nonzero bits and emit canonical lowercase hexadecimal.
`Generation` privately retains a positive `uint64` and emits a quoted canonical
decimal string. Revision, domain, timeline state, renewal state, time source,
and advance state are closed enums with invalid zero values
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/identity.go:12-150`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/generation.go:11-78`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/enums.go:11-358`).

The zero value cannot masquerade as a valid token, parsers reject alternate
spellings, and failed JSON decodes preserve their receiver.

### Exact timeline invariants

`Fact.Validate` owns the signed timeline rules:

- `notBefore < termEnd < graceEnd`;
- `notBefore <= renewalOpen < termEnd`;
- `notBefore <= paidThrough <= termEnd`;
- `issuedAt < graceEnd`; and
- `trustedTimeFloor <= issuedAt`.

Every constituent value validates before the comparisons
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/fact.go:71-130`).

The archive intentionally does not infer term or grace from plan text,
calendar cadence, price, or product policy. The issuer signs exact instants,
including a legally valid one-nanosecond grace interval
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:125-151`).

### Proof-carrying signature verification

`Document` is explicitly an untrusted structure containing one concrete
`Fact` and one `attest.Envelope[Domain]`. Structural validation does not claim
signature trust (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/document.go:12-35`).

`Verify` calls the real generic `attest.Verify` path with caller-selected
typed trusted keys and returns a private `Verified` value only after signature,
body extent, digest, domain, and trust closure
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/verified.go:8-64`).

The verified type retains the exact fact and attestation proof. It cannot be
constructed successfully with a struct literal and exposes only validated
value copies (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/verified.go:23-78`).

### Total boundary classification

Evaluation uses exact half-open intervals:

| Effective instant | Lease state |
| --- | --- |
| `< notBefore` | `not-yet-valid` |
| `>= notBefore && < termEnd` | `active` |
| `>= termEnd && < graceEnd` | `grace` |
| `>= graceEnd` | `expired` |

Renewal is independently `not-open` before `renewalOpen` and `open` at or
after it (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:186-234`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/evaluate.go:77-111`).

Hostile proof checks one nanosecond before, at, and after every start, renewal,
term, grace, and floor boundary, including the one-nanosecond grace interval
and `math.MaxInt64`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/lifecycle_hostile_test.go:13-89`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/lifecycle_hostile_test.go:465-518`).

The separation between timeline state and action permission is correct.
`active`, `grace`, and `expired` are facts; the later `gate` package decides
which concrete action remains allowed.

### Strong generation advance

`Advance` distinguishes four cases:

- lower generation: typed rollback;
- same generation and exact fact equality: unchanged, retaining the current
  proof;
- same generation with different fact: typed conflict; and
- higher generation: accept only after immutable and monotonic checks.

Identity, subject, revision, and not-before are immutable. Issued-at,
trusted-time floor, paid-through, term-end, and grace-end cannot regress.
Renewal-open may move either way within the candidate's independently valid
term (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/advance.go:8-116`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:244-277`).

The hostile field-classification table enumerates every private `Fact` field
and compares the classification list to the actual struct declaration, so a
future field cannot silently avoid an advance decision
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/lifecycle_hostile_test.go:280-391`).

This is one of the archive's best reusable patterns.

### Strict bounded wire closure

Fact and Document decoders reject missing, unknown, duplicate, reordered,
non-canonical, trailing, type-wrong, and oversized JSON. Failed decoding
preserves the receiver. Document proof uses real Ed25519 tampering, wrong
trusted keys, wrong domains, wrong body lengths and digests, and mutated
signatures (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/fact.go:198-242`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/document.go:38-85`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/verification_hostile_test.go:13-141`).

Fact and Document semantic fuzzers require validation and byte-exact
re-encoding; Document acceptance additionally requires authentic signature
verification (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/fuzz_test.go:12-88`).

The bounds themselves need redesign, described below. The strict closure
mechanics are worth preserving.

### Stable error identity

Core owns:

- `ErrLeaseContract`;
- `ErrLeaseVerification`;
- `ErrLeaseRollback`; and
- `ErrLeaseConflict`.

Each preserves the Primitive contract identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:52-55`). Lease wraps those errors with local
context and preserves nested Attest and JSON causes for `errors.Is` and
`errors.As` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/errors.go:8-33`).

Stable error identities belong in Core if Lease is admitted. Rendered labels
are diagnostics, not identities.

### Mechanical quality at the inspected tree

Fresh read-only recon gates at archive HEAD:

- `go test ./lease ./controlstate ./register` passed;
- `go test -race -shuffle=on -count=2 ./lease` passed;
- `go vet ./lease ./controlstate ./register` passed;
- `staticcheck ./lease ./controlstate ./register` passed; and
- `gocyclo -over 10` over Lease production files reported nothing.

The full-package `gocyclo` output names only test/fuzz helpers. Production is
at or below ten.

These results establish a strong implementation baseline. They do not prove
the missing scope and durable-time contracts.

### Primitive dependency graph

The archive dependency direction is:

```text
core --------+
temporal ----+--> lease --> controlstate --> register tests only
attest ------+
```

`lease` directly imports `core`, `temporal`, and `attest`; it imports no other
Primitive package (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:20-38`; verified with `go list`).

### All Primitive dependents

There is exactly one direct production dependent: `controlstate`.

- Its issuance request requires `lease.Verified`, not a raw document
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:17-45`).
- It extracts the authenticated fact and envelope and closes them into its
  nested document (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/components.go:20-70`).
- It verifies the nested document with Lease-owned verification at aggregate
  ingress (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:97-104`).
- Its binding explicitly includes `lease.Subject`, and snapshot validation
  rejects a nested fact whose subject differs from that binding
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/values.go:136-186`,
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:139-147`).
- Aggregate advance delegates Lease progression back to `lease.Advance`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/advance.go:363-374`).

`register` imports Lease only in test fixtures used to construct a complete
Controlstate document. The fixture demonstrates the typed issuer sequence
`NewFact` -> `attest.Sign` -> `Verify`, but it is not a production Lease
dependent (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/controlstate_fixture_test.go:214-256`).

No other archived Primitive production package imports `lease`, and no
package imports `register`. Controlstate's use proves that the Lease proof can
compose without copying its rules. It does not constitute an independent
deployed consumer.

## Consumer evidence

### Kernel

Kernel has no commercial Lease or old License import. Neither its committed nor
dirty Primitive pin contains `lease`.

Its current uses of the word `lease` are resource-custody mechanics. The
clearest production example is outbox delivery:

- pending, failed, and expired sending claims become eligible;
- Firestore claims each candidate transactionally;
- a successful claim changes status to sending and sets the next retry to
  `now + core.OutboxClaimLease`; and
- the SQL implementation uses the same Core-owned lease interval
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/outbox.go:396-448`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/outbox.go:464-499`;
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/sql/outbox.go:105-126`;
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/outbox_domain_contracts.go:10`).

Its hostile SQL proof checks the exact boundary: the intent is unavailable one
nanosecond before expiry and reclaimable at expiry
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/sql/outbox_test.go:294-319`).

The Kernel gem is not a commercial schema. It is the ownership rule that one
state owner must atomically grant a time-bounded capability, persist its expiry,
and define exact reacquisition boundaries. For commercial Lease, the analogous
owner is the consumer store that atomically installs a verified generation and
its durable time floor. Pure `lease.Advance` cannot replace that transaction.

Kernel provides no evidence for plan, billing, price, usage, or grace policy in
the Primitive Lease package.

### Witness

Witness is a direct production consumer of the older
`foundation/v2026/license` protocol. Its Primitive pin predates the archived
replacement, so this is migration evidence rather than new-package cutover.

The old signed `SubscriptionLeaseBody` carries:

- Lease identity and generation;
- device fingerprint;
- issue, paid-through, check-in-after, check-in-by, expiry, and write-grace
  values;
- subscription plan, billing period, and prepaid years; and
- a product-specific signing schema
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/lease.go:210-266`).

Witness's local owner deliberately retains only the mutable facts the signed
authority cannot own:

- latest independently signed server-time commitment;
- last successful check-in;
- durable state clock high water;
- Lease identity high water; and
- Lease generation high water.

It explicitly carries no usage tally and no teaching flag
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:12-26`).

Persistence validation requires Lease identity and generation floors to be
present together and validates every retained value
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:63-90`). Accepted check-in state:

- advances the check-in stamp only;
- merges the accepted observation with concurrent clock progress;
- bounds that clock by the signed Lease grace ceiling; and
- ratchets the Lease identity/generation pair
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:126-187`).

Most importantly, `GateClockHighWater` selects the maximum of:

- durable state clock high water;
- independently signed server-observed time;
- durable device clock high water; and
- the current wall clock, subject to the typed skew allowance.

That result, not a raw wall observation, drives Lease status
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:190-207`;
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/gate.go:35-67`).

Trust resolution also requires a pinned authentic grant, exact device match,
signed time-commitment binding where present, monotonic Lease progression, and
writability at the hardened time (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/gate.go:35-105`).

The online path sends the exact current Lease identity/generation, fresh nonce,
device fingerprint, binary digest/version, platform, and account token. It
commits only the authenticated nonce-bound response under store ownership
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:239-377`).

Witness's reusable gems are:

- signed server time independently bound to device, Lease identity,
  generation, and request nonce;
- state and device clock floors merged before evaluation;
- durable Lease identity/generation high water;
- exact-device scope closure;
- optimistic state merged under the store lock rather than overwritten;
- offline-first renewal quieting; and
- a deliberately small consumer-owned state schema.

None of those mutable facts belongs in the signed Lease document itself.

### Bug

Bug also consumes the older live License protocol, not the archived Lease
package.

Its old signed `SeatLeaseBody` demonstrates why the extracted Lease must stay
narrow. The body includes a Bug writer key, developer key, device fingerprint,
Bug plan, billing period, prepaid years, and collection/write-grace policy
alongside the timeline (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/lease.go:25-89`).

Bug's state owner retains the same signed-time and Lease progression floors as
Witness plus product-specific usage and one teaching flag
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:12-24`). Usage is a typed field-per-command
structure, not a map or string-key protocol
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:26-77`).

The important state mechanics are:

- command counting opens a typed usage window;
- preparing a report closes a copy but leaves live state unchanged;
- authenticated acceptance subtracts the submitted snapshot from current
  usage, preserving counts accumulated concurrently;
- accepted check-in merges clock progress and ratchets Lease progression; and
- offline import advances only clock and Lease progression, without erasing
  usage or pretending a check-in occurred
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:152-298`).

Bug resolves trust only after:

- a durable clock anchor exists;
- the signed grant verifies against the pinned server key;
- signed time commitment bindings match;
- the device fingerprint and writer key match;
- Lease status is writable at the hardened time; and
- the writer key is absent from an independently signed revocation set
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/gate.go:35-143`).

Bug's reusable gems are:

- the same typed durable-time merge proved by Witness;
- separate Lease identity and generation progression;
- exact device and delegated-writer binding;
- independently authenticated writer revocation;
- subtractive concurrent usage acceptance; and
- distinct online-check-in and offline-import transitions.

Only the first two belong near generic Lease. Writer authority, revocation,
usage, teaching, and product rendering remain with Bug or their own narrow
protocol owners.

### Peachfuzz

Peachfuzz has no commercial Lease or old License import. Its Primitive pin
does not contain `lease`.

Its `lease` values are exclusive resource grants:

- the scheduler owner returns a private typed grant binding target context,
  scheduler entry, and slot, and validation proves those facts agree
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/schedstate.go:53-75`);
- the same owner grants from its free-slot set, marks the target running,
  validates the release payload against the grant, and checks global slot
  accounting before persistence
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/schedstate.go:535-579`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/schedstate.go:581-679`);
- the worktree manager treats in-memory lease state as authority while durable
  markers are crash evidence for startup salvage
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/worktree/worktree.go:117-152`); and
- release takes a closed typed evidence-custody disposition and is idempotent
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/worktree/worktree.go:461-508`).

The Peachfuzz gem is ownership clarity: acquisition, release, and crash custody
are one owner's typed state machine. It does not justify adding scheduler,
slot, filesystem, or evidence-custody concepts to commercial Lease.

### Temporal and Timeproof

Archived `temporal.Instant` is a set non-negative exact Unix-nanosecond value.
It validates, compares, performs checked arithmetic, and emits canonical quoted
decimal JSON (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/instant.go:12-129`). It carries no provenance
about whether an instant came from a wall clock, persisted floor, signed server
commitment, or RFC 3161 authority.

`temporal.Observation` retains Go's monotonic component only while two live
observations remain in memory. Projecting it to `Instant` discards that
monotonic component by design
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/observation.go:9-51`).

`timeproof.AuthoritativeTimestamp` is a verified RFC 3161 timestamp over one
typed digest. Its `Instant` projection is authoritative only together with the
retained timestamp proof (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/timestamp.go:10-74`). Timeproof
also keeps authoritative and unofficial results as distinct closed variants
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/result.go:10-121`).

Those contracts yield two important ownership conclusions:

1. Lease must continue to depend only on `temporal`, `attest`, and Core; it
   must not import networked `timeproof`.
2. A bare `temporal.Instant` at `Evaluate` cannot prove that the caller merged
   the wall clock, signed authority, and durable high-water state correctly.

The composition root must produce a typed rollback-hardened evaluation
observation. The type may be owned by Temporal if it is universally useful, or
by Lease if its invariants are Lease-specific. It must not be an informal
instruction to `pass the maximum time`.

## Strong mechanics and proof

### Cross-consumer conclusion

| Need | Kernel | Witness | Bug | Peachfuzz | 2026 owner |
| --- | --- | --- | --- | --- | --- |
| Narrow signed commercial timeline | No current commercial consumer | Strong | Strong | No | `lease` |
| Exact active/grace/expired boundaries | Resource-lease analogue only | Strong | Strong | Resource-grant analogue only | `lease` |
| Expected subject/device binding | No | Strong | Strong, plus writer | Target/run binding analogue | `lease` expected subject plus composition-root device projection |
| Generation anti-rollback | Atomic claim analogue | Strong | Strong | Owner-controlled grant/release | `lease.Advance` plus consumer store |
| Durable time high water | No commercial use | Strong | Strong | Unrelated scheduler time | consumer persistence plus a typed temporal/Lease boundary |
| Signed server-time commitment | No | Strong | Strong | No | separate narrow authority protocol |
| Offline operation | No current paid CLI proof | Strong | Strong | Not commercial | Lease/Gate composition |
| Usage accounting | No | Explicitly absent | Strong and product-specific | Run evidence is unrelated | consumer |
| Writer key and revocation | No | No | Strong and Bug-specific | No | Bug/narrow writer authority |
| Plan and billing cadence | Kernel has its own free plan only | Product-specific | Product-specific | None | issuer/catalog, not `lease` |

The common denominator is smaller than the old `license` package and larger
than the archived evaluator's current input contract:

- signed generic timeline;
- nominal Lease identity and subject;
- authentic verification;
- exact state/renewal projection;
- same-generation conflict and monotonic advance;
- expected-subject closure; and
- an explicit typed rollback-hardened observation crossing into evaluation.

## Defects and blockers

### 1. The signed floor does not prevent general wall-clock rollback

The specification states:

> This maximum operation prevents a rolled-back local clock from extending
> active or grace time.

(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:186-202`).

Production receives only `Verified` and a bare `temporal.Instant`. It computes
the effective instant as:

```text
max(ObservedAt, Fact.TrustedTimeFloor)
```

(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/evaluate.go:8-20`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/evaluate.go:35-75`).

The signed floor is explicitly the newest authoritative instant known when the
issuer produced that generation, and it cannot be later than `IssuedAt`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:120-134`).

Consider one valid fact:

```text
trusted floor = 100
not before    = 110
term end      = 200
grace end     = 300
```

An observation at 301 correctly returns expired. If the machine later rolls
back to 150, the archived evaluator selects 150 because it is still above the
signed floor. The same authentic fact becomes active again.

This is not theoretical. Witness and Bug both persist later state/device clock
high-water values and independently signed server time, then merge them before
Lease status. Their real state exists precisely because the issuance-time floor
alone cannot remember later progress
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:190-207`;
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:301-313`).

The 2026 contract must make the missing fact compiler-visible. Acceptable
directions include:

- a `RollbackHardenedObservation` whose constructor requires current
  observation plus the validated durable and signed floors; or
- explicit typed `ObservedAt` and `DurableFloor` fields in `EvaluateRequest`,
  with Lease owning the maximum and provenance projection.

A comment telling callers to precompute the maximum is not a contract. A raw
instant with a field name such as `TrustedNow` is also insufficient unless its
constructor and `Validate` prove how it was closed.

This is an admission blocker.

### 2. Expected-subject binding is optional caller convention

`Fact.Subject` is supposed to bind the Lease to one issuer-defined commercial
subject (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:69-81`).

But `VerifyRequest` contains only `Document` and `TrustedKeys`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/verified.go:8-20`). `Verify` authenticates whatever subject the
document carries and returns `Verified`; it does not require the caller's
expected subject (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/verified.go:29-46`).

`EvaluateRequest` likewise contains only that proof and an instant
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/evaluate.go:8-20`). `Assessment` exposes Lease identity and
generation, but not subject (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/evaluate.go:23-33`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/evaluate.go:153-175`).

Therefore an authentic Lease for subject B can be verified and evaluated in
subject A's execution path unless every caller remembers an external
comparison. The archive's sole Primitive dependent performs that comparison
itself in Controlstate
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:139-147`). Witness and Bug separately compare
the older signed Lease's device fingerprint before trust
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/gate.go:68-105`;
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/gate.go:72-132`).

Repeated manual comparisons across every direct consumer are the hidden
protocol this reconstruction is supposed to eliminate.

The clean contract should require an `ExpectedSubject` at verification or
evaluation ingress and return a stable typed scope-mismatch identity. If one
Lease may legitimately be projected into multiple narrower scopes, that
projection must itself be a typed validated owner, not an omitted comparison.

This is an admission blocker.

### 3. `Advance` is not durable anti-rollback ownership

`lease.Advance` is a strong pure comparator, but its request requires both a
current and candidate `Verified` value and returns only the selected proof
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/advance.go:8-26`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/advance.go:97-116`). The specification explicitly
delegates initial install and persistence elsewhere
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:244-248`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:298-309`).

No inspected adopter has migrated the archived type into a crash-safe store.
The archive's Register fixture constructs one Lease only. Controlstate
delegates pure comparison but does not supply independent Lease-store migration
evidence.

Real Witness and Bug stores own more than generation:

- Lease identity and generation high water;
- durable state/device time floors;
- signed server-time progression;
- optimistic merge with concurrent observations; and
- distinct online and offline commit effects.

The 2026 package should remain pure, but admission proof must include one real
composition-root transaction demonstrating:

- first install;
- identical replay;
- equal-generation fork;
- lower generation;
- higher generation with regressed fact;
- crash before rename/commit;
- crash after commit;
- concurrent state/device clock progress; and
- offline replacement without falsely recording check-in.

Do not pull persistence into Lease. Do not claim system rollback resistance
until this owner is proved.

### 4. JSON ceilings are guessed literals, not schema-derived contracts

Core currently declares:

```go
LeaseDocumentJSONMaximumBytes = 2048
LeaseEnumJSONMaximumBytes = 32
LeaseFactJSONMaximumBytes = 1024
LeaseGenerationJSONMaximumBytes = 22
```

(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/lease_constants.go:3-22`).

Identity size is properly derived from its exact byte width, but Fact and
Document bounds are unexplained round-number ceilings. Document size is not
derived from exact field-name overhead, the exact Fact maximum, and
`AttestEnvelopeJSONMaximumBytes`. Enum maximum is not derived from the longest
closed token. Generation width duplicates a value already represented by
Core's unsigned-decimal JSON contract.

Later archive packages demonstrate the expected compiler-driven pattern:
schema maxima are sums of exact field-name widths, JSON punctuation,
constituent value maxima, and nested document maxima, followed by compile-time
fit assertions (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/controlstate_constants.go:71-205`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/rate_constants.go:58-224`).

The 2026 Lease bounds must be derived the same way. One schema change should
either update the owning term in the equation or fail compilation. A generous
round number that tests merely reuse is not a single source of truth.

### 5. The canonical-write implementation does not match its contract

The specification says canonical encoding uses a bounded fixed-capacity buffer
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:298-302`).

`Fact.WriteCanonical` instead calls `Fact.MarshalJSON`, which calls
`encoding/json.Marshal` to allocate the complete body, and then performs one
writer call (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/fact.go:180-206`). `Document.MarshalJSON` similarly
materializes Fact and Attestation byte slices before appending them into a
third complete buffer (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/document.go:38-63`).

The values are small and bounded, so this is not an unbounded-memory defect.
It is still a specification/implementation mismatch and misses the current
streaming preference. The clean implementation should use the shared canonical
field writer with a compiler-derived bounded buffer, or explicitly narrow the
specification if one bounded materialization is intentionally required by
Attest.

Tests must prove short writes, writer failures, exact maximum extent, and no
write after validation failure without depending on raw encoded strings.

### 6. Grace policy has two contradictory sources of truth

The Lease specification explicitly allows any positive grace interval down to
one nanosecond and says payment, outage, and legal durations remain issuer
policy (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/SPEC.md:136-146`). Production and hostile proof implement
that contract, including a one-nanosecond grace interval
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/fact.go:100-122`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/lifecycle_hostile_test.go:465-518`).

The root completion ledger still marks this requirement complete:

> enforce a compiler-owned minimum seven-day grace floor

(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:2280-2291`).

Both cannot be authoritative. Current code and the later detailed specification
agree with the product-neutral boundary; the stale seven-day requirement must
be removed or formally superseded in the 2026 plan. Leaving both creates exactly
the informal contract drift the reconstruction forbids.

The hostile test itself states that the seven-day floor was product commercial
policy and is no longer enforced by Primitive
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/lifecycle_hostile_test.go:414`). This makes the completion
ledger, not the code/specification boundary, the stale source.

### 7. Lease-private diagnostics and schema tokens are over-centralized

Core correctly owns stable Lease error identities
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:52-55`). It also owns every Lease diagnostic
label, including internal type names such as `lease.AdvanceResult`,
`lease.Assessment`, and `lease.VerifyRequest`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_constants.go:83-100`).

Those labels are not cross-package semantic contracts. Moving them to Core
couples a universal package to Lease's private implementation vocabulary.
Likewise, a token or bound belongs in Core only when another package imports
that exact semantic contract, not merely because an architecture test wants one
central string.

For 2026:

- keep stable `ErrLease*` identities in Core;
- keep truly shared identity widths, JSON primitives, and cross-package
  nested-size relationships in Core;
- keep Lease-only enum tokens, local diagnostic labels, and private
  implementation names with Lease; and
- use typed package constants and compile assertions rather than copied
  literals in either location.

This is ownership cleanup, not a reason to abandon the capability.

### 8. `PaidThrough` lacks an independent operational consumer

`PaidThrough` is signed, validated against the term, exposed, and monotonic
under advance (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/fact.go:12-23`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/fact.go:100-122`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/fact.go:160-166`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/advance.go:69-86`). It does not participate in Lease state or renewal
classification (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:lease/evaluate.go:77-111`).

No Primitive production dependent reads it. The older live License protocol
uses `PaidUntil` to constrain product-specific collection/check-in scheduling,
but status itself is based on expiry and write-grace boundaries
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/lease.go:125-207`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/lease.go:497-510`).

This does not prove `PaidThrough` is wrong. It proves the field needs an explicit
2026 owner and use:

- if it is an auditable commercial fact consumed by catalog, display, or
  renewal logic, retain it with that source-cited use;
- if `RenewalOpensAt` and `TermEndsAt` fully own behavior, do not preserve
  `PaidThrough` merely because the archive signed it.

Every signed field increases wire, advance, issuer, and migration obligations.
Schema admission requires a real reader.

### 9. No archived replacement cutover was proved

No inspected consumer Primitive pin contains `lease`. Witness and Bug still
use the older product-heavy License protocol. Therefore the archive does not
prove:

- migration of an existing stored grant;
- mapping old device fingerprint to the new opaque subject;
- mapping `TokenExpiresAt` and write-grace duration to term/grace instants;
- coexistence or clean replacement of old plan/billing fields;
- durable new-generation install;
- rollback-hardened evaluation through the new API;
- key rotation; or
- offline behavior after migration.

The old consumers prove the capability and supply design gems. They do not
waive migration proof.

## Primitive 2026 ownership and DAG

### Lease owns

- nominal continuing Lease identity;
- nominal commercial subject;
- revision and monotonic generation;
- the minimum proven signed timeline fields;
- its Attest signing domain and canonical document;
- authentic verification;
- expected-subject binding at the chosen ingress boundary;
- exact timeline and renewal classification;
- same-generation replay/conflict and higher-generation monotonic advance;
- typed Lease contract, verification, rollback, conflict, and scope error
  identities; and
- a typed input or capability proving evaluation uses the supplied durable
  floor.

### Temporal owns

- exact instants and durations;
- comparison and checked arithmetic;
- live observations and their in-process monotonic component;
- persistence projections for generic time values; and
- a generic rollback-hardened observation type only if its invariants are
  reusable beyond Lease.

Temporal must not learn Lease state, commercial boundaries, or product policy.

### Attest owns

- signing domains;
- canonical-body framing;
- envelopes;
- trusted-key capabilities; and
- proof-carrying signature verification.

Lease must not call raw Ed25519 primitives or duplicate Attest framing.

### Timeproof and signed check-in time own separate authority

`timeproof` owns RFC 3161 evidence over a digest, not a background wall-clock
service (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/SPEC.md:6-18`). A consumer's signed check-in-time
commitment owns server time bound to the exact device, Lease identity,
generation, and nonce
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/checkin_time_commitment.go:15-135`).

Lease may consume a derived validated instant/floor through a typed composition
boundary. It must not import network acquisition, CMS, certificate, nonce, or
check-in protocols.

### Consumer composition roots and stores own

- wall-clock observation;
- durable state and device clock high water;
- verification and ratcheting of signed time commitments;
- atomic install of Lease document plus progression/time floors;
- check-in cadence and retry quieting;
- online versus offline acceptance effects;
- device and writer projections into the Lease subject;
- usage, teaching, product prose, and local action invocation; and
- crash recovery and migration.

### Catalog/payment authorities own

- plan names;
- price and currency;
- billing cadence;
- prepaid term policy;
- provider identities and payment state;
- collection grace; and
- the exact signed timeline they issue from those facts.

Lease receives the resulting typed instants. It does not infer them.

### Core owns only shared invariants

Core candidates are:

- stable `ErrLease*` identities;
- a shared identity byte width only if multiple admitted identity types use the
  same semantic contract;
- universal JSON punctuation and integer-width constants;
- genuinely shared field names used across package boundaries; and
- compile-time equations required by an outer package that embeds a Lease
  document.

Lease-local tokens, diagnostic labels, AST names, and guessed ceilings are not
universal merely because the archive placed them in Core.

### Minimum dependency graph

```text
core --------+
temporal ----+--> lease --> gate/composition --> consumer state owner
attest ------+

timeproof or signed check-in time --> consumer state owner
```

`lease` does not import Gate, Timeproof, persistence, transport, payment, or
consumer code. Gate may consume a Lease assessment if its own interview admits
that dependency. Consumer state closes authentic time, durable progression,
Lease, and Gate only at the real operation boundary.

### Proposed 2026 contract shape

The exact names remain design work, but the semantic shape should be:

```go
type VerifyRequest struct {
    Document        Document
    TrustedKeys     attest.TrustedKeys
    ExpectedSubject Subject
}

type EvaluationTime struct {
    // private representation
}

type EvaluationTimeRequest struct {
    ObservedAt          temporal.Instant
    DurableFloor        temporal.Instant
    SignedAuthorityFloor temporal.Instant
}

type EvaluateRequest struct {
    Lease Verified
    Time  EvaluationTime
}
```

The actual design should avoid requiring a fake signed floor where one is not
yet available. If optional authority is needed, use a closed union such as
`AuthorityFloorAbsent`/`AuthorityFloorPresent`; do not use a nullable pointer,
zero-as-absent, or boolean plus dormant fields.

`EvaluationTime.Validate` must prove:

- all present inputs validate;
- the effective instant is the maximum of the current observation and every
  admitted floor;
- the source is a closed typed enum or set, not guessed from equality;
- no dormant union payload is populated; and
- persistence-origin and signature-origin facts are not mislabeled.

Lease's `Assessment` should retain or expose the expected subject so a caller
cannot drop scope after verification. If `Verified` is already subject-closed,
the assessment may derive it from that proof.

For issuance, either retain the typed `NewFact` -> `attest.Sign` composition or
add a Lease-owned `Issue(IssueRequest)` if issuer evidence shows the two-step
sequence is repeatedly misassembled. Do not add a compatibility wrapper. A new
intent operation is justified only if it owns a real invariant rather than
shortening call sites.

## Decision rationale and conditions

### Required admission proof

Before the 2026 Lease package is called admitted:

1. start with a red behavioral proof showing an expired Lease cannot become
   active after a rollback that remains above its signed issuance-time floor;
2. introduce one typed rollback-hardened evaluation boundary and prove all
   input provenance/union states;
3. start with a red proof showing an authentic Lease for subject B cannot be
   accepted in subject A's path;
4. bind expected subject in the owning request and preserve a stable typed
   mismatch identity through `errors.Is`;
5. decide every signed field from consumer evidence, especially
   `PaidThrough`;
6. derive every JSON maximum from field names, punctuation, exact scalar
   maxima, and `AttestEnvelopeJSONMaximumBytes`, with compile-time fit
   assertions;
7. make canonical writing match the documented bounded/streaming contract;
8. preserve real Attest signature verification and wrong-key/domain/body/
   length/digest/signature hostile proof;
9. preserve one-nanosecond before/at/after proof for every admitted time
   boundary;
10. preserve exhaustive enum closure and receiver non-mutation;
11. preserve total Fact-field classification under Advance;
12. prove identical replay retains current proof, equal-generation divergence
   conflicts, lower generation rolls back, immutable scope drift conflicts,
   and every monotonic field rejects a one-nanosecond regression;
13. prove maximum generation and maximum instant behavior without overflow;
14. prove one consumer-store transaction for first install, advance, replay,
   conflict, rollback, concurrent clock progress, crash, and recovery;
15. prove online check-in and offline import have distinct typed effects;
16. prove signed time commitment rollback, equal-time fork, wrong device,
   wrong Lease identity, wrong generation, wrong nonce, and wrong key;
17. prove migration from one real Witness or Bug stored grant without a shim
   in production;
18. remove or formally supersede the stale seven-day grace requirement;
19. keep production `gocyclo <= 10`, coupling coefficient zero, no maps, no
   product imports, and no direct clock/network/filesystem/process access;
20. run the project-local testing protocol, canonical repository gate, native
   race/shuffle proof, static/security analysis, and supported cross-builds;
   and
21. migrate one real consumer before declaring the package permanent.

Tests must use typed contracts, `Validate`, `errors.Is`, `errors.As`, Core-owned
shared constants, and hostile behavioral proof. They must not reproduce
production algorithms as a second oracle or match rendered error text.

### Recon implications

The Lease capability has real demand and supports topology consideration, but
the archived package must not be copied unchanged.

Preserve:

- the narrow product-neutral signed timeline;
- nominal Lease identity and subject;
- private immutable `Fact`;
- untrusted `Document` versus proof-carrying `Verified`;
- real Attest verification;
- exact half-open state and renewal boundaries;
- typed conflict and rollback identities;
- total generation advance with immutable/monotonic field classification;
- strict canonical receiver-preserving decoding; and
- pure, bounded, no-I/O, no-world-model implementation discipline.

Correct before admission:

- the false claim that an issuance-time signed floor alone prevents later wall
  rollback;
- the bare-instant evaluation boundary;
- optional-by-convention expected-subject comparison;
- guessed JSON maxima;
- the canonical-writer/specification mismatch;
- Lease-private diagnostic vocabulary placed in Core;
- the seven-day versus one-nanosecond grace contradiction; and
- any signed field without an identified reader.

Add from real consumers:

- a typed merge of current observation, durable state floor, durable device
  floor, and independently signed authority floor;
- persistent Lease identity/generation high water;
- atomic merge under store ownership;
- distinct online and offline acceptance transitions; and
- exact device/subject projection at the composition boundary.

Do not add:

- Witness or Bug plan enums;
- billing cadence or prepaid-year policy;
- device fingerprint directly to generic Lease when a typed subject projection
  suffices;
- Bug writer keys or revocation sets;
- usage, teaching, product copy, routes, retry policy, or check-in transport;
- scheduler/worktree resource-lease concepts from Peachfuzz; or
- persistence and network machinery.

This yields the useful result the reconstruction is after: keep the archive's
well-designed signed timeline and advance lattice, import the durable-time and
scope lessons proven by live consumers, and delete the product-heavy old
License world model once an actual consumer has migrated cleanly.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
