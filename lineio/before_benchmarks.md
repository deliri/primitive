> Historical whole-line measurements. The current fragment API and profiled results are in [the streaming review](../_docs/lineio_upgrade_20260909.md).

# lineio before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. bufio.Scanner with caller-declared initial and line ceiling; 64-line and 4096-line scans both 280 B/op, 5 allocs.
