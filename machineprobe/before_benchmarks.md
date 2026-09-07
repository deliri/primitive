# machineprobe before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Hot path is a typed parse/assemble door.

## Measured

FailureKind.UnmarshalJSON 98.02 ns/op 16 B/op 1 allocs
