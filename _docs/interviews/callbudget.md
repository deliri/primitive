# Callbudget package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole recon report for archived package `callbudget`. Primary
archive and consumer evidence and the independent Claude Opus gap review are
integrated.

## Evidence boundary

- Primitive archive: `d046f7b675fcb797398d7cdc87b5504f43978056`.
- Kernel HEAD: `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`;
  committed Primitive pin `0df2954a2d91`, dirty pin `e8b7172161a4`.
- Witness HEAD: `b9629af57b7058b68982be5d3b282be440b1e76e`;
  Primitive pin `773add8ba0fc`.
- Bug HEAD: `39ce96242240d7174d562c90bb255860946595dc`;
  Primitive pin `388e593231a2`.
- Peachfuzz HEAD: `2b2d080c455edaadf88502c1c253845605a4336a`;
  Primitive pin `3f74d8fc35b4`.

All four committed consumer pins predate this package. Kernel's dirty
Primitive pin contains a byte-identical copy. Direct-import counts therefore
cannot decide whether the capability belongs in the new Primitive.

## Capability ownership

`callbudget` is not a generic rate limiter and does not own HTTP retries or
elapsed operation deadlines. It owns one shared admission policy for a covered
lifecycle call:

- a four-call foreground burst;
- one foreground refill every 30 seconds;
- at most ten accepted calls in a rolling 30-minute window;
- foreground capacity protected from background work;
- a maximum of four simultaneous local reservations;
- bounded reservation and signed-fact lifetimes; and
- exact typed eligibility facts.

The composition root owns the decision that an operation is covered and whether
it is foreground or background. The server remains authoritative. Local
enforcement preserves workflow capacity and avoids futile calls; it is not a
security boundary (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/SPEC.md:8`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/SPEC.md:20`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/SPEC.md:170`).

The intended composition is:

1. validate a typed scope and operation;
2. reserve locally;
3. execute the operation through `exchange`;
4. authenticate the authoritative outcome;
5. reconcile or release the reservation.

`callbudget` must not open a network connection or infer coverage from a route,
command, product, or free-form string
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/SPEC.md:24`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/SPEC.md:35`).

## Archive evidence

### Archived architecture worth preserving

### Compiler-owned policy

`core/callbudget_constants.go` owns the numerical policy units and capacities.
`StandardPolicy` constructs typed `temporal.Duration` values and `Policy.Validate`
proves their arithmetic relationships
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/policy.go:8`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/store_types.go:43`).

There is no configurable token-bucket soup. A changed policy is intended to be
a new closed revision with an explicit migration decision.

### Typed scope and operation binding

Scope is a validated combination of Core-owned account, offering, and device
identities. `CallClass` is a closed enum. Consumer operation domains implement a
self-referential typed contract, and operation facts stream canonical,
domain-separated, length-delimited bytes into a hard-limited SHA-256 writer
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/SPEC.md:99`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/values.go:216`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/values.go:245`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/values.go:281`).

This is materially better than route-string keys or loose limiter maps:
credentials, retry counters, incidental encoding, and transport headers cannot
silently become operation identity.

### Exact bounded state

The authoritative rolling window is a fixed collection of the ten most recent
accepted calls rather than an unbounded history or a lossy fixed-window
summary. Local reservations are also fixed and bounded. Validation rejects
invalid counts, order, identities, durations, and cross-field contradictions.

Server admission, issue/verification, pure client evaluation, fixed accepted
history, and the local store are separated:

- authoritative admission:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/server.go:136`;
- signed document issue and verification:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/document.go:13`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/verified.go:26`;
- pure eligibility evaluation:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/evaluate.go:122`;
- fixed accepted-call state:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/calls.go:83`;
- local reservation store:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/store.go:97`;
- canonical operation digest:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/values.go:216`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/values.go:245`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/values.go:281`.

`controlstate` embeds, verifies, and advances the resulting budget document
instead of reproducing its rules
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/components.go:15`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:111`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/advance.go:380`).

## Consumer evidence

Every consumer ledger describes lifecycle calls that need one shared
composition policy even though their current Primitive pins predate the
package. That is evidence for the capability, not for copying the archive
unchanged.

### Kernel

Kernel's dirty Primitive tree contains the package and is the closest pending
adopter. Kernel also exposes the key distinction the final design must retain:
the call-admission budget is not the HTTP attempt timeout and is not the total
elapsed workflow budget. Those three facts need separate typed owners.

### Witness

Witness's multi-step remote workflows show why foreground reserve must survive
background maintenance. Its higher-level workflow policy belongs above
`callbudget`; the primitive must remain neutral about testimony, TSA selection,
or custody operations.

### Bug

Bug's update and diagnostic workflows need deterministic local admission and
authoritative reconciliation, but its current pin provides no reusable package
implementation. It contributes use cases, not API authority.

### Peachfuzz

Peachfuzz has both interactive and background service work. Its current
Primitive revision still composes limits locally. This supports the closed
foreground/background classification while warning against inferring class
from routes or process names.

## Strong mechanics and proof

### Dependency direction

Production imports are limited to standard library plus the declared
Primitive capabilities. It does not import `exchange` or consumer packages
and has no network client, raw wall clock, timer loop, map-based history, or
background goroutine. Its architecture ratchet checks this direction
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/architecture_test.go:112`).

## Defects and blockers

The archive is not complete enough to promote unchanged.

1. `core.ErrCallBudgetExpired` is declared but unused. A stable error identity
   without a reachable compiler-owned transition is dead contract surface.

2. `laterConstraint` discards a `temporal.Compare` error
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/evaluate.go:437`). An impossible comparison must
   fail typed; it cannot silently choose a constraint.

3. `acceptedCallAfter` also discards a `temporal.Compare` error while ordering
   projected reservation state
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/store.go:378`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/store.go:390`). Invalid ordering can
   therefore feed admission instead of failing typed.

4. Nil receiver handling is inconsistent. `Store.Reconcile` and `Store.Release`
   dereference receiver field `s.scope` before the common `begin` boundary,
   allowing a panic instead of the package's typed contract error
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/store.go:474`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/store.go:562`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/store.go:761`).

5. Recovery detaches with `context.WithoutCancel` and has no independently
   enforced hard store-recovery budget
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/store.go:702`). Detachment is sometimes required
   after cancellation, but unbounded detached persistence is not acceptable.

6. The specification's proof matrix is substantially ahead of the tests
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/SPEC.md:793`). Missing proof includes:

   - an independent arithmetic/reference oracle;
   - semantic fuzzing of decision and local-state transitions;
   - permission, residue, interrupted-write, and process-crash matrices;
   - authenticated outcome and replay matrices;
   - complete workflow composition with `exchange`;
   - exemption and upgrade integration; and
   - proof that contention waits rather than busy-spinning.

7. Existing fuzzers primarily prove JSON closure. They do not yet prove the
   admission state machine under hostile transition sequences
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/fuzz_test.go:10`).

The ordinary archive gates were green, including tests, race, vet,
Staticcheck, gosec, and `gocyclo <= 10`. Those gates do not substitute for the
missing semantic and crash proofs.

The archive does contain a real multi-process contention proof:
`TestStoreConcurrentProcessesPreserveEveryReservation` launches four
subprocesses and proves every reservation and the generation count survive
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/process_hostile_test.go:15`). The missing process
proof is specifically crash/interruption/residue behavior, not ordinary
concurrent reservation.

## Primitive 2026 ownership and DAG

The corrected `v2026.0.0` topology defers a standalone Callbudget package
because no current direct consumer justifies it. If future evidence reopens
the boundary, ownership would be:

- `core` owns shared identities, constants, error identities, fixed capacities,
  and wire/path contracts;
- `temporal` owns typed instants, durations, arithmetic, ordering, and bounded
  wait mechanics;
- `contextstate` owns trusted context ingress and terminal observation;
- a future `callbudget` owner would own admission state, reservation state,
  signed budget facts, and deterministic evaluation;
- `filestore` owns the exclusive bounded persistence capability;
- `exchange` owns HTTP execution, attempt limits, retries, and total operation
  timeout;
- the composition root owns coverage, foreground/background class, and business
  outcome interpretation.

No package may copy the standard policy, error strings, reservation paths, or
operation-domain spellings.

## Decision rationale and conditions

### Conclusion

`callbudget` contains useful built-ahead mechanics, but current demand does not
justify a package in `v2026.0.0`. Its typed fixed-state design and operation
binding remain reuse evidence only. Any later reconsideration must first close
error reachability, comparison failure, nil receivers, bounded recovery,
state-machine fuzzing, crash proof, and authentic Exchange integration.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
