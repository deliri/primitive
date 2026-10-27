# Preserve a getter result through its compiler home

Primitive425 correctly admits direct getter results and returned record
projections. Witness's real consumer still reports thirteen inventory fields:
its multi-field artifact is compared, inspected and validated inside a loop.
The retained native frame shows Go storing the getter result in a local home,
then loading that home for validation. A bare-call origin query cannot see
through that load.

| Assumption | Evidence | Status / consequence |
| --- | --- | --- |
| One scalar-field comparison represents a multi-field loop record |425's finite fixture passes while the real inventory and new native fixture fail | REFUTED; test the actual multi-field comparison and loop |
| Any prior getter store can lend authority to a later load | Whole/field replacement, neighbor replacement and escape controls | REFUTED; require the exact native home and existing stability guard |
| A later write replaces an already-loaded snapshot | Snapshot-before-write control retains the original value | REFUTED; preserve the read's actual instruction order |
| This requires an assignment history or new public query | Existing instruction loans, valueExpressionCall and receiverGateHomeStable suffice | REFUTED; extend the existing private producer lookup only |
| Native control-flow provenance proves every runtime path | The existing observation remains a call/error-flow floor | UNVERIFIED; no new path-completeness or alias guarantee |

The change borrows one actual non-heap home, initializing store and read. It
retains no store population, new registry, history, quota or state machine.
The source's existing native-call and field-slot checks still own identity.
Mutated or escaped homes grant no authority. No Witness allowlist or extra
product validation call is added.

Base41f10c15b2766998589129fbcf0a742ef3b56573 / published Primitive425.
All correctness execution uses installed Mac Anvil, darwin-arm64, JSONv2,
CPU1/RAM4GiB, race/shuffle/count1. Raw source, requests, native frame diagnostic,
receipts, output and profiles remain outside Git in the owned scratch root
/private/tmp/witness-native-unblock-z5l6ve72.

- Actual native loop red2026-10-27-accdacf8dffd9529fbecec8ed51f6f97868c74c163191eed5f1f4b6cd50244f9,
  sourcefbbdb5bf14d65165f6765410eb514d0fda12627d98dc13750d80a2de7acc3c50:
  one failed target. The complete native frame retains the initializing call,
  local store, field read and later loaded validation operand.
- Fixed exact target2026-10-27-f6fa9521399fd405cb774a3d67a499809174bb79cae393a7c8300b1b26e41ef2,
  source06c4dc887a0684e03ea5222c428b64e11aebcfcc2fc6808803032e4a23419595:
  one passed target, original seed6225899364829547156.
- Value/mutation/escape pressure:
  2026-10-27-40de2ec825c16dfaad1dd4281ea9742298519ed9d15d16a67d1c76e476f726f7,
  source8f6ffa6216965ac319488403e9b669ae2b8f72558fafb6466c4b87043df64b66:
  six passed nodes, no skips. Four distinct replacement/escape refusals plus
  the immutable loaded-snapshot control exercise the reused stability path.

All results above have independent raw receipt/output extent, digest and native
event-accounting verification. Expanded semantic seeds, the additional stored
record allocation workload, scoped tools and serial3s benchmark/fuzz proof are
recorded after actual readback. Unchanged425 direct/projection/custody proof is
retained with its original source. Publication, official Witness adoption and
the consumer retry remain separate steps; independent acceptance stays open.

Affected semantic seeds and allocation workload:
2026-10-27-42cd1eb10d594ee43d4be46c80892dade01b66fef3d7f9181ec3e07246b7397f,
source4b4916ae3723a50193af9eb6a53876a6ad70426c6031919a80af0d40970f2d44:
seventeen passed nodes, no skips (fifteen authored seeds, their parent and the
allocation target). At32/16,385 statements, stored returned record query
allocations are4,920/4,872bytes; unchanged direct and returned workloads remain
5,304/5,304 and4,872/4,872 respectively. Compiler storage is excluded; this is
finite workload evidence, not a universal memory or speed claim.

Scoped Go vet2026-10-27-a2407acc9f20debfffdf6b364a0673b9fa23c0a8db04c724fe211595bb14cd70
and Staticcheck2026-10-27-1dd7f9c37e38e86a0c6c4236958331a0eefc0c18c6948b66396c77ee7bbac0b5
bind that same source and each execute once, exit0/zero findings. Independent
receipt/artifact and analyzer stream checks verify zero analysis-output bytes.

While this ran, another publisher released426 ated228ac4, with changes in
cloud provider packages, Core error constants and the release number. Its
published commits were fast-forwarded intact into this branch; none touches
the getter query or its tests. Serial benchmark/fuzz proof below uses the
combined source. The next release number will be chosen after another fetch.

Combined-source profiled3s benchmark:
2026-10-27-ce6ac926d90426025295a1f1d53ebb2a75bff6fb23579153e74296913457010c,
sourceedda581b248ef437f76a6a3c7de7cbe30cb4e58fcbcf2e08f260fba3ffe7bdf3:
one actual measured row,212,629iterations,16,564ns/op,3,792B/op,62allocs/op.
Native package exits0/passes; benchmark target counters correctly report one
started/zero test passes. CPU profile18,899bytes SHA256
2cbd476c9529506e0edd499a70f5d7f58f18e3b3cf458c40d93a2d1f50dd914b;
memory profile4,625bytes SHA256
77c42de7c4452aa4fa643983673b701e6c4440af880684f74ea2f4c526449d62.
Both retained profile extents/hashes and the actual measured row verify.
Profiling perturbs timing and the prior benchmark uses a different workload;
these measurements are not a before/after speedup claim.

Combined-source Go vet2026-10-27-6402974c15f1fcaa1812e17b4905d746a87bf7de0088e71522264db9ebdddb56
and Staticcheck2026-10-27-5e361798b597af11a33c98b3b2c0d608fa4aecd529514f8e70b5039ef6389985
bind the same edda581b snapshot and each execute once, native0/zero findings;
independent raw receipt/artifact and analyzer stream checks verify zero bytes.
No repeated passing test campaign was requested for the disjoint publisher's
packages. Their published proof remains attached to their own commits.

Serial3s semantic fuzz2026-10-27-3e03a89d0665dfc098458a3a3ac8e18fbc45c2cfc7051ee772f6a108ed9455e2
binds the same edda581b snapshot: one passed target, no skips; native baseline
27/27 including cache,35reported executions/0new-interesting inputs. Raw
artifact extent/hash and native accounting checks verify. This is finite
development evidence, not a count of unique mutants or independent acceptance.

Final fetch retains origin/main ed228ac4 unchanged. Release427 is unoccupied;
only the authored proof and release number change after the executable proof.
The actual Witness consumer still requires official adoption and its exact
failed-target retry; no product completion is inferred from these unit checks.
