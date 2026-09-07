# runworkspace before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Residue scanners start at 256/1024 bytes with a 64 KiB line ceiling (bufio grows). Archive drain uses a 32 KiB copy buffer.
