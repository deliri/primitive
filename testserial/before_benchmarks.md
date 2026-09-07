# testserial before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Hot path is a typed parse/assemble door.

## Measured

TestIsolationDeclaration.Validate 8.297 ns/op 0 B/op 0 allocs (Declare is Validate then t.Fatal)
