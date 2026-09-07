# payment before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Hot path is a typed parse/assemble door.

## Measured

ParseSigningDomain 5.746 ns/op 0 B/op 0 allocs; ParsePaymentID 118.5 ns/op 48 B/op 1 allocs
