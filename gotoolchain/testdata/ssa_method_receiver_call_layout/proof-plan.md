# Native method call layout and callable custody

The prior method query authenticated the receiver but could lend it beside an
extra physical argument, cloned source callable or foreign callable program.
Four native regressions disproved those assumptions; native direct/bound
baselines passed. MethodReceiver now reuses Primitive's existing native call
signature, complete argument validation and function-value custody checks.
No new public contract, compatibility path or alternate executor is introduced.

The existing direct, bound, interface, untaken, malformed receiver, cancellation,
call-kind, local triad and fixed-memory scopes cover the changed query. The new
12-mode semantic partition crosses direct/bound/interface source and one-fact
layout/value/receiver damage. Native signature panics retain both Core output
identity and runtime.TypeAssertionError. Current native faults remove argument
validation, callable custody or the native cause and must fail assertions.

This is a finite test partition, not a claim to exhaust Go SSA or fabricated
generic-instance canonical identity. Compiler construction is a separately owned
whole-unit operation. Serial benchmark/fuzz phases use three-second development
budgets and retain actual startup/runtime, counts, source and output identities.
Independent acceptance belongs to the user.
