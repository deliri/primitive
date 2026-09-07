# distributionauth before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Hot path is a typed parse/assemble door.

## Measured

AssembleUpdate 1260 ns/op 170 B/op 9 allocs
