# attest before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Canonical object reuses caller buffer (0 alloc). Sign streams body; benches at 64KiB and maximum.
