# Exact GCS key lookup

`LookupGCSObject` reads official SDK metadata for one validated bucket/name.
It does not list prefixes, download bytes, issue receipts, or claim execution
acceptance. Its returned generation allows a caller to verify precisely those
bytes when recovering a stored object whose receipt commit failed.

Local evidence is retained under
`/private/tmp/peachfuzz-state-readonly-evidence/`. Every directory below contains
argv, source snapshots, revision/dirty facts, stdout/stderr digests and exit status.
These runs are development evidence based on c87eada, not independent acceptance.

- `primitive-exact-lookup-missing-entry-red`: compilation failed because the
  required typed public lookup did not exist. No behavioral failure is claimed.
- `primitive-exact-lookup-first`: 13 pass events, focused scope.
- `primitive-exact-lookup-package`: 795 pass events, three skipped live-provider
  tests (lifecycle/deletion/download capability), zero failed events. Skips are
  not successes and live-cloud coverage remains unavailable in this run.
- `primitive-exact-lookup-fuzz`: semantic provider identity/generation fuzz,
  2-second fuzz budget plus 1-second minimization budget, one worker, exit zero.
- `primitive-exact-lookup-binding-mutation-red`: disabling only the lookup's
  bucket/name equality check caused both foreign identity cases to fail (three
  failure events including parent). The mutation was discarded.
- `primitive-exact-lookup-restored-race`: 19 pass events, focused race scope,
  zero failed/skipped events.
- `primitive-exact-lookup-tool-0`, `-1`, `-2`: go vet, staticcheck, witness-lint
  respectively, each scoped to gcsobjects and exit zero.

Tests execute the official SDK against local HTTP provider fixtures. They pin
exact request path, no prefix/generation query, zero output on absence/refusal,
identity binding, generation validity and no request for invalid call inputs.
Existing bounded provider transport tests remain responsible for aggregate
metadata response limits. The new API is added to the compiler-visible operation
and struct inventories. This is not a claim to have swept all legacy package
tables, proved live IAM, benchmarked an improvement, or certified API recovery.
