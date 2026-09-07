# lineio before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. bufio.Scanner with caller-declared initial and line ceiling; 64-line and 4096-line scans both 280 B/op, 5 allocs.
