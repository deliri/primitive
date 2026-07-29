# Update package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole recon report for archived package `update`. Here, `update`
means a pure decision over one installed immutable release and one locally held,
authenticated Latest fact. It does not mean HTTP, cache persistence, artifact
download, executable installation, activation, rollback, diagnostics, a source
tree sync, a license-server notice, or a product command named `update`.

The archive has found a real reusable boundary. It has also exposed why the
boundary is needed: Witness and Bug currently trust a signed server decision
without independently enforcing a strictly increasing release version on the
client. The archived implementation closes that semantic gap, but its own
installed-identity provenance and current compiler-owned-contract ratchets are
not strong enough for Primitive 2026.

## Evidence boundary


The following repositories and exact revisions were inspected read-only:

| Repository or package | Revision or Primitive pin | Relevant state |
| --- | --- | --- |
| `foundation_back_up_july_27th_2026` | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` | Full archived package and its dependents |
| archived Update tree | `47643a8cc797e7e442dcd51984efba2510d9122b` | `HEAD:update` tree |
| initial Release/Update implementation | `4f8c134999f7a36c6534037b245f5d7b6a68a8f8` | Release proof and Update primitives introduced |
| Release canonical closure | `85a9429e3cec030878fab144a177d8c784ecaf16` | Canonical Release wire changed Update prerequisites |
| authority-closure proof | `c99f5a3ba11b5d5ded137816fcf234de281e7e20` | Update capability-closure tests added |
| Upgrade handoff implementation | `9251557b1c9c1464f3c148046b80b3144da17220` | PreparedRelease became Upgrade input |
| Core contract consolidation | `d259789e87bcadb829c5ffac72c6c91ccc604098` | Shared constants and identities centralized |
| latest package-affecting change | `6f35a55050caea6dac7b630f278d76aa6f58ceb5` | Temporal operating primitives consolidated |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | Committed pin `0df2954a2d91`; dirty worktree pin `e8b7172161a4` |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` | Pin `773add8ba0fc` |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` | Pin `388e593231a2` |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` | Pin `3f74d8fc35b4` |

The archived package is 4,218 lines across its specification, production
source, and tests. It contains 33 named tests and no fuzz target or benchmark.
The Core-owned stable Update error identities live in
`core/error_identity.go`, not in an Update-specific constants file
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:74-78`).

The focused package gates were rerun:

- `go test ./update`: green;
- `go test -race -shuffle=on -count=2 ./update`: green; and
- production-only `gocyclo -over 10 update/*.go`: no findings.

Those results prove the archived pure package on the current Darwin host. They
do not prove an authentic installed-binary provenance boundary, a production
Latest refresh, a real product composition, a native update, or the correctness
of Release and Upgrade as a complete system.

The archive worktree contains an unrelated untracked Core hostile test. It was
not read as Update evidence and was not modified. No archived or consumer
source was changed during this interview.

### Consumer pin, import, and dependency facts

An exact production import scan found:

- no `github.com/deliri/primitive/v2026/update` import in Kernel;
- no such import in Witness;
- no such import in Bug; and
- no such import in Peachfuzz.

The committed pins used by all four consumers predate the archived Update
package. Kernel's dirty worktree pin contains Update, but Kernel still does not
import it.

Inside archived Primitive, `upgrade` is the sole production dependent.
`upgrade.PrepareRequest` accepts exactly `update.PreparedRelease` and the exact
Temporal observation retained by that capability. Its `Validate` method rejects
an observation mismatch (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/types.go:137-153`).

The durable prepare path then:

1. validates the request before mutation;
2. replays an already prepared journal only if the complete expected journal is
   equal;
3. binds the prepared installed Manifest and Artifact to the current installed
   selection;
4. derives the candidate selection from the exact prepared Artifact;
5. checks disk admission; and
6. persists the prepared recovery state
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/prepare.go:16-88`,
   `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/prepare.go:91-187`).

Upgrade also owns a real compile-negative table. The positive control accepts
`PreparedRelease`; separate builds prove the compiler rejects
`AvailableRelease`, `PreparedFacts`, a raw Artifact, a raw version, and a URL
string (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/compile_negative_hostile_test.go:8-38`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/testdata/compile_negative/available.go:1-10`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/testdata/compile_negative/facts.go:1-10`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/testdata/compile_negative/artifact.go:1-10`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/testdata/compile_negative/version.go:1-10`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/testdata/compile_negative/url.go:1-7`).

That is substantive handoff evidence. It also makes the archive package index
stale: the index says Update's Upgrade handoff proof is pending while the
compile-negative proof and durable consumer are present
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:31-33`).

No four-product consumer cutover exists. The archive proves one internal DAG
edge, not product adoption.

## Capability ownership

The archived specification gives Update a narrow and mostly correct ownership:

> compare one installed immutable release with one locally available,
> authenticated Latest selection and return the exact next local action.

The closed outcomes are:

- current;
- a strictly newer verified release is available;
- cached Latest is missing or expired and needs refresh;
- cached Latest is not yet valid and should be reassessed at an exact local
  boundary;
- installed release facts are inconsistent;
- authenticated selection is a rollback; and
- the same immutable version has conflicting Manifest or Artifact identity
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:6-28`).

Update must own:

- the installed-Manifest closure;
- exact platform selection from the installed compiler identity;
- Latest freshness-before-version ordering;
- numeric release comparison;
- same-version immutable identity conflict detection;
- a closed current/available/refresh/reassess result;
- the capability transition from available to freshly prepared; and
- the stable Update error taxonomy.

Update must not own:

- clock observation;
- Manifest or Latest parsing, signing, or verification;
- HTTP, routes, retry, refresh, or cache persistence;
- download authorization or object transfer;
- filesystem paths or executable reads;
- staging, install, activation, rollback, or recovery;
- product plans, licenses, evidence formats, or retention;
- rendering, confirmation prompts, or diagnostics; or
- historical evidence migration
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:22-28`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:603-617`).

The composition root must own:

- obtaining an authoritative `temporal.Instant`;
- loading and verifying the installed Manifest;
- loading and verifying the cached Latest document;
- deciding whether and when a refresh directive may use the network;
- refreshing Latest through the proper Gate/Callbudget/Exchange path;
- rendering the result and obtaining any product confirmation;
- invoking Upgrade only with a freshly prepared capability; and
- retaining the exact historical Manifest and instrument for old evidence.

This is not a temporary discovery script or an updater tool. It is a small
semantic decision package between authenticated Release facts and the durable
Upgrade lifecycle.

## Archive evidence

### Pure local decision before call-home

The package imports only Core, Release, Temporal, `errors`, and `fmt`.
Production has no clock, HTTP, URL, filesystem, process, channel, sleep,
goroutine, or product dependency. The specification explicitly requires local
evaluation before any remote refresh and says a current fresh selection returns
Current with zero call-home (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:22-28`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:48-66`).

That separation is valuable. Latest being absent or expired is only a typed
directive to the composition root; it is not hidden permission for Update to
perform I/O.

The architecture test positively constrains the direct import set and the
transitive Primitive closure
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/architecture_test.go:15-47`).

### Closed cache absence

`CachedLatest` is a private fixed union. `MissingLatest` is the only valid
missing value. `CacheLatest` accepts a genuine `release.VerifiedLatest`.
The zero value is invalid, and inactive storage must be zero
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/cache.go:5-48`).

That prevents an omitted field or manually zero-initialized request from
silently authorizing refresh. The distinction matters because a refresh
directive can cause a later remote call even though Update itself has no
network capability.

Hostile tests cover:

- the zero value;
- contaminated inactive storage;
- a forged Release proof; and
- missing versus verified closure
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate_hostile_test.go:223-279`).

### Exact installed-release closure

Before considering Latest, `Evaluate` validates the caller's observation,
closes the installed identity against a verified Manifest, validates the cache,
and only then evaluates Latest
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate.go:9-35`).

The installed closure requires:

- installed offering equals Manifest offering;
- installed version equals Manifest version;
- the Manifest has the installed platform slot;
- the selected Artifact's complete embedded BuildIdentity equals the installed
  BuildIdentity; and
- the Manifest identity remains available
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate.go:38-77`).

This is stronger than comparing one version string or executable checksum.
Every BuildIdentity field participates through exact structure equality.

The hostile suite changes revision, offering, version, commit, authorization,
platform, import path, binary base name, tags, Go version, Garble version, and
policy identity one at a time. It also exercises all four Release targets
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate_hostile_test.go:95-221`).

### Freshness before semantic version

Verified Latest is first checked for offering equality. Release owns freshness
assessment. Update consumes the resulting closed state:

- expired becomes RefreshRequired;
- not-yet-valid becomes ReassessAt with Release's exact boundary;
- current alone proceeds to version comparison
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate.go:79-110`).

An authentic but expired Latest therefore cannot authorize Upgrade. An authentic
future Latest does not trigger a needless fetch; it carries the exact local
reassessment boundary.

The test feeds the returned boundary back into Evaluate and proves the first
boundary evaluation reaches the applicable current/available decision
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate_hostile_test.go:65-93`).

### Numeric rollback and immutable-version conflict

For a current Latest selection, Update uses Core's checked numeric
`ReleaseVersion.Compare`:

- installed greater than selected is `core.ErrUpdateRollback`;
- equal version requires equal Manifest identity and target Artifact identity;
- equal version with different identity is `core.ErrUpdateConflict`; and
- installed lower than selected is Available
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate.go:112-166`).

The equal-version rule deliberately compares semantic Manifest identity rather
than the outer signature envelope. Re-signing the same immutable Manifest with
another trusted key remains Current; changing the immutable release does not
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:254-281`).

The hostile suite includes adjacent patch/minor/year boundaries, maximum
numeric components, rollback, offering substitution, equal-version conflict,
and signer rotation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate_hostile_test.go:281-449`).

This decision is the central reusable capability. It is also the client-side
check missing from the current Witness/Bug server-driven updater.

### Proof-carrying availability

Available is not a version-plus-URL DTO. `AvailableRelease` privately retains:

- exact installed BuildIdentity;
- installed verified Manifest and Artifact;
- candidate verified Manifest and complete Artifact;
- enclosing verified Latest;
- current Latest assessment;
- installed platform; and
- a private construction witness
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/prepare.go:9-52`).

Its public summary is a validated value copy containing installed and candidate
versions, Manifest identity, Manifest-document digest, Artifact identity,
derived filename, complete integrity, platform, and Latest validity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/prepare.go:54-69`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/projections.go:29-58`).

That is the right presentation boundary: callers can render exact facts without
receiving authority to install.

The cross-release binding code re-selects installed and candidate Artifacts,
requires a strictly greater candidate version, closes the candidate Manifest to
the exact Latest Manifest identity and document digest, and binds the assessment
to the Latest timing fact
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/bindings.go:18-115`).

Hostile tests splice independently valid values from other signed releases into
every authority-bearing field. Each splice must fail. A separate test changes
CRC32C and derived filename while preserving Artifact identity, proving the
implementation compares the complete Artifact value where Upgrade needs it
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/available_hostile_test.go:16-159`).

### Fresh preparation immediately before Upgrade

`AvailableRelease.PrepareAt` first validates the complete private binding, then
reassesses the retained Latest at a fresh caller-supplied observation. Only a
current assessment constructs `PreparedRelease`; expiration returns refresh and
future validity returns reassess
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/prepare.go:71-129`).

The prepared capability retains the exact observation and assessment alongside
the installed/candidate Release facts. It has no public constructor or decoder.
Only `Preparation.Ready` returns it
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/prepare.go:132-149`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/prepare.go:151-226`).

The preparation boundary is tested at:

- one nanosecond before valid-from;
- exactly valid-from;
- one nanosecond before valid-until;
- exactly valid-until; and
- after valid-until
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/prepare_hostile_test.go:12-79`).

The compiler-negative Upgrade test makes that authority distinction structural,
not conventional.

### Sealed result unions

`Result` and `Preparation` are private fixed unions. Validation rejects
multiple, missing, or contaminated arms. The sealing functions discard a
rejected candidate and return the zero union beside the error
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/result.go:3-67`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/result.go:86-116`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/prepare.go:123-129`).

This closes a subtle failure mode: the accessors expose an arm based on private
state, so returning a populated invalid union would let a caller that ignored
the error consume the rejected capability.

The tests enumerate contaminated shapes and prove rejected Evaluate,
CacheLatest, and PrepareAt calls return zero-valued unions
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/union_hostile_test.go:12-105`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/available_hostile_test.go:161-251`).

### Stable typed error identity

Core owns:

- `core.ErrUpdateContract`;
- `core.ErrUpdateInstalled`;
- `core.ErrUpdateVerification`;
- `core.ErrUpdateRollback`; and
- `core.ErrUpdateConflict`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:74-78`).

Installed inconsistency, forged proof values, rollback, and immutable conflict
are errors rather than success-like result variants. Refresh and reassess remain
normal closed outcomes
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:283-325`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:439-466`).

The semantic identity is therefore usable with `errors.Is`; callers do not need
to parse error prose.

### Fixed complexity and historical non-mutation

Update retains a fixed number of proof values and one selected Artifact. It
does not scan release history or artifact bytes. The specification commits to
O(1) time and memory with respect to artifact extent, history, repository, and
fleet size (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:472-488`).

The allocation test compares minimum and maximum Artifact extents and requires
extent-independent allocation behavior. It currently permits at most 520
allocations for a fully verified Available decision
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate_hostile_test.go:501-544`).

The historical rule is also correct: evaluating a newer release must not
rewrite the older Manifest or change its document digest. Tests retain the old
signed value and compare it before and after evaluation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:399-411`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate_hostile_test.go:546-585`).

## Consumer evidence

### Kernel

Kernel does not currently perform immutable installed-binary Update. Its
`lfw sync` path upgrades a downstream project's Kernel source tree. That is a
different lifecycle and must not be renamed into Primitive Update.

The path still contributes useful composition patterns:

- prepare and display a plan before mutation;
- acquire the operation lock;
- recompute the plan under the lock;
- require exact Head/From/To equality with the preflight plan; and
- reject changes before isolated application
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/fsync.go:67-115`).

It then:

- creates a detached isolated worktree;
- copies only the validated environment;
- proves the upgrade in isolation;
- rechecks the live client HEAD and dirty state; and
- uses only a fast-forward merge
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/sync_apply.go:15-69`,
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/sync_apply.go:181-236`).

These are Upgrade/composition gems:

- preview is not authority;
- revalidate after acquiring exclusivity;
- prove the candidate away from the live installation;
- recheck the live source before activation; and
- use a monotonic activation primitive.

They do not belong in pure Update.

Kernel's local `.lfw-version` marker is compiler-owned and strictly parsed, but
its release enum currently admits only the one `2026.0.0` value
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/lfw_version_marker.go:11-68`).
It cannot represent general candidate ordering and is not a substitute for
Core `ReleaseVersion`.

Kernel's ledger explicitly plans to compose Release, Update, and Upgrade for
discovery, verified download, installation, activation, rollback, and recovery,
while keeping release publication outside Kernel
(`kernel@working-tree:.ledger_pending.md:129-139`).

### Witness

Witness has no archived Update import. It currently has two update-like
surfaces.

First, every license check-in hashes the running executable and sends a typed
payload containing version, SHA-256, platform, device, lease progression, and
nonce (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:285-324`).
The signed license response may carry a copied Primitive `UpdateNotice`; the
client renders it before deciding whether the lease was granted
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:345-385`).

That notice is only:

- product;
- availability boolean; and
- optional latest version
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/_foundation_source/core/update_notice.go:8-40`).

It has no authenticated Manifest, Artifact, freshness, installed closure,
rollback rule, or immutable-version conflict rule. It is presentation, not
Update authority.

Second, Witness's local release protocol contains a large signed online update
request/response. The request discloses installed version, release ID, commit,
binary SHA-256, product, and platform, plus an optional requested release
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/release/update_contract.go:56-157`).
The response embeds the complete product publication under a server-signed
decision (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/release/update_contract.go:159-275`).

There is a verified semantic defect: `verifyAvailableUpdate` rejects the same
release ID and verifies publication/platform binding, but never compares the
candidate version with the installed version. A valid server-signed decision
for a different lower ReleaseID can pass this client check
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/release/update_contract.go:291-306`).

The Witness gems are:

- running-executable SHA-256 as a diagnostic/install fact;
- signed request/response binding;
- nonce/idempotency binding;
- exact platform publication selection; and
- preservation of historical testimony under its producing instrument.

Executable-byte verification belongs to Upgrade/bootstrap/diagnostics, not
pure Update. The signed online decision should be retired in favor of locally
verified Latest plus Update's independent rollback/conflict decision.

Witness's ledger already requires the clean cut: compose Release, Update, and
Upgrade; delete the old deploy/update/release client and parallel signing facts;
then prove a real installed Witness binary through the full lifecycle
(`witness@working-tree:.ledger_pending.md:81-91`, `witness@working-tree:.ledger_pending.md:128-145`).

### Bug

Bug has the only complete current product updater among the four consumers, but
it imports the older broad Primitive Release package rather than archived
Update.

The current command:

1. loads installed build identity, executable digest, release ID, and path;
2. calls a signed update-check endpoint;
3. projects an available publication to one platform target;
4. streams the bounded candidate into a same-directory temporary file;
5. verifies exact size and SHA-256 while writing;
6. syncs and closes the candidate;
7. executes a bounded candidate self-test;
8. asks for confirmation;
9. hard-links the installed binary as rollback state;
10. renames and syncs the replacement;
11. self-tests the installed path; and
12. rolls back and syncs on failure
   (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:147-240`, `bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:293-375`,
   `bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:377-552`, `bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:554-645`).

Those are strong Upgrade gems:

- digest and extent are checked in one streaming pass;
- candidate storage is same-directory and durable;
- candidate behavior is proven before replacement;
- the installed path is proven again after replacement;
- rollback outcome is typed; and
- a bounded diagnostic survives failure for later delivery
  (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:668-701`).

They must migrate to the real Upgrade/composition owners, not be copied into
Update.

Bug has the same rollback-selection defect as Witness. Its
`targetFromResponse` constructs a target from any available signed publication
and validates target shape, but does not compare the candidate version to the
installed version (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:359-375`). The pinned Release
verifier also only rejects the same ReleaseID, not a lower version.

This is direct production evidence for the narrow Update decision. An
independently authenticated Latest fact is still not sufficient until the
client proves monotonic version and immutable identity.

Bug's ledger requires replacing `internal/release` and the CLI deploy/update
paths with the singular Release/Update/Upgrade chain, and later driving a real
installed Bug binary through activation and rollback
(`bug@working-tree:.ledger_pending.md:66-71`, `bug@working-tree:.ledger_pending.md:105-117`).

### Peachfuzz

Peachfuzz has no production software-update path and no Primitive Update or
Upgrade import. Its ledger is explicit that the product has not been released
and that the integration must be a clean cut
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:.ledger_pending.md:7-22`).

The planned lifecycle is:

- embed exactly one Core BuildIdentity;
- build and inspect four real targets;
- publish four binaries plus Manifest;
- advance Latest only after the complete five-object receipt set; and
- drive an installed native Peachfuzz binary through verified Latest,
  activation, recovery/rollback, and a bounded real fuzz slice
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:.ledger_pending.md:94-106`).

Peachfuzz contributes a daemon-specific Upgrade constraint. Its executable path
is embedded into generated launchd/systemd units
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/cli/unit.go:13-33`), while its compiler-owned
StopState determines whether supervisors restart or permanently stop
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/stop_contracts.go:9-40`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/stop_contracts.go:122-171`).

The launchd path uses a private durable restart marker and a bounded restart
budget. The systemd path uses on-failure restart, a fixed delay, a burst limit,
and compiler-derived permanent exit codes
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/supervise/marker.go:19-75`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/supervise/render.go:22-81`).

The local gem is that activation is not merely replacing the executable and
exiting.
Upgrade/composition must preserve the supervisor contract:

- an intentional incompatible persistent-state failure must stop;
- transient activation failure may restart only within a bound;
- launchd marker state and systemd exit-code policy must agree; and
- the installed candidate must survive a real daemon/fuzz smoke before final
  acceptance.

That belongs downstream of Update. Pure Update should only decide whether a
fresh, strictly greater candidate exists.

## Strong mechanics and proof

The archive proves a narrow pure decision over one installed immutable release
and one authenticated Latest fact. Its strongest mechanics are closed cache
absence, exact installed release closure, freshness before semantic version,
numeric rollback and immutable version conflict, proof-carrying availability,
fresh preparation for Upgrade, sealed result unions, stable typed errors, and
O(1) processing. The consumer comparison independently proves why a local
monotonic decision must replace signed server decision authority.

## Defects and blockers

### P1: installed identity provenance is not compiler-enforced

The specification says `EvaluateRequest.Installed` is read through
`core.EmbeddedIdentity` and claims the closure proves the binary's
compiler-owned declared identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:150-165`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:196-217`).

The actual request accepts an ordinary `core.BuildIdentity`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate.go:9-14`).
Core exposes `NewBuildIdentity`, so any caller can construct a valid identity
and pass it to Evaluate. `Evaluate` can prove that the supplied identity appears
in the supplied Manifest; it cannot prove that the identity came from the
running binary's linker-owned embedded symbols.

Core's real provenance boundary is a separate zero-argument function that reads
the embedded symbols and reconstructs the identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/build_identity.go:256-286`).
Nothing in the Update request type requires that function to have produced the
value.

This violates the governing rule: if the compiler cannot distinguish embedded
identity from caller-constructed identity, installed provenance is not a real
contract.

Primitive 2026 must choose one compiler-visible design:

- `Evaluate` obtains `core.EmbeddedIdentity()` itself; or
- Core returns a private-witness `EmbeddedBuildIdentity` capability that only
  the embedded-symbol reader can construct, and Update accepts that capability.

Passing a plain BuildIdentity and documenting its provenance is insufficient.

### P1: ingress validation is not owned by `EvaluateRequest`

The exported `EvaluateRequest` has no `Validate()` method. Validation is split
through unexported `evaluate`, `closeInstalled`, and later helpers
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/evaluate.go:9-77`).

The package correctly validates fields during execution, but the current
contract requires validation to belong to the type that owns the ingress rule.
The request's basic field validity and cross-field installed closure must be
compiler-visible through the owning request boundary, or the API must be
reshaped so a proof-carrying installed capability already owns that closure.

Do not add a decorative `Validate` that merely duplicates all of Evaluate.
Separate:

- structural request validity;
- installed-release closure construction; and
- semantic current/available/refresh/reassess evaluation.

Each should have one typed owner.

### P1: Release prerequisite is not admitted

Update is only as sound as Release's proof-carrying Manifest, Latest,
assessment, Artifact, and version types. The Update specification explicitly
blocks implementation until those contracts exist
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:68-79`).

The archive package index still records Release's canonical wire and full
hostile closure as pending
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:31-32`).
The separate Release interview has also rejected the archived package unchanged
for known proof and contract defects.

Therefore Update cannot be admitted ahead of corrected Core/Temporal/Attest and
Release. Green Update tests over a rejected prerequisite do not repair the
prerequisite.

### P1: the consumer defect has no end-to-end red proof

The archive's numeric rollback tests prove `Evaluate` in isolation. No test
drives either real consumer's dangerous case:

1. installed release `2026.2.0`;
2. independently valid signed Latest selecting `2026.1.9`;
3. server or stale cache otherwise authentic;
4. Update returns `core.ErrUpdateRollback`;
5. no refresh, download, preparation, or Upgrade capability is reachable.

That red slice is necessary because the exact failure exists today in Witness
and Bug. The rebuilt package needs a product-neutral composition fixture plus
real consumer integration tests before the old online decision paths are
deleted.

### P2: raw labels and enum tokens bypass compiler-owned constants

The stable error identities are Core-owned, but local error construction accepts
an arbitrary `string` label. Production repeats raw labels such as
`"update.installed"`, `"update.verification"`, and
`"update.CachedLatest"`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/errors.go:10-33`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/cache.go:28-46`).

The enum String methods also return raw literals such as `"missing"`,
`"verified"`, `"current"`, `"refresh-required"`, and `"ready"` directly
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/enums.go:32-40`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/enums.go:63-75`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/enums.go:99-107`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/enums.go:132-142`).

Under the 2026 contract, these must be owned:

- a typed local error-label enum or package constants for local context;
- package constants for every off-wire enum token;
- Core constants only for values genuinely shared across packages; and
- tests that consume those constants rather than duplicate literals.

Stable identity remains Core-owned and observable through `errors.Is`.

### P2: the architecture proof does not satisfy its own specification

The specification requires exact ratchets for:

- public API;
- four-parameter request structs;
- Core error ownership;
- dependencies and product neutrality;
- error producers;
- struct/data-flow roles;
- no maps;
- no clock, file, network, process, sleep, or goroutine;
- no reflection;
- no compatibility; and
- no aliases
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:576-592`).

The actual architecture file proves only:

- direct imports;
- transitive Primitive imports;
- a partial no-wire/product token scan;
- struct inventory;
- exported surface; and
- singular PreparedRelease construction
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/architecture_test.go:15-256`).

There is no explicit no-alias, no-compatibility, no-map, Core-error-producer,
four-parameter, or full forbidden-capability ratchet.

Worse, the ratchet implementation itself uses loose
`map[string]bool` and `map[string]string` allowlists and role registries
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/architecture_test.go:17-22`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/architecture_test.go:51-59`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/architecture_test.go:291-312`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/architecture_test.go:352-370`).
That conflicts with the current structure-to-structure and no-loose-map rule.

Primitive 2026 needs typed fixed inventories and direct AST/type checks whose
own structure fails compilation when a declared role or contract changes.

### P2: archive status records contradict proved state

`update/SPEC.md` says `Reviewed permanent contract`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:1-4`).
The package index says implemented/reviewed but handoff proof pending, even
though Upgrade has the proof. The same index says Release's closure is pending
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:31-33`).

These cannot all be treated as current truth. Primitive 2026 must derive status
from one compiler-visible/evidence-backed ledger, then update or delete stale
parallel status prose.

### P2: the allocation ratchet preserves an unnecessarily high floor

The O(1) extent-independent requirement is sound. The specific ceiling of 520
allocations for one pure Available decision is not a target worth copying
unchanged (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:update/SPEC.md:550-556`).

It may be an honest archive baseline, but Primitive 2026 should establish the
actual corrected baseline after Release reconstruction, split structural
validation from repeated canonical proof work, and set a local/global ratchet
that can only improve. Do not enshrine 520 as a permanent entitlement.

### P2: no real product or platform lifecycle proof

The pure package itself has no OS behavior, so lack of Linux/Windows tests is
not by itself an Update defect. The admitted capability still requires
composition proof:

- authentic installed identity;
- verified installed Manifest;
- cached verified Latest;
- missing/expired/future/current decisions;
- refresh without hidden call-home;
- monotonic candidate selection;
- fresh preparation;
- Upgrade download/activation/rollback;
- historical evidence remains bound to its old instrument; and
- a real smoke of the installed product.

None of Kernel, Witness, Bug, or Peachfuzz currently supplies that chain with
archived Update.

## Primitive 2026 ownership and DAG

The corrected ownership is:

- `core` owns OfferingIdentity, ReleaseVersion, BuildIdentity, Platform,
  embedded-identity provenance, stable Update error identities, shared fixed
  limits, and any cross-package error/path/protocol constants;
- `temporal` owns Instant and checked temporal comparison;
- `attest` owns signature/envelope verification;
- `release` owns immutable Artifact, Manifest, Latest, their canonical
  identities, proof-carrying verification, freshness assessment, and monotonic
  Latest advance;
- `update` owns only installed-versus-authenticated-Latest closure and the
  current/available/refresh/reassess decision;
- `gate` and `callbudget` authorize and admit refresh;
- `exchange` owns HTTP;
- the composition root owns cached Latest persistence and observation;
- `objectstore` owns bounded candidate transfer;
- `upgrade` owns installed bootstrap, byte verification, staging, durable
  journal, activation, self-test, rollback, and recovery;
- `unleash` owns build/inspection/Manifest construction and publication flow;
- OGS owns authoritative Latest selection and publication acceptance; and
- Kernel, Witness, Bug, and Peachfuzz own product commands, user confirmation,
  supervisor policy, product smoke tests, diagnostics, and historical evidence
  interpretation.

The dependency direction must remain:

```text
Core embedded-identity provenance
              |
       Temporal + Attest
              |
           Release
              |
            Update
              |
            Upgrade
              |
   product composition and smoke
```

The refresh branch is separate:

```text
Update RefreshDirective
          |
 composition policy
          |
 Gate -> Callbudget -> Exchange
          |
 verify Release Latest
          |
 persist exact document
          |
       Update again
```

Update must never import the refresh branch. A directive is data, not a hidden
callback protocol.

The implementation order is constrained:

1. correct Core embedded-identity provenance and typed Update constants/errors;
2. admit corrected Temporal, Attest, and Release;
3. write the rollback/conflict and provenance red slices;
4. implement the narrow pure Update package;
5. re-establish the compiler-negative PreparedRelease handoff;
6. admit corrected Upgrade;
7. migrate Bug first because it has the fullest executable lifecycle;
8. migrate Witness and delete its license UpdateNotice authority and parallel
   online update protocol;
9. compose Kernel only for real installed binaries, keeping `lfw sync`
   separate; and
10. add Peachfuzz with supervisor-aware activation and a real bounded fuzz
    smoke.

No alias, wrapper, compatibility decoder, or temporary translation package
should preserve the old Witness/Bug Release update stack. Update the real call
sites and delete the old authority paths in the same clean cut.

## Decision rationale and conditions

The narrow capability requires redesign before admission. The Primitive 2026
prerequisite and provenance fixes below are mandatory.

Retain:

- the pure installed-versus-Latest ownership;
- missing/verified cache closure;
- freshness-before-version ordering;
- numeric rollback and same-version immutable conflict;
- exact installed/candidate Manifest and Artifact binding;
- private AvailableRelease and PreparedRelease capabilities;
- fresh `PrepareAt` assessment;
- zero-on-rejected-union behavior;
- Core-owned stable error identities;
- extent-independent O(1) processing;
- historical Manifest non-mutation; and
- the compile-negative Upgrade handoff.

Do not copy unchanged:

- caller-supplied plain BuildIdentity as installed provenance;
- ingress without an owning request/capability `Validate` boundary;
- arbitrary raw string error labels;
- duplicated raw enum tokens;
- loose-map architecture inventories;
- specification claims without matching ratchets;
- the stale package-status records;
- the 520-allocation ceiling as a permanent target; or
- any consumer's old signed-server update decision, HTTP client, downloader,
  installer, self-test, diagnostic, or rollback machinery.

Admission requires:

1. corrected Release admission;
2. compiler-enforced embedded installed identity;
3. typed ingress validation ownership;
4. typed local labels and compiler-owned enum tokens;
5. complete typed architecture ratchets matching the specification;
6. a red and then green lower-version consumer composition proof;
7. the PreparedRelease compile-negative boundary;
8. one real Bug native lifecycle before deleting its existing updater;
9. subsequent Witness, Kernel-as-applicable, and Peachfuzz migrations; and
10. explicit independent review of the corrected package and product cutover.

The archive provides a strong semantic core, not a finished Primitive 2026
package. The right move is to rebuild the small decision boundary from its
best proof, strengthen installed provenance and compiler ownership, then use it
to eliminate the larger server-driven updater stacks that currently duplicate
and weaken the release decision.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
