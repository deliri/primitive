# Native source-call loans

`ExpressionCall` lends the actual result-producing native call and ordinal.
It keeps interface dispatch distinct from a named implementation. `ExpressionProducer`
uses the same origin walk, then requests Primitive's named-target observation.
No second origin graph, function catalog or source binding history is retained.

The 54-case native matrix covers 18 source-value forms in ordinary, lexical-child
and initialization frames. Copies, type changes, tuple slots, replaced aliases,
merges, runtime parameters, nil, dynamic and literal calls have independent
source-derived expectations. Additional native Go cases cover comparisons,
generic source frames and instantiated callees.

The custody matrix corrupts actual native objects and source facts after a valid
precondition: absent/foreign/cloned frames and expressions, context causes,
missing/surplus/foreign/ill-typed operands, block ordinals, detached/cloned calls,
typed nil values, address/debug metadata and missing/conflicting type facts.
Native source-value and callable assertions preserve both output identity and
native cause. Conversion self-edges and invalid tuple projections must refuse.

Population proof measures the first query at 32 and 8193 native instructions,
excluding compiler construction. The query must return the exact borrowed
interface call and retain no copied population. The benchmark fixes 257 source
uses, validates native call identity and reports one origin per operation.

The semantic fuzz oracle compiles canonical source fixtures and checks actual
frame, dispatch, named target and result ordinal. Non-vacuous cloned-function
and foreign-expression mutations must produce zero observations and typed
contract refusal. It supplements the named custody matrix.

Adjacent fault patches must cause native assertion failures for source/type
agreement, interface origin, tuple ordinal, callable refusal, native cause,
source-copy identity, conversion identity and copied instruction populations.
Retain ineffective attempts and their corrected controls; a compiler failure or
zero selected tests does not establish a useful mutation.

Run only affected gotoolchain tests. Reuse passing leaves after a correction;
retry only failed leaves. Race and focused tools precede serial development
benchmarks (`-benchtime=3s`) and semantic fuzzing (`-fuzztime=3s`). A native zero
exit during incomplete seed gathering is not complete fuzz proof.

Compilation is an explicit native whole-unit allocation. This query does not
claim constant memory for compiler construction, arbitrary native ancestry,
or undocumented instantiated-body debug availability. User acceptance remains
independent of implementation and retained proof.
