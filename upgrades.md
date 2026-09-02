# Primitive 2026.1.0 Architecture Upgrade

This checklist governs the breaking Primitive architecture upgrade. It records
the work required for review; compiler-owned Go contracts remain the source of
truth.

The release constant remains at `v2026.0.172` until explicit approval to publish
`v2026.1.0`.

## Accepted architecture

- [x] Primitive is the blind wall between product policy and real-world effects.
- [x] Primitive must not know the product, UI, database, provider deployment, or
  repository plugged into either side of a contract.
- [x] Every package has one hand-authored primary role plus orthogonal,
  compiler-observed traits.
- [x] Forge generates source facts and verifies authored intent; it does not
  invent package purpose.
- [x] Every real-world effect has exactly one compiler-owned Primitive package.
- [x] Non-owning packages consume effects only through the owning capability.
- [x] Cross-process domains place one typed, bidirectional agreement in the
  middle; independently deployed clients and servers import that same model.
- [x] Client/server documents are required only where a process or trust
  boundary exists. Foundational capabilities do not grow artificial sockets.
- [x] Domain meaning, authentication, wire mechanics, effects, product policy,
  and durable proof remain separate ownership concerns.
- [x] `proofledger` is blind chain machinery. Domain packages own event meaning.
- [x] `reviewcontrol` is the first new reference domain built on the pattern.
- [x] The upgrade may break obsolete APIs. It adds no shims, aliases, or dual
  paths.
- [x] Primitive follows the Unix shape: narrow tools compose through streams,
  each package does one owned job, and outputs can become another package's
  inputs without a world model.
- [x] Processing is streaming and O(1) memory whenever the governed operation
  permits it; bounded materialization requires an explicit compiler-owned
  ceiling.
- [x] Primitive rides directly on stable Go standard-library, operating-system,
  and network semantics through their one owning capability package.
- [x] Primitive does not invent mutable workflow engines or general state
  machines. Current projections are derived from typed facts and append-only
  events where history is required.
- [x] The architecture must remain easy to advance with Go's regular toolchain
  releases rather than preserving wrappers that fight the language or runtime.
- [x] No release bump, commit, tag, push, or installation occurs before the
  corresponding reviewed approval.

## 1. Package architecture contracts

- [x] Add a closed `PackageRole` contract with these initial roles:
  - value contract;
  - domain agreement;
  - authentication binding;
  - effect capability;
  - wire protocol;
  - orchestration.
- [x] Define the valid zero value, canonical JSON, strict decoding, and
  validation for every role.
- [x] Add one primary role to hand-authored `PackageStandardKnowledge`.
- [ ] Preserve orthogonal generated facts for contracts owned, authentication
  bindings, capabilities consumed, capabilities implemented, wire documents,
  and observed effect sites.
- [ ] Reject role combinations that violate ownership without rejecting valid
  supporting traits.
- [x] Add package-role external-ingress inventory, semantic fuzz coverage, and
  validation witnesses.
- [x] Add hostile role tables covering every enum arm, unknown future values,
  forbidden combinations, and neutral packages.

## 2. Unix and Go substrate discipline

- [ ] Inventory APIs that materialize complete files, repositories, ledgers,
  command output, HTTP bodies, object listings, or project graphs.
- [ ] Replace avoidable materialization with `io.Reader`, `io.Writer`, iterators,
  callbacks, cursors, or bounded pipelines.
- [ ] Give every retained buffer, page, line, document, archive, output capture,
  queue, and worker pool a compiler-owned limit.
- [ ] Make backpressure and saturation explicit outcomes; never hide an
  unbounded queue behind a convenience API.
- [ ] Make reader, iterator, body, file, row, process, and goroutine ownership
  visible with a close, cancel, and join path.
- [ ] Remove package-wide registries and in-memory world models whose truth can
  be discovered or streamed from the owning source.
- [ ] Reject mutable workflow/state-machine frameworks; represent real protocol
  states as closed values and derive projections from authoritative facts.
- [ ] Prefer direct standard-library types and semantics inside the owning
  capability over wrapper layers that add no invariant.
- [ ] Ratchet streaming-critical packages against whole-stream reads and
  unbounded allocation.
- [ ] Verify the repository with the current Go toolchain's `go fix`, `go vet`,
  standard-version checks, race detector, and compatibility diagnostics.

## 3. Single-owner effect capabilities

- [ ] Audit the existing `capabilities` catalog and
  `projectstandards.CapabilityOwnership` before extending them.
- [ ] Give each real-world effect exactly one Primitive owner.
- [ ] Cover at least filesystem, transport, process, temporal, entropy, host,
  secrets, locking, shutdown, and object storage.
- [ ] Make the standard-library-symbol-to-capability catalog a Primitive-owned
  contract rather than a Forge-owned allowlist.
- [ ] Preserve implementation, mediated, direct-bypass, and unresolved source
  sites as independent typed evidence.
- [ ] Validate implementation ownership per source site, not by collapsing a
  whole file to one posture.
- [ ] Permit an effect implementation to consume other Primitive capabilities
  through their owned packages.
- [ ] Reject direct effects in every non-owner package.
- [ ] Add mutation-pair tests proving one changed owner or selector changes the
  resulting classification.

## 4. Forge architecture projection

- [ ] Update Forge to read primary package roles from authored package
  knowledge.
- [ ] Update Forge to ask Primitive for effect-symbol ownership.
- [ ] Generate imports, declarations, files, tests, benchmarks, fuzz targets,
  request/response documents, dependency edges, and effect sites from live
  source.
- [ ] Never hand-maintain generated paths, imports, symbols, or edges.
- [ ] Keep unreviewed authored knowledge visibly uncommitted without blocking
  first-pass source discovery.
- [ ] Expose primary role and orthogonal traits in `.forge` JSON.
- [ ] Preserve bounded source coordinates and classification reasons.
- [ ] Ratchet regeneration so only `PackageStandardCode` is replaced and
  `PackageStandardKnowledge` is preserved.
- [ ] Regenerate every Primitive package and produce a complete `.forge`
  project projection.

## 5. Blind proof ledger

- [x] Add `proofledger` as an independent package.
- [x] Define validated ledger, event, request, actor, sequence, head, cursor,
  and page identities.
- [x] Define canonical typed payload constraints without `any`, loose maps, or
  `json.RawMessage` payload bags.
- [x] Define append intent with an explicit genesis or expected head.
- [x] Hash canonical ledger identity, event identity, request identity,
  sequence, previous hash, actor, recorded time, payload kind, and payload
  bytes.
- [x] Keep provider coordinates outside the canonical hash.
- [x] Define append receipts and attest them through `attest`.
- [x] Define exact idempotency:
  - same request and identical canonical intent returns the same receipt;
  - same request and different intent returns a typed conflict;
  - uncertain transport completion remains indeterminate;
  - resolution by request identity returns the original durable receipt.
- [x] Define bounded pages and a streaming iterator.
- [x] Verify strict sequence, previous-hash linkage, tampering, truncation,
  cursor consistency, deterministic order, and exact pagination.
- [x] Supply an in-memory implementation only as a capability proof fixture.
- [x] Keep Firestore, PostgreSQL, collection names, tables, and product event
  kinds outside Primitive.

## 6. Review-control agreement

- [x] Add `reviewcontrol` as the shared bidirectional domain agreement.
- [x] Bind every subject to exact project, repository, commit, module, package,
  repository-relative file path, SHA-256 digest, and byte extent.
- [x] Close all project/repository/module/package/file relationships in
  `Subject.Validate`.
- [x] Define bounded review identities, contracts, problems, completion
  conditions, code references, context references, check requirements, and
  proof requirements.
- [x] Define canonical, bounded packet JSON for the Copy action.
- [x] Define reviewer provenance separately from authority.
- [x] Define typed findings, severity, optional exact source locations, and
  advisory verdicts without an accepted verdict.
- [x] Reuse `runnercontrol` machine observations and receipts by reference.
- [x] Define human decision intent separately from verified human authority.
- [x] Make verified human authority nominal, non-decodable, attest-backed, and
  obtainable only through an approved verifier.
- [x] Bind acceptance and refusal to the exact review, subject, observation,
  contract, and request identity.
- [x] Derive current review projection by folding verified ledger events.
- [x] Keep product acceptance policy, roles, sessions, passkeys, provider
  persistence, repository hosts, and UI state outside Primitive.

## 7. Review authentication and wire binding

- [x] Keep the authority binding in `reviewcontrol`; a second
  `reviewcontrolauth` wire model is not required because the verified nominal
  value never crosses the wire.
- [x] Reuse `attest`, `controlwire`, `controlplane`, and `exchange` instead of
  creating a second signing or socket framework.
- [x] Define one domain-owned request and response type per operation.
- [x] Provide separately constructible issue, read, observation, and human
  decision capabilities.
- [ ] Ensure agent and tool processes do not compile against human acceptance
  capability.
- [x] Ensure a wire document can never construct verified human authority.
- [x] Bound, decode, validate, authenticate, delegate, validate, and encode at
  every socket boundary.
- [x] Add positive, negative, and neutral client/server layer triads.

## 8. Existing package-family inventory

- [ ] Classify every Primitive package by primary role.
- [ ] Record every package's contracts owned, capabilities consumed,
  capabilities implemented, authentication bindings, documents, and effects.
- [ ] Use existing complete families as source-derived reference patterns:
  - `submission` / `submissionauth`;
  - `distribution` / `distributionauth`;
  - `retrieval` / `retrievalauth`;
  - `payment` / `paymentauth`;
  - `controlwire` / `controlplane`;
  - `exchange` / `providerwire`.
- [ ] Review foundational capabilities without forcing client/server documents:
  - `exchange`;
  - `filestore`;
  - `process`;
  - `temporal`;
  - `entropy` or the current entropy owner;
  - `hostfacts`;
  - `secretstore`;
  - `filelock`;
  - `shutdown`.
- [ ] Identify packages mixing conflicting roles and split them at the actual
  ownership boundary.
- [ ] Remove duplicate client/server DTOs, JSON names, route facts, limits,
  enums, statuses, and error identities.
- [ ] Remove direct standard-library effect bypasses from non-owners.
- [ ] Remove obsolete wrappers, aliases, fallback paths, and dead code after
  real call sites migrate.

## 9. External service families

- [ ] Treat every external service integration as its own closed provider
  family: Twilio, Stripe, PayPal, Plunk, Skype, and later services do not share
  copied provider conventions.
- [ ] Put provider facts needed by more than one package in focused
  compiler-owned `core` contracts, including route segments, methods, header
  identities, revisions, media types, limits, and stable error identities.
- [ ] Keep provider constants nominally separate even when their current values
  are equal. Coincidental equality does not create shared ownership, and one
  provider revision must not silently change another provider's bound.
- [ ] Keep provider-specific mechanics with the owning provider family and keep
  product meaning outside it.
- [ ] Make both client and server/webhook sides import the same provider
  contracts instead of copying literals or maintaining equivalent DTOs.
- [ ] Keep authentication, request validation, response validation, replay
  policy, and canonical JSON ownership explicit within each provider family.
- [ ] Permit a server implementation to consume another external service as a
  client only through that service's typed agreement and Primitive-owned effect
  capabilities.
- [ ] Ensure external service packages use `exchange` for transport,
  `temporal` for time, `secretstore` for secrets, and the other single-owner
  effects rather than bypassing them.

## 10. Additional proof domains

- [ ] Keep review, smoke, and legal as separate domain agreements.
- [ ] Share proof-ledger, runner evidence, attestations, identities, hashes,
  extents, cursors, and receipts where ownership already exists.
- [ ] Define a smoke agreement that binds an exact revision and journey to
  browser execution, snapshots, reports, and retained object evidence.
- [ ] Define a legal agreement that binds an exact governed clause to enforcing
  code, hostile tests, accepted proof, and current revision.
- [ ] Ensure source description, review history, smoke proof, and legal proof
  remain distinct projections joined only by exact typed identities.
- [ ] Never use Markdown, screenshots, badges, logs, or UI state as durable
  truth; they remain projections or referenced evidence.

## 11. Document closure and evidence

Current `proofledger` and `reviewcontrol` slice:

- [x] Close every added external document with validation, canonical encoding,
  strict non-mutating decoding, and an exact byte ceiling.
- [x] Fuzz every added external JSON door with compiler-produced valid seeds
  and semantic round-trip or typed-rejection oracles.
- [x] Prove hostile source binding, authority separation, exact evidence
  matching, idempotency, cancellation, chain linkage, tampering, truncation,
  pagination, receipt verification, and review folding.
- [x] Benchmark packet encoding, event hashing, receipt verification, and
  streaming replay with allocations and observable sinks.

- [ ] Give every external document `Validate`, canonical `MarshalJSON`, strict
  non-mutating `UnmarshalJSON`, and an exact maximum encoded size.
- [ ] Reject unknown members, duplicate members, trailing data, partial values,
  oversized collections, and unknown enum values.
- [ ] Add semantic fuzz closure for every external JSON ingress.
- [ ] Use compiler-produced canonical valid fuzz seeds.
- [ ] Add hostile source-binding tests for changed commit, path, digest, extent,
  package, module, repository, and project.
- [ ] Prove reviewer observations cannot grant human authority.
- [ ] Prove exact idempotent replay and conflicting replay behavior.
- [ ] Prove cancellation and transport uncertainty never become success.
- [ ] Prove receipt-to-event and reference-to-manifest-to-disk consistency where
  durable evidence is retained.
- [ ] Add allocation-reporting benchmarks for canonical packet encoding, event
  hashing, attestation verification, and streaming chain replay.

## 12. Primitive-wide exit gate

- [ ] All packages have reviewed primary roles and generated orthogonal facts.
- [ ] Every governed effect has one owner and no unexplained direct bypass.
- [ ] Every cross-process family uses one shared domain model on both sides.
- [ ] Every authentication boundary produces verified nominal values rather
  than trusting claimant-controlled actor fields.
- [ ] Every durable mutation has typed idempotency and a verifiable receipt.
- [ ] Every append-only chain passes real writer, replay, fold, tamper, and
  truncation tests.
- [ ] `.forge` fully surfaces the project, packages, files, declarations,
  effects, roles, and relationships.
- [ ] `go fix ./...` produces no unreviewed semantic change.
- [ ] `go vet ./...` passes.
- [ ] `fieldalignment ./...` passes or reports only reviewed exceptions.
- [ ] Production `gocyclo` is at most 10.
- [ ] `goconst` passes or reports only reviewed compiler-owned exceptions.
- [ ] `nilaway` passes.
- [ ] `errcheck ./...` passes.
- [ ] `staticcheck ./...` passes.
- [ ] `deadcode ./...` and `deadcode -test ./...` are reviewed as library-aware
  evidence rather than blindly deleting exported contracts.
- [ ] `govulncheck ./...` passes.
- [ ] `gosec ./...` passes.
- [ ] `witness-lint ./...` passes.
- [ ] `go test -count=1 ./...` passes with cache reuse disabled.
- [ ] `go test -race -shuffle=on -count=2 ./...` passes.
- [ ] The complete changed-file, removed-path, validation, limitation, and
  migration surface is presented for explicit review.
- [ ] Only after explicit approval: set the release constant to `v2026.1.0`,
  commit, tag, push, and independently verify the remote release.
