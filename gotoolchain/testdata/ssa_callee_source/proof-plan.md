# Native callee source loan

Assumptions: argument identity and callee identity are distinct; knowing a source
frame does not prove timing or invocation. Both are enforced in native fixtures.
Go owns calls, function representations, closure bindings and source identity.
The implementation lends those native objects through one shared value projector.
It retains no graph and sets no argument or representation-depth quotas.

CalleeSource admits ordinary, deferred and spawned calls without timing meaning.
A source-backed local function lends its original actual value and source target.
Opaque, merged, nil, imported, synthetic, builtin and interface-dispatched targets
lend nothing. Native malformed input and cancellation return typed Core errors,
preserve native causes and zero the observation. LastFunctionArgument now uses
the same observation and projector; there is no compatibility alias.

The query, observation and method receiver boundaries have local triads. Tables
challenge call membership independently of operand order, cloned representations,
foreign functions/captures, complete argument layout, lexical ownership and native
signature causes. A survived membership mutation exposed an inadequate fixture:
a closure's independent order check masked the missing membership check. The new
named-call fixture uses constant arguments to isolate the independent obligation.
Both attempts remain retained; the first is not claimed as causal success.

The 18-mode semantic partition crosses source representation, call timing and
single-fact damage. It is a finite test partition, not exhaustion of all Go SSA.
The callee has one loan rather than conflicting fact reconciliation: malformed
custody is refusal, never relabeled contradiction to fill a numeric quota.
32/8193-argument allocation checks and a 257-argument benchmark test fixed query
scratch after native compiler construction. Construction itself is a separate
whole-unit operation. Benchmarks and semantic fuzz are serial 3-second development
runs; exact startup, native outputs and all attempts remain retained.

Canonical identity of fabricated generic-instance clones and arbitrary-depth
composed runtime remain unverified. Independent acceptance belongs to the user.
