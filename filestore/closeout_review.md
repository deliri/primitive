# Filestore author closeout — v2026.1.19

The user reviewed and approved this checkpoint. The subsequent requested release
gates passed; see the [release gate report](../evidence/release-v2026.1.19/README.md)
for the final source manifests and exact runtime scope. The author evidence below
is the package-sweep checkpoint, preceding mechanical Go fix and lint cleanup.
Native Linux/Windows runtime acceptance remains separate from the macOS evidence.

## What production now guarantees

- Stage, Commit, Recover and OpenStagedRead bind the exact rooted capability,
  inode, extent and permissions. Symlinks cannot borrow another inode’s receipt.
  Same-inode rename settlement consumes the owned staging name.
- Streaming uses Go io.Copy, io.LimitReader and io.ReadFull with bounded adapters
  for mechanical counts, cancellation, progress and typed error ownership.
  Invalid counts, short writes, joined errors and source unwinding retain exact
  acknowledgments and cleanup ownership.
- Native Go handles own durability, permissions, rooted acquisition, observation
  and closure. Inspection uses direct Lstat; EnsureDirectory avoids repeated
  ancestor acquisition. Missing-tree removal preserves Go’s absence semantics.
- Walk checks cancellation before each native delivery and before descent.
  Go still owns directory reads, scheduling, filesystem arbitration and errors.

There is no new production API, dependency, workflow engine or mutable product
state machine in this closeout. Scope is macOS, Linux and Windows. Streaming
memory is bounded; processing remains proportional to bytes/entries. Walk also
uses memory/handles proportional to depth, and lexical ordering retains a
bounded directory population. No O(1) time or power-loss durability proof is
claimed.

## Verification

- Full uncached Filestore tests: 5,515 passing Go events, zero failures/skips,
  89.1% statement coverage on Darwin/arm64.
- Full shuffled race run: the same 5,515 passing events, zero failures/skips or
  race reports, including the 10,000-writer proof and materialized directory
  ceiling boundaries. Shuffle seed: 1788825310639977000; parallel subtests: 4.
- Linux/amd64 and Windows/amd64 test binaries compile from the final source.
  They were not executed on those operating systems here.
- Scoped go vet, staticcheck, errcheck, formatting and production gocyclo ≤10
  pass. Witness lint has zero unwaived findings and three explained findings.
- Fifteen final deliberate semantic mutations fail the intended tests without
  build failures: cancellation, partial acknowledgments, native errors,
  recovery receipts, exclusive staging, permissions, ownership, native identity,
  batch delivery, staged admission, ingress inventory and Temporal ownership.
- Final 30-second semantic fuzz refreshes pass: Walk 20,307 executions and
  inspection 54,604, each with one worker. See the
  [fuzz summary](testdata/test-upgrade-20260907/final-fuzz-summary.json).
- The artifact audit verifies 644 execution records and 3,424 retained files
  with no hash mismatches or source drift during execution. Historical failed
  attempts remain identifiable; see
  [final artifact audit](testdata/test-upgrade-20260907/final-artifact-audit-2.json).
- Compass/version checks pass against the prepared v2026.1.19 declaration.

Go event counts include parent tests and fuzz seeds. They are not distinct
semantic row counts. Every execution retains exact command, source snapshots,
cache posture, exit, output and artifact hashes. See
[final-author-checks.json](testdata/test-upgrade-20260907/final-author-checks.json).

## Test cleanup and evidence

The remaining grouped examples and assertion-hiding helpers are replaced by
direct tables or retired into stronger existing matrices. Original inode
lifetime is pinned during namespace substitution. Dense/sparse allocation is
compared to native Go blocks, never an assumed physical-storage policy.
The [retirement map](testdata/test-upgrade-20260907/final-legacy-retirement-map.json)
names each replacement.

The Windows build failure, my wrong request type, incorrect error-hierarchy
oracle, unused import and doctrine findings remain separate failed attempts in
[final-closeout-corrections.json](testdata/test-upgrade-20260907/final-closeout-corrections.json).
They are not counted as production defects. Older corrections and production
reds remain in [the chronological review](upgrade_review.md).

The two exact EOF comparisons follow Go’s own io.EOF contract. errors.Is would
misclassify a joined EOF/native failure as clean termination. The root-opening
waiver preserves explicit Close-error handling before capability transfer.
These are narrow documented exceptions, not suppressed package-wide findings.

[Benchmark results](final_benchmarks.md) include slower observations, allocation
costs and all attempts. Five final workloads were refreshed after their EOF
oracle changed; CPU/memory profiles and binaries came from each same invocation.
Raw profiles and logs remain local. The user subsequently authorized deleting
the reviewed binaries; their hashes remain in the release disposal receipt. Commit candidates contain source,
small JSON/Markdown reports and promoted regression corpus only.

## Release boundary

The sole release declaration is compass/config.json, set to v2026.1.19.
The approved release includes the Exchange continuation and Filestore changes.
The user subsequently authorized and received the requested module analyzer gates
and uncached Exchange/Filestore tests. The release gate report supersedes earlier
pending-gate and pending-approval notes. Native Linux/Windows runtime proof still
requires those operating systems; cross-compilation does not supply that proof.
