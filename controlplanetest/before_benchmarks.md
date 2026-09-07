# controlplanetest before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Hot path is a typed parse/assemble door.

## Measured

IssueInstallation 178441 ns/op 4628 B/op 118 allocs
