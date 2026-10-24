# Source-expression sibling width

A Witness first-finding probe fails at4097 native decoder calls with242144 bytes
allocated. Native allocation profiling attributes234.70KiB to astutil.childrenOf
through ExpressionProducer.expressionValue. A new direct producer probe fails
at32 and8193 sibling calls, allocating3600 and487184 bytes respectively.

Keep the existing ExpressionRequest, ExpressionProducer and ExpressionNil
contracts. Replace PathEnclosingInterval with Go's PreorderStack, retain exact
node identity, and copy only its three outer nodes in the same reverse order
already passed to native SSA.EnclosingFunction. The existing descent through
Go's anonymous functions still handles compiler-generated iterator yield frames.
There is no new API, parser, node inventory or source/function model. Native
walker ancestry remains depth-sized; the proof addresses sibling width, not
arbitrary nesting or native compiler construction.

Retain both actual width reds and rerun only their failed leaves. Select the
affected producer/nil identity, clone/missing-fact, closure, package initializer
and iterator frame matrices. Assert native cancellation and original cause.
Measure first producer allocation after compilation, retaining the exact source
parameter/result identity. Restore the old sibling collector, choose the wrong
enclosing path or authorize a cloned expression by its position: each must fail
an actual assertion. Retain race/tools proof, then3s benchmark and3s semantic
producer fuzz in sequence. Allocate the compass release only after proof and
fresh upstream/config/index/owned-source checks; preserve concurrent exchange
and runnercontrol work. Adopt only published bytes matching the tested producer,
then rerun the failed Witness memory leaf. Independent acceptance stays external.
