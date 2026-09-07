# process before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. TruncatingWriter never retains the output limit. Stdout 64KiB vs 1MiB streaming benches exist.
