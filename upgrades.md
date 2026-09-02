# Primitive 2026.1.0 Architecture Upgrade

This is the ordered implementation checklist for the breaking Primitive
upgrade. Work proceeds from the first unchecked item downward. Later phases do
not redefine earlier ownership decisions.

Compiler-owned Go contracts are authoritative. This file records reviewed
intent, migration order, and completion evidence; it is not a second runtime
truth source.

The release constant advanced from `v2026.0.172` to `v2026.1.0` after explicit
approval to bump and publish. Deployment, installation, and consumer migration
remain separate operations.

## Execution rules

- [x] Preserve unrelated user changes in every repository.
- [x] Make clean breaks only: no aliases, shims, compatibility wrappers,
  duplicate DTOs, fallback paths, or old and new APIs living together.
- [x] Use typed structs, closed enums, owned `Validate` methods, typed errors,
  compiler-visible constants, explicit capabilities, and canonical bounded
  documents wherever correctness depends on a rule.
- [x] Keep processing streaming and O(1) memory whenever the operation permits
  it; every materialized collection or buffer has an explicit bound.
- [x] Follow each repository's local `/_docs/testing_protocol.md` when changing
  tests or evidence tooling.
- [x] Do not execute tests, fuzz targets, benchmarks, test-backed Forge
  observers, or doctrine gates until every production migration in Phases 1
  through 10 is complete. Test source may be upgraded before then.
- [x] Do not commit until the complete changed surface has been presented for
  explicit user review and approval.

## Queue discipline and completion

- [x] The first unchecked item in this document is the only active upgrade
  item. New discoveries are recorded beneath their owning item; they do not
  cause later phases to begin early or silently reorder the queue.
- [x] A checkbox means the current source tree satisfies the stated contract.
  Design intent, a partial repository scan, a compiling subset, or remembered
  terminal output is not completion evidence.
- [x] Production closure and test/evidence-source closure are separate. Phase
  10 closes production source. Phase 11 upgrades every affected test, fuzz,
  benchmark, fixture, inventory, companion, and generated projection. Phase 12
  is the first phase that executes those sources and records run evidence.
- [x] When an item spans repositories, it closes only after every named
  repository has been inspected and every discovered violation is either
  removed in that item or assigned to a later explicit checkbox in this file.

## Accepted ownership architecture

- [x] Primitive is the blind typed wall where Go structs meet across a
  real-world boundary. It owns the plug shape and mechanics, never the meaning
  of either world plugged into it.
- [x] Primitive owns narrow effects and mechanical invariants: filesystem,
  processes, HTTP, time, entropy, secrets, signals, host resources, isolated
  provider boundaries, bounds, validation, canonical encoding,
  authentication, append-only hashing, streaming verification, results, and
  durable receipts.
- [x] Primitive rides Go's standard library, the operating system, and the
  network stack directly. It does not create an alternative runtime or object
  universe.
- [x] Independently deployed sides exchange one shared Primitive agreement.
  Foundational local effects do not grow ceremonial client/server layers.
- [x] Products own meaning and policy: why a fact matters, what is permitted,
  who has authority, what state advances, and what completion means.
- [x] Primitive does not know Work, asks, tasks, agents, reviewers, human
  approval, Blink, Forge, Cink, Anvil, dashboards, browsers, screenshots,
  OWASP, KYC, legal promises, or product persistence layouts.
- [x] Provider integrations are isolated sockets. Stripe, PayPal, Twilio,
  Plunk, and any future Skype or other provider retain separate limits,
  errors, wire contracts, and terminology.
- [x] Reusable blind mechanisms belong in Primitive when multiple products need
  one typed implementation. Their consumers still own content and policy.
- [x] Every package remains Unix-shaped: narrow purpose, deterministic typed
  boundaries, bounded streaming, explicit lifetimes, and composability without
  a mutable workflow engine or in-memory world model.
- [x] Evidence remains distinct from assertion. Intent, execution,
  observation, authentication, independent verification, human authority,
  durable append, and receipt are separate typed facts.

## Phase 1 — `standard`: the shared Go project bar

`standard` is the compiler-visible bar applied to every Go repository,
including Primitive. It is not a service, runner, product, workflow, or Forge
feature. Forge observes and projects it.

- [x] Complete the clean-break rename to `standard` across Primitive, Forge,
  Blink Kernel, Bug, Witness, Peachfuzz, imports, identifiers, compiler
  function contracts, companion filenames, diagnostics, documentation, and
  `.forge` projections.
- [x] Remove every obsolete package path and name. Do not retain an alias or
  transitional import path.
- [x] Keep the shared hierarchy centered on project identity, package identity
  and primary role, source-file identity, exact source coordinates,
  declarations, imports, dependency edges, effect facts, and bounded evidence
  references.
- [x] Classify `standard` as a value-contract package, not a bidirectional
  deployed-peer agreement, and update authored package roles and fixtures.
- [x] Rename `ProductKnowledge` to `ProjectKnowledge`; this package describes a
  repository project and must not invent a product plugged into Primitive.
- [x] Keep evidence facts in `standard` only when they define the common bar a
  repository must meet. Move operational runner lifecycle and effects to
  `runnercontrol`, `machineprobe`, and `runworkspace` without duplicate DTOs.
- [x] Remove the project/package query service, HTTP client/server, machine
  query service, and their transport error identity. A standard has no socket
  or deployed peer.
- [x] Keep deterministic report projection as a pure bounded `io.Writer`
  operation; presentation and web routing remain consumer-owned.
- [x] Remove Forge and other consumer names from `standard` production source.
  Consumers may call the package, but the shared bar remains blind to them.
- [x] Update `core` package identities, purposes, paths, error identities, and
  parent relationships to name the current architecture only.
- [x] Keep companion regeneration out of the contract-bootstrap phase.
  Forge must first be able to classify the final production tree without a
  direct-effect, role, or ownership contradiction; Phase 11 then regenerates
  every companion once from the completed production and test source.

## Phase 2 — Package architecture contracts

- [x] Add a closed `PackageRole` contract with value contract, domain
  agreement, authentication binding, effect capability, wire protocol, and
  orchestration roles.
- [x] Define zero-value behavior, canonical JSON, strict decoding, and
  validation for every role.
- [x] Add one hand-authored primary role to `PackageStandardKnowledge`.
- [x] Preserve orthogonal generated facts for contracts owned, authentication
  bindings, capabilities consumed, capabilities implemented, wire documents,
  dependencies, and observed effect sites.
- [x] Reject role combinations that violate ownership without rejecting valid
  supporting traits.
- [x] Add package-role external-ingress inventory, semantic fuzz source, hostile
  role tables, unknown-future-value rejection, and validation witnesses.
- [x] Classify every current Primitive package by one reviewed primary role in
  `core.PrimitiveArchitecture()`. The closed catalog names exactly the current
  55 non-fixture source packages; pure local `manual` projection and authored
  `primitiveproject` policy are value contracts, not deployed-peer agreements
  or orchestration.
- [x] Keep generated orthogonal facts derived rather than hand-maintained.
  Their one final source-only refresh belongs in Phase 11 after direct-effect
  and package-boundary contradictions have been removed.
- [x] Review current packages for conflicting primary roles. Supporting typed
  contracts do not by themselves create a second primary role. The concrete
  boundary defects are assigned to their owning phases: mechanism scope in
  Phase 6, provider isolation (including the combined cloud-identity family)
  in Phase 7, and product Work policy such as `gate` in Phase 8.
- [x] Sequence obsolete-wrapper, dead-path, copied-contract, and fallback
  deletion with the real migrations that make each path obsolete. No Phase 2
  compatibility layer is introduced merely to make the catalog compile.

## Phase 3 — Unix and Go substrate discipline

- [x] Inventory APIs that materialize complete files, repositories, ledgers,
  command output, HTTP bodies, object listings, or project graphs.
- [x] Replace avoidable materialization with readers, writers, iterators,
  callbacks, cursors, or bounded pipelines.
- [x] Give every retained buffer, page, line, document, archive, output
  capture, queue, and worker pool a compiler-owned limit.
- [x] Make backpressure and saturation explicit outcomes.
- [x] Make reader, iterator, body, file, row, process, and goroutine ownership
  visible through close, cancel, and join paths.
- [x] Remove package-wide registries and in-memory world models whose truth can
  be discovered or streamed from the owner.
- [x] Prefer direct standard-library types and semantics inside the owning
  capability over wrappers that add no invariant.
- [x] Ratchet streaming-critical packages against whole-stream reads and
  unbounded allocation.
- [x] Finish the production-source review for hidden global state, unbounded
  work, artificial client/server layers, mutable workflow engines, and
  complexity. Remove Exchange's process-global transfer pool, return fresh
  mutable ASN.1 OIDs, parse embedded trust roots without process-global
  `sync.Once` caches, avoid a package-global CRC table, and replace the Git
  index numeric ceiling with a 256-MiB operation bound. The remaining globals
  are linker-injected immutable build facts, an internal error identity, and
  the `embed.FS` required by `go:embed`; the three production goroutine sites
  have cancel/close and join paths. A source-only AST complexity audit found no
  production function above 10. Client/server types remain only at genuine
  HTTP or independently deployed boundaries.

## Phase 4 — Single-owner effects

- [x] Establish `capabilities` and `standard.CapabilityOwnership` as the
  compiler-owned effect catalog.
- [x] Cover filesystem, transport, process, temporal, entropy, host, secrets,
  locking, shutdown, and object storage.
- [x] Make standard-library-symbol ownership a Primitive contract rather than
  a Forge allowlist.
- [x] Preserve implementation, mediated, direct-bypass, and unresolved sites
  as independent typed evidence with exact bounded coordinates.
- [x] Validate implementation ownership per source site instead of collapsing
  a whole file to one posture.
- [x] Permit an effect implementation to consume other Primitive effects only
  through their owning packages.
- [x] Add mutation-pair test source proving one changed owner or selector
  changes classification.
- [x] Resolve every current direct effect site: the owning capability may
  implement it; every non-owner must call the owner.
- [x] Remove direct `os`, `os/exec`, raw socket, clock, entropy, secret,
  provider, and host-resource bypasses from non-owners.
- [x] Remove duplicated effect helpers after all callers use the one owner.

Phase 4 result: the source-site audit over every cataloged production package
now reports zero direct bypasses. Filesystem namespace calls live in
`filestore`; process execution in `process`; sockets and HTTP transport in
`exchange`; clock acquisition in `temporal`; entropy in `keygen`; secrets in
`secretstore`; signals in `shutdown`; locks in `filelock`; and host queries in
`hostfacts`. Provider SDKs retain their isolated terminology and mechanics,
while their transport is bounded by `exchange` (or, for the one secret owner,
inside `secretstore` itself). The standard-symbol catalog now recognizes the
previously invisible root/stat, filesystem-identity, filesystem-capacity, and
terminal/host selectors instead of laundering them as unresolved calls.

The concrete duplicate was Hostfacts' private cross-platform filesystem tree
walker. It had no production consumer and reimplemented Filestore with raw
Unix and Windows handles, so it and its obsolete tree-only error identity were
removed. Hostfacts now obtains an exact no-follow held directory and opaque
filesystem identity from `filestore.OpenDirectory`, then performs only the
host-specific disk-capacity or block-device observation against that held Go
handle. Existing cgroup and sysfs reads likewise enter through Filestore. The
remaining uses of `*os.File` and `*os.Root` outside Filestore are borrowed
handles explicitly returned by Filestore for streaming and lifetime ownership,
not alternate namespace-opening paths.

## Phase 5 — Blind proof and receipt mechanics

- [x] Add `proofledger` as blind append-only chain machinery with typed ledger,
  event, request, actor, sequence, head, cursor, and page identities.
- [x] Keep the payload generic and typed without `any`, loose maps, or a
  `json.RawMessage` payload bag.
- [x] Define explicit genesis or expected-head append intent.
- [x] Hash canonical ledger identity, event identity, request identity,
  sequence, previous hash, actor, recorded time, payload kind, and payload
  bytes while keeping provider coordinates outside the canonical hash.
- [x] Define attested append receipts and exact idempotency: identical replay
  returns the original receipt; conflicting replay fails; uncertain transport
  remains indeterminate; request lookup resolves durable completion.
- [x] Provide bounded pages, streaming iteration, strict sequence and
  previous-hash verification, tamper/truncation refusal, cursor consistency,
  deterministic order, and exact pagination.
- [x] Keep the in-memory implementation as a capability proof fixture only.
- [x] Keep Firestore, PostgreSQL, collections, tables, product event kinds, and
  product completion outside Primitive.
- [x] Audit `receipt`, `attest`, and `proofledger` for one ownership path with no
  duplicated receipt identity, authentication, or chain mechanics.

Phase 5 result: `attest` is the only signer, canonical signature-frame owner,
trusted-key verifier, and producer of proof-carrying authenticated values.
`proofledger` is the only predecessor-hash, sequence, replay, pagination, and
chain verifier. `receipt` delegates authentication to Attest and retains only
accepted-evidence identity plus an opaque monotonic-history closure; it neither
signs independently nor computes a ledger chain.

The ledger result is now nominally an `AppendReceipt`, with
`AppendReceiptDocument`, issuance, verification, proof, error identity, and
signing domain carrying the same exact meaning. It has no second arbitrary
receipt ID: ledger identity, event identity, and request nonce already name the
durable operation. The separate `receipt.ReceiptID` remains the identity of an
accepted-evidence fact. Phase 6 still owns the required blindness audit of that
accepted-evidence schema; this phase settles mechanism ownership rather than
preserving its current account/offering vocabulary by implication.

## Phase 6 — Shared blind mechanisms for all products

These packages serve Blink Kernel, generated distros, Bug, Witness,
Peachfuzz, and future Go tools. They know mechanics, not product policy.

- [x] Audit `manual` as one bounded human and machine help/man-page projection;
  consumers own the content.
- [x] Audit `lease` as the generalized licensing mechanism for one exact
  authority-signed decision, opaque subject identity, bounded timeline,
  validation, verification, and monotonic advance; consumers own accounts,
  plans, prices, eligibility, persistence, and legal completion.
- [x] Audit `release` as exact artifact/revision release mechanics; consumers
  decide readiness.
- [x] Audit `deploy` as exact external publication and verified after-state;
  consumers decide permission and target policy.
- [x] Audit `distribution` and `upgrade` as one shared update/upgrade agreement
  and effect path with no product-specific meaning.
- [x] Audit `chit` as immutable bounded custody mechanics and `receipt` as
  authenticated accepted-evidence mechanics.
- [x] Ensure every mechanism has one narrow purpose, typed ingress and output,
  owned validation, bounded processing, stable typed errors, and exact durable
  proof where its contract requires proof.
- [x] Remove copied implementations from Primitive packages and consumers once
  they use the shared owner.

`manual` is the one blind help/man projection. A consumer supplies its own
closed command-topic type and complete `Book`; Primitive owns only the bounded
page/section/line contract, selection validation, deterministic streamed text,
and canonical machine report. It performs no product effect and contains no
command catalogue, readiness rule, or completion policy. Existing Forge and
Blink consumers construct their product prose directly against this shared
typed agreement rather than translating through a second DTO.

`lease` is the shared blind licensing agreement. Primitive authenticates and
assesses one fixed-size grant, refusal, or revocation bound to opaque offering,
entitlement, and device identities; it also rejects rollback, contradictory
same-generation decisions, and invalid time progression. It does not read a
clock, contact a server, persist state, choose a plan, inspect payment standing,
or authorize a product command. The issuer and consuming product retain all of
those meanings and policies.

`release` owns the mechanical release closure: exact clean repository and
commit verification, exact Go tool identity, deterministic bounded build
plans, native artifact inspection, signed manifest and latest documents,
freshness assessment, and installed-versus-candidate comparison. Its
`PreparedRelease` proves only that those mechanical bindings close. It does not
publish, download, install, persist, schedule, retry, grant permission, or
decide whether a product is ready to ship; consumers retain those decisions.

`deploy` is the narrow GCS release-publication effect. It accepts only
already-issued create-only capabilities whose commitments and exact source
integrity match an authenticated release manifest, uploads each fixed role once
in manifest order, and returns confirmed provider transfer receipts or the
confirmed prefix plus a typed failed-transfer fact. Capability issuance,
object naming, authorization, retries, activation, and product target policy
remain outside the package.

`distribution` and `upgrade` form one shared product-neutral path.
`distribution` owns the authenticated publication, latest-discovery, exact
upgrade-grant, request-commitment, and completion agreements imported by both
deployed sides. `upgrade` consumes the verified grant to stream an exact
artifact into the unselected slot, exposes that candidate for a
consumer-defined trial, and promotes only a typed passing report through an
atomic selection change. Neither package decides release authority, command
arguments, test meaning, user consent, retry, scheduling, or product policy.

`receipt` now names its namespace input as an opaque `PrincipalIdentity`
instead of claiming that the value is a customer account. The clean wire break
uses `principal_identity`; products may map an authenticated account or any
other owner to that nominal value, but Receipt sees only principal, offering,
object integrity, and monotonic accepted-history facts. `chit` composes those
authenticated receipts into authority-signed immutable versions, closes each
manifest in one streaming pass with O(1) retained memory, and exposes bounded
catalog pages. Neither package interprets evidence content, eligibility,
retention policy, customer identity, or product completion.

The source-owner audit found no second live implementation of these mechanics.
Consumer packages with names such as `release` retain only product-owned
composition, stamped product identity, readiness, and command policy while
importing Primitive's mechanical contracts. Checked-in `vendor` trees are
generated module snapshots rather than alternate owners; Phase 12 regenerates
them from the reviewed module/workspace closure after all clean contract breaks
and consumer call-site upgrades are complete.

## Phase 7 — Shared agreements, authentication, and providers

- [x] Reuse `attest`, `controlwire`, `controlplane`, and `exchange`; do not
  create a second signing or socket framework.
- [x] Require each genuine cross-process domain to define one shared request
  and response agreement imported by both independently deployed sides.
- [x] Keep nominal verified authority non-decodable. Wire documents carry
  authenticated intent and facts, not product-granted authority.
- [x] Bound, decode, validate, authenticate, delegate, validate, and encode at
  each socket boundary.
- [x] Keep foundational effects such as `exchange`, `filestore`, `process`,
  `temporal`, `hostfacts`, `secretstore`, `filelock`, and `shutdown` free of
  artificial client/server documents.
- [x] Isolate Stripe, PayPal, Twilio, and Plunk into separate provider families
  with nominally separate contracts, constants, limits, errors, authentication,
  replay rules, and canonical documents.
- [x] Split the combined Google Cloud and AWS identity acquisition family into
  provider-isolated packages. Shared token-looking fields do not justify a
  shared provider model; each provider retains its own wire, limits, errors,
  verification, and terminology while using the same lower effect owners.
- [x] Keep unimplemented providers, including Skype, outside the advertised
  `v2026.1.0` surface; adding one later requires its own isolated family.
- [x] Remove duplicate client/server DTOs, JSON names, routes, methods, limits,
  enums, statuses, and error identities from all agreement families.
- [x] Ensure provider implementations use `exchange`, `temporal`,
  `secretstore`, and other single-owner effects rather than bypassing them.

Phase 7 result: the obsolete combined `cloudidentity` production family is
replaced by provider-isolated `googleidentity` and `awsidentity` packages.
They have nominally separate audiences, policies, clients, requests, tokens,
bounds, provider decoders, and stable error identities; no provider enum or
shared token carrier can make their values interchangeable. Google consumers
import the Google contract directly. AWS accepts only its caller-signed SigV4
capability and retains its own STS XML and query terminology. Both perform the
HTTP effect through Exchange and use Temporal for admitted durations/instants.

The cross-process audit found one shared typed agreement per socket family and
no second client/server DTO set. Generic `any` occurrences in these families
are type-parameter constraints over nominal validated bodies, not wire payload
bags. Provider packages contain no direct `net/http`, raw clock, environment,
process, filesystem, or secret-store bypass. Skype has no package, catalog
identity, error identity, import, or advertised contract.

## Phase 8 — Remove product meaning from Primitive

- [x] Remove `reviewcontrol`, `smokecontrol`, `legalcontrol`, `taskmanager`,
  `taskprogress`, `machinecontrol`, and any equivalent product/workflow package
  from Primitive source, tests, core catalogs, error identities, documentation,
  generated companions, and `.forge` projections.
- [x] Keep only the blind mechanisms those product domains compose: typed
  agreements, attestations, identities, hashes, extents, evidence references,
  effects, append-only chains, and receipts.
- [x] Verify no Primitive type or diagnostic names Work, Ask, Task, agent,
  reviewer, human acceptance, browser approval, Smoke policy, Legal policy,
  OWASP, KYC, Blink, Forge, Cink, or Anvil product meaning.
- [x] Verify Primitive contains no product route, UI model, persistence layout,
  or completion transition.

Phase 8 result: the rejected product packages, their authored Taskmanager
interview, and their stale Forge file/package projections are absent. The
append-only historical ledger retains the rejection record. A final source
terminology pass also removed `SmokePlan` and `ProbeTargetSmokeSuite` from the
blind execution boundary: Primitive now exposes a named external-suite plan
and target, while Blink remains free to present that mechanism as its Smoke
product surface. Production declarations and diagnostics contain none of the
forbidden product/work authorities, and Primitive contains no HTML/view model,
browser approval, product persistence layout, or product completion fold.

The consumer adoption pass found and removed one final contradiction that the
first terminology audit had missed: `gate` encoded the product decision that a
Lease assessment permits new paid Work. Bug, Witness, and Peachfuzz now own
their own nominal in-process permits, denial identities, and policy over the
blind Primitive `lease.Assessment`; Primitive's package catalog and error tree
no longer advertise Gate. The remaining opaque metering agreement also uses
`UsageClass` and `UsageCount` rather than claiming to understand Work units.

## Phase 9 — Forge observation and projection

- [x] Make Forge read the authored primary package role independently from
  generated source facts.
- [x] Make Forge resolve standard-library effect ownership through Primitive's
  `capabilities` contract.
- [x] Derive imports, declarations, files, tests, benchmarks, fuzz targets,
  wire documents, dependencies, and effect sites from live source.
- [x] Keep unreviewed authored knowledge visibly pending without blocking
  first-pass source discovery.
- [x] Preserve bounded source coordinates and classification reasons.
- [x] Rewrite only `PackageStandardCode`; preserve
  `PackageStandardKnowledge` exactly.
- [x] Update every Forge import, function contract, companion filename, source
  matcher, and projection field for `standard` with no old naming path.
- [x] Ensure Forge observes the bar but does not define package purpose, invent
  source facts, run product policy, or become Primitive's effect owner.
- [x] Make companion and project projection regeneration source-only. It may
  parse and rewrite current source but must not invoke tests, a test-backed
  compiler observer, or an evidence gate before Phase 12.

Phase 9 result: Forge now imports only the shared `standard` package and uses
`standard_test.go` as the one external companion name. Source inspection
derives declarations, dependencies, and effect coordinates from bounded Go
source while preserving the hand-authored knowledge declaration byte for byte.
The refresh command accepts a Git-only `ProjectSourceObserver`; it cannot run a
Go command, execute a companion, or issue authored evidence. Completed authored
fields therefore project as `unverified`, pending scaffolds remain `pending`,
and claimed implementation sites remain direct until the later independent
compiler evidence phase validates them. The compiler-backed observer remains a
separate Phase 12 capability and is not reachable from regeneration.

## Phase 10 — Consumer adoption and Blink-owned Work

Phase 10 is an ordered multi-repository migration. Complete each numbered
surface from top to bottom. Primitive remains the one shared module for blind
mechanisms; Blink Kernel remains the reusable web kernel and distro factory;
Bug, Witness, and Peachfuzz remain independently shipped CLI products. The
CLI products appear as projects in Blink's control surface when a human needs
to inspect or accept their Work. They do not acquire separate administrative
web applications.

### 10.1 — Current Primitive contracts at every consumer

- [x] Use one explicit local Go workspace for Primitive, Forge, Blink Kernel,
  Bug, Witness, and Peachfuzz until the reviewed `v2026.1.0` module is
  published. Do not add local `replace` directives or compatibility modules.
- [x] Upgrade the production call sites in Blink Kernel, Bug, Witness, and
  Peachfuzz to current Primitive contracts with no shims or dual paths. Test
  callers and generated companions close once in Phase 11.
- [x] Apply `standard`, effect ownership, compiler contracts, owned validation,
  boundedness, typed errors, and evidence ratchets in every consumer.
- [x] Remove obsolete Primitive imports, copied contracts, copied paths,
  copied error identities, direct-effect bypasses, compatibility names, and
  generated records that describe the pre-upgrade architecture.
- [x] Confirm each shared CLI mechanism needed by Bug, Witness, and Peachfuzz
  has exactly one Primitive owner: `manual`, `lease`, `release`, `deploy`,
  `distribution`, `upgrade`, `chit`, `receipt`, and their narrower effect
  dependencies. Consumer packages retain product command policy and content.

The first two unchecked items above close through this ordered production
source audit. Do not check either broad item from one repository or one class of
effect alone.

- [x] Blink Kernel: remove every non-owning direct filesystem, process, HTTP,
  time, entropy, environment, signal, lock, secret, and host-resource effect;
  replace copied shared paths and error identities; keep web and product policy
  in Blink while using Primitive for the mechanical boundary.
- [x] Bug: prove the production tree uses the current `standard` contract and
  Primitive effect owners, contains no obsolete contract or copied mechanism,
  and retains only Bug-owned command and failure-record policy.
- [x] Witness: prove the production tree uses the current `standard` contract
  and Primitive effect owners, contains no ambient process mutation or copied
  mechanism, and retains only Witness-owned execution and evidence policy.
- [x] Peachfuzz: prove the production tree uses the current `standard` contract
  and Primitive effect owners, contains no obsolete contract or copied
  mechanism, and retains only Peachfuzz-owned fuzz and reproduction policy.
- [x] Shared CLI mechanism audit: trace every Bug, Witness, and Peachfuzz
  production call site for `manual`, `lease`, `release`, `deploy`,
  `distribution`, `upgrade`, `chit`, and `receipt`; delete duplicate mechanical
  owners and record any intentionally unused mechanism as absent rather than
  fabricating a consumer path.
- [x] Production closure scan: across all four consumers, find no obsolete
  Primitive name, non-owner direct effect, copied cross-package contract,
  compatibility path, or pre-upgrade generated production record. Record
  test-only findings for Phase 11 instead of hiding them with production shims.

Production call-site result: all four consumer package trees resolve through
the explicit workspace. A production-only source scan reports no import or
identifier for deleted `projectstandards`, `packagestandards`, `gate`,
`cloudidentity`, product workflow packages, `ProductKnowledge`,
`WorkUnitClass`, or `WorkUnitCount`. Bug, Witness, and Peachfuzz now apply
their own immediate-work licensing policy over Primitive `lease.Assessment`;
Google callers use the isolated `googleidentity` agreement; metering uses the
blind `UsageClass` and `UsageCount`; and Receipt consumers bind an opaque
`PrincipalIdentity`. Obsolete test callers remain visible for the clean Phase
11 test-source break rather than being hidden behind production shims.

The final production audit used the shared compiler-owned capability catalog
plus source scans for effect values a call-only AST catalog cannot see. Blink's
remaining raw listener, standard HTTP error/redirect/cookie calls, filesystem
walk, GitHub JWT entropy source, generated distro exit call, ambient working
directory, and namespace resolution now enter through `exchange`,
`filestore`, `keygen`, `process`, and `hostfacts`. The only reported consumer
effect sites are test-support packages and load-test comparison binaries,
which are assigned to Phase 11.

The CLI mechanism trace found real production use of all eight shared
mechanisms in each shipped tool. Bug imports `manual` in 2 files, `lease` in 5,
`release` in 15, `deploy` in 2, `distribution` in 6, `upgrade` in 2, `chit` in
6, and `receipt` in 5. Witness uses 2, 5, 13, 1, 6, 2, 6, and 3 files
respectively; Peachfuzz uses 3, 3, 15, 2, 7, 2, 20, and 3. Their local release
packages retain product-specific offering, build, publication, command, and
license policy while calling Primitive for the mechanical contracts. No
consumer declares a second `manual`, `lease`, `deploy`, `distribution`,
`upgrade`, `chit`, or `receipt` package.

Consumer `vendor/` trees remain the exact last-published Primitive snapshots
(`v2026.0.147` or `v2026.0.153`); they cannot truthfully contain the unpublished
upgrade. The explicit workspace is the Phase 10 source authority. Consumer
module-version and vendor refresh are assigned to Phase 13 after publication,
not disguised as current source or manually patched in advance.

### 10.2 — One Blink web/control surface

- [x] Keep Blink Kernel as the single reusable web/control surface and distro
  factory. Blink itself, generated customer distros, Primitive, Forge, Bug,
  Witness, Peachfuzz, and future Go tools can be selected as projects without
  making their source packages depend on Blink or know that a browser exists.
- [x] Put Work, Review, Smoke, Legal, browser, and verified-human-acceptance
  agreements in Blink Kernel, not Primitive.
- [x] Keep Code, Work, Smoke, Legal, and other Blink views as projections of
  one selected project. Remove the standalone generic Ledger destination;
  Work carries its own bounded semantic history and proof beside the Ask it
  exists to establish.

Phase 10.2 result: Blink owns one bounded `ForgePortfolio` and compiler-owned
selected-project routes. Its current projection includes Blink Kernel, Forge,
Primitive, Bug, Peachfuzz, Witness, and Anvil without adding a Blink import or
browser concept to any selected repository. `Info` and `Work` are local views
inside the selected project's Code surface; Smoke, Legal, Docs, Insights, and
Anvil remain sibling Blink projections. The obsolete standalone Forge Ledger
route, generated page, rail, template, browser script, and chunk test are
removed. Work history now appears only beside the Ask and tasks it proves.
Primitive's production tree contains no Work, task, reviewer, browser, human
acceptance, Blink, Forge, Smoke, Legal, OWASP, or KYC contract; it retains only
blind proof-ledger and effect mechanics.

### 10.3 — Ask, Work, and task contracts

- [x] Model one human Ask as one Work with a durable Work ID and one or many
  bounded tasks. A simple Ask is Work containing one task; a broad Ask may
  discover additional tasks without changing the parent Work identity.
- [x] Make Work independent of implementation category. An Ask may concern a
  checkout suggestion, an auth posture, KYC, documentation, legal text,
  smoke behavior, deployment, release, or any other product-owned outcome.
  Code requirements are only one possible task kind.
- [x] Give each task a Task ID, parent Work ID, exact requirement, acceptance
  conditions, source/package coordinates, required evidence classes, worker,
  claimed revision, independent verification, and human disposition.
- [x] Represent aggregate requirements explicitly. A broad Work such as an
  OWASP or KYC objective may resolve to one task in an already-strong project
  or many tasks in a weak one; no fixed checklist or universal workflow is
  embedded in Primitive.
- [x] Keep Work and task values nominal, closed, bounded, and owned by Blink:
  IDs, kinds, states, actors, decisions, evidence requirements, source
  coordinates, reasons, and transition drafts each define zero-value and
  `Validate` behavior.

Phase 10.3 result: Blink's `ForgeAsk` opens one durable `ForgeWork`; tasks are
separate append-only additions under the same Work ID, so planning may grow
from one task to the bounded maximum without changing the Ask. Work and task
kinds cover feature, defect, documentation, verification, smoke, legal,
deployment, and release outcomes without making code the universal unit.
Each `ForgeWorkTaskRecord` binds the task identity and requirements to its
worker, exact repository commit claim, independent verification and artifacts,
human decisions, and material disposition. Aggregate requirements are named
once on Work and explicitly discharged by accepted active tasks. All states,
actors, kinds, decisions, scopes, evidence requirements, drafts, IDs, text,
paths, and collections are Blink-owned nominal contracts with closed domains,
explicit bounds, and rejecting zero values.

### 10.4 — Semantic history on blind ledger mechanics

- [x] Define the minimal append-only Blink event union for Work opened, task
  added, material task disposition or replacement, exact completion claimed,
  independent verification recorded, human task decision recorded, and human
  Work decision recorded. Keep event payloads deterministic; server-owned
  event identity, authenticated actor, and accepted time belong to the
  envelope rather than claimant JSON.
- [x] Keep the Work ledger semantic and bounded. Do not record prompts, model
  chatter, tool calls, keystrokes, every assertion, transient retries, or
  micro commits. Git and the evidence producer retain that detail; the Work
  history binds the durable fact that matters to acceptance.
- [x] Use `proofledger.Envelope[ForgeWorkEvent]` as the one chain envelope.
  Blink owns only the closed semantic `ForgeWorkEvent` payload and its fold;
  it must not duplicate ledger sequence, previous-hash, event-hash, cursor,
  idempotency, append-receipt, or streaming-verification mechanics.
- [x] Implement one Blink persistence adapter that atomically appends the
  event, expected-head transition, idempotency record, authenticated append
  receipt, and bounded current projection. Reads verify retained receipts and
  chain linkage rather than trusting mutable projection records.
- [x] Derive the current Work view by replay/fold from append-only facts under
  explicit page, event, task, evidence, string, and document bounds. Agents
  cannot mutate history or make an old proof apply to new source.

Phase 10.4 result: `ForgeWorkEventKind` is a seven-arm semantic union for Work
opening, task addition, exact claim, independent verification, task decision,
material task disposition, and Work decision. Requests carry no actor, event
identity, or accepted time; the authenticated Blink control plane derives the
product actor and the server supplies event identity and time. Primitive's
envelope separately authenticates the durable ledger producer and owns the
request identity, sequence, previous hash, event hash, head, receipt, cursor,
and streaming verifier.

Firestore has one transaction for the immutable chunk, exact idempotency
record and authenticated receipt, expected-head transition, and bounded Work
projection. The projection is now explicitly a cache: every board read streams
the immutable chunks in order, verifies every chain transition and retained
append receipt, folds every Blink event, finishes at the stored head, and
requires the canonical replayed board to equal the stored projection. Appends
first verify that same durable pre-state and then rely on Firestore's
transactional head check for races. Prompt chatter, tool calls, retries,
keystrokes, and micro commits have no Work event arm.

### 10.5 — Worker, verifier, and human authority

- [x] Enforce authority separation: the worker publishes a completion claim; a
  different verifier validates the exact committed revision and retained
  evidence; only an authenticated authorized human on the Blink web surface
  can append acceptance and close the task.
- [x] Derive worker, verifier, and human-decision authority from authenticated
  server context and the route's compiler-owned capability. A request payload
  may name the subject and intended decision, but JSON cannot declare itself a
  human, verifier, reviewer, or authorized principal.
- [x] Require the verifier to differ from the worker for the claimed task and
  revision. Require a satisfied, authenticated verification receipt before a
  task becomes eligible for a human decision. A claimant's local green run,
  prose, badge, or self-issued receipt never becomes independent acceptance.
- [x] Permit acceptance routes only for authenticated browser interaction by
  an authorized human principal. Machine credentials, agents, request JSON,
  and verifier authority cannot append a human decision even when they name
  the same account.

Phase 10.5 result: Work mutation bodies contain subjects and drafts but no
actor or accepted-time field. Blink derives the actor from authenticated
request context at the control plane and selects worker, verifier, or human
authority from the compiler-owned operation being invoked. Claims must come
from the task's assigned worker. Verification is bound to a retained Anvil
receipt and is rejected when its authenticated verifier equals the claimant.
Task decisions require a satisfied verification and a third identity; Work
decisions reject every identity that worked on or verified a child task.
Human operations additionally require full scope, a nonzero authentication
method, browser interaction, and Blink's `ActionManageWork` policy. Machine
credentials and request JSON cannot manufacture that authority.

### 10.6 — Evidence binding and completion policy

- [x] Let Git own detailed commits, Witness/Anvil own execution evidence,
  object storage own screenshots/reports/profiles, and repositories own legal
  source. Bind them to Work through typed identities, full commit SHAs,
  digests, byte extents, immutable locations, and receipts.
- [x] Make evidence revision-exact: later source changes make earlier proof
  visibly historical or stale rather than silently applicable.
- [x] Define evidence policy by task kind. Code, smoke, legal, deployment, and
  release tasks may require different typed proof while sharing the same Work
  and ledger mechanics.
- [x] Close a parent Work only when Blink-owned policy confirms every required
  task and aggregate requirement has human acceptance. Each aggregate
  requirement must be compiler-visibly discharged by one or more active tasks;
  an accepted collection of unrelated tasks cannot close the Work.
- [x] Keep task and Work closure distinct. A human may accept an independently
  verified task without closing its parent; the parent closes only through a
  separate human Work decision after aggregate policy is satisfied.

Phase 10.6 result: claims retain Git's repository identity and full commit;
Anvil receipts retain the exact run plan, commit, measurements and artifact
set; GCS artifact references retain immutable object name, digest, extent,
media type and upload time. Blink binds those owner-issued facts without
copying their detailed histories. Task-kind policy requires test evidence for
code and verification, test plus screenshots for smoke, repository source for
legal and documentation, deployment proof for deployment, and publication
plus deployment proof for release.

`DeriveForgeTaskProofState` now compares the latest independently verified
claim with the selected project's current repository and commit. Historical
decisions remain immutable, but the Work projection labels their proof stale
after source advances. A new task acceptance or parent Work acceptance fails
closed unless every relevant proof is current. Task acceptance and parent
closure remain different events; parent acceptance additionally requires all
active tasks accepted and every aggregate requirement discharged by an
accepted active task.

### 10.7 — Bounded human Work projection

- [x] Load the selected project's bounded current Work board and one bounded
  history page in Code / Work. Support exact cursor pagination instead of
  loading the full ledger into browser or server memory.
- [x] Show the Ask, task identity, requirement, acceptance conditions, exact
  claimed commit, independent verifier and receipt, evidence references,
  current state, and semantic history adjacent to the human decision.
- [x] Render Accept and Reject controls only for eligible task or Work states.
  Require a bounded human reason, prevent duplicate submission, surface stale
  expected-head conflicts, preserve recoverable input on failure, and refresh
  from the authoritative append result.
- [x] Keep the no-JavaScript server projection truthful and make JavaScript a
  bounded enhancement that consumes compiler-owned routes, fields, limits,
  and enum values. It may not invent a second Work model or authority rule.

Phase 10.7 result: Blink's authenticated Forge API now returns a validated
`ForgeWorkBoardResponse` that binds the bounded current board, the exact live
repository and commit against which proof is judged, and an optional
human-specific eligibility projection. Proven absence is represented by a nil
board rather than a fabricated genesis event. Eligibility is derived by Blink
policy from the current folded Work, exact claim, satisfied independent
verification, actor separation, and proof freshness; every mutation still
rechecks those facts at the expected ledger head before append.

Code / Work consumes that response through compiler-generated routes, fields,
limits, enum values, HTTP conflict status, and genesis hash. It renders the
human Ask, bounded task plan, exact claim and verifier identities, receipt and
artifact references, proof freshness, current states, and one independently
verified ledger page. Exact cursor pagination replaces the current page rather
than accumulating the ledger in browser memory. Accept and Reject appear only
for server-derived eligible subjects, require a bounded reason, disable repeat
submission, preserve the reason across stale-head refresh, and reload the
authoritative append result. The static HTML shell makes no Work assertion
without JavaScript, and the obsolete hidden sample Work projection was removed.

### 10.8 — Consumer source closure

- [x] Leave every consumer ready for one final source-only companion and
  project-projection refresh after Phase 11 closes the test source.

Phase 10.8 result: the explicit Go 1.27 workspace names Primitive, Forge,
Blink Kernel, Bug, Witness, and Peachfuzz and remains the source authority for
the unpublished break. Blink Kernel, Bug, Witness, and Peachfuzz contain no
obsolete `projectstandard_test.go` or `packagestandard_test.go`, no conflicting
`standard_test.go`, no repository-root Go package the source scanner would
have to invent an identity for, and no abandoned companion rollback or Forge
staging tree. Their current production and test package surfaces are therefore
available to one final clean regeneration after Phase 11 finishes editing
test source.

Forge's refresh boundary accepts only a `ProjectSourceObserver` that can read
exact Git revision and dirty state; it has no Go command, companion executor,
or evidence-issuing capability. Missing `standard_test.go` companions and the
complete bounded `.forge` tree are prepared under one disk-backed rollback
transaction, the project projection is sealed last, and a failed source scan
restores both source companions and the prior sealed projection. No consumer
companion or projection was refreshed early, so Phase 11 cannot make generated
source stale immediately after this checkpoint.

## Phase 11 — Test and evidence source closure

Do not execute this phase's tests yet. This phase completes the test, fuzz,
benchmark, verifier, manifest, and evidence source needed by the final run.

- [x] Give every external document owned by changed packages `Validate`,
  canonical `MarshalJSON`, strict non-mutating `UnmarshalJSON`, and an exact
  maximum encoded size.
- [x] Reject unknown members, duplicate members, trailing data, partial values,
  oversized collections, invalid zero values, and unknown enum arms.
- [x] Inventory every external ingress and provide semantic fuzz closure with
  compiler-produced valid seeds, typed rejection, receiver preservation,
  canonical round-trip, and independent invariants.
- [x] Provide hostile boundary tables or exhaustive domains with earned rows;
  provide the 50-case producer-to-classifier matrix where that handoff exists.
- [x] Provide positive, negative, and neutral layer triads for every touched
  evidence layer.
- [x] Prove exact idempotent replay, conflicting replay, cancellation,
  transport uncertainty, sequence linkage, tampering, truncation, pagination,
  receipt verification, and final-state precedence where applicable.
- [x] Prove evidence-reference to manifest to disk consistency for every
  retained artifact.
- [x] Provide observable allocation-reporting benchmarks for canonical packet
  encoding, hashing, attestation verification, and streaming replay where
  those operations are changed.
- [x] Update data-flow struct inventories, validation witnesses, external-door
  inventories, and structural ratchets for all current production types and
  paths.
- [x] Remove obsolete tests that prove deleted compatibility or product paths;
  do not preserve production shims to keep old tests green.
- [x] Regenerate every Primitive `standard_test.go` companion from the completed
  source,
  preserving reviewed authored knowledge and replacing only generated facts.
  Fresh pending scaffolds take their roles from Primitive's
  compiler-owned architecture catalog and never guess capability ownership.
- [x] Regenerate Primitive's complete `.forge` projection; remove stale
  package/file records and prove every generated path exactly matches the final
  tree. Consumer projection refreshes remain later consumer-adoption work.

Phase 11 Primitive regeneration result: the final source-only refresh produced
55 current `standard_test.go` companions, 55 package projections, 1,190 source
file projections, and one schema-version-5 project projection. No
`projectstandard_test.go`, `packagestandard_test.go`, stage, previous, or
companion-rollback path remains. The refresh did not execute a companion, run a
test, or issue evidence.

Phase 11 Primitive source result: proof-ledger, receipt, release, Standard, and
the isolated provider families retain package-owned byte ceilings, strict
non-mutating decoders, typed errors, validation witnesses, external-door
inventories, hostile tables, semantic fuzz targets, layer-triad tests, and
observable allocation-reporting benchmarks. Deleted tree-measurement, transfer
buffer-pool, account-identity, generic receipt, smoke-plan, and process-exit
compatibility tests were removed or migrated to their current typed contracts.
The generated companions contain no reference to those retired paths. These
are source-closure facts only; Phase 12 remains the first execution and evidence
run.

### Phase 11.1 — Confirmed pre-gate contract defects

The Phase 11 source claim was a checkpoint, not acceptance. A later hostile
review found the following load-bearing defects. They are source-closure work
and must be fixed and ratcheted before Phase 12 executes any gate. Forge-owned
companions and project metadata are deliberately deferred until Primitive
`v2026.1.0` is published and Forge has upgraded to that breaking contract.

- [x] Make `paypal.PayPalAccessGrant.Validate` reject a zero lifetime with the
  PayPal contract identity; a nil diagnostic cause must never erase a typed
  refusal.
- [x] Make PayPal request validation host-neutral and leave live-versus-sandbox
  host binding to the configured client boundary.
- [x] Make Exchange selected-path response ceilings actually match admitted
  colon-action suffixes such as `:signBlob`.
- [x] Publish exactly one `ServerRuntime.Ready` result when `ServeListener`
  refuses an invalid listener after an attempt begins.
- [x] Preserve already-read Filestore bytes across cancellation and give a
  native read failure precedence over a simultaneous context failure.
- [x] Reject Exchange bearer tokens made only of `=` padding.
- [x] Make Stripe, PayPal, Twilio, and Plunk provider error constructors retain
  their typed identity when no diagnostic cause is available; ratchet Stripe
  timestamp overflow and Twilio signature-length classification.
- [x] Make Google metadata acquisition bypass ambient proxies and require the
  response-side `Metadata-Flavor: Google` binding before admitting a bearer.
- [x] Finish the real runner-control socket production-path triads added by the
  hostile review and ensure each fixture proves a non-vacuous effect.
- [x] Synchronize the repository-local testing protocol with the current
  34-rule evidence doctrine, including execution accounting, independent
  acceptance, and benchmark integrity.
- [x] Keep Forge generation out of Primitive source closure; Primitive must be
  completed, reviewed, released as `v2026.1.0`, and consumed by an upgraded
  Forge before Forge may regenerate companions or `.forge` metadata here.

### Phase 11.2 — Reopened hostile boundary defects

The first validation attempts did not close source work. A second hostile
review found five more mechanical boundary defects. Earlier build and analyzer
results remain append-only attempts, but none can accept source that changed
after them.

- [x] Make `filestore.Read` prove EOF at the caller ceiling and refuse a
  source that shrinks below its already-observed extent; a stale `Stat` must
  never turn a partial copy into a complete observation.
- [x] Confine Linux disk-rotation symlink resolution and its parent fallback
  to strict descendants of sysfs so absolute and relative targets cannot
  escape `/sys`, and `/sys` itself cannot fall through to `/queue/rotational`.
- [x] Translate canonical slash-separated source-archive member names to the
  native filesystem form before constructing a Windows `RelativePath`.
- [x] Make Windows `filestore.OpenRoot` reject non-directory and reparse-point
  paths before opening, then prove the opened root is the same directory that
  was inspected.
- [x] Restore `exchange.ListenAddress` refusal of IPv4, IPv6, and empty-host
  wildcard binds so callers cannot accidentally expose a listener on every
  local interface.
- [x] Ratchet every defect above with hostile typed-error evidence and fix the
  retained `staticcheck` finding from the superseded analyzer attempt without
  invoking Forge during Primitive closure.

### Phase 11.3 — Gate-exposed source closure

The first complete Primitive-only gate restart exposed source and ratchet debt
that focused boundary tests could not accept. These are production closure,
not gate exceptions.

- [x] Remove all 55 premature, untracked Forge `standard_test.go` companions;
  Forge-owned package knowledge cannot be emitted or accepted before Forge is
  upgraded to published Primitive `v2026.1.0`.
- [x] Remove unwired runner-control isolation/session model structs and the
  unused experiment-to-reservation convenience path instead of preserving dead
  public API as compatibility ballast.
- [x] Restore the exact `delivered` run-state wire token and bind it to a
  package-owned compiler string.
- [x] Make source-acquisition issue-only output prove its exact JSON through
  the distinct receive-only `SourceAcquisition` contract.
- [x] Compare artifact-entry experiment identities by value across the socket,
  not by pointer address, and retain the server-side refusal fact in failures.
- [x] Remove one-consumer HTTP status constructors and the one-consumer account
  member from Core rather than manufacturing shared coupling; keep account in
  its actual control-plane owner.
- [x] Reconcile the compiler-owned real-world effect inventory with Filestore's
  directory identity ownership and Exchange's standard HTTP response effects.
- [x] Remove duplicate control-plane JSON witnesses and keep the package-wide
  validation-witness inventory exact.
- [x] Keep provider error constructors fail-closed for nil causes while making
  every successful response, client, receiver, and observation validator return
  nil directly instead of routing success through an error constructor.
- [x] Preserve the established machine fingerprint bytes with explicit typed
  fingerprint projections, and bind the renamed `standard` package path into
  the experiment-ID fixture without allowing field alignment to rewrite either
  identity protocol.
- [x] Reconcile the exact Process, Receipt, Release, Standard, Timeproof, and
  Twilio structural ratchets with the reviewed clean-break source shape rather
  than weakening or deleting the ratchets.

## Phase 12 — Final validation and evidence run

This phase begins only after Phases 1 through 11 have no unchecked production
or test-source migration task. Runs are append-only evidence: failures,
retries, skips, cache posture, dirty state, and unavailable work remain visible.

Validation is repository-ordered. Primitive completes every applicable Phase
12 check and its retained evidence before Forge or any consuming repository is
validated further. Consumer compile findings may remain recorded, but they do
not interrupt or reorder Primitive closure.

A Forge refresh was mistakenly invoked in Primitive before this ordering was
made explicit. Its output is not Primitive acceptance evidence and must not be
used to close any Phase 12 item. Forge will not run again until Primitive
`v2026.1.0` is published and Forge itself has been upgraded to consume it.

Superseded attempt 1 ran `./scripts/gate.sh
.artifacts/gates/primitive-phase12-20260902` from the dirty tree at revision
`87ec85133eb91ef154d3ba95c700cf6e11b4e0e1` with Go 1.27.0 on Darwin/arm64 and
the shared workspace `/Users/d/code/go.work`. It passed the recorded version,
environment, revision, dirty-state, install, tidy, workflow, format, build,
`go fix`, vet, staticcheck, errcheck, nilaway, complexity, constants, security,
and vulnerability phases. It then exposed five false-positive
`doctrine/http/server_timeouts` findings for product structs named `Server`
and thirteen field-alignment findings. The exact false positives received
local typed-scope waivers and field alignment was repaired mechanically. The
attempt was deliberately cancelled before tests, fuzzing, and benchmarks
because it could no longer issue a green result; exit status 130 and all
partial logs remain in that attempt's evidence directory.

Superseded attempt 2 ran `./scripts/gate.sh
.artifacts/gates/primitive-phase12-20260902-attempt2` from the same dirty base
revision. It passed every recorded phase through vulnerability scanning,
including witness doctrine and field alignment. Dead-code then failed on the
55 premature Forge companions and six unwired runner-control declarations.
The uncached test phase independently exposed over-generalized Core exports,
a stale effect inventory, duplicate JSON witnesses, a wrong run-state token,
an issue-only source projection without its receive-only oracle, and pointer
identity used as artifact evidence. The attempt was cancelled with exit status
130 before race, fuzz, and benchmark phases; its failure output remains
append-only evidence in the attempt directory.

Superseded attempt 3 ran `./scripts/gate.sh
.artifacts/gates/primitive-phase12-20260902-attempt3` from the same dirty base
revision. Every recorded production analyzer through dead-code passed. The
uncached test phase and the race/shuffle repeat both failed on successful
provider validators routed through newly fail-closed nil-cause error helpers,
field-alignment changes to canonical machine and experiment identity material,
and stale exact structural expectations in Process, Receipt, Release,
Standard, Timeproof, and Twilio. The attempt was cancelled with exit status
130 before fuzz and benchmark phases. The complete failing outputs, commands,
durations, byte counts, and hashes remain in the attempt directory. The exact
previously failing test set subsequently passed both an uncached focused run
and a focused `-race -shuffle=on -count=2` replay; that local closure is
diagnostic evidence, not an acceptance receipt.

Superseded attempt 4 ran `./scripts/gate.sh
.artifacts/gates/primitive-phase12-20260902-attempt4` from the same dirty base
revision. The production analyzers again passed except for the constants gate,
which correctly refused three newly restored stable public strings until their
unrelated ownership domains were recorded in the exact goconst admission
table. The attempt was stopped before tests, fuzzing, and benchmarks. Its
failed constants result and every preceding execution fact remain in the
attempt directory.

Superseded attempt 5 ran `./scripts/gate.sh
.artifacts/gates/primitive-phase12-20260902-attempt5` from the same dirty base
revision with Go 1.27.0. Every production analyzer, the uncached all-package
test run, and the race/shuffle repeat passed. The attempt was stopped before
executing a fuzz target or benchmark when the installed compiler advanced to
Go 1.27.1 and the release compiler contract therefore changed. Its successful
results cannot certify the new compiler-bound source; the partial evidence is
retained and the unexecuted phases remain explicitly not run.

Primitive is now pinned to exact Go 1.27.1 in `go.mod`, the shared workspace
declaration, and Release's closed compiler identity. `go version` reported
`go1.27.1 darwin/arm64`, and the exact release-toolchain ratchet passed
uncached. This focused diagnostic proof does not replace the eventual final
evidence run. Fuzz and benchmark execution remains deferred until that final
run so exploratory validation does not consume the evidence budget or wall
clock.

- [ ] Record exact revision, clean/dirty tree state, complete command, working
  directory and package scope, toolchain, environment/profile identity, cache
  posture, attempts, skips, unavailable and not-run counts, exit status,
  signals, artifact paths, hashes, and byte counts for every run.
- [x] Run production formatting and verify no unexpected diff.
- [ ] Run the production-only build for Primitive through the shared workspace.
- [ ] Run `go fix ./...` and review every semantic change.
- [ ] Run `go vet ./...` and standard-version checks.
- [ ] Run `fieldalignment`, `gocyclo`, `goconst`, `nilaway`, `errcheck`,
  `staticcheck`, `deadcode`, `govulncheck`, `gosec`, `sentinel`, and
  `witness-lint` with reviewed typed exceptions only.
- [ ] Run `go test -count=1 ./...` with cache reuse disabled in Primitive.
- [ ] Run `go test -race -shuffle=on -count=2 ./...` in Primitive.
- [ ] Run configured fuzz targets once per fuzz phase and benchmarks once,
  serially and last, retaining crashers, profiles, stdout, stderr, and manifests
  according to the testing protocol.
- [ ] Independently verify ledger chains, evidence manifests, exact source
  binding, and acceptance receipts. Forge projections are excluded until the
  post-publication Forge upgrade.
- [ ] Confirm every checked task above is supported by current source or exact
  retained evidence rather than prose or remembered terminal output.

## Phase 13 — Review and release

- [ ] Present the complete changed-file list, removed paths, contract and
  ownership changes, generated artifacts, exact validation, failed attempts,
  skipped or unavailable work, remaining limitations, and consumer migration
  requirements for explicit review.
- [x] Obtain explicit user approval for the reviewed slice before committing.
- [ ] After approval, set the release constant to `v2026.1.0` and create one
  coherent clean Primitive checkpoint without running Forge.
- [ ] Publish Primitive `v2026.1.0` to GitHub only after the exact committed
  revision has its required independent evidence and explicit authorization.
- [ ] Move to Forge only after Primitive publication, upgrade Forge to the
  breaking `v2026.1.0` contract, and prove Forge against that version.
- [ ] Return to Primitive only after Forge is upgraded, then regenerate and
  verify Primitive's Forge-owned companions and `.forge` projection.
- [ ] Only after Primitive and Forge are complete, and with explicit
  consumer-version authorization, update Bug, Witness, and Peachfuzz to the
  published module, regenerate each complete vendor tree from its module
  graph, and verify no obsolete Primitive package or version remains in
  `vendor/modules.txt` or vendored source.
- [ ] Treat commit, tag, push, module publication, installation, deployment,
  document publication, and consumer version bumps as distinct authorized
  operations, each with its own verified completion receipt.
