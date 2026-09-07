# gitrepo before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Hot path is a typed parse/assemble door.

## Measured

DefaultConfiguration 7.409 ns/op 0 B/op 0 allocs; WorktreeSelection.String 2.287 ns/op 0 B/op 0 allocs
