# Borrowing a captured callable

Go stores a function variable in a native cell when an inner closure, including
a generated range callback, captures it. The existing native cell query already
authenticates one dominating store and read-only captures. Its pointer-only
admission prevented the shared source-function query from borrowing the callable
stored there. This change admits function signatures through that same query.
The function-value projector follows only the authenticated store; it does not
create an alias registry, callback environment or execution claim.

Assumptions and limits:

- VERIFIED: the actual nested iterator counterexample fails on Primitive438.
  Original eb0af8a9 has two failed nodes; exact original-seed cb205725 restores
  the leaf and parent, two passes and no skips.
- VERIFIED: 16 source cases distinguish native named/literal/conversion/range
  captures from multiple stores, replacement before or after use, child/sibling
  mutation, conditional initialization, escapes, opaque and nil values. Seven
  custody cases challenge cloned function/load/closure identity, a closure from
  another frame cancellation cause and missing/typed-nil stored callables. Run53456a5a
  verifies the original21cases plus two parents,23passes/no skips, with exact
  native target and original operand. The missing-cell corruption exposed an
  actual panic in4da11c81. Three nodes started, two failed and one had no terminal
  event because of the panic; it is not counted as passing. The three-line
  native-value guard restores the exact failed leaf c9d738c3, two passes. The
  previously unfinished typed-nil row c848551d also passes with its parent. Both
  assert ErrGoToolchainOutput, zero loan and a successful native baseline.
- VERIFIED: source-backed stored functions retain the existing native owner,
  same-frame/dominance and typed cause checks. No new public API is needed.
- VERIFIED: affected native queries57930f69 pass766nodes/no skips on the query
  extension before the final missing-value guard. Unchanged positive/negative
  native proof is retained; only the newly failing/unfinished leaves run after
  that guard. The guard tests themselves verify a real positive baseline.
- UNVERIFIED: final tools and extended semantic fuzz are pending. Publication
  and real Witness Core improvement must wait for those results and an actual
  consumer check.
- UNVERIFIED: arbitrary-depth constant allocation or a throughput improvement.
  The query reuses native referrers and lexical ancestry; it retains no source
  population. Compiler construction is separate from query memory.

The existing iterator semantic fuzz now makes the helper be captured by a later
range callback. Its independently chosen field, method, yielded coordinate,
consumer coordinate and cause erasure still determine the expected result.
Only the previously missing storage representation changes.

Earlier attempts, native outputs, receipts, exact source bindings and timing stay
in the owned scratch warehouse; raw proof is not committed. The public API and
Primitive capability ownership remain unchanged. Independent acceptance belongs
to the user.

The extended semantic fuzz bbdb4611 failed at seed#1 before any generated
executions: the second of two yielded errors through a captured helper refused
with ErrGoToolchainContract. Native diagnostics identify a captureless sibling
function in the producer. Such an SSA Function can have a lexical parent but is
not a storage operand. The callback-operand comparison now excludes that native
function representation and leaves its source authentication to the shared
function projector. The failing seed and promoted named native regression pass
in5fd69751, four nodes/no skips. The oracle and typed contracts are unchanged.
Temporary diagnostics were removed. Debug01 initially wrapped nil errors and is
not used as causal proof; debug02 preserves nil and debug03 supplies actual
callee/frame/argument identities. All attempts remain retained. Final tools and
semantic fuzz are rerun after the one-line correction, not inferred from earlier
passing tool results.

Fuzz4e7c5b47 subsequently reached the different-method control at seed#4 and
correctly failed the nil-refusal oracle: an unrelated captureless sibling was
sent through the receiver-storage comparison in methodFieldErrorObservedOperand.
A native Function cannot identify the selected receiver's record storage. That
representation now yields absence before the storage query; malformed actual
storage still follows its existing typed refusal. Native diagnostic proof and
original failure are retained. The named different-method regression, failed
seed#4 and previously unfinished seed#5 pass inae62b4f3, five nodes/no skips.
Tools and semantic fuzz are repeated only because production changed again.

## Final retained development evidence

Final native source snapshotd487b21c40ec445a5db0e3180d602482694c2c6261db40cc97eefaef1e14b26d
binds the negative regressionae62b4f3, final Go vet3fcc7557, final Staticcheck
a18193ae and semantic fuzz3f1eb702. Both analyzers have one actual execution,
exit0 and independently verified zero-byte analysis streams. Serial3s fuzz
passes20native executions/0new-interesting after its12baseline inputs. Native
receipt counters are retained; they do not claim20unique semantic mutations.
The failed earlier fuzz attempts and temporary diagnostics remain retained.

The earlier766-node affected query proof and the focused restorations above
remain distinct source revisions; they are not represented as766rerun final
checks. The final one-line native-function absence correction affects the new
negative path and is covered by its real failed seed, promoted regression and
final semantic fuzz. No new public boundary, ownership catalog, graph, quota or
product policy is introduced. Real Witness adoption and Core results remain
pending; this development proof does not grant independent acceptance.
