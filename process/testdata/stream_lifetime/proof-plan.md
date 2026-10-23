# Native stream lifetime contract

Assumptions: a finite producer can exit before its consumer drains; a duplex
peer can delay its own EOF until it receives EOF. Both are native pipe behavior,
not application state. Go os/exec documents WaitDelay zero as EOF-owned drain
and nonzero as cancellation/after-exit pipe bounds. Reader and writer interfaces
can contain typed nil; Core owns their presence predicates. Descriptor origin
cannot be established from its number alone; trusted inheritance is a precondition.

The minimal production changes admit validated zero WaitDelay in Request and
Plan, and close each relay-owned output immediately after its copy ends before
joining the opposite direction. No buffering world, scheduler or catalog is added.

Proof: TestZeroWaitDelayDrainsAfterChildExit holds the real child's destination,
observes its identity gone, applies after-exit deadline pressure, then releases
and requires exact output, successful native exit and joined ownership. Existing
direct/group cancellation proofs select zero and require native wait/cause and
whole-tree absence. Nonzero WaitDelay retains exec.ErrWaitDelay proof. Plan and
request zero boundaries, canonical JSON closure and semantic fuzz seeds cover
admission without changing the rest of the wire contract.

Relay's pipe-backed dependent-EOF row fails if NearOutput is not closed before
upward join, and requires every byte plus far close. Both close native causes
are independently reachable. Four typed-nil rows require the core contract error,
an exactly zero result and no write/close. Existing three-process inherited relay
rows execute the changed destination contract natively. mutants.json declares
compiling behavioral mutations; build failures do not count as kills.
