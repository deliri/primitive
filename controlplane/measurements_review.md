# Independent reported measurements

Usage windows can now carry independent additive counters in a closed ordinal
vocabulary. Products own the ordinal meanings. Primitive validates positive
counts, ascending unique classes, real supporting work, canonical bytes, device
signature binding, immutable verified snapshots, and watermark hashing. It does
not sum different measurement dimensions or interpret their meaning.

No additional request or provider operation is introduced. The existing signed
check-in and append-only accepted-window chain bind the new counters. Products
must explicitly project their own measured facts into this shared agreement.

Evidence is retained under `/private/tmp/peachfuzz-state-readonly-evidence/`:

- `primitive-measurements-first-20260914`: 1075 passing test events.
- `primitive-measurements-binding-20260914`: 1076 passing test events.
- `primitive-measurements-alias-mutation-20260914`: deliberately removed the
  verified snapshot's measurement copy; the caller-mutation test failed.
- `primitive-measurements-restored-20260914`: restored production; 1212 passing
  events across controlplane, controlplanetest, and permit, no skips.
- `primitive-measurements-vet-20260914` and
  `primitive-measurements-staticcheck-20260914`: exit zero.
- `primitive-measurements-fuzz-20260914`: semantic decoder fuzz phase exited
  zero; configured fuzz and minimization budgets were 3 seconds each, with
  19.618 seconds total process time including corpus startup and shutdown.

Each execution record retains source snapshots, dirty-base revision, command,
output hashes and byte counts, and outcome facts. Counts include subtests.
These are local development facts, not independent acceptance. The existing
hostile usage-window tables and external decoder fuzz oracle remain in place;
the new tests add measurement-specific boundaries, exhaustive ordinal admission,
signature tampering, source isolation, and chain binding. Consumer projection,
durable accounting, and marketing publication are separate downstream proof
surfaces and are not claimed by this Primitive slice.
