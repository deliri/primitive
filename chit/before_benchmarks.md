# chit before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. ManifestAccumulator is one-pass O(1) retained memory; 128 vs 4096 object benches pin that.
