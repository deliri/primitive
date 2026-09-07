# objectstore before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Inspect streams a fixed buffer through sha256/crc32c/blake3; 1 KiB / 1 MiB / 16 MiB keep 11 allocs.
