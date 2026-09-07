# core before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. JSON reader grows from 4 KiB. Path parse 0 B/op. DigestWriter 160 B/op at 1 KiB and 1 MiB.
