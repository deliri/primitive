# gotoolchain before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Analysis metadata slice capacity is min(len(data)/2, PackageMaximum).
