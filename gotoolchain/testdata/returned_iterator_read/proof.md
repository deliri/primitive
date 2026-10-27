# Receiver field reads in consumed returned iterators

First principles and assumptions:

- VERIFIED by actual native compiler fixtures: ranging a returned Go function invokes its body. The original query follows a factory call but skips its returned closure, falsely withholding receiver-field reads. A discarded or untaken iterator grants no read.
- VERIFIED: Go's native return operands and existing Primitive sourceFunctionValue loan identify the real source function, preserve named function conversions, and own closure/capture custody. This change reuses that authority; no product function/alias/field registry is added.
- The claim remains a field-read floor, not proof of an invariant or all-path execution. Unknown returned factories and iterator variables remain conservative absence; this slice does not extend child-error propagation.
- UNVERIFIED here: exact Witness self-lint reduction and application catalog verdicts until published adoption and actual focused consumer verification.

Mac Anvil, Go1.27.3,JSONv2,race and retained shuffle, published436basefe82bd87b57700b1c5c182f5c3e2cfb4ca691eb2:

- RED a1083a601c6e2960f0868c0bf062012be04be060b97a599cbe1f2035a3bc0cd3:50distinct source/binding/field cases (five call forms × actual/foreign/shadowed/discarded/untaken origin × read/unread sibling).45negative leaves PASS;5genuine captures fail plus parent.
- First repair3da3e9dc32a6302016fff87828230d1dd0a014e74fa5e6c7cc7a9019b18d4d54:88affected native checks PASS,0skip.
- Named-return counterexample03ca09015ca69bb3cfc8d357f51e87c8dbad3584cd458a7f8c3eab07a78bc432: local named iterator, standard iter.Seq, standard iter.Seq2 each still fail under the first repair (3leaves+parent). This forced reuse of the existing native function-value loan rather than duplicate conversion/capture interpretation. One earlier request draft had unsorted source.extras and was refused before submission; it is retained as a caller error, not a runner defect.
- Final4f3c6879a6221fe1b7858f777808c73cd03e9753bdaec72e234a1e3acec311db: all56nodes PASS,0skip, covering the50-case matrix, real named/standard iterators and the existing no-syntax-population allocation contract. All runtime production bytes are retained/bound to that source; the earlier88checks cover unchanged direct-call paths.
- Final affected vet35a88a6e andStaticcheck61859fe5 exit0/zero findings; receipts/stdout/stderr/native counts and analyzer output hash/extents independently verify.

Raw requests/receipts/output stay outside Git in the owning Witness scratch/warehouse. No independent owner acceptance, deployed consumer or whole-package claim.

- Final serial3s semantic fuzz after all tests/tools: 2026-10-27-77f63bf8e3836fa9bbf32bfd4b6fa3d848b274b2f8e162675446d7aa39fbce89; observed95executions/22new-interesting inputs, one passing fuzz node; independently verified raw outputs/counts.
