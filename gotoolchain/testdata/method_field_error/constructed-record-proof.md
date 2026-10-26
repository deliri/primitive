# Native constructed-record error propagation

The actual Witness Core validation path projects ReplayOutcome into a new
ReplayComputationInputs before calling its validation method. A native selected
receiver field remains the same source fact across that construction.

| Claim | Evidence / status | Consequence and smallest check |
| --- | --- | --- |
| Passing a field through a record literal loses its native origin. | DISPROVED: original query refuses three valid compiler-produced keyed, unkeyed and repeated-origin paths while direct and rejection controls pass. | Follow native field objects and ordinals; do not invent a Witness validation exception. |
| Go field bindings authenticate the constructed slot. | VERIFIED: copied and wrongly named field objects refuse; removing object identity admits a copied field, and the original-seed restored retry passes. | Use go/types lookup and exact native object identity, without a field registry. |
| Early successful propagation can ignore damaged later native keys. | DISPROVED: removing complete-key validation admits authority over a damaged tail; restoring it refuses with the existing Core output error. | Validate the borrowed literal before lending any matching binding. |
| One source field can bind more than one destination slot. | VERIFIED: a real duplicate-origin literal propagates only through its checked nonfirst slot. | Stream candidate bindings; do not retain a slot list or return the first spelling match. |
| The query constructs a second syntax population. | DISPROVED for the measured path: the constructed-record allocation ratchet measures 3,512 allocated query bytes at both 32 and 16,385 preceding statements. | Retain only active call ancestry; this does not claim constant Go compiler memory or constant runtime. |
| Arbitrarily wide records have acceptable throughput. | UNVERIFIED: native field lookup owns its own cost; current literal cases have two fields. | No broad throughput claim; a wide-record workload needs its own measurement before optimization. |
| Producer success closes Witness's validation findings. | UNVERIFIED: publication, adoption and actual affected self-lint remain separate. | Publish the proved Primitive revision, then inspect the consumer output. |

The change extends the existing private binding query to lend native record
slots through an iterator. It adds no public record type, product validator,
mutable state machine, cached graph, name registry or new error identity.
Go owns record layout, field identity, method selection and source compilation.
Primitive owns the query; Witness retains policy.

Preserve original RED and its three failed leaves, exact corrected observations,
five damaged-binding boundaries, two semantic guard faults with restored retries,
existing affected helper/generic/cancellation/refusal cases and the real memory
ratchet. The semantic fuzz oracle independently varies the selected source field,
method identity, returned cause and direct/keyed/unkeyed/unvalidated-slot paths.
Run tools before the serial three-second constructed-record benchmark, then the
three-second semantic fuzz phase. Those development results are not independent
acceptance. Keep raw output, profiles, source identities and failed attempts in
Anvil custody, outside Git.

Focused development proof on the Mac: the corrected native boundaries and
affected regressions pass, including the 54-node helper/generic/refusal/memory
request. Both field-identity and damaged-tail semantic controls fail at their
intended assertions, and their original-seed corrected retries pass. Independent
vet and staticcheck children have complete retained receipts and empty native
output; the first vet child never executed and its cancellation remains retained.

The serial profiled benchmark executes 374,752 queries at 9,915 ns/op, 3,208 B/op
and 56 allocations/op. Its complete output and both profiles match their
retained receipt. These are one workload's measurements, not a speedup claim.
The initial three-second fuzz attempt spent its window replaying 14 of 16 seeds
and supplies no mutation-depth proof. Seven distinct seeds retain the required
semantic boundaries without redundant combinations; the revised three-second
phase completes all seven, executes 26 inputs and reports seven new interesting
inputs (5.388 seconds native package duration including startup/shutdown).
Original attempts remain retained. This is focused development proof, not
independent acceptance or whole-package closure.
