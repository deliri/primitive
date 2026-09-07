# chitauth before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Hot path is a typed parse/assemble door.

## Measured

Assemble 1848 ns/op 208 B/op 13 allocs
