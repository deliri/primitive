# Owned Go-event grammar refusal

Witness's actual JSON parse-emission test fails on an unclosed record under
both its original traversal and the Primitive-backed traversal. The native
error tree contains a general JSON contract, Primitive contract identities and
free diagnostic strings, without a typed grammar-origin fact. Treating every
JSON contract as corruption would discard nominal and native read refusals.

| Assumption | Evidence / status | Consequence and smallest check |
| --- | --- | --- |
| General JSON contract implies observed grammar corruption. | DISPROVED: nominal negative elapsed and native read-count failures carry the general contract without grammar provenance. | Add one Core grammar identity at the owning parser operation; do not add a Witness string classifier. |
| The migration itself caused the original grammar refusal. | DISPROVED: the exact unclosed-record leaf also fails with the original Witness traversal and original shuffle seed. | Preserve both attempts; repair the missing producer fact. |
| Diagnostic text is an error-cause identity. | DISPROVED as an authority assumption: an English diagnostic is not a typed grammar contract. | Preserve operation text as native fmt context, while only typed identities and original native errors enter the cause tree. |
| A matching grammar sibling can hide I/O or cancellation. | DISPROVED by joined-cause and real source-error boundaries. | Require every native cause to belong to the owned parser refusal. |
| A JSON codec type owns any wrapped I/O cause. | DISPROVED: both real codec wrappers around io.ErrClosedPipe failed red under generic codec acceptance; private-origin corrected leaves pass with the original shuffle seed. | Keep the native source cause; bind any atomic unquote refusal to its actual owning operation. |
| Actual EOF accompanying the last bytes is a source refusal. | DISPROVED by the DataErrReader boundary. | Actual io.EOF remains framing; wrapped/joined EOF with native failure remains a refusal. |
| A prefix frame proves the complete stream. | DISPROVED: one valid frame followed by malformed tail still returns a typed grammar refusal. | Existing callbacks remain provisional prefix observations. |
| Adding the identity may renumber existing identities. | VERIFIED unchanged ordinals by appending before the closed-domain limit; focused closed-domain, parent and JSON tests pass. | Keep the JSON parent relationship and existing unknown-value refusals. |

Primitive's existing flat cmd/go framing and scalar logic remains the owner.
Go's jsontext owns string unquoting, and native errors/fmt own wrapping. This
change adds no grammar state, registry, cache, population model, quota, product
classification or input acceptance exception. The existing fixed windows and
prefix borrowing remain unchanged; no new arbitrary extent claim is made.

Required proof before publication: original native typed-identity RED and its
original-seed corrected leaf; uncovered malformed/neutral/positive frame cases;
real source and callback refusals; borrowed-codec negative boundary; a semantic
control that drops the all-cause check and fails its own counterexample; scoped
tools; serial three-second benchmark with same-run retained profiles; then
three-second semantic fuzz phases with actual depth. Publish before Witness
adopts the new query, then retry only the original failed consumer leaves.
All earlier failures and cutover refusals stay retained. No independent
acceptance or whole-package closure is claimed by these development checks.

Actual counterexample: replacing the all-cause predicate with an errors.Is grammar search fails TestGoEventGrammarCannotHideNativeCause on its first mixed native I/O cause. Restoring the predicate passes that exact leaf with the original seed. The authored patch is native-cause-becomes-grammar.patch. Native invalid UTF-8 reaches AppendUnquote and proves its concrete native syntax cause remains reachable. General codec types cannot lend that private operation origin.

Go vet (Core and Runnercontrol) and staticcheck (both scopes) complete with zero findings; each child receipt and raw output is verified independently. The aggregate tool receipt is retention-pending, so no sealed parent claim is made. The release44 batch retry produced actual native passes but an unavailable aggregate result; the original leaves were therefore verified individually, with original-seed provenance. Initial unexported-sentinel build failure remains a separate zero-execution attempt.

Three-second development measurements: grammar-cause classification executes21,546,386 iterations at165.3ns/op,0B/op,0allocs/op; both CPU and memory profile artifacts from that same execution verify. The joined-native-cause semantic fuzz target records154 executions and3 newly interesting inputs. The existing native stream semantic-closure target records19 executions and0 new inputs: this short probe is shallow, not a deep grammar campaign. All native stdout/stderr extents and hashes were checked against their verified receipts. No product completion, speed improvement, or independent acceptance is inferred.
