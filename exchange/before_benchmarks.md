# exchange before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Body reservation capped at 4 KiB then grows; BenchmarkServerJSONBoundaryByLimit proves cost tracks received bytes not RequestBodyLimit.
