# Upgrade package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole reconstruction report for archived package `upgrade`.
The archive is evidence, not authority. No archived production or test source
was copied.

The capability is real. Witness and Bug already ship product-local update
transactions that download exact signed artifacts, stream and verify their
bytes, execute bounded self-tests, replace an installed executable, and attempt
rollback after post-replacement failure. Kernel and Peachfuzz do not currently
self-update, but both contain production-grade local durability and bounded
shutdown patterns that the reconstructed capability needs.

The archived package is a strong prototype of the common installation half of
that transaction. It consumes only `update.PreparedRelease` as release
authority, streams through `objectstore` and `filestore`, retains a canonical
selection and crash journal, changes a stable launcher's selection rather than
renaming over the running process, classifies indeterminate replacement
outcomes, retains rollback state, and exposes explicit recovery actions.

It is not admissible unchanged. The archive's strongest claims exceed what its
types prove:

- bootstrap proves only that a file named like the launcher is nonempty and
  executable; it does not prove launcher identity or trusted bytes;
- startup success is a caller assertion carrying public, reproducible facts,
  not a witness minted by the launch that actually occurred;
- cancellation-detached persistence and cleanup use unbounded
  `context.WithoutCancel`, so hostile or stuck dependencies can block forever;
- Upgrade's local transition counter is represented by `release.Generation`,
  importing ownership from the wrong domain;
- raw string error sites and raw relative-path assembly keep important
  operation and layout contracts outside compiler ownership; and
- no consumer cutover and no required native filesystem matrix were proved.

The archive also puts extensive Upgrade-local wire, enum, cleanup, diagnostic,
and sizing implementation details in Core. The 2026 implementation must retain
Core only for genuinely shared path and error identities, keep package-local
rules in `upgrade`, and expose typed contracts at every crossing.

The 2026 direction is therefore a clean reconstruction informed by the archive
and all four consumers. It is not a copy operation.

## Evidence boundary

- A repository pin establishes what the consumer could compile against. Dirty
  working-tree evidence is separately qualified and never presented as
  committed evidence.

### Source revisions and pins

| Source | Exact revision or Primitive pin | `upgrade` availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`, `Harden capability inventory evidence`) | Present. Specification introduced by `c99f5a3ba11b5d5ded137816fcf234de281e7e20`; implementation introduced by `9251557b1c9c1464f3c148046b80b3144da17220`. | One unrelated untracked file, `core/api_http_boundary_hostile_test.go`; inspected Upgrade and Core sources are clean against HEAD. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | Committed `go.mod` pins `0df2954a2d911a5d7d775691d023d569affa2c20`, where `upgrade` is absent. Dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` (`2026-07-27T00:33`, `2026-07-27T00:47-04`, `2026-07-27T00:00`), where `upgrade` is present and byte-identical to archive HEAD. Current Kernel production does not import it. | Broad pre-existing dirty migration. The committed and dirty pins are distinct facts. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9`, where `upgrade` is absent. | Only the pre-existing untracked ledger was observed. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3`, where `upgrade` is absent. | Only the pre-existing untracked ledger was observed. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75`, where `upgrade` is absent. | Only the pre-existing modified ledger was observed. |

The archive repository contains all five recorded Primitive Git objects.
Direct `git cat-file` checks prove that only Kernel's dirty `e8b7172161a4` pin
contains an `upgrade` tree; the four committed consumer pins do not. A
repository-wide production import scan found no import of
`foundation/v2026/upgrade` in Kernel, Witness, Bug, or Peachfuzz.

The material archive history is:

- `c99f5a3ba11b5d5ded137816fcf234de281e7e20`: specify the recoverable
  installation protocol;
- `7d4fe90ad580bac154dd9ad3ba9af66bc4dc3126`: add the required Filestore
  owned-root and verified-read primitives;
- `9251557b1c9c1464f3c148046b80b3144da17220`: implement the recoverable
  upgrade lifecycle;
- `d259789e87bcadb829c5ffac72c6c91ccc604098`: centralize constants and close
  capabilities; and
- `6f35a55050caea6dac7b630f278d76aa6f58ceb5`: consolidate Temporal operating
  primitives.

At archive HEAD, the package contains:

- 5,378 production Go lines;
- 5,059 test and fuzz Go lines;
- 10,437 total Go lines; and
- a 1,320-line permanent-contract specification.

Archive status records agree that an implementation exists but do not establish
2026 admission. The specification calls itself a reviewed permanent contract
and the implementation a review candidate (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:1-4`).
The specification index calls Upgrade a reviewed permanent implementation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:33`). The pending ledger also calls it reviewed
production (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_pending.md:46-50`), while the completed ledger
records implementation, review, user acceptance, and historical gates
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1747-1815`).

Those are archive-era claims. The verified defects and missing consumer/native
proof below remain authoritative for reconstruction.

## Capability ownership

The reusable capability is one crash-recoverable transition between immutable
installed release artifacts:

```text
trusted prepared release
    -> durable candidate download
    -> independent byte and executable proof
    -> bounded product diagnostic
    -> durable launcher-selection activation
    -> witnessed normal startup
    -> committed current + retained rollback
    -> bounded cleanup
```

The archive describes that boundary directly
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:5-21`) and explicitly excludes release discovery,
entitlement, subscriptions, leases, gates, pricing, publishing, signing, and
release authorization (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:14-16`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:1297-1317`).

The reconstructed package should own:

- one exclusive typed `Installation` capability for one absolute root;
- bootstrap over installer-proved launcher and initial artifact facts;
- one package-owned installation `Scope`;
- sole authority ingress through a validated `update.PreparedRelease`;
- a package-owned transition generation, distinct from release publication
  generation;
- versioned immutable executable slots derived from typed build and artifact
  facts;
- a bounded O(1) artifact transfer into a create-only durable stage;
- independent re-verification of the staged executable;
- a bounded product diagnostic boundary with bounded output;
- canonical selection and journal documents with package-local, schema-derived
  ceilings;
- intent-before-effect activation and rollback;
- exact classification of prior, candidate, foreign, and indeterminate
  selection outcomes;
- a non-persistable launch witness bound to the exact selection and executable
  facts;
- a startup proof protocol that cannot be fabricated from public document
  fields;
- explicit, bounded recovery work after cancellation;
- current, rollback, quarantine, and active-candidate custody with fixed
  cardinality;
- explicit cleanup outcomes rather than best-effort invisible deletion; and
- stable Core-owned error identities for contract, verification, integrity,
  busy, download, diagnostic, activation, rollback, recovery, persistence,
  cleanup, conflict, and indeterminate results.

It should not own:

- release availability, authorization, or download-target issuance;
- account, payment, gate, lease, or product eligibility policy;
- product-specific diagnostic meaning or argv;
- launcher placement policy or operating-system installer UX;
- network transport or filesystem system calls already owned by lower
  Primitive packages;
- unbounded detached work;
- caller-invented startup success;
- release-domain generation for local journal ordering;
- legacy layout decoders, migration shims, aliases, or compatibility wrappers;
  or
- product-specific reporting and telemetry.

## Archive evidence

### Sole authority ingress through `update.PreparedRelease`

`PrepareRequest` accepts an `update.PreparedRelease`, validates it, validates
the supplied observation, and requires the prepared facts to carry that same
observation (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/types.go:137-152`). The specification prohibits
every alternate raw Release authority seam
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:43-50`).

This is the correct DAG boundary. Upgrade may retain immutable Release
vocabulary projected from Update, but it must not independently accept a
manifest, version, artifact, URL, or latest document as authority.

### Explicit installation capability and exclusive ownership

`Open` validates the request, acquires `filestore.OwnRoot`, maps a contended
root to `core.ErrUpgradeBusy`, and validates the selection/journal relationship
before returning the capability (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/open.go:11-49`).

The capability also serializes in-process operations and requires explicit
close. A concurrent call fails busy, while `Close` refuses to race an active
operation and closes retained stages and the root owner
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/open.go:120-175`).

The resulting ownership model is much stronger than product-local conventions
that write a lock filename and hope. It should survive reconstruction.

### Stable launcher selection instead of running-image replacement

The archive makes versioned executables immutable and changes only one small
canonical selection. `ResolveLaunch` loads and validates that selection,
requires its scope to match, derives the selected executable path, streams the
file through an integrity accumulator, checks executable facts, and returns a
private-witness `LaunchTarget` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/open.go:88-118`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/storage.go:148-175`).

The specification correctly says the stable launcher itself does not change
during ordinary upgrade and must execute the typed absolute target without
PATH lookup or a shell (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:344-400`). This avoids the
platform-specific hazards of renaming over the process that is currently
running.

### Journal-first bootstrap and replay

Bootstrap derives the installed artifact from the verified manifest and exact
installed build identity (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/bootstrap.go:66-100`). It proves the
initial versioned executable before installing control documents
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/bootstrap.go:127-149`).

Document installation reconciles exact expected bytes, rejects a
selection-without-journal partial state, writes the journal first, and then
writes the selection (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/bootstrap.go:157-215`). Exact replay is
idempotent, while divergent initialized state fails closed
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/bootstrap.go:226-249`).

That intent-first order is a genuine crash-recovery gem.

### Bounded O(1) transfer and independent byte proof

Download records a durable `downloading` intent before creating or writing the
candidate (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/download.go:27-48`). The candidate write stage is
bounded to the expected artifact extent, and Objectstore receives exact
SHA-256, extent, and CRC32C integrity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/download.go:51-104`).

The result is accepted only when the transfer reports the same direction and
all three integrity facts, the durable stage commit completed, and the
temporary object was removed (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/download.go:114-157`).
Verification later streams the durable candidate again rather than trusting
the transfer report (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/storage.go:148-175`).

The production implementation is streaming; artifact extent affects disk, not
heap growth. This is the required O(1) shape.

### Intent-before-effect activation and exact reconciliation

Activation proves the diagnosed candidate, removes the diagnostic workspace,
and persists an indeterminate activation intent before replacing the selection
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/activate.go:21-88`).

After the write attempt it always reads the actual selection and classifies it
as the prior value, the candidate value, or indeterminate/foreign. Candidate
records activated, prior records not-applied, and anything else preserves an
explicit reconciliation action (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/activate.go:91-140`).

This is materially stronger than treating a rename return value as the whole
truth. The same model applies to rollback.

### Closed recovery state machine and bounded retention

Each externally meaningful operation advances one named state and returns a
typed recovery action. Recovery does not scan a directory and guess. It loads
the canonical journal and dispatches from the durable state.

The package retains current plus one prior rollback/quarantine selection at
rest, with only one active candidate during transition. Cleanup uses a fixed
maximum of two targets rather than an unbounded slice. The specification makes
that bounded custody explicit (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:1313-1314`) and tests
the full cleanup-plan closure, including overflow and inactive-array residue
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:1243-1248`).

### Canonical, bounded control documents

The selection persists identity, manifest facts, artifact facts, generation,
and digest, but does not persist derived version directories or filenames
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:349-362`). Decode re-encodes and compares canonical
bytes, and the specification explicitly protects field order from
field-alignment rewrites (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:364-378`).

Unlike generic whole-document ceilings, the archive derives most selection,
journal, policy, and diagnostic maxima from typed field constants and nested
type maxima (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/upgrade_constants.go:6-103`). The ownership location
needs correction, but schema-derived bounding is worth preserving.

### Stable typed error identities

Core contains stable Upgrade identities for contract, verification, integrity,
busy, download, diagnostic, diagnostic panic, activation, rollback, recovery,
persistence, cleanup, conflict, and indeterminate outcomes, matching the
specification's prerequisite list (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:66-103`).

Callers can therefore classify errors with `errors.Is`. Reconstruction should
retain the identities while replacing raw-string operation labels with typed
site information.

### Hostile-proof ambition

The specification requires compiler failures, real temporary filesystems, real
Filestore stages, real Objectstore HTTP transfers, real child processes, fixed
Temporal facts, typed errors, and independent state/digest oracles. It rejects
fake filesystems, fake processes, sleeps, string-matched errors, raw JSON maps,
and green-only fuzzing (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:1271-1282`).

It also lists dependency, no-raw-path, no-OS-file, no-network, no-product,
no-map, no-world-build, shared-memory, public-surface, alias, compatibility,
and waiver ratchets (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:1265-1269`).

That proof posture belongs in the reconstruction, subject to the project-local
testing protocol and the additional blockers in this report.

### Primitive dependency graph

At archive HEAD, direct production imports are:

```text
upgrade
  -> core
  -> contextstate
  -> temporal
  -> filestore
  -> hostresource
  -> objectstore
  -> release
  -> update
```

The package also imports only standard-library byte, context, hashing, JSON,
error, formatting, I/O, arithmetic, filepath, conversion, and synchronization
facilities. It does not import `os`, `os/exec`, `net/http`, a cloud SDK, a
database, a product package, or another orchestration package.

Repository-wide archive inspection found no other Primitive production
package importing `upgrade`. It is a leaf orchestration package.

The intended semantic DAG is:

```text
core
  <- contextstate
  <- temporal

core
  <- filestore
  <- upgrade

core
  <- objectstore
  <- upgrade

core
  <- hostresource
  <- upgrade

core <- release <- update <- upgrade
```

The edge `release <- update <- upgrade` is important: Update is the authority
owner, while Release contributes immutable vocabulary. Upgrade must not add a
second authority edge accepting raw Release facts.

The archived direct `contextstate` edge should be reviewed. The specification
says Contextstate is reached only through Temporal's blocking-boundary
validation in Update (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:52-57`), but Upgrade directly
imports it. If Upgrade only needs validated blocking contexts, that behavior
should remain owned by Temporal and the direct edge should disappear.

## Consumer evidence

### Kernel

Kernel has no product self-update or `upgrade` import. Its committed Primitive
pin lacks the package; its dirty pin contains the archive implementation but no
Kernel production call site. Kernel is a server and command-suite repository,
so absence of use is meaningful: generic Upgrade must remain optional and must
not leak into application boot or server lifecycle merely because it exists.

Kernel nevertheless supplies two useful local gems:

1. Scout converts an absolute report path to a typed Primitive path, writes
   through `durability.Write` with an exact maximum, and invokes the returned
   recovery capability when the write fails
   (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:scout/report.go:51-73`).
2. Exodus persists a cursor with `durability.Write`, preserves the returned
   recovery action, and reads the result through `durability.ReadBounded`
   (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/cursor.go:138-165`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/cursor.go:237-256`).

Those uses support the archive's intent/effect/recovery split. They also expose
a design requirement: recovery is a typed capability produced by the durable
operation, not a raw function name, path convention, or assumption that an
error means nothing happened.

Kernel does not contribute installer policy, launcher layout, startup proof, or
an Upgrade call site. It cannot be cited as adoption evidence.

### Witness

Witness has no archived Upgrade dependency because its pin predates the
package. Its current `internal/updatecmd` is a complete product-owned update
transaction:

- `Execute` performs check, verification, download, candidate self-test,
  consent, replacement with rollback, installed self-test, and diagnostic
  reporting under a caller-held update lock
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/update.go:95-135`);
- a candidate is downloaded to a neighboring temporary file, streamed through
  SHA-256, bounded to exactly one byte past signed size, fsynced, closed, and
  returned only after exact extent and digest match
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/update.go:397-451`);
- candidate self-test executes the exact absolute candidate path with
  compiler-owned argv, no shell, an empty environment, a timeout, and bounded
  output (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/selftest_exec.go:13-52`,
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/update.go:462-479`);
- the self-test is bound to product, version, commit, platform, binary digest,
  pinned release key, pinned server key, and passed status
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/update.go:482-503`);
- replacement creates a hard-link backup, renames the candidate over the
  installed binary, syncs the directory, tests the installed path, and restores
  the backup plus directory sync after a post-swap failure
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/update.go:552-624`); and
- one bounded diagnostic slot preserves the first undelivered failure and only
  deletes the exact diagnostic after a verified server receipt
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/store.go:17-23`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/store.go:52-87`,
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/diagnostic.go:29-69`).

Witness's most important gem is the self-test binding. The candidate is not
accepted merely because some process exited zero. Its bounded typed result must
identify the exact signed target and pinned authorities. Reconstructed Upgrade
should require a similarly exact diagnostic result or explicitly name a weaker
result as caller-reported.

Witness also shows what should remain outside generic Upgrade:

- explicit operator consent (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/update.go:505-519`);
- Witness-specific product and key identities;
- diagnostic delivery to the product control plane; and
- CLI output and requested-release flags.

Witness's current in-place replacement is not itself the target architecture.
The stable-launcher selection protocol eliminates the open-running-image and
rollback hazards that this local code must manage.

### Bug

Bug also predates Upgrade and contains an independent product-local updater
rather than importing Witness.

Its flow streams the candidate under a request timeout, limits the reader to
signed extent plus one, hashes while writing, rejects truncation and padding,
fsyncs, and closes before acceptance
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:377-392`, `bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:428-464`). It then executes a bounded self-test
and binds the result to product, target version, commit, platform, executable
digest, release key, server key, and passed status
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:478-528`).

Bug's replacement creates a sibling backup, renames the candidate over the
installed executable, syncs the directory, self-tests the installed path, and
restores the backup when post-replacement proof fails
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:582-644`).

The independent implementation corroborates the common capability:

- exact bounded download;
- pre-install diagnostic;
- durable activation;
- post-install proof;
- retained rollback;
- typed phase and rollback reporting; and
- product-owned consent and diagnostics.

Bug also corroborates the need for centralization: Witness and Bug carry nearly
parallel updater code, but neither could adopt archived Upgrade because their
pins predate it and the archive never proved a cutover.

Bug-specific CLI prompting, product keys, control-plane diagnostics, and
release-request UX remain in Bug. The reconstructed Primitive package should
replace only the common installation state machine.

### Peachfuzz

Peachfuzz has no updater, no Upgrade import, and no evidence that a daemon
should self-modify. Its absence establishes that Upgrade must be an explicit
composition-root choice, not an automatic Primitive concern.

Peachfuzz does contain the strongest consumer evidence for cancellation-safe
bounded cleanup:

- daemon shutdown is a typed Primitive `shutdown.Plan` with a 30-second
  per-step budget and 60-second total budget
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/shutdown.go:13-44`);
- the daemon joins worker and shutdown outcomes and preserves cancellation
  identity (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/loop.go:73-102`);
- scheduler close detaches from parent cancellation but immediately adds a
  finite timeout (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/loop.go:464-468`); and
- post-process evidence custody uses the same
  `WithTimeout(WithoutCancel(...))` shape
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/cycle.go:224-230`).

That is the local gem archived Upgrade is missing. Cancellation-detached
durability work may be necessary, but it must always be bounded by a typed
budget. Bare `context.WithoutCancel` is not an acceptable execution contract.

Peachfuzz also demonstrates product-owned shutdown sequencing. Upgrade should
not import product shutdown policy; a product diagnostic may compose Shutdown
behind a narrow validated boundary.

## Strong mechanics and proof

The four consumers establish these conclusions:

| Requirement | Kernel | Witness | Bug | Peachfuzz | 2026 owner |
| --- | --- | --- | --- | --- | --- |
| Typed durable write plus recovery capability | Production use | Local durability | Local durability | Durable stores | `filestore`/durability primitive |
| Exact bounded O(1) download | No updater | Production updater | Production updater | No updater | `objectstore` + `upgrade` orchestration |
| Exact product self-test identity | No updater | Product/version/commit/platform/digest/key proof | Same independent pattern | Product runner diagnostics | Product result type + Upgrade binding |
| Explicit consent | Not applicable | Yes | Yes | Not applicable | Consumer |
| Crash journal and stable launcher selection | No use | Missing | Missing | Missing | `upgrade` |
| Rollback after post-activation failure | Recovery pattern only | In-place backup restore | In-place backup restore | Shutdown recovery patterns | `upgrade` |
| Bounded cancellation-detached work | Recovery invoked | HTTP/report timeouts | HTTP/self-test timeouts | Strong explicit pattern | `temporal`/`shutdown` policy + `upgrade` |
| Product diagnostics/reporting | Not applicable | Durable signed-receipt slot | Product update report | Product logs/evidence | Consumer |

The archive adds the capability none of the consumers currently has: an
installation-wide crash journal and stable launcher-selection transaction.
The consumers add proof the archive lacks: real product self-test binding and a
finite budget around cancellation-detached cleanup.

There is no compatibility requirement because no inspected consumer ever
compiled against Upgrade. Reconstruction can be a clean contract with
coordinated new call sites.

## Defects and blockers

### 1. Bootstrap does not prove launcher identity

The specification says the installer places the stable launcher and Bootstrap
proves the installer-prepared root
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:414-422`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:1308-1312`).

Production derives the expected launcher path and calls `proveLauncher`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/bootstrap.go:127-149`). That proof only:

- reads the named file through Filestore;
- caps it at `core.ReleaseArtifactByteMaximum`;
- requires a complete, nonzero read; and
- checks executable mode where applicable
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/storage.go:178-203`).

It has no expected digest, extent, BuildIdentity, launcher revision, or trusted
manifest. Any nonempty executable file with the expected mode at the expected
name passes. Bootstrap will then publish selection and journal documents that
delegate all future launches to that unproved executable.

The filename and executable bit are an informal naming contract, not
compiler-owned launcher identity. This is an admission blocker.

Required correction:

- define a typed `LauncherArtifact` or `VerifiedLauncher` owned by the installer
  protocol;
- bind it to exact digest, extent, platform, launcher contract revision, and
  installed scope;
- validate it at Bootstrap ingress;
- stream and prove the actual launcher bytes against those facts; and
- hostile-test wrong executable, correct name/wrong bytes, wrong platform,
  truncation, padding, link/reparse substitution, and replay.

The installer remains responsible for placing bytes. Upgrade is responsible
for refusing to initialize control state over unproved bytes.

### 2. Startup observation is forgeable from public facts

`StartupRequest` is a public struct containing only selection digest, build
identity, observation, and outcome
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/types.go:210-227`).

`ObserveStartup` loads the journal and accepts the request when:

- request identity equals candidate and current identity;
- request selection digest equals the current selection digest; and
- observation is not before activation
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/startup.go:10-40`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/startup.go:62-75`).

It then records `StartupStarted`, followed later by caller-supplied
`StartupSucceeded`, `StartupFailed`, or `StartupIndeterminate`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/startup.go:78-138`).

Every equality fact is available in the launch target or persisted selection.
Any caller that can read those facts can claim Started and Succeeded without
having executed the stable launcher, the selected binary, or a normal product
startup. The type proves internal consistency, not occurrence.

This contradicts the value proposition that Upgrade observes one normal
startup (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:9-12`). It is an admission blocker.

Required correction:

- `ResolveLaunch` must mint a private, non-persistable `LaunchAttempt` witness
  bound to installation scope, exact selection digest, exact executable facts,
  and a one-use attempt identity;
- the stable launcher must pass an unforgeable or capability-carrying startup
  token into the exact selected process through a reviewed channel;
- startup reports must consume that typed attempt rather than reconstruct its
  fields;
- terminal success must bind to the same attempt and the product's explicit
  readiness contract; and
- replay, process restart, stale selection, copied token, foreign executable,
  pre-activation time, and success-without-start must red-fail.

If cross-process unforgeability is deliberately out of scope, the API and state
must be renamed to `ReportStartup` and `StartupReportedSucceeded`; Primitive
must not claim proof it does not possess.

### 3. Cancellation-detached durability work is unbounded

The package uses bare `context.WithoutCancel(ctx)` throughout verification,
download abort/reset, diagnostic workspace/reset/commit, activation
classification, startup persistence, rollback, recovery, abort, cleanup, and
stage recovery. Representative sites include:

- `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/download.go:63-76`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/download.go:105-121`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/download.go:140`;
- `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/activate.go:103-120`;
- `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/startup.go:35-38`; and
- `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/recover.go:66-137`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/recover.go:178-201`.

`Policy` contains retained disk reserve, diagnostic budget, and diagnostic
output limit, but no persistence, reconciliation, cleanup, or total recovery
budget (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/types.go:68-85`).

Detaching cleanup from a cancelled caller can be correct. Detaching without a
deadline means a blocked Filestore stage, filesystem operation, or hostile
implementation can prevent the operation from returning forever. That violates
bounded execution and makes shutdown behavior unknowable.

Peachfuzz shows the correct consumer pattern:
`context.WithTimeout(context.WithoutCancel(parent), budget)`
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/loop.go:464-468`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/cycle.go:224-230`).

Required correction:

- add typed nonzero persistence, recovery, and cleanup budgets at their owning
  policy boundaries;
- centralize bounded detach in Temporal rather than repeating raw context
  composition;
- ensure every detached context is cancelled;
- distinguish deadline, cancellation, indeterminate, and persistence
  identities with `errors.Is`; and
- hostile-test dependencies that never return until context cancellation.

### 4. Local transition ordering uses `release.Generation`

`release.Generation` is declared in Release and reports errors labeled
`release.Generation` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/values.go:70-85`). Upgrade nevertheless
uses it for local selection and journal transition ordering:

- Bootstrap creates `release.NewGeneration(1)`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/bootstrap.go:103-124`);
- activation advances that generation before sealing a new local selection
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/activate.go:59-88`); and
- startup advances it again for a local journal transition
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/startup.go:147-162`).

A release publication/authorization generation and a local installation
journal generation are different semantic identities. Reusing the Release type
only because both contain a positive `uint64` creates a false cross-package
contract and gives local corruption the wrong error owner.

Required correction:

- define package-owned `TransitionGeneration` with private representation,
  constructor, `Validate`, checked increment, canonical encoding, and typed
  overflow error identity;
- use Release generation only where a retained authority fact genuinely carries
  it; and
- add compile-negative substitution tests between release and installation
  generations.

This is a clean upgrade. No alias or compatibility wrapper is permitted.

### 5. Error sites are raw string protocols

Every local error helper accepts `label string` and formats it through a shared
format string (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/errors.go:10-59`). Call sites pass literals such
as `"Open"`, `"Download.stage"`, `"Activate.selection"`,
`"ObserveStartup.identity"`, `"journal.JSON"`, and many others
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/open.go:20-27`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/download.go:58-77`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/activate.go:95-120`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/startup.go:62-75`).

Core-owned sentinel identity survives wrapping, which is good. The operation
and failure-site dimension does not. Typos, duplicate spellings, renames, or
changed site relationships do not break the build, and callers cannot inspect
them with `errors.As`.

Required correction:

- retain stable Core-owned sentinel identities;
- define typed package-owned `Operation` and `FailureSite` enums, or concrete
  typed errors that expose them;
- validate every typed error before external return;
- use `errors.Is` for stable identity and `errors.As` for structured context;
  and
- prohibit tests that match rendered error text.

Human-readable `Error()` text remains diagnostic output, never a protocol.

### 6. Layout assembly contains raw relative string conventions

`relativeVersionSlot` concatenates a directory constant, `"/"`, and a version
string into a raw string. `childFile` accepts any `relative string`, validates
it as a token, joins it to the root, and parses the result back into a typed
absolute path (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/layout.go:9-18`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/layout.go:49-80`).

The final path is typed, but the important intermediate relationships, such as
the version-slot path for an identity or the control file at a root, are loose
string protocol. The compiler cannot distinguish a version slot, control
document, diagnostic output, or arbitrary relative path.

Required correction:

- define typed package-owned layout facts such as `VersionSlot`,
  `ControlDocumentPathSet`, and `InstallationPathSet`;
- derive them through constructors from typed root, platform, build identity,
  artifact filename, and path-contract constants;
- make impossible path substitutions fail to compile;
- validate containment and no-link/reparse rules at the owning path boundary;
  and
- expose only the truly shared stable-launcher path contract through focused
  Core path contracts.

No generic `childFile(root, string)` escape hatch should remain.

### 7. Core owns Upgrade-local implementation details

`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/upgrade_constants.go:4-105` contains:

- cleanup maximum count;
- diagnostic output filename;
- diagnostic report, journal, policy, scope, and selection field counts and
  maximum byte calculations;
- document revision token;
- enum JSON maximum;
- installation namespace reserve;
- journal and selection filenames;
- launcher name and Windows suffix; and
- transition and version directory names.

Some of these may be shared with a separately built installer or stable
launcher. Most are private implementation details of one package. The current
doctrine says Core owns shared and cross-package invariants, not every constant.

Two details are especially weak:

- `UpgradeEnumJSONMaximumBytes = 32` is an unexplained fixed ceiling rather
  than a maximum derived from the compiler-owned enum tokens
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/upgrade_constants.go:23-24`); and
- document field counts and local diagnostic/workspace filenames do not need
  cross-package ownership.

Required correction:

- keep stable error identities in `core/error_identity.go`;
- keep only installer/launcher-shared path names and protocol revision in a
  focused `core/path_contracts.go` or a dedicated shared typed contract;
- keep document schemas, field counts, enum maxima, cleanup cardinality,
  diagnostic paths, and local reserve calculations in focused Upgrade files;
- derive every enum ceiling from the actual typed token set; and
- ratchet against copied constants in consumers.

Everyone may import Core. Packages must not import each other merely to share a
literal, but that rule does not turn package-private implementation details into
global Core contracts.

### 8. Diagnostic success is only as strong as a caller behavior interface

The specification allows a product `DiagnosticRunner` and says Upgrade owns
only the ordered state machine (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:59-64`). Production
bounds output, contains panic, records start/end observations, persists output
integrity, and verifies candidate bytes again. Those are strong controls.

However, Upgrade cannot independently prove that an arbitrary runner actually
executed the selected binary or that passed means product readiness. A runner
can return structurally valid success without starting a process. Archive tests
using a real child process prove one implementation, not every caller.

Witness and Bug show the missing binding: their result includes exact product,
version, commit, platform, binary digest, pinned authority identities, and
passed status (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/update.go:482-503`,
`bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:502-528`).

Required correction:

- decide the semantic contract explicitly;
- either accept a product-owned typed `DiagnosticProof` bound to candidate
  BuildIdentity, artifact integrity, selection/attempt identity, and bounded
  observations;
- or name the state `diagnostic-reported-passed` and avoid claiming independent
  process proof;
- preserve bounded output and timeout at the execution owner; and
- keep product-specific checks and authority identities in consumers.

This is distinct from the startup blocker: pre-activation diagnostic may be an
explicitly trusted product capability, while post-activation startup is the
event Upgrade claims to observe.

### 9. No consumer cutover proves the launcher protocol

All four committed consumer pins predate the package, while Kernel's dirty pin
contains it without a production import. Witness and Bug still replace their
running executable paths directly. Kernel and Peachfuzz have no self-updater.

Therefore the archive has no end-to-end production proof that:

- an installer lays out the stable launcher and first version correctly;
- the separately built launcher consumes `ResolveLaunch` correctly;
- argv, environment, stdio, exit status, and signal forwarding work;
- the selected product reports a real startup through the intended boundary;
- startup failure causes durable rollback;
- a launcher/process crash at every journal edge is recoverable; or
- a product release can be cut over without compatibility code.

Package tests are necessary but cannot substitute for a real installer,
launcher binary, and at least one migrated consumer.

Required admission proof includes one clean Witness or Bug cutover behind a
new installation layout, with the old in-place updater removed rather than
wrapped.

### 10. Required native filesystem proof was not reproduced

The specification requires native evidence on macOS/arm64 with APFS,
Linux/amd64 and Linux/arm64 with the documented filesystem matrix, and
Windows/amd64 with NTFS and ReFS where available
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:1284-1290`).

It explicitly says cross-builds are insufficient for activation, selection
durability, executable mode, open-running-file behavior, rename/replace,
directory sync, link/reparse rejection, ownership locking, process shutdown,
and rollback (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/SPEC.md:1292-1295`).

The completed ledger records clean test, race/shuffle, vet, static analysis,
cyclomatic complexity, security, dependency, and platform build gates
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1812-1815`). It does not provide reproducible
native APFS, Linux filesystem, NTFS, and ReFS run artifacts at the inspected
revision.

The local reconstruction interview reran only package-level host gates. It did
not reproduce the required native matrix. Admission remains blocked until
native evidence is attached to the exact source revision, platform, and
executed command set.

### 11. The archive's 1,247-line document file weakens ownership locality

`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/documents.go:1-1247` is 1,247 lines and contains selection, journal,
policy, diagnostic, enum-presence, canonical encoding, decoding, and
transition-shape responsibilities.

The file may pass cyclomatic complexity while still making ownership difficult
to inspect. The current doctrine asks for focused files and aggressive
single-purpose splitting.

Reconstruction should separate:

- selection document and canonical codec;
- journal document and canonical codec;
- policy wire contract;
- diagnostic report wire contract;
- transition shape validation;
- cleanup plan persistence; and
- schema-derived ceilings.

This is not a standalone semantic blocker, but it is a required implementation
ratchet. Splitting must not duplicate field constants, revision tokens, or
canonical ordering.

### Mechanical gate evidence at the inspected archive

The following commands were run against the read-only archived source at
`d046f7b675fcb797398d7cdc87b5504f43978056`:

| Gate | Result |
| --- | --- |
| `go test ./upgrade` | PASS (`9.126s`) |
| `go test -race -shuffle=on -count=2 ./upgrade` | PASS (`19.835s`) |
| `go vet ./upgrade` | PASS |
| `staticcheck ./upgrade` | PASS |
| production-only `gocyclo -over 10` | PASS; no findings |

These results establish that the inspected archive compiles and its package
tests are mechanically green on the local host. They do not discharge the
semantic defects, consumer cutover, or native matrix.

No full-module gate was claimed in this interview because the assignment was a
read-only package reconstruction report and the archive contains unrelated
working-tree state.

## Primitive 2026 ownership and DAG

### Admit to `upgrade`

Admit the following after redesign:

- installation `Scope`;
- package-owned transition generation;
- exclusive `Installation`;
- bootstrap document transaction;
- version-slot and installation path-set types;
- prepared-release projection;
- candidate custody;
- O(1) download orchestration;
- independent durable executable proof;
- diagnostic request/result binding;
- canonical selection;
- canonical crash journal;
- activation and rollback intent/effect/reconciliation;
- launch attempt witness;
- startup report/proof transition;
- recovery planning and execution;
- fixed-cardinality cleanup;
- package-local schema ceilings;
- typed operation/site errors; and
- package-local dependency and public-surface ratchets.

### Admit to `core`

Core should own only genuinely shared invariants:

- stable `ErrUpgrade*` identities;
- exact stable-launcher and installation control-path contracts shared by the
  installer, launcher, and Upgrade;
- shared file-mode constants;
- shared build, platform, digest, extent, CRC32C, and absolute-path types; and
- shared canonical JSON field constants only where multiple packages genuinely
  exchange the same wire vocabulary.

Core should not own Upgrade's private:

- cleanup cardinality;
- local enum maximum;
- diagnostic workspace/output name;
- selection or journal field counts;
- policy/selection/journal maximum formulas;
- transition state names;
- local operation sites; or
- installation transition generation.

### Preserve in lower Primitive packages

- `update` owns release availability, authority verification, and
  `PreparedRelease`.
- `release` owns immutable release artifact, manifest, filename, integrity, and
  authority generation vocabulary.
- `objectstore` owns exact bounded transfer and provider integrity.
- `filestore` owns rooted no-link filesystem access, exclusive root ownership,
  durable stages, activation, sync, verified read facts, and bounded removal.
- `hostresource` owns typed capacity observation.
- `temporal` owns validated instants, durations, deadlines, context
  classification, and a typed bounded-detach constructor if one is shared.
- `shutdown` owns generic bounded step execution where a product diagnostic
  needs lifecycle-aware cleanup.

Upgrade orchestrates these facts. It must not duplicate their system calls,
error strings, constants, or validation rules.

### Keep in consumers

Consumers own:

- whether and when to offer or require an update;
- explicit operator consent;
- installation-root selection and installer UX;
- product diagnostic argv and readiness meaning;
- product authority key configuration;
- startup readiness point;
- CLI text and requested-version UX;
- telemetry, signed diagnostic delivery, and customer messaging;
- rollout, rollback, and retry policy above one local transition; and
- service-manager integration.

Witness and Bug should delete their duplicate in-place installer paths only
after a clean cutover. No shim should preserve both.

### Required DAG

```text
                         core
          ________________|________________
         |          |          |           |
     temporal   filestore  objectstore  hostresource
         |          |          |           |
         |          +-----+----+-----------+
         |                |
      release ---------- update
         |                |
         +---------+------+
                   |
                upgrade
                   |
        product composition root
          /       |       \
     installer  launcher  application
```

Clarifications:

- the installer and launcher may share typed Core path/protocol contracts, but
  they do not import Upgrade merely for constants;
- the launcher calls the narrow resolution/attempt boundary and never decodes
  selection JSON itself;
- Update remains the only release-authority ingress;
- Upgrade never imports a product package;
- products may import Upgrade and lower packages, but products do not become
  shared-contract owners; and
- no consumer-to-consumer edge is admitted.

### Required admission proof

Before this report can become complete, the reconstructed package must provide:

1. A written 2026 contract with the ownership and DAG above.
2. A typed installer-to-Bootstrap launcher proof carrying exact identity,
   digest, extent, platform, and protocol revision.
3. A package-owned transition generation with compile-negative proof against
   `release.Generation`.
4. Typed layout structures with no generic relative-string assembly.
5. Typed operation/site errors preserving Core sentinel identity through
   `errors.Is` and structured context through `errors.As`.
6. O(1) download and re-verification with hostile truncation, padding,
   corruption, premature EOF, cancellation, and indeterminate commit tests.
7. A finite typed budget around every cancellation-detached persistence,
   cleanup, recovery, and reconciliation operation.
8. A startup protocol that either proves the exact launch attempt or honestly
   names the input as a caller report.
9. A product diagnostic contract bound to exact candidate facts, with product
   semantics left outside Primitive.
10. Exhaustive state/action/transition adjacency and restart tests at every
    intent/effect boundary.
11. Canonical selection and journal codecs with schema-derived ceilings,
    widest-value tests, field-order ratchets, and no loose maps.
12. Fixed-cardinality retention and cleanup proofs, including partial cleanup,
    replay, duplicates, foreign targets, and overflow.
13. Dependency, public-surface, no-raw-path, no-OS-file, no-network,
    no-product, no-map, no-world-build, alias, compatibility, waiver, gocyclo,
    and coupling ratchets.
14. Stable launcher integration tests covering exact path execution, no shell,
    no PATH lookup, argv/env/stdin/stdout, signals, exit status, and stale or
    foreign selections.
15. A clean Witness or Bug cutover that removes the old in-place updater
    rather than wrapping it.
16. Native APFS, Linux amd64/arm64 filesystem, NTFS, and ReFS evidence tied to
    the exact source revision, platform, and executed command set.
17. All project-local canonical, race/shuffle/repeat, vet, static analysis,
    security, complexity, dependency, witness, and sentinel gates.
18. Independent review after the implementation and evidence are complete.

Tests must follow `foundation@working-tree:_docs/testing_protocol.md:1-12`: typed fixtures,
hostile boundaries, independent oracles, real compiler failures, real
filesystem and process proof where required, no string-matched errors, no
sleep-based synchronization, and no implementation-coupled wrapper
assumptions.

## Decision rationale and conditions

The capability requires redesign before admission.

Admit the capability because:

- two independent consumers already implement its core field behavior;
- stable launcher selection is safer and more portable than their in-place
  replacement;
- the archive has a strong authority boundary, streaming transfer, canonical
  persistence, intent-first activation, reconciliation, rollback, and bounded
  retention design; and
- the capability composes cleanly as a leaf above existing Primitive
  primitives.

Reject the archive as-is because:

- launcher authenticity is not proved;
- startup occurrence and success are forgeable caller assertions;
- cancellation-detached critical work is unbounded;
- installation ordering borrows a Release-owned generation type;
- important error and path relationships remain raw string protocols;
- Core contains substantial package-local implementation policy;
- diagnostic success is semantically weaker than the package language implies;
- no consumer cutover proves the architecture; and
- the required native filesystem matrix is absent from reproducible evidence.

No compatibility debt is justified. No inspected consumer ever compiled
against Upgrade, so the 2026 implementation can use the correct types and
update real call sites directly.

The recon report is complete. The capability remains unadmitted until the
reconstructed implementation, consumer cutover, native evidence, canonical
gates, and fresh implementation review are complete.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
