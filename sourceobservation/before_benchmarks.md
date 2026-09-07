# sourceobservation before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Membership consumers retain one previous path (O(1) state).
