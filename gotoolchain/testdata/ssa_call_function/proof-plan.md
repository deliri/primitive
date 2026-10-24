# Native named-call object

Source-frame availability and named Go object identity are different facts.
Imported functions often have no source body in the unit. CallFunction lends the
native types.Func from Go's StaticCallee result, using existing call membership,
signature, argument layout and callable custody checks. It creates no symbol
catalog, function population, call graph or invocation simulation. Objects and
signatures remain owned by Go; package capability ownership remains Primitive's
compiled catalog.

A nil object is neutral for opaque, merged, literal, builtin, dynamic/interface
and other cases where Go supplies no exact named target. This query deliberately
reports StaticCallee semantics; explicit representation conversions are not
claimed resolved. Call/defer/go records do not imply an execution-timing policy.
Do not substitute this contract for a source-frame loan.

Producer, schema and native cause checks challenge nil/typed-nil and detached
calls, foreign programs, absent signatures, extra arguments, malformed block
identity, cancellation and zero output. Go's Func.Signature getter supplies a
trivial signature when Type is nil; the schema regression caught that assumption,
and validation now checks the underlying Type. Missing package/signature,
absence and a valid named declaration form the schema triad.

Named/source/opaque forms, call timings, 18 semantic modes and 32/8193 native
arguments pressure the contract without a growing collection. Current mutations
remove membership, layout, underlying-type validation and native cause; each
must fail actual assertions. Fixture compile failures and the schema red remain
retained separately from passing native evidence. Race/tools and serial
three-second benchmark/fuzz facts are development proof, not user acceptance.
