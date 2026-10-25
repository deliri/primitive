# Native selected-argument invocation loans

The consumer needs the selected native argument at each immediate lexical call,
its actual call site and its caller ancestry. Primitive owns those observations;
Witness owns benchmark setup policy. Existing ArgumentBindings owns exact
parameter/capture bindings but does not carry invocation ancestry.

Material assumptions and checks:

- Equal names, shapes or copied SSA values establish ownership: false. Root and
  binding refusals exercise foreign arguments, copied arguments/calls, detached
  units and malformed native blocks. Accept only the original native identities.
- Every lexical function is invoked: false. The source matrix distinguishes
  uncalled literals, immediate calls, repeated calls, goroutines and defers.
- Repeated calls may share a delivered invocation: false. The nested-call test
  requires distinct actual outer call sites and their corresponding ancestry.
- Callback setup inherits the parent's selected argument: false. The exact
  configured native method introduces a fresh selected callback parameter; an
  unrelated method with the same name does not.
- A plain native function with a compatible callback signature qualifies as a
  callback method: false. The receiver is required. A new focused test pairs
  an actual method callback with refusal of a native plain function using the
  same callback signature. Native focused proof retains the permitted method and
  refused plain-function cases, plus its compiling counterexample.
- A native call site proves that every runtime path executes or returns:
  unverified and not claimed. These are source observations. The API neither
  executes the program nor proves universal runtime control flow.
- A borrowed frame pointer removes Witness's repeated value-address escapes:
  verified by compiler escape diagnostics and the failed-only allocation rerun.
  This does not prove zero allocation or constant memory in active call depth.
- Width proof implies depth proof: false. First-loan tests independently vary
  native width and native nesting, excluding the compiler-owned graph from the
  measured operation. Retaining a child retains its borrowed caller chain.
- A green table proves that its assertions can detect a wrong implementation:
  verified for the original seven source controls: each compiled and failed its
  intended assertion. The final dispatch extraction has a separate focused
  behavior run; all seven updated controls also compiled and failed their intended
  assertions. Their receipts are retained separately. Compilation failure
  or iterator-protocol panic cannot substitute for assertion proof.
- Project callback/evidence routing is required for this native proof: false.
  Anvil permits omission for fleet-local SSD/HDD custody. These Mac-only runs
  use that existing path; optional project routing is not claimed.

Fundamental facts: Go owns SSA functions, calls, parameters, captures and lexical
parents. Primitive already authenticates individual calls and selected argument
bindings. A caller link plus the native call pointer preserves one invocation's
context without copying a call graph. A stopped consumer needs no suffix.

The implementation therefore lends one private root frame, then lends native
child bindings as calls are visited. Lexical descent follows native parents.
Exact configured callback methods select a fresh native parameter. Stop,
cancellation, native refusal and consumer panic preserve their original meaning.
There is no product environment, deduplication catalog or arbitrary flow quota.

Behavioral scope:

- Finite source matrix: ten call/capture forms, two selected operands and an
  owned/foreign root. The twenty foreign-root repetitions exercise one rejection
  class; forty matrix cases are not forty independent earned boundaries.
- Root triad: an intact native root before each hostile mutation, then exact nil
  refusal and Core identity. Cancellation preserves both the standard identity
  and original cause.
- Callback-method boundary: a real method lends the exact root and callback;
  a plain function with the same callback signature refuses at request validation,
  root admission and streaming admission without delivering a frame.
- Binding triad: selected, neutral and rejected actual calls; exact native
  function, local argument, call and caller identities.
- Nested/repeated calls: exact ancestry and distinct actual call sites.
- Stopped/cancelled prefix: a poisoned native tail must remain unobserved.
- Consumer panic: the original panic escapes; no blanket recovery is introduced.
- First-loan memory: independent width32/8193 and depth8/257 cases. These measure
  delivery of the first loan, not complete traversal or O(1) active depth.
- Semantic fuzz: the native source producer and independent selected-binding,
  timing and ownership oracle run inside each callback.
- Benchmark: fixed257 neutral calls, then two actual calls or an uncalled
  literal; every iteration validates the native loans and observes its result.

The patches use zero context; apply them with `git apply --unidiff-zero`.
The mutation manifest selects only the exact affected test or leaf. Each source
control changes one load-bearing fact. Preserve ineffective controls, build
failures and infrastructure failures separately; none counts as an effective
counterexample. Restore the owned source before the next control. After source
proof, run the affected data-flow-role ratchet and final tools, then serial3s
benchmark and serial3s fuzz through Anvil on darwin-arm64. Reuse unchanged passing
evidence and rerun only failed leaves. No author-issued independent acceptance.
