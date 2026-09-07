# shutdown before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Plan holds [64]Step by contract (MaximumSteps), not a 1 MiB hidden buffer.
