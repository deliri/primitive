# id before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. ParseULID 0 B/op; ParseUUIDv7 48 B/op (stdlib uuid.Parse); AppendText reused buffer 0 B/op.
