# Gate package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole reconstruction report for archived package `gate`.
The archive is evidence, not authority. No archived source was copied.

The generic authorization capability is real. Bug currently gates licensed
mutation commands through the older Primitive License gate; Witness evaluates
the same older gate when reporting commercial standing; Kernel has independent
typed authorization and route-protection lattices; and Peachfuzz consumes a
request-bound, short-lived upload authorization capability.

The archived package contains a strong reusable center:

- one authentic, product-neutral, signed action-disposition policy;
- a closed typed action set with exhaustive projections;
- exactly one rule per action;
- private proof-carrying verification;
- a nonzero private permit or a typed actionable denial;
- explicit recovery behavior outside the ordinary validity interval;
- total generation advance; and
- strict bounded canonical JSON.

The archive must not be copied unchanged. Its statement that the signed
`TrustedTimeFloor` prevents clock rollback is stronger than the implementation.
`Evaluate` computes only `max(ObservedAt, policy.TrustedTimeFloor)`. After a
consumer has observed a later instant, rolling the wall clock back to any value
above that old signed floor can move an expired policy back inside its validity
window. Witness and Bug already prevent this exact attack by merging signed
server time, durable state high water, durable device high water, and the
current observation before evaluating standing.

The standalone `Verify` boundary also authenticates any well-signed policy
without binding it to the caller's expected subject. `controlstate` compensates
for that omission when Gate is nested inside its outer aggregate, but direct
Gate use remains spliceable. In addition, the public API is implemented as
seventeen one-line wrappers over duplicate private functions, `PolicyRequest`
has no owner `Validate` method, error construction accepts an arbitrary string
label, denial rendering contains raw string literals, and several JSON ceilings
are guessed powers of two rather than compiler-derived schema bounds.

No current consumer Primitive pin contains the archived package. The corrected
`v2026.0.0` topology therefore defers a standalone Gate package. The report
preserves narrow signed-authorization mechanics for reconsideration only after
one real consumer proves the authority boundary.
Product command classification, durable clock custody, check-in, persistence,
transport, warning copy, teaching state, route authentication, and storage
upload grants remain outside Gate.

## Evidence boundary


### Source revisions and pins

| Source | Exact revision or Primitive pin | Archived `gate` availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`); Gate tree `458b6952d439098783072b184b7446dc22e516db` | Present. Introduced by `3653d627ff38155edcd784b4c8e6f3adb36aed62` on 2026-07-25. | One unrelated untracked file, `core/api_http_boundary_hostile_test.go`; Gate and the inspected Core contracts are clean against HEAD. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` (`2026-07-23T04:01`, `2026-07-23T04:52-04`, `2026-07-23T04:00`) | Committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins `0df2954a2d911a5d7d775691d023d569affa2c20`, where Gate is absent; dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43`, where Gate is present but unused by Kernel production. | Broad pre-existing dirty migration. The committed and dirty pins are reported separately. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` (`2026-07-24T15:52`, `2026-07-24T15:58-04`, `2026-07-24T15:00`) | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9`. That revision does not contain the archived package; Witness consumes the older `foundation/v2026/license` gate. | Only the pre-existing untracked ledger was observed. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` (`2026-07-24T15:52`, `2026-07-24T15:54-04`, `2026-07-24T15:00`) | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3`. That revision does not contain the archived package; Bug consumes the older `foundation/v2026/license` gate. | Only the pre-existing untracked ledger was observed. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` (`2026-07-24T15:52`, `2026-07-24T15:50-04`, `2026-07-24T15:00`) | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75`. That revision does not contain `gate`. | Only the pre-existing modified ledger was observed. |

The four committed consumer pins predate the archived Gate introduction. Exact
tree lookup proves Gate is absent at those pins and present only at Kernel's
dirty `e8b7172161a4` pin. No Kernel production code imports it. Consequently,
none of the four consumer repositories is a cutover proof for this package.

The material archive history is:

- `3653d627ff38155edcd784b4c8e6f3adb36aed62`: add the signed action Gate
  primitive;
- `85a9429e3cec030878fab144a177d8c784ecaf16`: close canonical wire contracts;
- `d259789e87bcadb829c5ffac72c6c91ccc604098`: centralize constants and close
  capabilities; and
- `7cec8d33faefa21bc48659d456aa68ebb02bc33d`: add the hardened Controlstate
  aggregate that composes Gate.

At archive HEAD, the package has:

- 2,293 production Go lines;
- 1,074 test and fuzz Go lines;
- a 444-line specification;
- 53 Gate-specific Core constant lines;
- fifteen named tests; and
- one semantic fuzz target.

The archive ledger calls Gate reviewed and records green native, race/shuffle,
fuzz, vet, static, security, lint, complexity, and cross-build proof
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1226-1262`). A fresh focused check during this
interview also passed:

```text
go test ./gate
go test -race -shuffle=on -count=2 ./gate
```

No production function above `gocyclo` ten was found. These results establish
that the archived implementation is internally coherent on its declared
contract. They do not repair a missing contract or prove a consumer migration.

## Capability ownership

Gate is a pure signed authorization-policy primitive. Its intended operation is:

```text
verified signed policy
        +
typed action
        +
typed effective time input
        |
        v
private nonzero Permit
        or
typed DenialError with total resolution
```

The package intends to own:

- one continuing opaque policy identity;
- one opaque subject identity;
- one protocol revision;
- one positive monotonic generation;
- issue, validity-end, and trusted-floor instants;
- one closed authoritative posture;
- one complete action-to-rule table;
- one bounded recovery-action set;
- temporal fallback resolutions;
- the Gate attestation domain and canonical document;
- authentic verification;
- action evaluation; and
- replay, rollback, conflict, and advance classification.

It deliberately does not own:

- account, offering, payment, billing, plan, or product state;
- a route or HTTP authentication mechanism;
- a clock read;
- durable clock custody;
- filesystem or database persistence;
- check-in, retry, or transport;
- a callback or service;
- a human-readable business reason;
- product copy; or
- the authoritative service that ultimately enforces a network operation
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/SPEC.md:6-39`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/SPEC.md:364-380`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/SPEC.md:433-444`).

The intended public surface contains constructors for nominal scalar and union
values plus four major intent operations:

```go
NewPolicy(PolicyRequest) (Policy, error)
Verify(VerifyRequest) (Verified, error)
Advance(AdvanceRequest) (AdvanceResult, error)
Evaluate(EvaluateRequest) (Permit, error)
```

The specification explicitly rejects booleans, callbacks, global defaults,
compatibility surfaces, product APIs, and a Gate service
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/SPEC.md:41-74`).

## Archive evidence

### Archived strengths worth preserving

### 1. Narrow product-neutral dependency boundary

Gate imports only standard-library packages plus `core`, `attest`, and
`temporal`. It does not import Lease, Controlstate, Callbudget, Status, Update,
Register, a consumer, or any I/O package.

The architecture test scans production imports and rejects undeclared
Primitive siblings, product names, loose maps, reflection, direct clock reads,
and network/process access
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/architecture_test.go:15-47`).

That direction is correct. Gate should decide an already-authenticated generic
policy. It must not absorb:

- Lease status calculation;
- consumer clock persistence;
- product command dispatch;
- Kernel route authentication;
- Peachfuzz upload transport; or
- OGS account and payment databases.

### 2. Closed typed action vocabulary

`Action` is a private-width `uint8` enum with an invalid zero and exactly
seventeen current values:

1. local execution;
2. Lease renewal;
3. Controlstate refresh;
4. Status retrieval;
5. registration;
6. device recovery;
7. rating submission;
8. Receipt refresh;
9. Submission authorization;
10. Submission completion;
11. diagnostic reporting;
12. Update refresh;
13. Upgrade download;
14. account access;
15. remediation;
16. appeal; and
17. generic authenticated control-plane contact
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/enums.go:122-143`).

Each action has:

- a compiler-owned wire token;
- a closed `ActionKind` projection of local or network; and
- an exhaustive `AllowedOutsideValidity` projection
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/enums.go:146-241`;
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/gate_constants.go:4-20`).

Only Lease renewal, Controlstate refresh, Status retrieval, registration,
device recovery, account access, remediation, appeal, and control-plane contact
are eligible for a policy's recovery set. Eligibility alone never grants the
action; the corresponding ordinary rule must also permit it
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/enums.go:228-241`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/policy.go:156-173`).

The hostile test sweeps every `uint8`, checks exact validity, token, kind, and
outside-validity membership, and round-trips every admitted value
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/contract_hostile_test.go:14-85`).

The mechanics are strong:

- invalid zero cannot silently become a real action;
- projections are attached to the owning type;
- the action domain is bounded;
- the signed rule table is indexable without a map; and
- future actions force deliberate changes.

The exact seventeen-member catalog is not yet admitted; consumer evidence for
that catalog is evaluated below.

### 3. Complete immutable policy

`Policy` stores:

- nominal identity and subject;
- revision and generation;
- issue, validity end, and signed floor;
- posture;
- a private `[actionLimit-1]Rule`;
- a private fixed recovery array plus count; and
- not-yet and expired resolutions
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/policy.go:12-53`).

Construction copies caller slices into private fixed storage. Validation
requires:

- every constituent value to validate;
- `trustedTimeFloor <= issuedAt < validUntil`;
- exactly one correctly ordered Rule for every valid action;
- no populated trailing recovery storage;
- a non-empty, bounded, sorted, distinct recovery set;
- only eligible network actions in recovery;
- every recovery action's ordinary rule to permit;
- local execution denial under permanent-denial posture; and
- every denial and temporal fallback to carry a valid total resolution
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/policy.go:55-190`).

This is substantially better than a loose action map or a sparse allowlist. A
policy cannot omit a newly declared action and accidentally inherit an
implicit default. The fixed action table also makes evaluation bounded and
constant-size.

### 4. Strict rule and resolution unions

`Rule` is an immutable closed union:

- permit carries action and `OutcomePermit`, with no resolution; or
- deny carries action, `OutcomeDeny`, and exactly one valid resolution.

Dormant union storage must be zero
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/rule.go:10-67`).

`Resolution` is a second closed union:

- a non-empty bounded set of distinct network next actions; or
- one bounded opaque support reference.

Again, dormant storage must be zero
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/resolution.go:75-190`).

Policy validation then closes the semantic relationship:

- every next action named by a denial must itself be permitted by this policy;
- next actions in not-yet and expired fallback resolutions must also belong to
  the recovery set; and
- support references remain opaque tokens rather than messages, URLs, email
  addresses, account data, or business reasons
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/policy.go:175-190`;
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/SPEC.md:181-205`).

That prevents impossible "resolve denial by invoking another denied action"
loops without teaching Gate any product workflow.

### 5. Private proof-carrying permission

`Evaluate` does not return `bool`. A successful result is a private `Permit`
whose valid state cannot be constructed outside the package. It carries:

- policy identity;
- generation;
- action;
- ordinary or recovery scope;
- effective instant; and
- effective-time source
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/evaluate.go:24-61`).

A denied action returns:

- zero `Permit`;
- non-nil `*DenialError`;
- stable `core.ErrGateDenied` identity through `Unwrap`; and
- validated accessors for policy identity, generation, action, posture, reason,
  effective instant, time source, and resolution
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/evaluate.go:63-147`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/evaluate.go:252-273`).

The result convention is valuable:

- permission is represented by an unforgeable capability, not a flag;
- denial is a typed error with stable `errors.Is` identity;
- callers can use `errors.As` for structured remediation;
- support references do not leak into rendered error text; and
- zero-result-on-error is consistent across constructors and evaluation.

This should remain the center of the 2026 Gate design.

### 6. Exact ordinary and recovery boundary lattice

Inside the signed window, Gate follows the action's rule. Before issue and at
or after validity end:

- a policy-listed recovery action receives a recovery Permit;
- a non-recovery action receives a typed temporal denial.

At exact issue, all permitted actions are ordinary. At exact validity end,
only recovery behavior applies
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/evaluate.go:149-218`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/SPEC.md:225-282`).

The focused hostile table proves:

- far before and one nanosecond before the signed floor;
- equality with the floor;
- one nanosecond before issue;
- exact issue;
- inside validity;
- one nanosecond before expiry;
- exact expiry;
- one nanosecond after expiry; and
- maximum `int64` observations
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/evaluate_hostile_test.go:10-96`).

It also proves authenticated control-plane contact can be explicitly admitted
to recovery and that a floor equal to issue selects ordinary evaluation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/evaluate_hostile_test.go:98-138`).

The boundary arithmetic is clear and worth preserving. The provenance and
durability of the effective-time input are not yet sufficient, as detailed
below.

### 7. Authentic verification

`Document` is the concrete canonical Policy plus an `attest.Envelope[Domain]`.
Structural validation binds the declared domain to `DomainPolicy`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/document.go:12-31`).

`Verify`:

- validates the request;
- invokes real `attest.Verify`;
- retains the proof capability;
- retains the exact Policy; and
- refuses to expose a valid `Verified` without both proof and Policy validation
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/verified.go:8-80`).

The hostile verification triad covers:

- valid signature;
- mutated signed Policy;
- neutral zero request;
- mutated signature; and
- signer absent from the trust set
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/advance_verification_hostile_test.go:114-191`).

This is real cryptographic verification, not a verifier interface or a test
fake pretending to establish authority.

### 8. Total generation advance

`Advance` compares two proof-carrying policies:

- lower generation returns `core.ErrGateRollback`;
- identical same generation returns unchanged and retains the current proof;
- divergent same generation returns `core.ErrGateConflict`;
- higher generation must preserve identity, subject, and revision; and
- issue, trusted floor, and validity end cannot regress
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/advance.go:8-84`).

Rules, recovery, posture, and resolutions may change only with a higher
generation, because the authenticated policy issuer owns those dispositions.

The hostile table covers lower, equal, higher, maximum generation, each
immutable identity field, every monotonic time field at a one-nanosecond
regression, and representative mutable policy changes
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/advance_verification_hostile_test.go:13-112`).

The separation between pure candidate classification and durable installation
is correct. `Advance` should not absorb persistence or claim to make the
selected generation durable.

### 9. Canonical receiver-preserving JSON

Policies, Rules, Resolutions, actions, scalar values, and Documents use:

- strict structure decoding;
- canonical token forms rather than ordinals;
- bounded input;
- semantic validation;
- byte-exact re-encoding; and
- assignment only after complete success.

Rejected input therefore leaves the receiver unchanged
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/policy.go:247-294`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/rule.go:119-157`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/resolution.go:206-296`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/document.go:34-81`).

The Policy hostile table attacks nil, null, empty, wrong top-level types,
leading and trailing whitespace, duplicate fields, unknown fields, escaped
tokens, wrong field types, truncation, excess size, and deep unclosed nesting
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/contract_hostile_test.go:210-262`).

The semantic fuzzer accepts input only when:

- strict decode succeeds;
- `Validate` succeeds;
- canonical re-encoding is byte-identical; and
- the canonical value round-trips exactly
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/fuzz_test.go:11-48`).

This is a good baseline for hostile wire proof.

### Archived Primitive dependents

### Direct production dependent: Controlstate

`controlstate` is the only archived production package that imports Gate.
Its composition is material:

- `IssueRequest` requires a proof-carrying `gate.Verified`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:17-45`);
- the outer Snapshot stores the exact Gate Document, not a copied projection
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/components.go:12-81`);
- Snapshot scope validation compares `Policy.Subject()` with the outer
  `Binding.GateSubject()`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:139-147`);
- revision validation requires the Gate revision to match the outer protocol
  revision (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:179-198`);
- Snapshot expiry is bounded by Gate `ValidUntil`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:244-305`);
- non-active registration requires local execution denial, and terminal
  registration additionally requires an account/recovery/remediation/appeal/
  contact recovery path
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:356-385`);
- outer verification independently verifies the nested Gate signature with the
  Gate key set (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:97-145`); and
- outer advance delegates the Gate field to `gate.Advance`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/advance.go:363-393`).

Controlstate therefore supplies a useful anti-splice composition example. It
also demonstrates that Gate's standalone scope is incomplete: Controlstate has
to carry a separate `GateSubject` and compare it outside Gate.

The Controlstate reconstruction report independently rejects the archived
aggregate as mandatory world-building. Gate must not depend on Controlstate
merely to obtain correct subject binding or durable time.

### Transitive production dependent: Register

Register production imports Controlstate and verifies and persists a
Controlstate document during enrollment
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/client.go:457-538`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:633-700`).

Register imports Gate directly only in fixtures that construct a complete
Controlstate document
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/controlstate_fixture_test.go:266-326`).

That is transitive integration evidence, not a direct Gate consumer and not a
consumer-repository cutover.

### No other archived production dependent

No other archived production package imports Gate. Lease, Status, Update,
Submission, Upgrade, Exchange, Timeproof, and the consumers remain independent.
That is the correct DAG direction.

## Consumer evidence

### Kernel

Kernel does not import archived Gate and is not a commercial-policy consumer.
It has two separate authorization domains that must not be conflated with this
package.

First, `core/policy` owns a typed RBAC vocabulary:

- `Decision` has `Deny` as its zero value;
- `Action` is a closed typed enum;
- `Evaluate` is a pure deny-by-default matrix;
- invalid roles and actions cannot produce Allow
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/policy/policy.go:17-44`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/policy/policy.go:80-157`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/policy/policy.go:183-234`).

The role loader returns a typed result and propagates store failure. Its hostile
test proves an outage does not become a false allow and the failure does not
poison a later recovered lookup
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:service/policy/policy.go:172-219`;
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:service/policy/failclosed_cache_hostile_test.go:49-80`).

Second, `core.RouteGate` classifies HTTP route protection:

- zero is explicitly `GateUnclassified` and invalid;
- the enum distinguishes public, authenticated, CSRF, step-up, admin, worker,
  internal, signed-contract, machine, and workload-identity boundaries;
- exact projections say which gates are private and which prove request
  authenticity
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/route_gate.go:9-101`).

`CheckUnsafeRouteProtected` fails assembly of an unsafe route unless the route:

- has an authenticity-proving gate;
- is non-browser-facing; or
- appears in the explicit method-and-path exemption set
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/route_ratchet.go:8-117`).

The hostile suite sweeps every `uint8`, pins invalid zero, pins exact projection
membership, and anchors dangerous lookalikes such as public, rate-limited,
worker, internal, and public-machine gates as non-authenticating
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/route_gate_hostile_test.go:19-105`).

Kernel's reusable Gate lessons are:

- fail closed at the zero value;
- make every protection projection exact and exhaustive;
- keep authentication proof separate from action disposition;
- make unsafe operation assembly fail when no protection is classified; and
- do not translate authority-load failure into a default permission.

Kernel's users, roles, routes, CSRF classes, and product action range remain
Kernel-owned. They do not belong in Primitive Gate.

### Witness

Witness consumes the older `foundation/v2026/license` Gate, not the archived
replacement.

The old `GateInput` contains:

- a typed Lease;
- current observation;
- durable clock high water;
- warning window;
- typed Lease trust; and
- a teaching-shown flag.

Validation explicitly returns `GateReasonClockRollback` when the durable high
water is after the current observation
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/gate.go:160-210`).

The old decision has four outcome classes:

- allow;
- warn;
- teach; and
- refuse.

Lease valid permits silently, warning and grace remain writable with warnings,
first untrusted use can teach, and invalid clock/trust/Lease or expiry refuses
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/gate.go:13-79`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/gate.go:213-306`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/gate.go:308-405`).

Witness's local trust owner strengthens that generic input. `ResolveLease`
requires:

- a durable clock anchor;
- authentic signature under the pinned server key;
- exact device binding;
- signed time-commitment binding where present;
- non-regressing Lease progression; and
- writable status at the hardened time
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/gate.go:35-105`).

`State.GateClockHighWater` selects the maximum effective authority from:

- durable state clock high water;
- signed server-observed time;
- durable device clock high water; and
- current wall observation, with the typed skew allowance
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:190-207`).

The hostile time-commitment proof demonstrates a wall clock rolled back by
thirty days still evaluates at the later signed server floor, while an honest
later wall observation wins
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/time_commitment_test.go:284-304`).

Witness currently uses the old Gate when reporting License standing. It
persists the observation after evaluation
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:468-503`). It does not use commercial standing
to block Witness's core verification capability. A separate Primitive contract
requires Witness verification to need neither active License nor network
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/_foundation_source/core/access_contracts.go:76-99`).

Witness's reusable gems are:

- durable and signed time must be merged before Gate evaluation;
- the signed Lease or policy is not itself durable-time authority;
- exact subject/device and progression trust precede disposition;
- product text is rendered after the generic decision;
- verification remains permanently ungated; and
- Gate observation is durably ratcheted after use.

The old warning, teaching, License state, operator copy, device, check-in, and
persistence types remain outside generic Gate.

### Bug

Bug is the strongest current operation-gating consumer. It also uses the older
Primitive License gate.

Bug's `LicenseCommand` is a closed typed enum for every countable command.
`LicenseAccess` is a closed `free` or `licensed` classification. One indexed
table is the source of truth:

- reads and inspection commands stay free;
- repository, hook, and ledger mutations require a License
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/core/license_contracts.go:39-123`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/core/license_contracts.go:125-204`).

Startup validation sweeps every valid command and rejects missing or invalid
access-table entries
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/core/contract_validation.go:347-363`). The hostile policy table
pins every sold access decision and proves a missing row fails the same typed
License contract
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/core/license_contracts_test.go:216-284`).

At dispatch, Bug:

1. resolves the typed command access;
2. returns immediately for free commands;
3. resolves an authentic device-bound Lease;
4. passes current observation and durable high water into Primitive's old
   Gate;
5. persists the Gate clock observation;
6. permits allow;
7. permits with product warning on warn; and
8. refuses teach, refuse, and unknown outcomes
   (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:642-752`).

Trust resolution also requires:

- a durable clock anchor;
- authentic signed grant;
- signed time-commitment binding;
- exact device fingerprint and writer binding;
- writability at the hardened time; and
- absence from an independently signed writer-revocation set
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/gate.go:35-143`).

Bug deliberately separates Primitive decision from product copy. Unknown
reasons fail closed to an activation hint
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/gate.go:12-32`).

Bug's reusable gems are:

- one compiler-owned product-command-to-access table;
- a boot-time completeness sweep for that table;
- read/inspection operations remain available;
- durable time is persisted at the actual operation gate;
- unknown outcomes fail closed;
- advisory warning does not become authorization;
- product rendering follows typed decision; and
- writer revocation is checked before commercial permission.

The command enum, access table, usage counts, teaching behavior, writer
certificate, revocation, exit code, and operator copy remain Bug-owned. Gate
needs the resulting generic action, not Bug's entire command catalog.

### Peachfuzz

Peachfuzz has no commercial Gate or old License import. Its relevant
authorization is the run-evidence upload ceremony.

The typed `RunEvidenceUploadRequest` contains exact signed evidence and schema.
The typed grant contains:

- the exact evidence descriptor;
- signed upload URL;
- bounded headers;
- expiry;
- provider;
- method; and
- schema
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/run_evidence_upload.go:250-305`).

`RunEvidenceUploadGrant.ValidateRequest` recomputes the request descriptor and
requires exact equality. `RunEvidenceUploadResponse` is a strict union:

- already present carries no grant; or
- upload required carries exactly one matching grant
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/run_evidence_upload.go:306-367`).

The publisher:

- acquires workload identity;
- sends the exact typed authorization request;
- validates the response against that request;
- rejects an expired grant;
- streams only the exact authorized artifact;
- reauthorizes after a precondition race; and
- never retries a terminal upload rejection
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/evidence_http.go:44-176`).

The real-socket hostile table proves already-present no-op, exact upload,
precondition reauthorization, terminal refusal, bounded retry exhaustion,
malformed authorization, expired capability, descriptor substitution, and
caller cancellation
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/evidence_http_test.go:66-87`).

Peachfuzz's reusable Gate lessons are:

- authorization capabilities must bind the exact requested action object;
- temporal validity must be checked at execution, not only decode;
- union states must make capability presence unambiguous;
- retry after a race must reauthorize, not reuse stale permission; and
- successful authorization of one descriptor cannot authorize another.

The upload URL, headers, storage provider, workload identity, evidence bytes,
retry policy, and transport remain Objectstore/Exchange/Peachfuzz concerns.
Generic Gate may authorize `submission-authorization`,
`submission-completion`, or `diagnostic-reporting`, but it must not replace the
request-bound upload grant.

## Strong mechanics and proof

### Cross-consumer conclusion

| Need | Kernel | Witness | Bug | Peachfuzz | 2026 owner |
| --- | --- | --- | --- | --- | --- |
| Closed action classification | Strong RBAC and route classes | Commercial standing only | Strong command access table | One evidence-upload operation | Gate action vocabulary plus consumer-owned mapping |
| Fail-closed zero/unknown | Strong | Strong | Strong | Strong union closure | Gate and every consumer mapping |
| Authentic signed generic policy | No current commercial use | Older signed License | Older signed License | Request-bound upload authority | Gate via Attest |
| Nonforgeable positive permission | Middleware/role result analogues | Older decision only | Older decision only | Exact upload grant | Gate Permit |
| Typed actionable denial | Typed failures | Typed reason plus copy | Typed reason plus copy | Typed contract/unavailable errors | Gate denial; consumer renders |
| Durable clock high water | No commercial use | Strong | Strong | Grant expiry checked against injected clock | consumer store plus typed Temporal/Gate input |
| Expected subject/action binding | User/route scoped | Device and Lease bound | Device/writer/Lease bound | Exact descriptor bound | Gate expected subject; consumer action projection |
| Warning/teaching | No | Product-specific | Product-specific | No | Lease assessment and consumer UI, not permission |
| Recovery outside validity | Route recovery is unrelated | Check-in/account flows | License admin/check-in flows | Reauthorization only | Gate signed recovery set plus consumer mapping |
| Durable install/advance | Store-owned analogues | Strong | Strong | Scheduler/archive owners | consumer store |

The common reusable capability is narrower than the old License gate and more
carefully scoped than the archive currently proves:

- authentic complete action policy;
- compiler-owned action and projection types;
- expected-subject closure;
- rollback-hardened typed evaluation time;
- private Permit or typed Denial;
- bounded recovery and resolution;
- total generation advance; and
- strict canonical wire contracts.

## Defects and blockers

### P0: signed issuance floor does not prevent later clock rollback

The specification states:

```text
effective = max(observedAt, trustedTimeFloor)
```

and claims this prevents local clock rollback from extending validity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/SPEC.md:225-248`).

Production implements exactly that maximum
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/evaluate.go:197-218`).

Consider:

1. signed floor = 100;
2. issue = 100;
3. validity end = 1,000;
4. consumer legitimately observes 1,100 and receives an expired denial;
5. local wall clock rolls back to 900; and
6. `max(900, 100)` is 900, so the same policy becomes ordinarily valid again.

Nothing in `Verified`, `EvaluateRequest`, or `Permit` retains the prior 1,100
observation. A signed floor fixed at issuance protects only against observations
below that old floor. It cannot protect progress made after issuance.

The hostile table tests observations below the signed floor, but it never
performs two sequential evaluations where the second observation regresses
from a later accepted instant while remaining above the signed floor
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/evaluate_hostile_test.go:10-96`).

Witness and Bug prove the missing requirement with durable state/device high
water and signed server time. The 2026 Gate boundary must consume a typed
rollback-hardened evaluation time or an explicit durable floor. A comment
instructing callers to pass the maximum is an informal protocol and is not
acceptable.

### P0: standalone verification does not bind expected subject

`VerifyRequest` contains only `TrustedKeys` and `Document`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/verified.go:8-20`).

Therefore an authentic Policy issued for subject B verifies successfully in
subject A's direct Gate path. `EvaluateRequest` also carries no expected subject
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/evaluate.go:10-22`).

Controlstate compensates by:

- storing an expected Gate subject in its Binding; and
- comparing the nested Policy subject during Snapshot validation
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/values.go:136-187`;
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:139-147`).

That proves the invariant matters; it does not make standalone Gate safe.
Expected subject must be part of the owning Gate boundary, preferably
`VerifyRequest`, so `Verified` itself is scope-closed.

A red proof must show a valid signature over subject B fails in subject A's
path with a stable typed mismatch identity.

### P0: no real consumer cutover or action-mapping proof exists

No committed consumer pin contains this package. Kernel's dirty Primitive pin
contains it without a production call site.

The archive proves its own action enum and its Controlstate fixture. It does not
prove:

- how every Bug command maps to one Gate action;
- that every new Bug mutation command is forced to choose an access class and
  action;
- that Witness verification remains outside every commercial denial;
- that a network operation cannot be relabeled as generic control-plane contact
  to bypass a specific denial;
- that Peachfuzz authorization and completion are separately enforced; or
- that a consumer atomically installs the selected policy and durable time
  floor.

This is important because the generic catalog contains overlapping choices such
as specific refresh actions and generic control-plane contact. The security
property lives partly in the consumer's exhaustive compiler-owned mapping.

The package cannot be declared permanent until one real consumer supplies that
mapping and executes the returned Permit at the protected operation boundary.

### P1: `PolicyRequest` does not own `Validate`

`PolicyRequest` is a public ingress structure with twelve semantic fields but
has no `Validate` method
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/policy.go:12-25`).

`newPolicy` performs three length checks and then relies on `Policy.Validate`
after copying into private storage
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/policy.go:55-78`).

The constructor rejects bad input, but the request owner does not expose its
contract at ingress or package crossing. Under the 2026 compiler-owned
discipline, `PolicyRequest.Validate` must own:

- constituent validation;
- exact rule count and order;
- recovery count/order/eligibility;
- timeline;
- posture/local rule relationship; and
- resolution relationships.

`NewPolicy` should call that method and then close private storage. Tests should
attack the request owner directly and the constructor boundary separately.

### P1: the public API is implemented as forbidden wrapper duplication

Every exported function in `public.go` delegates one-to-one to a private
function with the same semantic signature:

- `NewIdentity` -> `newIdentity`;
- `ParseIdentity` -> `parseIdentity`;
- `NewSubject` -> `newSubject`;
- `NewGeneration` -> `newGeneration`;
- `NewSupportReference` -> `newSupportReference`;
- `NewNextActions` -> `newNextActions`;
- `PermitAction` -> `permitAction`;
- `DenyAction` -> `denyAction`;
- `NewPolicy` -> `newPolicy`;
- `Verify` -> `verify`;
- `Advance` -> `advance`; and
- `Evaluate` -> `evaluate`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/public.go:5-88`).

These are not compatibility shims, but they are still wrapper functions and
create two names for every operation. The 2026 package should implement the
real operation directly under its exported name and update internal call sites.
No alias or compatibility layer should survive.

### P1: error construction is not fully compiler-owned

Core owns stable Gate error identities
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:56-60`) and many diagnostic labels
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_constants.go:52-79`).

However:

- `contractError` accepts arbitrary `label string`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/errors.go:10-18`);
- `DenialError.Error` contains raw `"gate: invalid denial"` and
  `"gate: %s for %s"` literals
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/evaluate.go:90-95`); and
- most contract failures are formatted sentinel chains rather than typed
  boundary errors that can identify the owning contract through `errors.As`.

Stable `errors.Is` behavior is present, and the typed Denial is strong. The
remaining diagnostic protocol is still string-shaped and partially
duplicated. The rebuild should use:

- Core-owned stable shared identities;
- a typed Gate contract kind or package-owned typed boundary enum where
  structured classification matters;
- compiler-owned formats only where rendered text is necessary; and
- no free-form label parameter.

Package-private diagnostic names should not be moved into Core merely for
convenience. Core owns only identities and values genuinely shared across
packages.

### P1: JSON ceilings are guessed, not schema-derived

Core declares:

- Policy maximum = 16 KiB;
- Document maximum = 24 KiB;
- Rule maximum = 1 KiB; and
- enum maximum = 48 bytes
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/gate_constants.go:23-51`).

The package checks these limits, but no compile-time equation derives the
Policy maximum from:

- exact field-name lengths;
- punctuation;
- exact scalar maxima;
- exactly seventeen maximum Rules;
- eight recovery actions;
- maximum temporal resolutions; and
- the Attest envelope maximum.

No proof shows the maximum valid Policy or Document is accepted, or that a
future token/field growth fails the build rather than failing at runtime.

The 2026 ceilings must be schema-derived with compile-time fit assertions and
exact-maximum/one-over proof.

### P1: canonical writing materializes the complete policy

`Policy.WriteCanonical` first calls `MarshalJSON`, materializing the full Policy
byte slice, and only then performs one destination write
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/policy.go:229-255`).

The specification says signing streams the bounded canonical body through
Attest (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/SPEC.md:354-362`). The implementation is bounded and thus
asymptotically O(1), but it is not the documented streaming pipeline.

The rebuild should choose one truthful contract:

- direct bounded streaming canonical output with exact short-write behavior; or
- explicit bounded materialization with a derived maximum.

The current mismatch should not be copied.

### P1: the seventeen-action catalog is insufficiently consumer-derived

The catalog is coherent, but current consumers do not prove all seventeen
entries as one stable cross-product wire domain.

Examples:

- Bug proves generic licensed local mutation plus update/check-in flows, not
  every archived action;
- Witness proves commercial standing and permanent ungated verification;
- Peachfuzz proves request-bound evidence upload authorization, not a reusable
  blanket diagnostic permit; and
- Kernel's route and RBAC actions are separate authority domains.

Every signed action becomes a permanent schema and issuer obligation. The first
2026 revision should include only actions with named issuers, readers, consumer
mappings, and hostile integration proof. If the full set is retained, that
evidence must be written before code.

The answer is not a stringly extensible action name. Extension remains a clean
new protocol revision with compiler-visible call-site updates.

### P1: proof is broad internally but narrow externally

The archive has one semantic fuzzer, and it targets only Policy JSON
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/fuzz_test.go:11-48`).

Missing hostile proof includes:

- Document semantic fuzz with real signature preservation/rejection;
- Rule and Resolution dormant-payload fuzz;
- expected-subject mismatch;
- sequential durable-time rollback;
- maximum valid schema-sized Policy and Document;
- consumer action-mapping completeness;
- consumer persistence crash/recovery;
- execution-time proof that the Permit is checked at the actual operation;
- stale Permit reuse after a higher-generation denial is installed;
- policy install concurrent with operation dispatch; and
- one live consumer migration from the old License gate.

The package test suite should preserve its current exact enum, signature,
boundary, advance, and receiver-nonmutation proof while adding these behavioral
layers.

### P2: the architecture ratchet is partly a raw-string convention

`TestGateDependencyAndProductNeutralityRatchet` reads source bytes and rejects
substrings such as `"bug"`, `"witness"`, `"map["`, and `"time.now("`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:gate/architecture_test.go:15-47`).

The import allowlist is useful. The product-neutrality substring scan is
brittle:

- it can reject unrelated identifiers or comments;
- it can miss semantically equivalent coupling;
- it is not tied to a compiler-owned package identity; and
- it cannot replace behavior.

The 2026 ratchet should prefer:

- exact parsed import-path closure;
- typed dependency inventory;
- production struct and exported-intent budgets where justified;
- sentinel coupling proof; and
- behavioral tests that fail if policy begins to depend on product facts.

AST/source-shape proof may supplement those contracts, not substitute for them.

## Primitive 2026 ownership and DAG

### Potential future Gate ownership

- nominal Policy identity;
- nominal Policy subject;
- protocol revision and monotonic generation;
- the admitted closed generic action vocabulary;
- each action's local/network and recovery-eligibility projections;
- immutable Rule and Resolution unions;
- the complete signed Policy;
- Gate's Attest domain and canonical document;
- authentic verification;
- expected-subject closure;
- a typed rollback-hardened evaluation-time boundary;
- private Permit;
- typed Denial with total remediation;
- exact ordinary/recovery time boundaries;
- replay, conflict, rollback, and higher-generation advance; and
- stable Gate error identities shared through Core.

### Core owns

- stable `ErrGate*` identities;
- genuinely shared identity widths and JSON primitives;
- shared wire-field names used by multiple packages;
- shared function-name/path contracts needed by ratchets; and
- any cross-package protocol revision relationship.

Core must not become a dumping ground for Gate-private diagnostic labels or
every action token merely because code generation is convenient. If an action
token is a cross-package wire contract, Core ownership is justified; otherwise
the Gate type owner should own it.

### Temporal owns

- exact instants and durations;
- comparison and checked arithmetic;
- live observations and their in-process monotonic component;
- persistence projections for generic time values; and
- a generic hardened-time capability only if its provenance rules are reusable
  beyond Gate and Lease.

Temporal does not own Gate policy, action, recovery, or consumer persistence.

### Attest owns

- signing domains;
- canonical-body framing;
- envelopes;
- trusted-key capabilities;
- signing; and
- proof-carrying verification.

Gate does not call raw Ed25519 or duplicate Attest framing.

### Lease owns commercial standing, not action permission

Lease owns:

- authentic commercial timeline;
- active/warning/grace/expired assessment;
- renewal boundaries; and
- Lease generation advance.

Gate may consume a Lease assessment at a consumer composition root, but Gate
must not duplicate Lease expiry/grace policy or import Lease merely to decide
an action.

Warnings are not permission. A consumer may:

- receive a Gate Permit; and
- separately render a Lease warning.

The old combined `GateOutcome` should not be copied wholesale into generic
Gate.

### Consumer composition roots own

- wall-clock observation;
- durable state and device clock high water;
- signed server-time commitment verification;
- mapping product operations to admitted Gate actions;
- proving every product operation has a classification;
- atomic Policy and clock-floor installation;
- policy refresh;
- stale-Permit lifetime rules;
- invoking the protected operation only with a valid Permit;
- product copy;
- teaching state;
- read/free operation exclusions;
- offline versus online transitions; and
- migration from old License state.

### Authority services own

- account, payment, fraud, abuse, or product reasons;
- policy issuance;
- policy identity and generation custody;
- subject projection;
- signing-key policy;
- authoritative network enforcement;
- request-bound upload/download grants; and
- auditing of permits and denials.

Gate receives only the signed generic disposition. It never receives raw
payment-provider or business-reason state.

### Kernel authentication remains separate

Kernel owns:

- session, CSRF, Origin, webhook, signed-contract, and workload-identity route
  protection;
- user role lookup;
- route assembly; and
- authoritative service-side enforcement.

Gate disposition never proves caller authentication. A network operation
requires both:

```text
authenticated request authority
              +
valid Gate Permit for the action/subject
```

Neither proof substitutes for the other.

### Peachfuzz request-bound capabilities remain separate

A Gate Permit for diagnostic reporting or Submission authorization does not
contain:

- object descriptor;
- upload destination;
- headers;
- provider;
- expiry at transfer time; or
- content digest.

Objectstore/Peachfuzz must still use an exact request-bound grant. Gate can
authorize entering that ceremony; it cannot replace the grant.

### Minimum dependency graph

```text
core --------+
temporal ----+--> gate --> consumer composition root
attest ------+                |-- durable time owner
                              |-- lease assessment
                              `-- product action map and execution

Kernel authentication / authority enforcement --> network operation

Objectstore request-bound grant --> object transfer
```

Gate imports only Core, Temporal, and Attest. Consumers may import Gate.
Consumer packages must not import one another merely to share action tokens,
error identities, or persistence conventions.

### Proposed 2026 contract shape

Exact names remain design work. The semantic direction should be:

```go
type VerifyRequest struct {
    Document        Document
    TrustedKeys     attest.TrustedKeys
    ExpectedSubject Subject
}

func (r VerifyRequest) Validate() error

type EvaluationTime struct {
    // private validated maximum of all admitted time authorities
}

type EvaluationTimeRequest struct {
    ObservedAt   temporal.Instant
    DurableFloor temporal.Instant
    // optional signed authority is a closed union, never a nullable convention
}

func NewEvaluationTime(EvaluationTimeRequest) (EvaluationTime, error)
func (t EvaluationTime) Validate() error

type EvaluateRequest struct {
    Policy Verified
    Action Action
    Time   EvaluationTime
}

func (r EvaluateRequest) Validate() error
func Evaluate(EvaluateRequest) (Permit, error)
```

If Temporal owns a generic hardened-time type, Gate should consume that type
instead of defining `EvaluationTime`. The ownership decision must follow the
Temporal and Lease reports and preserve provenance. A bare `Instant` plus an
informal "caller already took the maximum" convention is not acceptable.

`PolicyRequest.Validate` must exist and own its complete cross-field rules:

```go
func (r PolicyRequest) Validate() error
func NewPolicy(PolicyRequest) (Policy, error)
```

The implementation should live directly in the exported operation. Do not
recreate the archive's exported wrapper over a same-purpose private function.

For consumer mapping, each product should own a closed request such as:

```go
type OperationGateClass struct {
    // private product operation -> access/action classification
}

func GateActionFor(Operation) (gate.Action, AccessClass, error)
```

The actual type belongs in the consumer's Core and must have a compile-visible
completeness ratchet. It must not be a loose map, string switch, or default
fallthrough.

Permit lifetime needs an explicit decision. A Permit currently captures Policy
identity, generation, action, and effective instant, but not a validity end.
For an immediate synchronous operation, a private Permit passed directly to the
execution owner may suffice. For any queued, persisted, or delayed operation,
Gate must issue a separate request-bound capability with explicit expiry or
require re-evaluation at execution. A generic Permit must never become an
informal reusable bearer token.

## Decision rationale and conditions

### Conditions for future reconsideration

Before a future topology change may admit Gate:

1. start with a red sequential test showing a Policy expired at a later
   observation cannot become ordinary-valid after rollback to an instant above
   its signed issuance floor;
2. introduce a typed hardened-time boundary and prove current observation,
   durable floor, device floor, and optional signed authority provenance;
3. prove the effective instant is the maximum of every admitted source;
4. prove invalid or dormant authority union states fail closed;
5. start with a red test showing an authentic Policy for subject B cannot verify
   in subject A's path;
6. bind expected subject at the owner boundary and preserve a stable typed scope
   identity through `errors.Is`/`errors.As`;
7. make `PolicyRequest.Validate` own all ingress and cross-field rules;
8. remove all exported-to-private one-line wrappers and update real call sites;
9. replace arbitrary string error labels and raw denial format strings with
   typed/compiler-owned contracts;
10. decide the first protocol action catalog from named issuer and consumer
    evidence;
11. prove all 256 values of every admitted enum and every action projection;
12. prove a consumer operation-to-action/access table is complete and
    fail-closed;
13. prove no denied specific network action can be executed through a generic
    action classification;
14. preserve private nonzero Permit and zero-Permit-on-error behavior;
15. preserve typed Denial accessors, `core.ErrGateDenied`, and no support-token
    leakage in rendered errors;
16. prove each Rule and Resolution union rejects dormant payload;
17. prove every denial next action is permitted and temporal fallback next
    actions are recovery-admitted;
18. preserve exact before/at/after issue and validity-end behavior;
19. preserve symmetric recovery behavior only for signed, eligible, permitted
    recovery actions;
20. preserve real Attest signature, wrong-key, wrong-domain, body mutation,
    length, digest, and signature hostile proof;
21. derive every JSON ceiling from exact schema constants with compile-time fit
    assertions;
22. prove exact maximum valid Policy/Document acceptance and one-over rejection;
23. make canonical writing match the documented streaming or bounded
    materialization contract;
24. preserve receiver non-mutation on every rejected decode;
25. fuzz Policy, Document, Rule, and Resolution semantic closure;
26. preserve identical replay, equal-generation fork, lower-generation
    rollback, scope drift, and every monotonic time regression proof;
27. prove a higher-generation Policy cannot be installed without atomically
    ratcheting consumer generation and time floors;
28. prove stale delayed work re-evaluates or carries an explicitly bounded
    request capability;
29. prove consumer persistence survives partial write, crash, restart, replay,
    rollback, and concurrent clock progress;
30. prove Witness verification remains ungated;
31. prove Bug free/read commands remain available while every mutation is
    classified;
32. prove Peachfuzz request-bound transfer authorization remains required after
    any generic Gate Permit;
33. migrate one real Witness or Bug path from the old License gate without a
    compatibility shim;
34. remove the retired old path and update the real call sites in the same clean
    migration;
35. keep production `gocyclo <= 10`, zero coupling, bounded memory, no maps,
    product imports, direct clock, filesystem, network, environment, process,
    callbacks, or global defaults;
36. run the project-local hostile testing protocol, canonical repository gate,
    race/shuffle repeats, static/security analysis, Witness lint, and supported
    native cross-builds; and
37. land Primitive, authority, and first-consumer changes atomically only after
    explicit user review.

Tests must use typed structures, closed enums, owner `Validate` methods,
`errors.Is`, `errors.As`, Core-owned shared constants, exact boundary values,
and hostile behavioral proof. They must not match rendered errors, duplicate
action tokens, reconstruct production algorithms as an oracle, or rely on
wrapper shape.

### Recon implications

The generic Gate mechanics are valuable evidence, but the corrected
`v2026.0.0` topology defers a standalone Gate package because no current
consumer justifies that authority boundary. The archived package is also
rejected as-is.

Preserve:

- narrow product-neutral ownership;
- Core/Temporal/Attest-only dependency direction;
- nominal Policy identity and subject;
- proof-carrying authentic verification;
- complete fixed action-to-rule Policy;
- invalid-zero closed enums;
- exact action kind and recovery projections;
- strict Rule and Resolution unions;
- private nonzero Permit;
- typed Denial with stable identity and total remediation;
- exact ordinary/recovery time boundaries;
- immutable canonical wire values;
- replay/conflict/rollback/advance lattice;
- receiver-preserving strict decode; and
- bounded, no-I/O, no-world-model implementation discipline.

Correct before reconsideration:

- the false rollback-prevention claim;
- the bare `ObservedAt` evaluation boundary;
- missing expected-subject binding;
- absent `PolicyRequest.Validate`;
- one-line public/private wrapper duplication;
- arbitrary string error labels;
- raw denial-rendering literals;
- guessed JSON ceilings;
- canonical-writer/specification mismatch;
- speculative action entries without named issuers and readers; and
- narrow fuzz and consumer-execution proof.

Add from real consumers:

- Witness/Bug durable state and device clock high water;
- independently signed server-time commitment input;
- consumer-owned atomic Policy/generation/time-floor installation;
- Bug's exhaustive product operation-to-access/action classification;
- fail-closed unknown outcome handling;
- separation of permission from warning/teaching/product copy;
- Witness's permanently ungated verification invariant;
- Kernel's invalid-zero and exact protection-projection lessons;
- Kernel's failure-is-not-permission rule; and
- Peachfuzz's exact request-bound, execution-time capability validation.

Do not add:

- account, payment, fraud, abuse, or product-reason models;
- Bug or Witness command enums;
- usage tallies;
- teaching flags;
- human copy;
- routes, roles, CSRF, sessions, or workload identity;
- check-in, retry, transport, or persistence;
- device fingerprint or writer certificate when a typed subject projection
  suffices;
- object URLs, headers, providers, or transfer grants;
- Lease timeline calculation;
- Controlstate as a mandatory aggregate dependency;
- stringly extensible actions;
- compatibility aliases, wrappers, or shims; or
- a Gate service/global default.

The result is the useful clean cut: keep the archive's typed signed
authorization kernel, import the durable-time, exhaustive-mapping, fail-closed,
and request-binding lessons already proven by real consumers, and delete the
old product-heavy License gate only after one consumer has migrated through the
new compiler-owned boundary.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
