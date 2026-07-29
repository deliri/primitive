# Contextstate package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole recon report for archived package `contextstate`. End-to-end
Codex evidence and the independent Claude Opus gap review are integrated.

## Evidence boundary

- Archive: `d046f7b675fcb797398d7cdc87b5504f43978056`.
- Kernel HEAD: `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`;
  committed pin `0df2954a2d91`, dirty pin `e8b7172161a4`.
- Witness HEAD: `b9629af57b7058b68982be5d3b282be440b1e76e`;
  pin `773add8ba0fc`.
- Bug HEAD: `39ce96242240d7174d562c90bb255860946595dc`;
  pin `388e593231a2`.
- Peachfuzz HEAD: `2b2d080c455edaadf88502c1c253845605a4336a`;
  pin `3f74d8fc35b4`.

Witness and Bug pins predate the package. Peachfuzz consumes retired
`contextcheck`. Kernel's dirty tree begins migrating a local context check to
`contextstate`; the package is byte-identical between the dirty pin and archive.

Lack of committed direct imports is migration state, not negative evidence.
Archived Primitive has approximately 85 `contextstate.Validate` references
across 13 capability packages; the exact raw count varies with inclusion of
architecture tests.

## Capability ownership

`contextstate` owns:

- nil-context ingress rejection;
- bounded hostile-safe observation of standard terminal identity;
- a closed typed context state;
- exact trusted cancellation/deadline projection; and
- stable shared context observation errors.

It does not create contexts, observe clocks, wait, cancel, retry, own product
timeout policy, serialize arbitrary causes, or decide application outcomes
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/SPEC.md:8`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/SPEC.md:20`).

## Archive evidence

### Strong archived mechanics

### Closed state

`State` is a closed `uint8` enum with invalid zero and declared `None`,
`Cancelled`, and `DeadlineExceeded`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/state.go:13`). It has total validation and strict
receiver-preserving canonical JSON.

Hostile tables cover invalid enum bounds and 22 canonical/malformed JSON shapes
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/state_test.go:12`). The semantic fuzzer accepts only the three
canonical byte encodings, proves exact round trip and typed rejection, and
proves receiver non-mutation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/contextstate_fuzz_test.go:11`).

### Hostile context observation

Ingress validation calls `ctx.Err()` exactly once, catches panics, rejects
nonstandard terminal state, and normalizes only bounded `errors.Is` matches
against `context.Canceled` or `context.DeadlineExceeded` to the exact Go
sentinels. There is no recognition table for third-party context types
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/observe.go:7`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/public.go:9`).

Tests cover nil, active, value, future, `WithoutCancel`, standard cancellation,
deadline, cancellation cause, malformed, panicking, typed-nil, and
contradictory contexts
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/validate_test.go:83`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/validate_test.go:240`).

### Deterministic precedence

Generic classification gives cancellation precedence over deadline
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/classify.go:9`). Joined errors in both orders,
nested contradictory trees, duplicates, wrappers, and hostile identity methods
are covered.

This is terminal identity precedence only. A consumer may classify a typed
causal fact differently; it must not obtain policy accidentally by reversing
`errors.Is` checks.

### Archived Core-owned error graph

`core.ObserveErrorIdentity` performs bounded, panic-contained traversal without
unbounded child materialization
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_contracts.go:45`). It distinguishes matched,
unmatched, and unobservable results and proves cycles, maximum depth/width,
one-past limits, hostile ordering, and non-comparable errors
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_contracts_hostile_test.go:103`).

Stable identities are Core-owned:

- `core.ErrContextStateContract`;
- `core.ErrContextObservation`; and
- `core.ErrNilContext`.

The landed 2026 Core does not include `ObserveErrorIdentity`.
`ErrorIdentity.Matches` traverses Core's own closed identity-parent table, not
a caller-supplied error graph. Bounded external-error traversal therefore
starts package-local in `contextstate`; it moves to Core only after a second
named Primitive package proves the same contract.

## Consumer evidence

### Peachfuzz causal timeout

Peachfuzz separately records:

- parent context state;
- owned wall-context state; and
- whether the child actually observed cancellation.

Only `{parent none, wall deadline, child cancelled}` is classified as a wall
timeout
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/context_contracts.go:5`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/context_contracts_test.go:45`).

This is the strongest consumer requirement. Contextstate supplies trusted
terminal states; the causal aggregate belongs Peachfuzz Core or a separately
proved generic contract.

### Witness process outcomes

Witness has a closed `TerminationKind` and command-result invariants
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/types.go:2718`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/command_result.go:114`). Its worker pool preserves the first
typed worker cause while still sealing canceled downstream work
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/pool.go:86`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/check_stage_cancellation.go:10`).

Process termination and pipeline precedence remain Witness-owned.

### Bug fail-safe discovery

Bug preserves `Unknown` when process/file evidence is unavailable and never
maps cancellation/deadline to unused
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/discover/process_file_use_unix.go:21`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/discover/process_test.go:137`).

Contextstate supplies trusted terminal identity; Discover owns evidence policy.

### Typed causes

Arbitrary `context.Cause` remains owner-specific. Shutdown demonstrates the
right pattern with private typed `SignalCause`, stable Core identity, and
validated provenance
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/signal.go:136`).

Contextstate must not serialize arbitrary causes or absorb signal, retry,
process, exit, or restart policy.

### Bounded detachment

Kernel, Bug, and other consumers use `context.WithoutCancel` for cleanup. Safe
uses immediately add a finite deadline:

- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:server/server.go:283`;
- `bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:687`; and
- `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/git.go:582`.

Detachment composition belongs shutdown/temporal/call-budget enforcement, not
Contextstate state classification.

## Strong mechanics and proof

### Architecture ratchets

The public API and import surface are exact and compiler/AST-ratcheted
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/architecture_test.go:15`). The package imports
only Core among Primitive packages; standard-library imports remain explicit.

Archived consumers include `exchange`, `filestore`, `hostresource`,
`objectstore`, `rate`, `register`, `shutdown`, `status`, `submission`,
`temporal`, `timeproof`, `upgrade`, and `callbudget`.

## Defects and blockers

### Terminal-event observation

Ingress `Validate` correctly allows an active context. After a caller has
received from `ctx.Done()`, active state is impossible: a closed Done with nil,
malformed, or panicking `Err` must fail as an observation error.

This rule is currently repeated:

- `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/context.go:19-31`;
- `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/client.go:748-775`.

`archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/signal.go:330-337` is a looser related case: it observes after
`Done()` and forwards validation into cancellation, but does not yet translate
active-after-Done into the typed observation failure used by Temporal and
Exchange.

`contextstate` should own a typed terminal-event observation or narrowly named
terminal validator. Ingress validation and post-Done observation remain
distinct contracts.

### Observable versus unobservable classification

Internal classification returns `(State, bool)`, but public `Classify` drops
the boolean and maps hostile/cyclic/over-budget observation to `StateNone`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/classify.go:9`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/public.go:18`).

Policy-bearing callers can mistake an unsafe inspection for a non-terminal
context. The 2026 surface returns `(State, error)`: matched and safely unmatched
graphs return a valid state and nil, while hostile, cyclic, or over-budget
graphs return `StateUnknown` with `core.ErrContextObservation`. Loose booleans
are not sufficient.

### Cycle contract mismatch

`archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/SPEC.md:99` excludes cycles, while the archived Core primitive
explicitly supports bounded cycle-safe traversal. That stronger behavior should
be the package-local 2026 contract, with direct Contextstate
cycle/over-budget proof so wiring cannot regress unnoticed.

The specification's stated rationale is also false: it claims cycle handling
would require retained graph state or a leaking timeout goroutine, while the
archive already proves bounded self- and two-node cycles with a visit counter
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_contracts.go:80`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_contracts_hostile_test.go:126`).

### Global no-clone ratchet

The current architecture check proves the retired directory is absent but does
not prevent:

- new `contextcheck` imports;
- copied `StateOfError` implementations;
- repeated `ctx == nil` plus `ctx.Err` package crossings; or
- compatibility aliases.

The new repository needs an import/AST ratchet for Primitive package
crossings. Consumer-local policy may still consume the trusted state.

## Primitive 2026 ownership and DAG

### Cross-consumer debt

- Kernel and Bug sometimes give deadline precedence merely because their local
  classifier checks it first.
- Witness has scattered local ingress helpers and may project raw
  `cause.Error()` into persisted diagnostics.
- Peachfuzz's old `contextcheck` relies directly on standard `errors.Is` and
  lacks panic/cycle containment and architecture ratchets.
- Source-grep tests for bounded `WithoutCancel` can be bypassed through local
  variables and do not prove semantics.

## Decision rationale and conditions

### Conclusion

Archived `contextstate` is a strong built-ahead primitive with real internal
and cross-consumer demand. It should remain narrowly focused.

The main upgrades are:

- typed post-Done terminal observation;
- `(State, error)` observable/unobservable classification;
- direct package-local bounded-cycle proof and aligned specification; and
- global enforcement against retired imports and copied classifiers.

Causal timeout, typed signal cause, process termination, retry, diagnostics,
and product exit policy remain in their owning packages.

### Primitive 2026 surface resolution

The public surface is deliberately smaller than the archive:

- `Validate(context.Context) error` fuses nil, typed-nil panic containment,
  active-state admission, and exact standard terminal identity at ingress.
  It is an admission gate over what `Err` can report: cancellation and deadline
  return their exact Go sentinels, and unsafe inspection returns
  `core.ErrContextObservation`. A typed nil whose `Err` is nil-safe is
  indistinguishable from an active implementation without reflection and is
  therefore admitted; this is a documented limit, not a detection claim. No
  exported nil-check helper exists.
- `Classify(error) (State, error)` owns cancellation precedence and returns
  `core.ErrContextObservation` rather than silently converting unsafe
  inspection to `StateNone`. Precedence is total only across the safely
  observable traversal prefix: cancellation before a hostile node is decisive,
  while the same cancellation after that node is not reached.
- `ObserveAfterDone(context.Context) (State, error)` owns the repeated
  post-`Done()` rule from Temporal and Exchange, with Shutdown as the divergent
  third site. The name carries the caller's event-order precondition without
  adding a witness token or wrapper context.
- `State` remains a closed enum with `Validate`, `IsValid`, and a
  diagnostic-only `String`. `OffWireEnum` declares that decision to the pinned
  doctrine analyzer; a separate external structural test proves that neither
  receiver implements any standard JSON, text, or binary marshaling interface.
  No named consumer persists this state, so the archive's JSON and token parser
  are not admitted; `String` has no inverse parser and is not a wire format.
- Bounded error traversal remains private to `contextstate`. Core promotion
  requires a second named package to prove the same primitive.

Package documentation must explicitly exclude Peachfuzz's causal wall-timeout
aggregate, `context.WithoutCancel` detachment composition, and process or
pipeline termination precedence. Step 5 must also add the global no-clone
structural ratchet described above; removing wire behavior does not remove that
architecture obligation.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
