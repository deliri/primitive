# sourceproof before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Hot path is a typed parse/assemble door.

## Measured

State.UnmarshalJSON 100.7 ns/op 17 B/op 2 allocs
