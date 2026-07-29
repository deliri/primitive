# Unleash package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole recon report for archived design directory `unleash`. The
directory is not a buildable Go package. It contains one specification and no
implementation. The report therefore distinguishes reviewed design intent from
implemented evidence throughout.

## Evidence boundary


### Archive

- Repository: `/Users/d/code/foundation_back_up_july_27th_2026`.
- Committed HEAD: `d046f7b675fcb797398d7cdc87b5504f43978056`.
- Immutable tag: `foundation-backup-2026-07-27`.
- The committed `unleash/` tree contains only `unleash/SPEC.md`. No `.go` file,
  test, fuzz target, generated implementation, or production entry point exists.
- The specification first appeared in
  `605c9030ba27bb27ee18a9b60a7855dda96dbcb8` and was later amended for Core,
  Filestore, Objectstore, and Release ownership.
- The archive ledger truthfully labels the state as a reviewed specification
  with implementation pending and says no capability is claimed
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1614-1647`,
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_pending.md:46-53`).

The archive itself is clean for this evidence. Its unrelated untracked
`core/api_http_boundary_hostile_test.go` does not affect `unleash`.

### Consumer revisions and pins

| Consumer | Committed HEAD | Committed Primitive pin | Working tree qualification |
| :-- | :-- | :-- | :-- |
| Kernel | `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | `0df2954a2d91` | `go.mod` is dirty and advances Primitive to `e8b7172161a4`; examined `cmd/lfw/build.go` is unchanged from HEAD |
| Witness | `b9629af57b7058b68982be5d3b282be440b1e76e` | `773add8ba0fc` | examined production files are unchanged from HEAD; `.ledger_pending.md` is untracked |
| Bug | `39ce96242240d7174d562c90bb255860946595dc` | `388e593231a2` | examined production files are unchanged from HEAD; `.ledger_pending.md` is untracked |
| Peachfuzz | `2b2d080c455edaadf88502c1c253845605a4336a` | `3f74d8fc35b4` | examined production files are unchanged from HEAD; `.ledger_pending.md` is modified |

No Go file in Kernel, Witness, Bug, Peachfuzz, or the archive imports
`github.com/deliri/primitive/v2026/unleash`. No pinned Primitive revision
can provide an importable package because even the July 27 archive contains
only the specification.

Consumer ledger statements are useful plans, but dirty or untracked ledger
statements are identified as working tree evidence rather than committed
production facts.

## Capability ownership

### Archived intent

The archived specification describes a complete release construction and
publication coordinator for one Go command. Its intended responsibilities are:

- verify one signed construction authorization;
- project one exact cross compilation policy;
- materialize one exact source commit in an isolated workspace;
- construct and inspect the fixed Release target set;
- retain crash recoverable target and publication state;
- ask Release to construct and verify Artifact and Manifest facts;
- verify signed publication capabilities;
- publish artifacts and Manifest with create only semantics; and
- reconcile replay, conflict, and ambiguous provider commitment.

The purpose and top level exclusions are explicit
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:6-22`).

### Intended nonownership

The useful nonownership boundary is:

- Core owns shared build identity, version, platform, digest, path, error, and
  protocol values.
- Attest owns signature envelope mechanics.
- Temporal owns trusted instants and arithmetic.
- Garble owns tool version and seed derivation mechanics.
- Release owns target order, Artifact, ArtifactSet, Manifest, and verification.
- Filestore owns root confined durable files, readers, transitions, and
  activation.
- Objectstore owns provider transfer and provider outcome identity.
- The caller owns product gates, source authorization policy, signer policy,
  bucket and object naming, channel selection, Latest advancement, and receipt
  acceptance.

The archive dependency list mostly reflects that direction
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:24-81`).

### Primitive 2026 boundary

Primitive 2026 should preserve a narrower `unleash` coordinator only if it
owns the relationship among already verified capabilities. It must not become
a second owner of process execution, Release facts, filesystem transitions,
object transfer, or product policy.

## Archive evidence

### Specification only

`unleash/SPEC.md` declares nine package operations and five workspace
operations, but none exists as Go code
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:83-110`). Its state types, request types,
closed results, persistence records, platform implementations, and error
producers are therefore design claims, not compiler visible contracts.

The required hostile proof section is extensive, but it is also unimplemented.
It requires exact command and environment projection, real Git isolation, four
real cross builds, binary inspection, crash subprocesses, authorization
renewal, signed publication grants, provider ambiguity, fuzzing, and
architecture ratchets
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:832-988`).

### Implemented adjacent prerequisites

Core implements a private representation `BuildIdentity`, validates every
owned field, and exposes `EmbeddedIdentity` without importing a lifecycle
package
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/build_identity.go:31-85`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/build_identity.go:128-154`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/build_identity.go:241-279`).

Release implements:

- an Artifact that binds target, integrity, authorization, and BuildIdentity;
- a fixed target set in Windows AMD64, Darwin ARM64, Linux AMD64, Linux ARM64
  order; and
- an ArtifactSet that rejects missing order, duplicate artifact identity, and
  cross slot build divergence.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/values.go:194-281` and `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/values.go:283-389`.

Core also contains ten `ErrUnleash*` identities
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:125-134`) and a large set of
Unleash named tokens, bounds, environment names, build flags, and state tokens
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/unleash_constants.go:3-63`). Those declarations prove
that adjacent preparation occurred. They do not prove an Unleash producer or
consumer.

### Archive internal use

There is no implemented Primitive importer. Release, Update, and Upgrade
specifications refer to Unleash only to define dependency direction. Update and
Upgrade explicitly prohibit importing it. Core architecture tests permit
future `unleash` process ownership, but no production file exercises that
allowance.

The later capability freeze audit supersedes part of the original ownership
plan. Its four consumer comparison finds an independently repeated need for a
generic Primitive process capability and states that Unleash should be a
future composer of Process, not the generic process owner
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:205-245`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:311-339`).

## Consumer evidence

### Kernel

Kernel has no Unleash import and no complete four target command release
lifecycle. Its current distribution path is a weaker product build flow:

- three product service commands perform product frontend preparation;
- each calls one Garble build;
- Garble and UPX are resolved through bare PATH lookup;
- the Garble seed is random;
- UPX may mutate the built binary;
- output proof is limited to streaming SHA256 and extent; and
- no executable format, Go build information, source revision, or embedded
  BuildIdentity inspection occurs.

See `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/build.go:821-929`. The streaming hash itself is a
valid bounded mechanic
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/build.go:931-945`).

Kernel's untracked ledger schedules later composition only after real tool
identities, repositories, authorization, publication grants, and object
targets exist. It then requires four target construction, binary inspection,
Manifest sealing, five immutable publications, ambiguity reconciliation, and
native Update and Upgrade proof
(`kernel@working-tree:.ledger_pending.md:257-272`).

Kernel product frontend preparation, service selection, optional compression,
App Engine deployment, terminal messages, and channel policy stay downstream.
Bare tool lookup, random release seed, ambient process authority, and hash only
inspection are retirement evidence.

### Witness

Witness has a real product release command and independently proves demand for
most of the intended capability:

- it resolves a deterministic Garble seed, exact commit, toolchain, gate
  evidence, and target set;
- it validates a release plan;
- it serially builds every target;
- it binds signed tool bundle attestations to platform and commit;
- it constructs and signs a Manifest; and
- it retains custody and download index evidence.

See `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-release/main.go:50-138`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-release/main.go:165-217`, and `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-release/main.go:263-299`.

Witness preflight rejects a dirty tree, commit mismatch, missing seed, random
seed, invalid toolchain, and invalid gate evidence
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_preflight.go:13-90`).
Its typed build request projects commit, deterministic seed, build tags,
linker policy, target, output, and import path
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_build.go:11-73`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_build.go:92-149`).

The current execution path still invokes `exec.CommandContext` directly and
inherits ambient environment through `os.Environ`
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-release/main.go:219-227`).
Its product tool bundles, self tests, gate evidence, vulnerability snapshot,
custody record, signer selection, and native smoke policy remain Witness owned.
Its local build, Manifest, download, and release protocol copies should retire
only after one complete Primitive cutover.

### Bug

Bug contains the strongest implemented mechanics relevant to the missing
package.

The build pipeline defines typed and validated Builder, Inspector, Gate, input,
and result boundaries
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/pipeline.go:20-161`). It builds each planned
request serially, inspects before deriving an Artifact, hashes through a bounded
reader, synchronizes the artifact and directory, constructs a Manifest, signs
it, and reads it back for verification
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/pipeline.go:198-234`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/pipeline.go:296-392`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/pipeline.go:395-460`).

Its binary inspector:

- distinguishes Mach O, ELF, and PE;
- validates executable kind and target architecture;
- rejects forbidden symbol or debug sections;
- enforces positive bounded regular files; and
- performs a bounded streaming search for embedded release stamps.

See `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/inspection.go:17-67`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/inspection.go:69-139`, and `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/inspection.go:142-230`.

Bug deployment verifies signed Manifest and plan binding, asks its authority for
signed upload targets, rechecks every on disk artifact against the Manifest,
uploads, and verifies the final response
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:19-112`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:143-235`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:238-275`).

Its FactSource boundary resolves commit, clean state, tool versions, module
provenance, vulnerability facts, and signing facts before plan construction
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/facts.go:34-71`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/facts.go:92-170`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/facts.go:196-250`).

Two mechanics must not survive:

- BuildEnvironment inherits nearly the entire parent environment and replaces
  only GOOS, GOARCH, and CGO
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/build_environment.go:10-39`).
- Any build failure removes the whole release root instead of retaining a
  durable pending state for crash reconciliation
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/pipeline.go:176-195`,
  `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/pipeline.go:245-274`).

Bug gates, product identity, vulnerability policy, signer acquisition, command
UX, and control plane route selection stay downstream.

### Peachfuzz

Peachfuzz has no release builder, but it provides two adjacent mechanics.

Its worktree manager validates typed lease requests, grants exclusive RunID
ownership, prepares a detached exact commit, writes custody, and cleans residue
before reuse
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/worktree/worktree.go:30-152`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/worktree/worktree.go:171-232`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/worktree/worktree.go:251-280`).

This is requirement evidence, not an implementation to copy. Peachfuzz uses Git
linked worktree bookkeeping and force cleanup. Archived Unleash instead requires
an independent repository with no shared object directory, alternates, hard
links, or caller worktree registry
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:301-325`).

Peachfuzz computes SHA256, CRC32C, and extent in one bounded streaming pass
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/blob_upload.go:19-41`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/blob_upload.go:76-134`). Its `VerifyBlob` streams read back into fixed digest state and
compares the complete descriptor
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs.go:450-482`).

Its create only path treats provider status 412 as immutable conflict without
first proving byte equality
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs.go:240-271`).
Its upload helper may automatically retry after a failed body transmission
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs.go:432-447`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs.go:496-519`). Those semantics are unsafe for ambiguous immutable publication.

Peachfuzz bucket policy, object naming, retention, archive lifecycle, daemon
worktree pooling, and fuzz run evidence remain Peachfuzz owned.

## Strong mechanics and proof

### Shared construction mechanics

The cross consumer evidence supports these product neutral requirements:

- exact source commit authority;
- deterministic Garble custody and target separated seed derivation;
- absolute executable capabilities with identity recheck;
- closed arguments and an allowlisted child environment;
- serial target construction with one truthful observation per target;
- bounded stdout and stderr with owned cancellation and wait paths;
- no shell and no arbitrary hook;
- native PE, Mach O, and ELF inspection;
- Go build information, VCS revision, settings, and embedded BuildIdentity
  verification;
- one streaming pass for extent, SHA256, and CRC32C; and
- activation only after inspection succeeds.

The archived specification captures these mechanics at
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:270-415` and `archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:417-452`.
Bug supplies the strongest current binary inspection implementation. Witness
supplies the strongest current product gate and tool bundle provenance evidence.
Kernel supplies a simple streaming artifact hash. Peachfuzz supplies exact
commit leasing and streaming read back.

### Bounded state and time

The archived fixed state model has four target slots and one Manifest slot. It
durably records build observation, authorization generation, and exact signed
document digest before process start. External process and transfer operations
run outside the workspace lock, then completion reacquires ownership and
reconciles the same identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:454-506`).

Each `ConstructNext` call selects one missing target. Each `PublishNext` call
selects one missing publication slot. This prevents a call start instant from
pretending to observe future process or network events
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:508-537`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:662-668`).

The design is O(1) in source size, artifact size, process output, release count,
and provider history
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:813-827`).

### Immutable publication

The signed publication grant binds:

- Manifest and slot;
- expected Artifact or Manifest identity;
- extent, SHA256, and CRC32C;
- provider;
- upload target digest;
- download target digest; and
- generation and validity.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:601-652`.

A create only conflict is never accepted as replay until streaming read back
proves the exact expected extent and digest. Unknown commitment remains
indeterminate. A second upload cannot begin before successful read back, and
provider generation or ETag is not content equality
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:654-696`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:718-743`).

### Proof quality distinction

Bug and Witness production paths are implementation evidence for selected
mechanics. Peachfuzz provides implementation evidence for selected worktree and
object verification mechanics. The Unleash state machine, authorization
protocol, publication grant, process containment, crash recovery, and complete
five object publication have no implemented proof in the archive or consumers.

## Defects and blockers

### Direct ownership contradiction

Section 7 says Unleash creates and owns the isolated checkout beneath its
version workspace. Section 19 says source checkout creation is a non goal
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:301-325`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:996-1004`). Both statements cannot remain. The 2026 specification must assign
source materialization to exactly one owner and remove the conflicting claim.

### Consumer identity in a generic specification

The archived specification names OGS as:

- supplier of signed publication targets;
- authorization signer;
- subject whose assertions are checked;
- receipt consumer; and
- owner of Latest advancement.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:47-49`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:396-399`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:635-652`, and `archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:741-749`.

Primitive specifications must remain product neutral. Those references must
become typed authority, caller, or control plane contracts. Product route,
channel, signer, object name, and Latest policy stay downstream.

### Superseded process ownership

The archived specification makes Unleash directly own process tree
cancellation, wait, output ceilings, and cleanup
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:394-415`).

The later four consumer audit independently establishes that Kernel, Witness,
Bug, and Peachfuzz repeat generic process mechanics. It concludes that
Primitive needs a generic Process owner and that Unleash should only compose
it
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:205-245`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:311-339`).

Unleash must not retain a parallel executor, environment builder, process tree
owner, or bounded output implementation.

### Core contains package private truth

`core/unleash_constants.go` owns Unleash workspace states, target states,
publication states, build argument fragments, environment values, and package
limits even though no second implemented package consumes them
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/unleash_constants.go:3-63`).

Stable `ErrUnleash*` identities remain Core owned under the current engineering
contract. Truly shared BuildIdentity, platform, path, byte, digest, and process
contracts also belong in Core or their exclusive package owner. Unleash only
state enums, revision tokens, limits, and orchestration constants should belong
to `unleash` unless synthesis proves an independent cross package invariant.

### Proof absence

The 1010 line specification declares a permanent contract but supplies no
compiler visible API, Validate method, error producer, state transition,
platform implementation, hostile test, fuzz target, race proof, native proof,
or live provider proof. A proof requirement is not proof.

The archive ledger recognizes this correctly
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1644-1647`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_pending.md:469-476`).

### Oversized responsibility surface

The archived package simultaneously specifies:

- authorization protocol;
- Git materialization;
- executable capabilities;
- process orchestration;
- binary parsing;
- durable workspace state;
- Manifest sealing;
- publication grant protocol; and
- object reconciliation.

Before a 2026 specification is accepted, the import DAG must remove everything
already owned by Process, Release, Filestore, Objectstore, Attest, Garble,
Temporal, Contextstate, or Core. The remaining orchestration boundary must be
small enough for production functions to satisfy `gocyclo <= 10` without
creating wrapper layers or implicit protocols.

### No admission from archive status

The archived `Status: Reviewed permanent contract` line is not authority. It
predates the clean Primitive 2026 ownership contract and the later process
comparison. Admission depends on current independent demand, corrected
ownership, and reviewed proof obligations.

## Primitive 2026 ownership and DAG

### Proposed dependency direction

```text
core
 |
 +-- contextstate
 +-- temporal
 +-- attest
 +-- garble
 +-- process
 +-- filestore
 +-- objectstore
 +-- release
       |
       v
    unleash
```

The diagram shows dependency layers, not permission for sibling packages to
import each other merely to borrow constants.

### Package responsibilities

`process` owns:

- validated executable capability;
- exact typed argument vector;
- exact environment projection;
- process tree containment;
- bounded stdout and stderr streaming;
- exit facts;
- cancellation;
- wait; and
- cleanup.

`release` continues to own target order, Artifact, ArtifactSet, Manifest,
BuildIdentity closure, filename derivation, and verification.

`filestore` owns root confinement, file ownership, staged writes, activation,
synchronization, streaming readers, random access readers, and durable record
installation.

`objectstore` owns one upload or download attempt, provider request semantics,
integrity enforcement, and stable provider outcome identities. It must not
silently retry an ambiguous upload.

`attest` owns signature envelope construction and verification. `temporal` owns
trusted instants and checked arithmetic. `garble` owns tool identity, custody,
seed derivation, and its exact argument prefix. `contextstate` owns trusted
context identity.

`unleash` may own only:

- build authorization facts and verification;
- projection of verified release inputs into one Process request;
- exact commit release workspace sequencing;
- binary inspection specific to Release construction;
- per target authorization provenance;
- Release Artifact and Manifest coordination;
- publication grant binding;
- fixed slot durable state;
- one slot per call selection; and
- replay, conflict, refresh, unavailable, and indeterminate reconciliation.

### Source materialization decision

The 2026 synthesis must make one closed choice:

1. Unleash owns release specific exact commit materialization and its state; or
2. a separately admitted, independently evidenced source capability owns it and
   Unleash composes that capability.

The current archive cannot support both Section 7 ownership and Section 19
nonownership.

### Product boundary

The caller or control plane owns:

- which command is released;
- release gate policy and evidence;
- authorization and signer policy;
- source repository selection;
- object names, buckets, provider credentials, and signed target delivery;
- channel and Latest policy;
- receipt acceptance;
- native smoke and product self tests; and
- operator UX.

Primitive receives typed validated facts and capabilities. It does not name a
consumer or infer commercial policy.

## Decision rationale and conditions

The recon found valuable coordinator mechanics, but the corrected
`v2026.0.0` topology defers a standalone Unleash package because no current
implemented Primitive importer justifies it.

The capability is justified because Witness and Bug already duplicate most of
the release construction lifecycle, Kernel carries a weaker product
distribution builder, Peachfuzz supplies exact commit and immutable object
mechanics, and all four working plans require a final shared release
construction and publication path.

Immediate admission is rejected because the archived boundary conflicts with
itself, contains consumer identity, owns generic process behavior superseded by
later evidence, places package private truth in Core, and has no implementation
proof.

The report preserves the mechanics as evidence. It does not authorize a
package specification or reserve a future package name.

Retirement is rejected because doing so would leave multiple release builders,
binary inspectors, environment planners, Manifest builders, and publication
reconcilers downstream.

Reconsideration may begin only after:

1. Process ownership is admitted and its contract is fixed.
2. Source materialization has exactly one owner.
3. Product names and policy are removed.
4. Core retains only shared invariants and stable error identity.
5. Release, Filestore, Objectstore, Attest, Temporal, Garble, Contextstate, and
   Process interfaces are accepted.
6. The public API is reduced to typed request and result structs with Validate
   methods at every ingress, package crossing, persistence, execution, and
   external output boundary.
7. The specification names exact O(1) bounds and crash states.
8. Hostile proof covers real cross builds, native process containment, binary
   inspection, durable crash recovery, publication ambiguity, strict wire
   closure, fuzz oracles, race, and leak checks.
9. The user reviews the specification slice before implementation.

Implementation may begin only after every direct and transitive prerequisite is
implemented and proven. Consumer cutover must be atomic and cannot use copied
source, a local replace, compatibility shims, or parallel release paths.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
