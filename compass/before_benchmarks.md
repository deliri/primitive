# compass before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Decode is core.DecodeStrictJSON (grows from 4 KiB, not the 1 MiB document ceiling).
