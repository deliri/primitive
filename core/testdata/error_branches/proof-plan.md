# Native error branches

Witness currently expands Go's single and joined error causes itself before
allowing subject grammar to replace a native read refusal. Moving that mechanical
traversal to Primitive must preserve every unrelated cause.

| Assumption | Evidence / status | Consequence and smallest check |
| --- | --- | --- |
| One matching error anywhere authorizes the whole error tree. | DISPROVED by a real errors.Join(schema, I/O failure); errors.Is can find schema while I/O still needs propagation. | Require each joined branch, including nested and wrapped joins, to reach its own accepted cause. |
| A product can own an atomic native wrapper. | VERIFIED as an existing Witness requirement: native JSON syntax and semantic nodes have classification meaning at their owning node. | Lend the non-joined error and its immediate Unwrap result; policy may explicitly accept that atomic subtree. Joined siblings remain independent. |
| Error traversal needs a collected graph or registry. | DISPROVED for measured flat joins: query allocation is zero at widths 32 and 16,385. | Use Go's Unwrap methods directly, loop through linear wrappers and retain only active joined ancestry. Measure flat widths 32 and 16,385. |
| Every provider returns a finite error tree. | UNVERIFIED for arbitrary user-defined methods; this is also the standard errors.Is traversal precondition. | Document the native contract. Do not invent a visited graph or an arbitrary depth quota. |
| Nil, an empty join or an unowned terminal has authority. | DISPROVED by the focused native boundary checks. | Each must return false; a nil callback is an invalid request and refuses. |
| The new query closes Witness's schema-classification findings. | UNVERIFIED: producer proof, publication, adoption and actual consumer proof are separate. | Preserve existing joined I/O/cancellation/grammar counterexamples and check the consumer only after publication. |

Core already owns stable native error identities and shared pure contracts.
The capability catalog derives Core's ownership from the compiled architecture;
there is no second error-family table here. The new request supplies only a
borrowed native error and policy predicate. Its observation is ephemeral flow,
not a persisted schema verdict. There is no new error identity, cache, index,
state machine, population aggregate or acceptance quota.

Before publication: inspect real native boundary and allocation output; run a
semantic control that turns all-branches into any-branch and require the mixed
native failure case to fail; restore and retry only that failed leaf. Run scoped
tools before the serial three-second benchmark, then the three-second independent
bit-oracle fuzz target. Preserve every attempt and actual depth. None supplies
independent acceptance or whole-Core closure.

Mac development proof: all 19 selected boundary, early-stop, contract, native
allocation and inventory nodes pass. Replacing all-branches with any-branch
actually fails the mixed EOF/closed-pipe leaf; the restored exact failed-only
retry passes with the original shuffle seed. Vet and staticcheck both execute
and return empty native output. Their child receipts and output are verified;
the aggregate plan still reports retention pending and is not itself a sealed
acceptance receipt.

The serial profiled three-second benchmark checks a fixed 16,385-branch native
join: 6,356 iterations, 541,860 ns/op, 2 B/op and 0 allocs/op as reported by Go.
Both profiles and native output match the receipt. The separate isolated
allocation ratchet reports zero allocations at both tested widths; this does
not erase the benchmark's measured byte accounting or claim bounded native
error-tree construction. The three-second semantic fuzz phase reaches 996
executions and five new interesting inputs (native package duration 5.241s,
including startup/shutdown). No speedup ratio or independent acceptance claim.
