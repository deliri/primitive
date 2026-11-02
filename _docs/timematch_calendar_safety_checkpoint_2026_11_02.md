# TimeMatch calendar safety checkpoint

This SDK checkpoint contains calendar-provider corrections, Temporal civil-day
authority, linear ICS/invitation processing, and the native Go module lock
coordinate. It does not complete the Kernel booking service or establish live
Google, Microsoft, or iCloud behavior.

The changed calendar behavior has retained native tests against local provider
servers. First failures, setup failures, misfiltered attempts and deliberate
semantic controls remain separate from passing results. Unchanged passing
behavior was reused rather than rerun for this release.

| Surface | Retained proof |
| --- | --- |
| Leap-year December 31 | `99aca155` genuine failure; `f07a3f08` 37 passing nodes after the exact native sentinel correction |
| Exact civil-day bounds | `f806e525` 22 passing nodes; omitted day-completeness guard fails, restored failed-only scope passes |
| Google revoke classification | `1af6d831` fails; `259c4` passes; new outcome means only the submitted credential was already invalid |
| Unapplied Google/Apple create recovery | `4768d5b1` fails; `2a770` passes under deterministic ID / conditional-create guards |
| Graph conditional mutation | `cb24b6e5` exposes unconditional writes; `f49a3ebd` and `002ba3a1` pass after observed-revision binding |
| Apple resource preservation | `11a31f02` exposes lost notes/alarms and missing preflight; proper and failed-only corrections pass; deliberate preservation omission fails |
| Basic credential custody | Native secret-header tests and a scratch-cleanup omission control; restored exact leaves pass |
| ICS resource cost | Claude's `8eda645d` fails on old source; `6ec7b7e2` passes after linear unfolding and physical-line output |
| Native module lock coordinate | `871252aa` passes; wrong carrier `3748c3a5` permits the real Go download and fails; proper `1981dd98` passes |

Actual three-second semantic fuzz runs cover the changed Google revoke/recovery,
Graph metadata, Apple property preservation, Basic header grammar and the three
ICS/invitation targets. Graph metadata's final native result is retained without
inventing unavailable execution counters. A short successful fuzz run is not
exhaustive exploration.

The first group benchmark `d9187100` executes eight declared matching workloads,
plus the two changed ICS benchmarks. Its actual CPU profile is 55,690 bytes,
SHA-256 `07cf804db9a7ac2a1507088c6bc41a898b6f392f64a879140a39aa71d1cf65c8`;
heap profile is 7,294 bytes, SHA-256
`e9e4c858c48044b822bf8ad9265bf304c17b6f42b1f353dec98bffe6a48e3018`.
Both retrieved files match the retained receipt. These measurements are not a
matched before/after speed comparison. The largest dense group cases still
allocate substantially; the profiles identify preparation and edge sorting as
the next performance surface.

Root's independently rehashed local original indexes are
`/private/tmp/primitive468-SOURCE-ORIGINALS.json`,
`/private/tmp/primitive468-CHILD-ORIGINALS.json`,
`/private/tmp/primitive-claude-ics-originals-VERIFIED.json`, and
`/private/tmp/primitive-lock-source-originals-ROOT-VERIFIED.json`.
The corresponding retained Anvil receipt/execution pairs carry the full run,
source, toolchain, command, outcome and native test identities.

Final affected vet/static checks have no findings. Existing findings in
unchanged source remain open and unwaived; this is not a whole-SDK lint claim.
The location-directory improvement is collected from Claude's committed branch
and retains its own allocation counterexample and proof packet.

Remaining provider limits are explicit: iCloud free/busy support is unverified
and refusal remains unknown; Microsoft and Apple cannot promise programmatic
provider-wide credential revocation; Graph conditional-write behavior has no
live-account proof. TimeMatch never gains authority to alter an unrelated event.
No SDK can atomically lock a user's independent calendar edits together with a
Kernel database transaction. Kernel must bind every recovery attempt to its
current durable booking revision and consent before dispatch.
