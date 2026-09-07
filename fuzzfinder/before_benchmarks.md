# fuzzfinder before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Find streams Filestore.Walk; retains at most 128 names (typed bound, not a hidden ceiling reservation of the tree).
