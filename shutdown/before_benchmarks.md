# shutdown before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. Plan holds [64]Step by contract (MaximumSteps), not a 1 MiB hidden buffer.


Current package-sweep measurements and retained CPU/memory profiles are recorded
in [the September 9 upgrade review](../_docs/shutdown_upgrade_20260909.md).
The values above remain historical reservation-audit evidence.
