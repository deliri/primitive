# Shared password admission queue

`passwordhash.Deriver` now waits for its caller-owned concurrency slot instead
of refusing valid overlapping work. Waiting ends when a slot becomes available
or the caller context is cancelled or expires. The native Argon2 operation still
holds its slot until it ends, and cancellation cannot publish a partial key.
There are no additional timers, workers, retries, or application defaults.

The queue matrix exhausts Derive/Verify with release, cancellation, and deadline
outcomes. Concurrent native derivations prove that excess calls wait and every
released call completes, while the existing validation and cancellation checks
remain covered. Tests use small native costs; they do not measure application
throughput or App Engine capacity. This is an admission slice, not a claim that
all Primitive packages or every consumer are complete.

Furnace proof is retained under
`/data/evidence/primitive/2026-10-08/media-projection/`:

- `password-queue-red01`: published base
  `0c3abecbba9d30b0d78ca3cee867a388932896b2` with the new regression test,
  zero passing/seven failing/zero skipped events; immediate refusal is exposed.
- `password-queue-green01`: candidate full passwordhash package, uncached
  race/shuffle, 67 passing/zero failing/zero skipped events.
- `password-queue-fuzz01`: 60-second semantic fuzz attempt passed with 1,964,288
  executions. Its configured duration exceeded this repository's three-second
  protocol budget; it is retained as an over-budget attempt, not the required
  budgeted release proof.
- `password-queue-protocol-fuzz169-01`: three-second semantic fuzz budget,
  one passing/zero failing/zero skipped target; actual process time 3.409 seconds.
- `password-queue-vet169-01` and `password-queue-doctrine169-01`: passed.
- `password-queue-static169-01`: failed before analysis because the installed
  analyzer cannot read Go 1.27 export data. The existing compatible
  `staticcheck-go127` passes as `password-queue-static169-02`.

All attempts retain command, source-tree state, toolchain, output digests and
exit facts. Candidate runs are dirty-source execution facts. The new untracked
test must be bound to committed bytes before claiming committed-source proof.
The full-package build, consumer integration, and deployed capacity proof are
separate surfaces. Independent acceptance remains with the user.
