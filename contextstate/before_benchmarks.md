The September 8 Contextstate upgrade now has a profiled original-production baseline and a final paired comparison. See [the review](upgrade_review.md) and [complete command/artifact evidence](upgrade_evidence.json). Each benchmark retains CPU and memory profiles plus its matching binary.

The earlier filestore-specific finding below is historical; it is not the new benchmark baseline.

# contextstate before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Validate/Observe call context.Err only (0 B/op).
