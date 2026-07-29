# Shutdown package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole recon report for archived package `shutdown`. Primary archive,
internal-dependent, and consumer evidence is integrated.

## Evidence boundary

- Primitive archive: `d046f7b675fcb797398d7cdc87b5504f43978056`.
- Kernel HEAD: `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`;
  committed Primitive pin `0df2954a2d91`, dirty pin `e8b7172161a4`.
- Witness HEAD: `b9629af57b7058b68982be5d3b282be440b1e76e`;
  pin `773add8ba0fc`.
- Bug HEAD: `39ce96242240d7174d562c90bb255860946595dc`;
  pin `388e593231a2`.
- Peachfuzz HEAD: `2b2d080c455edaadf88502c1c253845605a4336a`;
  pin `3f74d8fc35b4`.

Kernel committed code targets an older shutdown API while its dirty Primitive
pin contains the newer package. Witness and Bug still own local shutdown
composition. Peachfuzz is an older direct adopter. Those are distinct migration
states, not one current API.

## Capability ownership

The archive contains two related primitives:

1. a bounded LIFO cleanup plan; and
2. an owned OS-signal controller that converts authentic signals into typed
   cancellation cause and supports explicit force/release behavior.

The package is intended to own shutdown mechanics, not a product's list of
services, exit code, process topology, or persistence policy.

## Archive evidence

The archived implementation provides two strong primitive baselines. Their
mechanics and hostile proof are assessed below.

## Consumer evidence

### Kernel

Kernel's HTTP shutdown path uses `sync.Once`, a bounded server shutdown
context, and joined cleanup errors. It also exposes important ordering:
admission/listeners stop before telemetry flush and process termination
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:server/server.go:199`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:server/server.go:306`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:server/server.go:319`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:server/server.go:323`).

Kernel's process-group behavior shows that child process ownership cannot be
reduced to closing Go services. Second-signal behavior must be an explicit
typed composition decision; the library must not silently reinterpret it.

### Witness

Witness uses distinct budgets for different cleanup domains and tracks owned
processes. Its force cleanup and exit-130 behavior are product-level policy
worth preserving above the primitive
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/signals.go:16`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/signals.go:36`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/signals.go:366`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/signals.go:396`).

Its local implementation also shows the failure risk of ad hoc shutdown:
callbacks and process cleanup can outlive the coordinator, leaving orphaned
work. Primitive should supply mechanics that make join ownership visible
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/signals.go:407`).

### Bug

Bug currently relies mostly on `signal.NotifyContext` and direct exit behavior.
This is small, but it lacks typed signal cause, structured cleanup results, and
proof that all resources were joined. It is a migration use case, not reusable
shutdown architecture (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/main.go:279`, `bug@39ce96242240d7174d562c90bb255860946595dc:cli/main.go:283`).

### Peachfuzz

Peachfuzz is the strongest adopter. Its controller call site already uses the
current second-signal force policy (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:cmd/peachfuzz/main.go:83`);
its LIFO plan result field access remains older. It composes daemon joins,
persistence, scheduler teardown, and LIFO cleanup
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/shutdown.go:28`). Its process escalation behavior
is useful evidence for graceful-then-force phases.

The current escalation goroutine is not fully joined. That is precisely the
kind of lifecycle leak the new contract must make impossible or observable.

## Strong mechanics and proof

### Bounded cleanup plan

`Plan` validates a bounded collection of steps and executes them in reverse
registration order. The plan has a typed per-step budget and total budget,
detaches cleanup from the already-cancelled parent, contains callback panics,
and returns structured step outcomes
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/plan.go:12`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/plan.go:29`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/plan.go:59`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/plan.go:127`).

The implementation uses early termination rules and deterministic error
precedence. It caps the number of registered steps at 64 rather than allowing
unbounded cleanup state.

### Signal ownership

The controller owns its signal subscription and goroutine, exposes idempotent
`Close`, and records authentic signals in a validated `SignalCause`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/signal.go:136`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/signal.go:294`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/signal.go:319`).

Signal cause is kept distinct from generic context terminal state. This is the
correct ownership split: `contextstate` can say that a context is cancelled;
`shutdown` can prove that its own controller observed a supported OS signal.

Force and release behavior use bounded typed policies rather than raw sleeps.
The caller decides whether a second signal should force termination; the
primitive must not hide an `os.Exit` policy.

### Context and time dependencies

The package consumes `contextstate` for ingress and `temporal` for budgets and
waits. Cleanup detachment uses `context.WithoutCancel` intentionally, then adds
its own bounded plan context
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/plan.go:233`).

That is the correct pattern only when the new bound is always applied and all
owned goroutines can be joined.

## Defects and blockers

1. **Lifecycle phases are prose, not types.** The plan is one unphased LIFO
   list. It cannot prove a sequence such as stop admission, drain work, persist,
   flush observability, release resources, then force. If Primitive claims an
   ordered lifecycle, the phases need a closed enum and validated transition
   model.

2. **One step budget is applied to every step.** `PlanPolicy.StepBudget` cannot
   express the consumer evidence that HTTP drain, persistence, telemetry, and
   process termination require distinct bounds
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/plan.go:29`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/plan.go:241`). Each step or phase needs an
   explicitly owned typed budget, constrained by the total.

3. **A cooperative callback can run forever.** Context cancellation cannot
   preempt arbitrary Go code. The report must state this truthfully. In-process
   callbacks require cooperative contracts and join proof; hard termination
   remains composition-root/process-supervisor policy.

   The concrete wedge is `Controller.Close`: it waits for `c.done`, while the
   controller runs the force action synchronously. A non-cooperative force
   action therefore blocks the mandatory join indefinitely
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/signal.go:303`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/signal.go:427`,
   `archive@d046f7b675fcb797398d7cdc87b5504f43978056:shutdown/SPEC.md:283`).

4. **Plan concurrency proof is incomplete.** Tests do not yet exhaust
   concurrent registration/execution/close, mutation after first execution,
   post-first-signal behavior, callback reentrancy, and controller/plan races.

5. **Native signal proof is incomplete.** Linux and Windows behavior is not
   proven to the same depth as the portable state machine. Current ingress
   validation rejects invalid signal sets before Windows projection; retain
   that fail-closed rule as a forward invariant.

6. **No leak-enforcing test harness.** Package tests need a `TestMain` or
   equivalent ratchet that proves all controller, timer, callback, and
   escalation goroutines terminate on every hostile path.

7. **Do not mistake the primitive for orchestration.** The specification
   correctly excludes HTTP, database, telemetry, exit, worker discovery, and
   orchestration policy. The implementation is a plan and signal primitive;
   Primitive 2026 must preserve those exclusions even if it adds typed phases.

8. **Terminal observation inherits a contextstate gap.** After `<-Done()`, an
   active observation is impossible and must fail typed. Current consumers
   duplicate terminal-event inspection in different ways.

## Primitive 2026 ownership and DAG

- `core` owns shared lifecycle phase values, limits, stable error identities,
  and any cross-package shutdown constants.
- `temporal` owns typed budgets, checked budget relationships, deadlines, and
  bounded waits.
- `contextstate` owns hostile-safe context ingress and terminal-state
  observation.
- `shutdown` owns authentic signal observation, controller/goroutine lifecycle,
  typed shutdown phases, bounded step execution, and structured reports.
- composition roots own which resources participate, phase assignment,
  graceful-versus-force policy, process exit, and exit codes.

The final design must retain lint-required lifecycle interfaces only where they
create compiler-enforced ownership. It must not add compatibility wrappers for
the older consumer APIs.

## Decision rationale and conditions

The archived signal controller and bounded LIFO plan are strong reusable
primitives. Typed lifecycle phases, per-step or per-phase budgets, complete
join/leak proof, native signal matrices, and explicit hard-termination
ownership are admission blockers for the broader 2026 capability.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
