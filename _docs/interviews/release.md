# Release package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole recon report for archived package `release`. It integrates the
archived implementation and specification, every Primitive package that depends
on Release, and current Kernel, Witness, Bug, and Peachfuzz release-like
capabilities. The product-neutral fact capability belongs in Primitive 2026,
but the archived package is **not admissible unchanged**.

## Evidence boundary


| Source | Exact revision or pin | Release availability and relevance |
| --- | --- | --- |
| Primitive archive | `d046f7b675fcb797398d7cdc87b5504f43978056` (`Harden capability inventory evidence`, 2026-07-27) | Archived Release tree `a4d8ad0fe9dd50a4475e97b519a6618fd343fc48`. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | Committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins Primitive `0df2954a2d911a5d7d775691d023d569affa2c20`, Release tree `7167b44f13c3d710cff7545745c88595564cb55e`. The pre-existing dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43`, whose Release tree is exactly the archive tree. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins Primitive `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9`, Release tree `e9deff1f45d3df0af30431e0339d9a8de852f849`. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins Primitive `388e593231a28434f6faae9f0ab9dffcf332dfc3`, Release tree `9363d8263cef781ef60dfd5b4351d6eab2203280`. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins Primitive `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75`, Release tree `420575ec846c0efc7d31791db6de46cdba80e747`. |

The archived package's decisive clean-break history is:

- `ce9ef4dce304ee6e3d218aaabb5c920a7d41f25a`, specify authenticated
  release facts;
- `4f8c134999f7a36c6534037b245f5d7b6a68a8f8`, add Release proof and
  Update primitives;
- `85a9429e3cec030878fab144a177d8c784ecaf16`, close canonical wire
  contracts;
- `9251557b1c9c1464f3c148046b80b3144da17220`, add the recoverable
  Upgrade lifecycle;
- `ab617e4cf6a1acec9f08ed0199ab101251e8ffad`, compose Callbudget and
  Status protocols;
- `d259789e87bcadb829c5ffac72c6c91ccc604098`, centralize constants and
  close capabilities; and
- `7cec8d33faefa21bc48659d456aa68ebb02bc33d`, integrate the hardened
  Controlstate aggregate.

The difference between pins is material, not cosmetic. The archive has 13
Release files: one specification, six production files, and six test files. The
committed Kernel, Witness, Bug, and Peachfuzz revisions contain 52 or 53 files
under `release`, including build planning, command execution, gate evidence,
deploy clients/transports, update clients/endpoints, signing, paths, and product
publication models. Those old packages own a broad release system. The archive
deliberately replaces that system with a narrow fact package.

Kernel's dirty pin contains the exact archive tree but current Kernel production
has no direct Release import. Witness has 14 and Bug has 18 production files
importing their pinned broad Release API. Peachfuzz has no production import.
Those current callers are evidence for requirements and migration work; they are
not adoption proof for the archived API.

## Capability ownership

Release owns one narrow capability:

> define, authenticate, and compare the immutable product-neutral facts for one
> complete software release and the authoritative selection of the latest such
> release.

It owns:

- the closed four-target release set and canonical target order;
- immutable artifact integrity, identity, build binding, and derived filename;
- an exact four-artifact set representing one build across four platforms;
- build-authorization and sealing provenance references;
- the total signed Manifest fact and proof-carrying Manifest verification;
- the nested signed Latest fact, with separate Manifest and Latest trust
  capabilities;
- deterministic freshness and clock-rollback assessment; and
- the sole typed replay/advance/rollback/conflict decision for Latest.

Release does not build, test, fuzz, invoke Garble, inspect binaries, publish,
upload, download, persist, install, activate, diagnose, read a clock, choose a
route, own a provider limit, or decide product policy
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:6-20`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:971-987`).

The intended ownership socket is sound:

```text
Core build facts ----\
Temporal instants ----> Release immutable facts
Attest signatures ----/           |
                         +---------+----------+
                         |         |          |
                      Update    Upgrade   Controlstate
                         ^
                         |
                      Unleash (specified producer; not implemented)
```

The archive's production dependency direction matches that intent. `go list`
reports only the standard library plus `core`, `temporal`, and `attest`. There is
no Objectstore, Exchange, Filestore, Unleash, Update, Upgrade, Controlstate,
Timeproof, filesystem, process, or HTTP import
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:22-58`).

## Archive evidence

### Fixed artifact and build closure

`ArtifactIntegrity` is a private-representation proof over exact positive
extent, SHA-256, and CRC32C under the Core-owned release-artifact ceiling.
`Artifact` derives its identity from a domain-separated frame over revision,
build authorization, target, extent, and SHA-256, retains the whole
`core.BuildIdentity`, and derives its portable filename rather than accepting a
second name (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/values.go:87-132`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/values.go:194-280`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/values.go:504-519`).

The distinction between identity and equality is valuable. CRC32C and embedded
build fields are retained but intentionally absent from `ArtifactIdentity`.
Consumers deciding that two artifacts are identical must compare the complete
immutable `Artifact`, not only the identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:179-261`).

`Targets` fixes exactly Windows/AMD64, Darwin/ARM64, Linux/AMD64, and
Linux/ARM64. `ArtifactSet` places one Artifact in each corresponding array slot,
rejects duplicate identities, and compares every build field other than platform
across all slots. This closes the important attack in which four individually
valid artifacts come from four different builds
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/values.go:283-389`).

`ManifestFact` repeats every build field that must be visible at the release
boundary and validates it against every embedded Artifact's BuildIdentity. It
re-derives its identity during every validation and length-frames its typed
canonical projection
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/manifest.go:12-139`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/manifest.go:167-247`).

### Nested proof-carrying documents

Manifest issuance signs the complete canonical Fact through Attest.
`VerifyManifest` requires caller-owned trusted keys and expected offering,
performs real signature verification, derives a distinct complete-document
digest, and returns a private proof-carrying `VerifiedManifest`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/manifest.go:249-323`).

Latest accepts only a `VerifiedManifest`; callers cannot smuggle a plausible raw
Manifest beside a signature. Its signed Fact binds Latest identity, offering,
positive generation, the complete nested ManifestDocument, issue and validity
instants, and an authoritative-time reference. Verification independently
requires Manifest and Latest key capabilities, re-verifies both layers, and
returns a private `VerifiedLatest`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/latest.go:137-275`).

`AssessLatest` uses `max(observation, issue)` after verification has proved that
the independently attested time reference does not exceed issue. It reports the
effective instant, exact rollback delta, clock state, freshness, and next
boundary as typed values. The five-minute rollback-tolerance and validity
boundaries are deterministic; Release reads no wall clock
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/latest.go:277-365`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/latest.go:469-535`).

The private proof-carrying values are the right package-crossing contract.
Update and Upgrade do not need to accept loose Fact structs, raw versions,
separate digest strings, or caller assertions that verification happened.

### Strict bounded wire and compiler-owned errors

Every wire decoder is bounded, strict, validates the reconstructed typed value,
re-encodes it, and requires byte equality with the input. ManifestDocument and
LatestDocument contain concrete Attest envelopes rather than generic maps or raw
JSON (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/json_values.go:714-755`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/json_values.go:894-935`).

Core owns stable Release error identities. Release adds local labels while
preserving `core.ErrReleaseContract`, `core.ErrReleaseManifest`,
`core.ErrReleaseVerification`, `core.ErrReleaseLatest`,
`core.ErrReleaseRollback`, and `core.ErrReleaseConflict` for `errors.Is`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/errors.go:10-30`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:726-756`).

All retained production state is fixed-capacity. Canonical hashing streams into
fixed hash state; no operation loads artifact bytes or release history, opens
storage or a connection, starts a goroutine, sleeps, or reads a clock
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:758-773`).

### Complete Primitive dependency and demand graph

The production graph recomputed with `go list` has three direct Release
dependents and one transitive dependent:

```text
core + temporal + attest
          |
        release
       /   |    \
 update  upgrade  controlstate
   |       |          |
   +-------+       register
```

No other Primitive production package reaches Release.

### `update`: installed/candidate closure

Update closes the caller's installed `core.BuildIdentity` to a
`VerifiedManifest`, then selects the exact platform Artifact and requires its
complete BuildIdentity to match. It assesses cached Latest through Release and
rejects a selected version below the installed version
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate.go:9-76`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate.go:79-137`).

For equal versions it requires the same Manifest identity and Artifact identity.
The Manifest identity re-derivation transitively closes the complete artifact,
so this is not relying on Artifact identity alone. For a greater version it
retains the proof-carrying installed/candidate Manifests, VerifiedLatest,
LatestAssessment, and complete candidate Artifact
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate.go:139-188`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/projections.go:74-123`).

Its outward `AvailableSummary` carries distinct Manifest identity,
ManifestDocument digest, Artifact identity, derived filename, complete integrity,
versions, platform, and validity. That is a useful exact handoff rather than a
URL-and-checksum bag (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/projections.go:29-57`).

### `upgrade`: proof consumer and durable installer

Upgrade does not reinterpret release selection. Bootstrap selects the Artifact
from a `VerifiedManifest`, requires exact installed BuildIdentity equality, and
seals Manifest identity, ManifestDocument digest, complete Artifact, and
generation into its selection document
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/bootstrap.go:66-124`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/documents.go:151-205`).

The installer streams SHA-256, CRC32C, and byte count while reading the
executable and compares all three to `release.ArtifactIntegrity`; no artifact
bytes are world-built in memory
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/storage.go:112-175`).

Download projects the same immutable Release integrity into Objectstore only at
the transfer boundary, validates the returned transfer against every component,
then durably commits the staged file before advancing the recovery journal
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/download.go:91-157`). This is precisely why Release
must not import Objectstore itself.

### `controlstate` and `register`: authoritative aggregate

Controlstate verifies the nested Latest with separate Manifest and Latest key
sets and the snapshot's expected offering
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:124-135`). Its aggregate advance
delegates the nested decision to `release.AdvanceLatest`; it does not reproduce
generation, replay, or rollback logic
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/advance.go:389-393`).

Register imports Controlstate but not Release. It is a transitive consumer of the
signed aggregate, not a second owner of release facts.

### Missing producer

`unleash` contains only `SPEC.md`; there is no production implementation.
Its specification says it will construct Release Artifacts, assemble the exact
ArtifactSet, issue and re-verify the Manifest, and only then seal the release
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:436-444`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:554-595`).

Consequently the archive has real downstream consumers but no production issuer
for its Manifest API. That is a 2026 DAG requirement, not permission to move
build and publication execution back into Release.

## Consumer evidence

### Kernel

Kernel currently has no direct Release import. Its release-adjacent LFW
materializer contributes a useful product-local pattern: it enumerates the
current tracked source path set, captures typed file state, sorts entries, binds
an exact source code reference, and installs the manifest with durable replace
and recovery (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/kernel_manifest.go:18-62`,
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/kernel_manifest.go:103-159`).

Its distro rebuild plan separately binds the exact source ref, classifies every
path into a closed role, and validates strict order
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/distro_rebuild.go:11-64`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/distro_rebuild.go:74-145`).

Reusable implications:

- exact source identity and sorted manifest projection belong in the future
  producer/composition flow;
- durable materialization remains with the product or a storage owner;
- Kernel's file-manifest and distro policy are not software Release facts and
  must not be collapsed into Primitive Release.

### Witness

Witness still imports the broad pinned Release package. Its local
`ReleaseManifestPayload` is a signed product umbrella that pins the engine
commit, Go/Garble toolchain, vulnerability database snapshot, module/go.sum tool
provenance, gate evidence, and per-platform engine/tool-bundle artifacts
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_manifest.go:15-23`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_manifest.go:85-173`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_manifest.go:176-248`).

The real command preflights a requested clean commit and seed, builds each
platform, hashes the engine, verifies each pinned tool-bundle attestation,
assembles and signs the manifest, then writes custody and a download index
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-release/main.go:1-80`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-release/main.go:145-217`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-release/main.go:230-300`).

Reusable gems are the real clean-tree/build inspection boundary, exact toolchain
and vulnerability-corpus provenance, nested tool-bundle attestation, trusted
signer mode, and custody/timestamp record. Generic immutable artifact and
Manifest authentication can migrate to Primitive Release. Tool bundles,
vulnerability policy, gate qualification, custody, TSA policy, command
execution, and publication remain Witness or future Unleash/composition
responsibilities.

### Bug

Bug also imports the broad pinned Release package and vendors/copies its old
protocol surface. Its current pipeline gathers real Git, toolchain, Garble, and
vulnerability facts; rejects a random seed; applies Release preflight; and binds
the resulting plan to a product release stamp
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/facts.go:34-71`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/facts.go:92-170`).

Build execution runs fast and final gates before opening the product root,
constructs the release layout, builds and inspects artifacts, creates and signs
the manifest, re-verifies built bytes, syncs the directory, and removes the
layout on failure (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/pipeline.go:137-195`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/pipeline.go:198-274`).

Deploy verifies the signed Manifest and exact Plan binding, obtains and verifies
a signed server plan, re-reads every artifact to reject drift, uploads the
bound targets, and verifies the finalized response
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:55-140`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:143-212`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:215-275`).

Those are valuable producer, evidence, publication, and product-policy
requirements. They must not be copied into the narrow Release fact package.
Bug requires a clean caller migration from the old 52-file API; a compatibility
shim would recreate the ownership failure the archive already removed.

### Peachfuzz

Peachfuzz has no production Primitive Release import. Its worktree release
is a different lifecycle: a closed disposition records whether fuzz evidence
custody must be preserved or may be settled
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/worktree_release.go:3-65`).

Its archive `PackManifest` is also a different fact. It orders content-addressed
object digests, proves contiguous offsets and bounds, streams each object through
the pack, descriptor, and digest accumulator, durably commits the temporary
pack, and publishes pack plus manifest with immutable idempotent names
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/pack.go:71-142`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/pack.go:174-264`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/publish.go:104-140`).

The local gems are streaming content verification, immutable publication,
crash/retry convergence, and explicit custody disposition. They inform future
Unleash/Objectstore integration but are not software Release adoption and must
retain their distinct archive/evidence types.

## Strong mechanics and proof

The archive provides strong signed manifest, multi-artifact closure,
monotonic latest-state, and bounded streaming mechanics. Consumer evidence
demonstrates real release workflows and additional custody requirements. The
combined evidence supports a corrected package contract rather than
preservation of the archive unchanged.

## Defects and blockers

### 1. Equal-generation replay ignores the signed envelope

The specification requires equal generation to be replay only when the complete
canonical `LatestDocument` is byte-identical; any divergence is conflict
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:611-630`).

`AdvanceLatest` discards both documents, extracts only `LatestFact`, and
`compareLatestFacts` returns replay when those Facts compare equal
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/latest.go:377-423`). Two identical Facts signed by two
different independently trusted Latest keys are distinct canonical
LatestDocuments but are accepted as replay. `VerifiedLatest` already retains the
complete document and exposes it, so the missing comparison is not forced by the
type model (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/latest.go:207-275`).

This is a real authority-state defect, not a prose mismatch: Controlstate relies
on this operation as the sole nested advance decision.

### 2. Higher generation admits authoritative-time rollback

The specification requires issue and authoritative-time-reference instants not
to regress across **every** higher-generation shape
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:632-655`).

The implementation checks issue, valid-from, and valid-until only for an
equal-version refresh. It never compares the authoritative-time-reference
instant. For a greater version it checks only version order and changed Manifest
identity, so both issue and the authoritative time reference may move backward
while the proposed document remains internally valid
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/latest.go:426-455`).

That permits a newly numbered authoritative selection to weaken the signed time
floor consumed by Update and Controlstate.

### 3. Authorization references cannot prove target association

The specification says `AuthorizationReferenceSet` contains exactly one
reference per target in canonical target order and requires target-reorder
rejection (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:306-325`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:829-840`).

`AuthorizationReference` contains only document digest, observation instant, and
generation. It carries no target. The fixed set validates each value but cannot
prove which target a reference authorized or reject a permutation of four valid
distinct references (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/values.go:427-501`).
Manifest canonicalization merely pairs array indexes with Artifact indexes
without evidence that the caller placed the authorization in the correct slot
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/manifest.go:213-233`).

The claimed one-per-target provenance is therefore an informal convention the
compiler cannot see. The 2026 model needs a typed target-bound reference or an
equivalent constructor input whose association can be validated.

### 4. The specified total artifact extent is absent

The specification declares checked `ArtifactSet.TotalExtent` and includes exact
total artifact extent as field 14 of `ManifestFact`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:263-304`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:332-352`).

The implementation has no `TotalExtent` operation or Manifest field/accessor.
`ArtifactSet.validateClosure` checks slots, build equality, and duplicate
Artifact identities, but never performs checked total-extent arithmetic
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/values.go:312-389`). `ManifestFact` and its canonical
wire likewise omit the total (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/manifest.go:12-48`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/manifest.go:191-233`).

The signed fact thus cannot report or independently ratchet a requirement its
permanent contract says it owns.

### 5. Release inherits Attest's canonical envelope-order contradiction

Attest specifies canonical envelope JSON order as signer, body digest,
signature, body length, then domain
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/SPEC.md:191-205`). Its implementation emits domain
first, followed by signer, body digest, signature, and body length
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:attest/json.go:104-135`).

Release embeds that concrete envelope in every ManifestDocument and
LatestDocument canonical JSON
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/json_values.go:714-755`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/json_values.go:894-935`). Release's own
outer field order is strict, but persisted Release documents transitively inherit
the unresolved Attest wire-contract mismatch. Admission must wait for one
compiler-owned Attest order and then pin Release vectors to it.

### 6. Required behavior and architecture proof is largely absent

There is no behavioral call to `AdvanceLatest` anywhere in Release tests. The
specification demands the complete generation/version/Manifest/timeline lattice,
including the exact two defects above
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:917-936`).

The package also lacks:

- a data-flow struct inventory;
- any top-level `LayerTriad` test;
- the transitive import-graph ratchet;
- exact public API and four-parameter ratchets;
- Core ownership, product-neutrality, error-producer, and no-world ratchets;
- allocation-independence proof;
- BuildTagSet and advance/assessment semantic fuzz targets; and
- consumer-side target-set ratchets in Unleash, Update, and Upgrade.

These are not optional aspirations. The Release specification itself requires
them (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:943-969`). The project-local testing
protocol independently requires red/green behavioral evidence, hostile serious
boundary tables, production-path honesty, local layer triads for manifest and
verification layers, structural ratchets after behavior, and a classified
data-flow inventory for trust-boundary packages
(`foundation@working-tree:_docs/testing_protocol.md:149-226`, `foundation@working-tree:_docs/testing_protocol.md:382-516`, `foundation@working-tree:_docs/testing_protocol.md:518-642`,
`foundation@working-tree:_docs/testing_protocol.md:862-891`, `foundation@working-tree:_docs/testing_protocol.md:1010-1098`).

The six present fuzzers do use semantic closure oracles for Generation, enums,
Artifact, ArtifactSet, ManifestDocument, and LatestDocument. They are useful but
do not cover the required transition state machines. The protocol explicitly
says that absence of a panic is insufficient and requires an independent semantic
invariant inside each fuzz callback
(`foundation@working-tree:_docs/testing_protocol.md:1210-1288`).

The tests use `reflect.TypeFor` to inspect wire structs
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/json_values_hostile_test.go:16-108`) even though the
package specification explicitly forbids reflection in its proof regime
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:962-969`). The project testing protocol permits
narrow structural reflection, so this is a Release-spec contradiction that must
be resolved explicitly rather than silently waived.

The specification says `public.go` owns the stable operations, but no such file
exists; operations are dispersed through `values.go`, `manifest.go`, and
`latest.go` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/SPEC.md:82-112`). File placement alone is
not a behavioral defect, but the absent exact-API ratchet leaves the declared
surface unproved.

Finally, Git history does not preserve red-first evidence for the main slices.
Production and hostile tests were added together in `4f8c134...`, and production
wire code and new hostile tests were added together in `85a9429...`. No retained
evidence demonstrates the required failing state or classifies each addition as
a pure ratchet over already-correct behavior.

### Fresh gate evidence

Read-only gates were run against the exact archive checkout at
`d046f7b675fcb797398d7cdc87b5504f43978056`. Release remained source-clean.

- `go test ./release`: pass;
- `go test -race ./release`: pass;
- `go test -shuffle=on -count=10 ./release`: pass;
- `go test -coverprofile=... ./release`: pass, 79.5% statement coverage;
- `go vet ./release`: pass;
- `staticcheck ./release`: pass;
- `gosec -quiet ./release`: pass;
- `gocyclo -over 10 release`: no findings;
- test-binary compilation for Windows/AMD64, Darwin/ARM64, Linux/AMD64, and
  Linux/ARM64: pass; and
- each of the six existing fuzz targets, run serially for two seconds after the
  ordinary gates: pass.

These green gates establish ordinary build/tool hygiene. They do not cure the
known untested behavioral failures, missing compiler-visible association, or
absent architecture proof.

## Primitive 2026 ownership and DAG

Keep Release provisionally, but rebuild the admission boundary as a clean slice:

- `core` owns offering/version/build identities, platform vocabulary, tool and
  tag types, stable errors, fixed capacities, field names, and wire ceilings;
- `temporal` owns instant/duration representation and checked comparison;
- `attest` owns signing domains, envelopes, trusted-key verification, and one
  resolved canonical envelope order;
- `release` owns immutable Artifact/Manifest/Latest facts, their exact canonical
  identities, proof-carrying verification, assessment, and monotonic advance;
- `unleash` owns source authorization verification, build execution, inspection,
  artifact construction, Manifest issuance, publication, and custody handoff;
- `update` owns installed-vs-candidate selection and user-facing update
  classification;
- `upgrade` owns download, durable staging, verification against Release
  integrity, activation, rollback, and recovery;
- `controlstate` owns signed aggregate closure and delegates nested Release
  advance;
- `objectstore` owns provider transfer, provider ceilings, and transfer
  integrity DTOs; and
- Kernel, Witness, Bug, and Peachfuzz own product policy, evidence, custody,
  command execution, diagnostics, and presentation.

The implementation order is constrained:

```text
Core contract audit
        |
Temporal ------\
               +--> Attest canonical-order resolution
                          |
                          v
               Release hostile red slices
               (target-bound provenance,
                total extent,
                exact document replay,
                non-regressing time)
                          |
                 Release structural proof
                          |
          +---------------+---------------+
          |               |               |
        Update         Controlstate     Unleash
          |               |               |
        Upgrade         Register      Objectstore
          |
   product migrations: Witness / Bug / Kernel as applicable
```

Do not preserve the old 52/53-file Release API through aliases, wrappers, or a
compatibility subpackage. Migrate real call sites to the new owners and delete
the old paths. Witness and Bug are the main caller migrations; Peachfuzz has no
Release caller, and Kernel's current dirty exact pin has no direct adoption.

## Decision rationale and conditions

**The capability requires a corrected Primitive 2026 implementation before
implementation can begin.**

Admission requires, at minimum:

1. red tests proving equal-generation full-`LatestDocument` replay and conflict;
2. red tests and a fix for authoritative-time non-regression in both
   higher-generation shapes;
3. a compiler-visible target binding for every authorization reference;
4. one resolved contract for total artifact extent, with checked sum,
   overflow rejection, and an independent ratchet;
5. the upstream Attest canonical envelope order resolved and Release exact-byte
   vectors updated;
6. the complete hostile Latest advance lattice and consumer target ratchets;
7. the project-required data-flow inventory, local layer triads, production-path
   proof, transitive import/API/ownership ratchets, and semantic transition
   fuzzers; and
8. clean migrations for Witness and Bug without compatibility debt.

The archive is unusually strong design evidence: it has the correct narrow
ownership idea, fixed typed values, real nested signatures, separate trust
capabilities, strict canonical decoding, fixed memory, and genuine downstream
demand. Those strengths justify retaining the capability. The verified
authority-state defects and proof gaps make unchanged promotion unsafe.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
