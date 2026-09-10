# Payment baseline — 2026-09-10

Captured before production edits on Furnace, Go 1.27.1/linux-amd64, GOMAXPROCS=8, GOWORK=off. The main and supplemental baseline command records, raw output, CPU/memory profiles, binaries, actual timing and source snapshots are retained in `/home/d/engineering-evidence/primitive/payment-upgrade-20260910/baseline-profiles` and `baseline-additional-profiles`.

The original tiny domain-parser case hit the Go iteration ceiling before thirty seconds and remains exploratory. Its replacement measures a fixed batch of sixteen parses. See `after_benchmarks.md` for all matching absolute results, failed checks, limitations, and source binding.

---

## Historical notes retained

# payment before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Hot path is a typed parse/assemble door.

## Measured

ParseSigningDomain 5.746 ns/op 0 B/op 0 allocs; ParsePaymentID 118.5 ns/op 48 B/op 1 allocs
