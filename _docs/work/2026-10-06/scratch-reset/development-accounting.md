# Scratch reset development proof

Baseline: 468c1bd (Primitive JSON object evidence checkpoint); source changes were uncommitted during this run.
Go 1.27.1 darwin/arm64. Command: go test -json -race -count=1 ./filestore ./compass -run 'TestScratchReset|TestCurrent' -timeout=3m. Exit 0. Selection is explicitly filtered; this is not a full Filestore test claim. Timed fuzz and benchmarks were not run.

Nine byte extents, each across three actual OS write/reset/read cycles, prove byte disposal and restoration of the write coordinate. Cancellation, absent handles, closed handles and read-only handles refuse. An actual production mutation changed SeekStart to SeekCurrent and disabled the zero-coordinate guard: every extent failed, including the second zero-length cycle. The mutation was restored before the retained passing race run.

The first mutation diagnostic printed whole sparse content and tool output was truncated; it is unavailable as complete evidence. The diagnostic was bounded to length plus a 32-byte prefix, and the same mutation rerun is retained completely in write-coordinate-mutation.stdout. This is development evidence, not independent acceptance.
