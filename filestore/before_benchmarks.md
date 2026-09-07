# filestore before

lexical Walk reserved DirectoryEntryMaximumLimit (65536 DirEntry slots) on every directory, including a one-file tree.

## Measured

BenchmarkLexicalSparseDirectory 1059390 B/op, 12 allocs/op

This is the filestore defect class: a compiler-owned maximum used as a
heap reservation instead of an admission limit. Primitive must not do the
scan; Go's standard library (or the exact payload) must.
