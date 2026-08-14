# Primitive 2026 Ledger

Last updated: `2026-08-13`

## Current

- closed immutable Chit authority issuance, 2026-08-13, prepared
  `v2026.0.100`. `chit.Issue` now accepts the authority document already stored
  under the candidate collection/version slot and the authority trust set. A
  fresh issuance signs and independently verifies its exact payload before any
  document escapes. An occupied slot first authenticates the persisted
  document, returns that exact document for a byte-exact payload retry, and
  returns zero plus `ErrChitConflict` when any UUIDv7 identity, scope, manifest
  closure, acceptance instant, retention promise, or version changes. Primitive
  owns no persistence or transaction engine; OGS supplies the slot read inside
  its own atomic transaction.

  The meaningful red state was the compiler-visible absence of persisted-slot
  and trusted-authority inputs on `Issuance`. Hostile proof covers ten versions
  from one through `uint64` maximum, ten independent valid content changes,
  non-vacuous mutation checks, foreign signer trust, forged persisted
  signatures, zero-value neutrality, exact signed-document convergence, and
  the existing independent signature/substitution verification matrix.

- closed the authority-side registration and check-in transaction gap,
  2026-08-13, prepared `v2026.0.99`. `VerifyRegistrationAuthority` now matches
  only the authority's persisted one-way verifier, derives the exact
  Controlwire replay commitment, returns fresh or byte-exact retry, refuses any
  changed second use, and destroys the presented token on every return path.
  Its sealed result retains only non-secret build, nonce, device,
  installation, revision, replay, and disposition facts.
  `IssueRegisteredInstallation` derives every signed certificate identity fact
  from that proof, so callers cannot substitute an offering, build, device, or
  revision beside it.

  `CommitCheckIn` now takes one authenticated device request, the watermark read
  inside the authority's own transaction, and the authority-selected policy
  cursor. It performs one O(1) comparison and returns accepted, exact replay, or
  neutral conflict without owning persistence or product policy.
  `IssueCommittedCheckInResponse` derives the disposition and watermark and
  binds them to the exact request, account, installation, offering, revision,
  policy, provider time, and signed Lease before output. The new hostile proof
  includes the real Controlwire HTTP receiver, ten distinct fresh/exact
  registration pairs, twelve registration refusals, every offering across all
  three usage dispositions, policy and watermark neutrality, twelve signed
  response substitutions, real Ed25519 certificate/Lease/response signatures,
  zero-proof refusal, token destruction, and a complete compiler-visible
  Controlplane data-flow inventory. The tests exposed and fixed a pre-existing
  error-identity gap where decision disagreement escaped a check-in response
  without `ErrControlPlaneCheckInResponse`.

- closed the missing authority replay record, 2026-08-13, prepared
  `v2026.0.98`. Every successful `ReceiveRoutedJSON` now returns a validated,
  canonical, persistable `ReplayIdentity` derived from the request-owned
  offering, route, revision, nonce, and a domain-separated SHA-256 commitment
  over the exact canonical request bytes. `CheckReplay` returns only fresh or
  exact; reuse of one nonce for different request facts is a typed
  `ErrControlWireReplayConflict`, while a lookup under the wrong nonce is a
  contract/nonce refusal. The implementation owns no persistence policy and
  returns no stored value, leaving OGS to own atomic storage and the identical
  external refusal.

  The red state was the absent compiler-owned API and socket result. Hostile
  proof covers an independent standard-library digest oracle, every mutable
  registration fact, invalid device/installation recombination, the complete
  disposition byte domain, ten accepted JSON representations, twenty-four
  malformed/boundary documents, non-mutating rejection, and two live semantic
  fuzz campaigns (295,371 and 275,721 executions). Focused, doubled shuffled
  race, and full repository tests pass; Staticcheck, Deadcode-with-tests, and
  Witness-lint are clean.

- closed inline secret and bearer disclosure through Go's invalid `%p`
  formatting path, 2026-08-13, prepared `v2026.0.97`. Exchange header values,
  Cloudidentity tokens and AWS signed requests, Objectstore signed URLs and
  fields, and Release signing/Garble seeds now keep bearer bytes behind owned
  indirection while retaining their one explicit projection boundary.
  Objectstore's complete upload/download targets and every nested bearer layer
  implement compiler-witnessed redaction. Release seeds now ride Core's
  destroyable fixed-capacity secret custody; `MaterialResponse.Destroy` clears
  unopened material, and `Open` consumes the response on every terminal path
  before yielding the longer-lived destroyable capabilities.

  Four deliberate inline-secret mutations went red specifically on `%p`
  disclosure before restoration. Hostile formatter matrices exhaust all Go
  verbs plus flags, widths, precision, value/pointer forms, zero values,
  provider variants, and nested targets. External semantic fuzzing proved
  Exchange header agreement with Go's HTTP grammar; Google and AWS token
  ingress; Objectstore upload/download canonical closure and bearer-free
  projections; and Release material plus the complete JSON-door inventory.
  Focused and race suites pass. Direct `staticcheck ./...`,
  `deadcode -test ./...`, and `witness-lint ./...` report zero findings.

- closed exact payment-page traversal and request binding, 2026-08-13,
  prepared `v2026.0.96`. Payment catalog pages now carry a nominal,
  domain-separated commitment to the exact device-signed query. Verification
  binds account/offering scope, all-or-specific selection, start/after cursor,
  page ceiling, build, request nonce, and protocol revision before returning a
  page. Specific selection admits only an end page containing zero or one
  exact selected receipt. Payment and Chit therefore expose the same bounded,
  newest-first, O(1)-with-respect-to-history traversal contract without a
  consumer collecting account history.

  Hostile proof closes deterministic and distinct commitments across every
  variable request fact, empty through exact-maximum page extents, strict
  newest-first order, tagged end/more unions, cursor and page-limit
  substitution, exact specific-result cardinality, foreign receipt refusal,
  neutral zero outputs, and strict JSON receiver preservation. Payment's
  compiler-visible structure inventory and external-door semantic fuzz
  inventory include the new commitment; Paymentauth's real signed response
  fixture uses it rather than a test-only bypass. No compatibility form or
  unbound catalog path remains.

- added the missing logical-CPU host observation, 2026-08-13, prepared
  `v2026.0.95`. `hostfacts.ObserveLogicalCPUCount` now returns one immutable,
  validated fact directly from Go's `runtime.NumCPU`; worker budgets and
  scheduling remain consumer policy. The zero value refuses with the stable
  Hostfacts contract identity, the observation leaf preserves the Hostfacts
  operation and observation identities, and the compiler-visible export,
  struct-role, validation-witness, operation-domain, package-document, and
  capability inventories all carry the new door.

  A public external-package test pins the value to the real runtime oracle and
  rejects the zero fact. Incrementing the production observation by one makes
  that proof fail on the exact semantic mismatch. After restoration, the full
  Hostfacts suite and direct `staticcheck ./...`, `deadcode -test ./...`, and
  `witness-lint ./...` checks pass. No cache, scheduler, state machine, host
  world model, compatibility layer, or consumer policy entered Primitive.

- closed the compiler-visible call-shape and bounded-allocation findings exposed
  by Witness's imported-enum and streaming analysis, 2026-08-13, prepared
  `v2026.0.94`. Every production helper with four or more inputs now receives
  one named request structure, and each new internal carrier is registered in
  its package's data-flow inventory. Domain-separated commitments in
  Controlplane, Objectstore, Release, Retrieval, and Submission now hash their
  already-bounded parts directly through `core.DigestWriter` without assembling
  a second whole buffer. Hostfacts reuses fixed stack storage for line and mount
  decoding, Timeproof allocates only its exact final DER output, and Exchange's
  aggregate reader uses one exact authorized reservation while continuing to
  treat an absent or understated transport extent as advisory only.

  The existing hostile Exchange matrices caught and refused the first incorrect
  implementation that trusted an understated declaration; after correction,
  the affected Controlplane, Core, Exchange, Filestore, Hostfacts, Objectstore,
  Release, Retrieval, Submission, Timeproof, and Upgrade suites pass. Direct
  `staticcheck ./...`, `deadcode -test ./...`, and `witness-lint ./...` report
  zero findings. No compatibility wrapper, waiver, or unwired path was added.

- closed the exact-extent stream constructor and the remaining Objectstore
  transfer-door proof, 2026-08-13, prepared `v2026.0.93`.
  `objectstore.NewExactReader` now accepts compiler-owned `core.ByteLength`
  instead of a raw signed integer and returns a typed construction failure.
  Nil sources are rejected before `bufio.Reader` can defer the fault into a
  panic. Objectstore upload and authenticated GCS upload/download consume the
  same constructor directly; no conversion wrapper or second extent rule
  remains.

  The hostile extent matrix carries fourteen exact valid streams and twenty-six
  short, long, empty, minimum, power-of-two, KiB, internal-buffer, and adjacent
  boundary cases through the real reader. It requires both source and integrity
  identities on every mismatch, proves clean neutral emptiness, withholds the
  final declared chunk when an overlong source is discovered, and pressures
  `MaxInt64` without allocating it. Removing the nil-source constructor gate
  makes the targeted proof red before restoration. Existing real provider tests
  exercise `UploadCloudflareImages` and `DownloadS3` directly.

- closed Objectstore client ownership and the v2026.0.91 architecture drift,
  2026-08-13, published `v2026.0.92`. `objectstore.NewClient` now admits the
  caller-owned standard-library `*http.Client` through Exchange inside
  Objectstore, so six transfer consumers no longer import Exchange merely to
  construct Objectstore's capability. Every real call site moved cleanly with
  no wrapper or compatibility path. The compiler catalog, Primitive policy,
  and README now declare the actual publication and Release test edges while
  preserving the nine-edge coupling ceiling.

  Hostile boundary proof pressures nil plus `MinInt64`, negative, one below,
  one above, positive, and `MaxInt64` client timeouts; every refusal requires
  both Objectstore and Exchange identities, a zero capability, and unchanged
  caller state. Removing Objectstore's error ownership made the targeted test
  fail at the nil and extreme-timeout doors before restoration. The formerly
  vacuous two-blank-verifier test now requires two real JSON refusals, all
  three typed identities, two neutral receivers, and failed equality.

- closed authenticated release publication for installed tools, 2026-08-13,
  published `v2026.0.91`.
  `distributionauth` now carries the installation certificate beside both the
  exact-manifest publication request and the provider-evidence completion.
  The authority certificate is verified before its nominated device key is
  admitted; release-manifest trust remains a separate authority; and the
  installed build is bound on both legs. Completion verification requires the
  original authenticated request, the authority-signed upload grant, the same
  certificate, the device signature, the request commitment, authorization
  nonce, manifest identities, and every URL-free provider evidence fact.

  Hostile proof crosses every offering and the real streaming upload path,
  pressures authority, device, build, manifest, grant, request, nonce,
  evidence, and certificate substitutions, and covers ten valid plus twenty
  hostile representations at both bounded JSON doors. Semantic fuzz campaigns
  completed 5,242 request and 5,220 completion executions with verifier-backed
  oracles and receiver preservation. The suite exposed and corrected a real
  ordering defect: unsigned completion mutations previously reached manifest
  binding before authentication; grant and completion signatures now verify
  before any payload binding. Removing the completion-to-certificate build
  gate deliberately made the targeted hostile test fail before restoration.

- closed authenticated bounded download-all traversal, 2026-08-13, published
  `v2026.0.90`. Retrieval replaces its ambiguous one-entry `All` request with
  typed start, after, and specific selections. Every authority grant binds one
  exact next sequence and an end-or-more fact to the authenticated Chit
  manifest count; issuance and client verification reject skipped, repeated,
  premature-end, false-more, and cross-selection grants. A verified grant
  projects the sole valid next selection, so consumers retain neither a
  manifest nor an informal continuation convention.

  Hostile proof covers the numeric cursor extremes, every contradictory tagged
  union arm, one- and two-entry traversal, resumption, specific older entries,
  signed mutation, canonical external decoding, and receiver preservation. A
  deliberate mutation removing manifest-bound termination made both false-end
  tests fail. Live semantic fuzz campaigns completed 28,802 request-payload,
  2,384 grant-payload, and 32,857 grant-document executions. The focused suites
  and checkpoint deadcode, staticcheck, and witness-lint gates report zero
  findings.

- closed the external-ingress slice used directly by Witness, Bug, and
  Peachfuzz, 2026-08-12, published `v2026.0.78`. Attest now inventories both
  public JSON receivers and semantically fuzzes the fixed-size signature value;
  ID inventories and fuzzes both text and JSON UUIDv7/ULID doors; Process
  ratchets every argv and environment constructor/parser exercised by its
  semantic fuzzers; and Cloudidentity inventories the acquisition, AWS signed
  URL, audience, and Google command-output doors used by Bug's release path.
  Existing provider integration tests remain the network proof instead of
  constructing a server per fuzz iteration.

  The new ID JSON oracle exposed a production ownership defect: malformed
  UUIDv7 and ULID JSON retained the shared JSON identity but dropped ID's typed
  package identity. Both receivers now preserve `core.ErrJSONContract` and
  `core.ErrIDContract` while leaving populated receivers unchanged. Live fuzz
  campaigns completed 190,840 Attest signature executions, 467,310 ID JSON
  executions, and 215,760 Google command-output executions without another
  semantic failure.

- closed the signed-package external-door fuzz inventory, 2026-08-12,
  published `v2026.0.77`. Core, Release, Receipt, Lease, Chit, Controlwire,
  Submission, Payment, and Timeproof now enumerate every public JSON receiver
  behind compiler-visible selectors and AST drift ratchets; text, identifier,
  enum, canonical-hex, JSON-token, and HTTP-status admission paths have direct
  semantic targets. Accepted values validate and reach canonical fixed points;
  rejected values preserve populated receivers and retain typed owner and JSON
  identities. Signed documents execute their independent production verifiers
  and return no proof on failed authentication.

  Live fuzzing exposed and fixed three production boundary defects: Lease JSON
  values and Timeproof semantic JSON refusals had dropped the shared JSON
  identity, while Core canonical-hex decoding mutated its destination before
  refusing a noncanonical spelling. Canonical hex now validates before its
  allocation-free decode while retaining the standard library's
  `hex.InvalidByteError` for invalid bytes. Upgrade's real Exchange-backed
  integration proof is declared as a test-only compiler-catalog edge and in
  Primitive policy. The focused package suites and checkpoint deadcode,
  staticcheck, and witness-lint gates report zero findings.

- closed Upgrade's unexecuted staging mechanics, 2026-08-12, published
  `v2026.0.76`. Download source validation, exact streamed candidate download,
  progress reporting, independent artifact verification, owned-byte cleanup,
  capacity arithmetic, durable-selector authority, cancellation, and public
  path projections now carry hostile positive, negative, and neutral proof.
  The selector decision consumes a private validated projection from Release's
  authenticated `PreparedRelease`; Release remains the only public signature,
  freshness, and embedded-build authority.

  A transport failure removes only the candidate bytes owned by that attempt;
  a mutation after Objectstore confirms the stream is caught by Upgrade's
  independent verifier and removed; a stale primary seals both returned facts;
  and pre-effect cancellation performs no transport and preserves the durable
  selector. The redundant provider check was deleted because Objectstore's
  typed download capability already closes that domain. Upgrade passes twice
  under the race detector with shuffled order; deadcode, staticcheck, and
  witness-lint report zero findings.

- replaced weak rejection oracles across 24 hostile-test files, 2026-08-12,
  published `v2026.0.75`. Controlplane verification, check-in, response,
  product-status, signing-domain, and usage-window proofs now require their
  narrow typed identities and unusable sealed outputs. Controlwire revision,
  nonce, token, cursor, and exchange-policy proofs distinguish owner rejection
  from `*json.SyntaxError` and preserve receivers. Filestore, Process,
  Shutdown, Hostfacts, Chit, Release, Distribution, and Upgrade refusal paths
  now require their real package-owned error chains and exact zero, nil,
  unchanged, owned, or unowned outcomes.

  The stronger proofs exposed two previously hidden ownership facts: a refused
  composite exchange policy is owned by Exchange at its validation boundary,
  and a failed publication transport carries Deploy, Objectstore, and Exchange
  transport identities together. Focused package suites pass twice under the
  race detector with shuffled order; deadcode, staticcheck, and witness-lint
  report zero findings.

- completed the repository typed-error table ratchet and repaired the failures
  it exposed, 2026-08-12, published `v2026.0.74`. Core numeric, path, JSON,
  catalog, and canonical-hex tables; Hostfacts cgroup and storage tables; and
  Release metadata, artifact, closure, dependency, and toolchain tables now
  carry typed error expectations. Metadata syntax failures use `errors.As` for
  `*json.SyntaxError`; owned failures use `errors.Is`. Refused value-producing
  paths prove exact zero, sealed receiver, or transactional preservation.

  The stronger Hostfacts proof exposed and fixed partial rejected cgroup
  membership values and a mount-point error that escaped under Core's identity.
  The full Core run also exposed two stale architecture facts: GCSObjects'
  `exchange` and `testserial` test edges are now compiler- and policy-declared,
  and duplicate Timeproof JSON witnesses were removed in favor of its single
  witness inventory.

  Proof: Core, Hostfacts, and Release pass twice under the race detector with
  shuffled order, and deadcode, staticcheck, and witness-lint report zero
  findings. The repository boolean-error-table count falls from 24 to zero.

- published the third typed-error table migration batch, 2026-08-12,
  published `v2026.0.73`. Cloudidentity audience, Google bearer output, timeout
  policy, signed Amazon request, and namespaced Amazon response tables now name
  the Cloudidentity contract directly. Process environment lookup names its
  contract, and Controlplane usage windows name the narrower usage-window
  identity across class, ordering, arithmetic, and freshness attacks. Rejected
  Google output also proves the returned token is exactly zero.

  Proof: Cloudidentity, Process, and Controlplane pass twice under the race
  detector with shuffled order, and deadcode, staticcheck, and witness-lint
  report zero findings. The repository boolean-table count falls from 31 to 24.

- published the second typed-error table migration batch, 2026-08-12,
  published `v2026.0.72`. Garble's three external token parsers, Keygen's
  bounded random-token request, Objectstore's received upload capability,
  Filestore's reported-allocation contract, and Shutdown's step identity now
  name their compiler-owned error identities instead of reducing rejection to
  booleans. Rejected parsers, capabilities, byte projections, and step
  identities prove their exact zero or sealed outcome. Keygen now also pins
  midpoint and near-ceiling admission.

  Proof: the five affected packages pass twice under the race detector with
  shuffled order, and deadcode, staticcheck, and witness-lint report zero
  findings. The repository boolean-table count falls from 38 to 31.

- published the first typed-error table migration batch, 2026-08-12,
  published `v2026.0.71`. Signed grant lifetime, Process containment and
  identity, and Hostfacts terminal width and geometry no longer encode failure
  as a boolean. Each row names its compiler-owned error identity; refused
  grants prove zero payload and bearer, refused process identities return zero,
  and invalid terminal geometry preserves the Hostfacts identity through both
  accessors. Terminal widths now cover ten exact floor, conventional, midpoint,
  and ceiling boundaries.

  Proof: Submission, Process, and Hostfacts pass twice under the race detector
  with shuffled order, and deadcode, staticcheck, and witness-lint report zero
  findings. The repository boolean-table count falls from 43 to 38.

- published hostile expansion of the thinnest remaining validation,
  verification, and fold tables, 2026-08-12, published `v2026.0.70`. Cgroup
  unlimited folding now exhausts all seven nonempty root/parent/current
  interface-presence masks under both cgroup v1 and v2 and proves the closest
  present typed source and exact interface path. Disk-floor refusal now covers
  twelve capacity/availability shapes at equality, one above, and far above
  the device. Signed update and upgrade request verification each carry
  thirteen exact mutations across trust, candidate or build closure, nonce,
  signing domain, signer, body digest, and body length; every rejected sealed
  proof returns a zero payload under the Distribution verification identity.

  Proof: Hostfacts and Distribution pass twice under the race detector with
  shuffled order, and deadcode, staticcheck, and witness-lint report zero
  findings.

- published hostile closure for the remaining falsely exhaustive verification
  and admission tables, 2026-08-12, published `v2026.0.69`. Check-in response
  issuance now refuses seventeen independently mutated invalid payload facts,
  requires the owning response or consistency identity, and returns an exact
  zero document. Installation-certificate verification now pressures thirteen
  structural, trust-set, signature, body-commitment, envelope, and domain
  substitutions; every row requires both the registration identity and the
  precise nested identity while every sealed accessor stays unusable.
  Policy-activation admission now pins twelve numeric boundaries and walks
  every one-hot and one-cold bit pattern across the complete uint64 width.

  Proof: Controlplane and Controlwire pass twice under the race detector with
  shuffled order, and deadcode, staticcheck, and witness-lint report zero
  findings.

- published an exact-fact hostile matrix for the Submission grant-binding
  layer triad, 2026-08-12, published `v2026.0.68`. Fifteen independently valid
  near misses now change one declared media type, extent edge, digest, CRC,
  upload identity, collection identity, entry name, object count, build
  version, commit, operating system, architecture, offering, or request nonce
  at a time. An independent request commitment oracle proves each mutation is
  distinct before the authenticated grant verifier must refuse it with the
  compiler-owned response-binding identity. Every refusal also proves the
  returned authority stays sealed: validation fails, payload is exact zero,
  and the bearer capability is unset.

  Proof: the Submission suite passes twice under the race detector with
  shuffled order, and deadcode, staticcheck, and witness-lint report zero
  findings.

- published ordinary-CI execution for the complete authenticated GCS object
  lifecycle, 2026-08-12, published `v2026.0.66`. The official SDK constructor,
  exact create-only media and file uploads, provider metadata read-back,
  bounded integrity-bound reads, exact generation-safe deletion, and client
  close lifecycle now execute against a bounded hostile provider. Three
  ten-case tables pressure empty, one-byte, short, exact, and long sources;
  digest and CRC mismatch; provider absence, conflict, and refusal; destination
  failure; soft-delete ambiguity; generation corruption; and post-delete
  reappearance.

  Proof: package statement coverage rose from 54.4 percent to 83.9 percent;
  every previously unexecuted customer-byte effect leaf now measures between
  66.7 and 100 percent, and deadcode, staticcheck, and witness-lint report zero
  findings.

- published semantic fuzz authorities for all seventeen Distribution external
  decoder doors, 2026-08-12, published `v2026.0.65`. Sixteen targets cover the
  closed signing-domain parse/JSON pair plus every publication request, grant,
  completion, update request/response, upgrade request/grant, and commitment
  decoder. Accepted signed alterations run the real Publication, Update, or
  Upgrade verifier and must produce a typed verification/binding identity and
  an exact zero proof. Receive-only upload/download bearers remain undisclosed
  and are compared through their typed commitments.

  Proof: sixteen live campaigns executed 86,453 cases, the Distribution suite
  is clean, and deadcode, staticcheck, and witness-lint report zero findings.

- published semantic fuzz authorities for every Retrieval and Retrievalauth
  external decoder, 2026-08-12, published `v2026.0.64`. The request payload,
  request document, request commitment, grant payload, bearer grant document,
  and credentialed request doors now require typed JSON/retrieval refusals,
  transactional receiver preservation, validation, and canonical fixed points
  wherever the type owns a marshal boundary. Structurally valid alterations
  are carried into the real attestation, grant, or credential verifier and
  must yield the compiler-owned verification/binding identity with an exact
  zero proof. The receive-only grant bearer remains deliberately undisclosed;
  its oracle compares typed commitments and signed facts instead of adding a
  marshal escape hatch.

  Proof: six live campaigns executed 58,329 cases, the Retrieval and
  Retrievalauth suites are clean, and deadcode, staticcheck, and witness-lint
  report zero findings.

- published semantic fuzz authorities for all three Process external ingress
  doors, 2026-08-12, published `v2026.0.63`. Argument vectors prove exact
  ordered projection against independently evaluated cardinality, per-value,
  NUL, and aggregate limits. Exact environments independently classify name,
  separator, value, count, aggregate, and duplicate-name boundaries, then
  require byte-exact ordered projection. Effective environments validate every
  caller projection before comparing Primitive's result with Go's independent
  `os/exec.Cmd.Environ` last-value-wins authority, including the exact-empty
  versus inherited distinction. Every refusal requires
  `core.ErrProcessContract` and an exact zero result.

  Proof: three live campaigns executed more than 75,000, 86,000, and 100,000
  cases respectively; the complete Process suite is clean.

- published the remaining trust-boundary layer triads, 2026-08-12, published
  `v2026.0.62`. Controlplane's signed verification triad is now joined by local
  top-level positive, negative, and neutral triads in Wiring, ID, Deploy, and
  Filelock. Wiring proves a connected runtime graph, a typed missing dependency
  with zero manifest, and an absent graph that invents no root or components.
  ID carries both UUIDv7 and ULID through canonical construction and parse,
  rejects noncanonical text to typed zero identities, and proves zero values
  emit neither plausible text nor JSON. Deploy drives a complete release
  through a real TLS loopback provider, requires a typed zero-prefix failure on
  the first provider loss, and proves an absent plan makes zero requests.
  Filelock proves an exclusive hold, cancellation before any lock effect, and
  immediate contention as the neutral not-held outcome rather than fake
  failure.

  Proof: the complete Wiring, ID, Deploy, and Filelock suites are clean.

- published the complete Controlplane external-decoder sweep, 2026-08-12,
  published `v2026.0.61`. Eighteen semantic fuzz targets now cover the 22
  public parse and JSON ingress doors across registration, signed documents,
  check-in requests and responses, headers, usage windows and watermarks,
  product status, and signing domains. The six closed external enums derive
  their admitted values by exhausting all 256 backing values, seed every
  compiler-owned canonical token, and require typed parse/JSON refusal,
  receiver preservation, exact value recovery, and a stable canonical fixed
  point. Six unsigned structural doors add production-generated valid
  mutations and, whenever the structure becomes part of a signed document,
  run the real verifier and require authentic success or typed zero-proof
  refusal.

  The sweep found and fixed a production ownership defect rather than weakening
  its oracle: RegistrationPayload could reject a nested malformed header
  without preserving `core.ErrControlPlaneRegistration`. Its `Validate`
  boundary now retains both the owning Registration identity and every more
  specific nested, consistency, or outcome identity.

  Proof: live campaigns passed for all twelve new targets, including more than
  145,000 ProductStatus cases, 123,000 SigningDomain cases, 248,000 WorkUnit
  cases, and 245,000 Outcome cases; the complete Controlplane suite is clean.

- published complete signed Controlplane fuzz authorities, 2026-08-12,
  published `v2026.0.60`. The five signed-document fuzz targets now seed every
  load-bearing class named by the external-boundary contract: body facts,
  signing domain, signer, signature, body length, body digest, request nonce,
  account, and build. Invalid domains are derived by substituting JSON tokens
  marshaled from the closed compiler-owned enum, never copied text. Accepted
  structural mutations must fail the real Ed25519 verifier with a typed
  Controlplane identity and an exact zero sealed proof; authentic canonical
  documents must return a revalidating proof. Rejected decodes preserve the
  prior receiver projection exactly.

  Proof: all five live fuzz campaigns passed after gathering 35, 27, 95, 36,
  and 80 baseline cases respectively, and the complete Controlplane suite is
  clean.

- published hostile external-boundary proof, 2026-08-12, published
  `v2026.0.59`. Controlplane's installation certificate, certificate body,
  registration, check-in, and check-in-response decoders now carry live fuzz
  oracles over genuinely signed Ed25519 documents. Each oracle proves typed
  JSON and Controlplane rejection, receiver preservation, canonical fixed-point
  projection, authentic acceptance through the real verifier, and rejection of
  a structurally valid but cryptographically corrupted digest. A named local
  layer triad executes the positive, hostile, and zero-document paths of all
  four verification authorities.

  Filestore's oversize fuzz path no longer skips the boundary it was meant to
  attack: it streams the exact maximum-plus-one prefix through the real rooted
  writer and stage doors, requires the typed size identity, zero result, and no
  target or temporary artifact. Exchange proves the negative body-extent
  boundary with its typed contract identity. Chit and Retrieval now exhaust or
  fuzz their identifier, numeric, and signing-domain ingress with preservation
  and canonical round-trip oracles. Timeproof exhausts both closed enums and
  drives issuer-and-serial validation through exact, reordered, duplicated,
  foreign, truncated, negative, oversized, and trailing-data CMS identities.
  Distribution's three request commitments prove their exact domains, and its
  completion document proves both direct production projections and a stable
  structural round trip.

  Proof: all affected package suites and live fuzz campaigns are clean;
  `deadcode -test ./...`, `staticcheck ./...`, and `witness-lint ./...` are
  clean.

- published compiler-owned HTTP header values and restored structural
  ratchets, 2026-08-12, published `v2026.0.58` in one commit. `exchange.Header`
  no longer accepts `[]string`: each value is constructed through
  `NewHeaderValue`, rejects an unset zero, exact extent overflow and every
  untransmittable byte, redacts every formatting verb, and exposes bytes only
  through the explicit execution/provider `Value` projection. Request and
  response leaves consume the typed value; Cloudidentity and Objectstore now
  construct their provider-owned values at their own boundary. Captured
  external response metadata is raised into the same type before it can escape
  Exchange, and a malformed capture closes the response while returning no
  partial status, attempts, bytes, or headers.

  The semantic fuzz oracle compares arbitrary bytes against Go's independent
  HTTP grammar plus Exchange's compiler-owned extent, then requires exact
  typed errors, zero refusal, exact accepted projection, enclosing Header
  validity and five-verb redaction. A live five-second campaign executed more
  than 335,000 cases. The first focused Core proof also caught Wiring's omitted
  `ValidatedJSONMarshaler` witness; the witness now lives in its compiler-owned
  inventory file. Process's own architecture ratchet then exposed a local map
  and a four-parameter projection helper. The helper now receives one typed
  request, and exact-environment duplicate detection delegates linear
  last-value-wins identity to `os/exec` before refusing any cardinality
  collapse, with no Primitive-owned environment map or quadratic scan.

  Proof: the complete affected Exchange, Objectstore, Cloudidentity, Process,
  Core, Release, Currency, Payment, ID, Temporal, and Wiring suites are clean;
  `deadcode -test ./...`, `staticcheck ./...`, and `witness-lint ./...` are
  clean; the repository-wide suite reached and passed every package except the
  two Process structural red states that this same commit removed, after which
  the complete Process suite passed.

- published bounded runtime wiring proof, 2026-08-11, published
  `v2026.0.57` in commit `4a64457`. `wiring` derives one immutable connected,
  acyclic component graph from the actual runtime objects a command constructs
  and binds each component to exact compiler-owned Primitive package doors.
  Duplicate, missing, disconnected, cyclic, test-support, and invalid doors
  fail closed before a command reports ready; defensive iteration cannot mutate
  the stored graph.

- blind customer-custody, completed-upload, authenticated distribution, and
  provider namespace contracts, 2026-08-10, published as `v2026.0.56`.
  `chit` and `payment` now own device-signed all-or-specific catalog queries;
  `chitauth` and `paymentauth` bind those queries to the exact authenticated
  installation, device key, account, build, nonce, revision, and bounded page
  selection. `distributionauth` applies the same certificate-first binding to
  the product-neutral update and exact-candidate upgrade requests already
  owned by `distribution`. Controlwire owns the eleven exact product route
  families, including Chits, Payments, release publication and completion,
  update checks, and upgrades, so tools and OGS project the same paths without
  knowing one another.

  Submission gained the missing second half of an evidence upload. A device
  signs URL-free, bearer-free, path-free provider completion evidence against
  the exact original declaration, request commitment, upload-capability
  commitment, authority nonce, build, extent, SHA-256, CRC32C, and provider.
  `submissionauth` authenticates the same installation certificate and
  original request before its nominated device key may authenticate that
  completion. The verified result exposes only transfer facts appropriate for
  authority reconciliation; source content never enters the agreement.

  `gcsobjects` now provisions create-only buckets through the official Google
  Cloud Storage SDK from one typed request carrying a validated project,
  bucket, open provider location, and closed flat-or-hierarchical namespace.
  Existing buckets are typed conflicts and failures return zero evidence.
  Compiler-owned segment and prefix requests compose root prefixes, nested
  prefixes, and exact object names without copied slash conventions or fake
  directory objects. The flat namespace remains logical; hierarchical mode is
  selected explicitly at bucket creation.

  The testing protocol now makes semantic fuzzing mandatory at every external
  ingress and specifies compiler-produced valid seeds, typed rejection
  identities, receiver preservation, canonical fixed points, real signature
  verification, load-bearing mutations, exact resource ceilings, bounded
  oracle work, and retained red/green regressions. No-panic is explicitly not
  evidence. The identical protocol was copied to Kernel, Witness, Bug,
  Peachfuzz, Offgridsoftware, and Cleanlift. New fuzz targets exercise every
  new JSON/signature/certificate boundary plus the touched Objectstore
  transfer, capability and commitment decoders and every GCS project,
  location, bucket, object, prefix, cache, and segment parser. Hostile suites
  separately pin valid, rejected, and exact-boundary matrices, independent
  signed recombinations, zero-value neutrality, provider conflict,
  cancellation, and real local TLS/official-SDK paths.

  The release battery completed with clean Fix, Vet, Fieldalignment,
  production Gocyclo at 10, Goconst, NilAway, Errcheck, Staticcheck,
  test-root Deadcode, Govulncheck, Gosec, Witness-lint, the complete ordinary
  suite, and the doubled shuffled race suite. The production-only Deadcode
  invocation correctly reported that Primitive has no main packages; it
  reported no unreachable production symbol, and no fake executable was added
  to manufacture a zero exit. Every new semantic fuzz target was also run as
  a live campaign with its typed oracle and completed cleanly.

- published blind evidence-submission agreement, 2026-08-10, published
  `v2026.0.54` in one commit. `submission` now owns the one product-blind
  agreement shared by Bug, Witness, Peachfuzz, and their control-plane
  authority: a device signs an exact content type, nonzero byte length,
  SHA-256, CRC32C, build identity, revision, and request nonce; an authority
  signs the exact request commitment, a distinct nonzero authorization nonce,
  the upload-capability commitment, issued-at and expiry instants, and a
  retain-until promise. The separately transported bearer must match both the
  signed commitment and exact expiry, and a grant is admitted only from
  issuance through the nanosecond before expiry. `submissionauth` authenticates
  the authority-issued installation certificate before its nominated device
  key can authenticate the request. Neither package knows product evidence,
  plan policy, server implementation, or transfer mechanics; Objectstore
  remains the transfer owner and OGS remains the commercial decision owner.
  Controlwire gained the shared `/submissions` route family. The new
  `controlplanetest` test-support package constructs genuine authority-signed
  installation certificates through the real nested owners without granting a
  production dependency. The architecture catalog now records 29 production
  packages, two test-support packages, 90 production edges, and nine test-only
  edges. Process and Shutdown also consume Core-owned interrupt and terminate
  labels, retiring two duplicated literals without raising the constants
  admission ceiling.
  Proof includes every offering through the same request and credential path;
  wrong authority and device keys; independent signed-field, request,
  capability, and expiry substitutions; every lifetime member and both strict
  one-nanosecond boundaries; all 32 nonce byte positions; canonical digest and
  nonce attacks; ten accepted and more than twenty rejected framing cases at
  each document boundary; exact maximum-minus-one, maximum, and
  maximum-plus-one byte limits; nil and zero receivers; neutral issuance
  failures; receiver preservation; and compiler-visible data-flow inventories.
  Release proof: `go fix ./...`, `go vet ./...`, `staticcheck ./...`,
  `deadcode -test ./...`, `witness-lint ./...`, `nilaway ./...`, and
  `go test ./...` all completed cleanly. Additional preflight checks for
  formatting, field alignment, production complexity, constants, security,
  and vulnerabilities were clean. Consumer surgery remains deliberately
  outside Primitive: Witness gets an explicit source-free custody-proof upload
  command, Bug gets an explicit red/green process-testimony upload command,
  Peachfuzz invokes the same agreement automatically for its selected capture,
  and OGS mounts the authority handler that applies its existing plan and gate
  booleans before issuing a grant. Those four direct adapters are the next
  eligible frontier.

- published object address, 2026-08-09, published `v2026.0.53` in one commit.
  `gcsobjects` would publish an object and then decline to say where it had
  put it: `UploadMedia` exists to write "an object a browser or CDN will
  fetch", and `GCSObjectMetadata` returned bucket, name, generation, length,
  CRC32C, content type, cache control and three instants without ever naming
  the address. Consumers therefore rebuilt the provider URL from a copied host
  and two slashes, which is the projection rule broken by omission: the owner
  refused the address, so the address grew copies outside the owner. Two
  distros were about to grow the same one. `ObjectAddress(bucket, name)` and
  `GCSObjectMetadata.Address()` close it as pure value derivations that contact
  nothing and prove nothing about existence, and both state the limit the
  string cannot: an address is a name, not a grant, so a stored blob in a
  private bucket has a correct address that answers 403. Composition stays in
  `gcsobjects` because it owns the object; `core` gains only the two protocol
  values the composition is built from, and gains them because each already had
  two homes. `GoogleCloudStorageHost` was duplicated between `objectstore` host
  validation and this new address, and `"https"` was duplicated between
  `core.httpsSchemeText` and `objectstore.httpsScheme`; both are now single
  authorities that `objectstore` consumes. The first attempt put a
  `NewHTTPSEndpoint` composer in `core` and the export-ownership ratchet
  refused it at one named consumer, which is the rule doing exactly its job and
  is why the composition moved to its owner instead. Proof: a fourteen row
  address table pinning the exact rendered URL across flat and nested names,
  the shortest legal bucket, dotted and dashed buckets, and every character a
  path may not carry literally, so a space, a query marker, a fragment marker,
  a percent and non ascii each encode while the separator inside a nested name
  stays hierarchy; a refusal table proving a zero bucket, a zero name and both
  zero yield the object-store identity and the zero endpoint; zero metadata
  refused; a distinctness proof that two dated names never share an address and
  two objects in one bucket share an origin; and one mutation removing the name
  gate, which went red on the rejection arriving from the wrong owner rather
  than merely on an error being absent. Gates: `gofmt`, `go vet`, `staticcheck`,
  `nilaway`, `witness-lint`, `deadcode`, `gocyclo`, `fieldalignment`, and the
  full `core`, `objectstore` and `gcsobjects` suites.

- runnable-path door and gate debt, 2026-08-09, published `v2026.0.48` in
  three commits. Process gains `ResolveExecutable(ctx, path)`, the
  path-shaped twin of `Resolve` for the consumer holding a configured
  override, a sibling binary, or a derived build artifact: one
  `exec.LookPath` leaf carries the host's own runnability answer for this
  process's effective identity, the native nonexistence and permission
  identities stay reachable, and the answer is parsed rather than echoed
  because Windows may resolve an executable extension the caller did not
  spell. Proof: a twenty-five row portable table sweeping the mode spectrum
  on both sides of the execute boundary and every link state, each accepted
  row pinning the exact returned path; a unix leaf pinning EISDIR, ELOOP,
  and ENOTDIR; a windows leaf proving both extension spellings, checked
  under a windows vet; a gate-order row; a real Run round trip; and one
  compiled mutant killing exactly the thirty-two rows it must while the
  three pre-substrate gates survive. The standing gocyclo note closed:
  `DeleteGCSObject` delegates its generation observation to
  `currentGCSGeneration` and reads 8. The constants gate drifted red under
  the newer goconst, whose detection is complete where the admission set
  was written against less; twelve same-fact diagnostics now have one
  named spelling each across process, hostfacts, core, id, filelock, and
  release, and the four remaining findings are coincidental label
  collisions between distinct closed domains, admitted with reasons under
  a maximum ratcheted to exactly fifteen. The capabilities index also
  gains the filestore rows for `Remove`, `RemoveTree`, and `Walk` that
  shipped with their doors. Canonical gate all zero across eighty-four
  rows. Consumer surgery recorded, not performed: witness processprobe's
  path branch drops its Canonicalize plus Inspect existence gate for
  `ResolveExecutable`, restoring the resolve-time runnability deadcode's
  selection chain pins.

- the three remaining witness gaps, 2026-08-08, published `v2026.0.47` in
  one commit. Hostfacts gains `UserHomeDirectory` and `UserCacheDirectory`
  beside the existing config and temporary bases, same admission and same
  observation identity, and `ObserveDiskRotation(ctx, DiskRotationRequest)`
  answering the closed `DiskRotation` domain: rotational, non-rotational,
  unavailable when no single block device backs the directory, unsupported
  where no portable interface exists off Linux. The Linux probe takes the
  device identity from the same held root capability `AssessDisk` opens and
  resolves it through the kernel's own `/sys/dev/block` index, disk first
  and then the partition's parent, so no mount-table text and no
  device-name heuristic ever decides which disk a path lives on. Core gains
  `HTTPStatusOK()` beside the octet-stream constructor, dogfooded at
  objectstore's upload and download expectations and at cloudidentity's
  acquisition, whose dead error branch and lone `net/http` import die with
  it. That left the numeric constructor `NewHTTPStatusCode` with one named
  primitive consumer, so the admission moved onto the type as
  `HTTPStatusCode.AdmitInt`, the receiver unchanged on rejection; exchange
  admits response codes through it and the export-ownership gate holds two
  named consumers for every core export. Proof: full module suite bare;
  fix, vet, gofmt, fieldalignment, gocyclo, staticcheck, deadcode, and
  nilaway silent; Linux and Windows cross-builds; mutation reds for the
  wrong cache base, the lying 201 constructor, the swapped rotational
  tokens, and the lying off-Linux answer, each restored byte-identical.
  Consumer surgery recorded, not performed: witness exec/checks.go:2365,
  run/run.go:1700, testimony.go:51, cmd/witness/tools.go:247,431,839, and
  cmd/build-tools/build.go:359 ride the new bases; witness
  plan/machine_linux.go deletes its mountinfo walk and partition-suffix
  trimming for one `ObserveDiskRotation` call; witness notary, license
  source, updatecmd source, and cmd tools name `core.HTTPStatusOK()` where
  they currently spell `NewHTTPStatusCode(http.StatusOK)`, which the repin
  makes a compile break to fix in the same slice. Standing note: gocyclo
  reports `gcsobjects.DeleteGCSObject` at 11 on committed history from the
  `.42` split, outside this release's scope, left for its own slice.

- filestore held-standing door, 2026-08-08: the file-identity gap witness
  names at both of its TOCTOU custody checks closed as one portable
  observation inside filestore's inspection surface.
  `ObserveHeldStanding(ctx, held, path)` answers whether one absolute path
  still names the exact entry a held handle is open to, as the closed
  standing `HeldStandingSame`, `HeldStandingReplaced`, or
  `HeldStandingAbsent`. Identity rides `os.SameFile` over values the
  standard library already returned; the final component is never followed,
  a hard link to the held entry is the held entry, and a missing entry, a
  missing parent, and a non-directory parent are all the absent observation
  rather than errors, while a permission refusal stays an error. No new
  package, no new edges, no new error identity, and no request struct: the
  door is positional beside `Inspect` and `ObserveSharing`. Consumer
  surgery recorded, not performed: witness testimony
  `pidLockPathNamesFile` and ledgerfile `verifyRecoveryAppendIdentity`
  drop their raw stat plus `os.SameFile` pairs and act on the standing.
- id package, 2026-08-08: the time-ordered identifier gap named by three
  consumers at once closed as one new B3 value package. Bug carried a local
  UUIDv7 in its core, Witness mints `uuid.NewV7` for RunID with a dependency
  that draws entropy past keygen, and the Peachfuzz custody key is a ULID
  that crosses the control plane wire, which makes the mechanic Primitive's
  by the ownership law. `id` owns `Request{Observation, Entropy}`,
  `NewUUIDv7`, `ParseUUIDv7`, `NewULID`, and `ParseULID`: pure construction
  from one `temporal.Observation` and exactly-minimum `core.SecretMaterial`,
  canonical-text-only admission (lowercase dashed hex; uppercase Crockford
  with the leading ceiling), strict JSON round trip, and no effect leaf, so
  the clock and entropy effects stay with temporal and keygen. Core grew
  `PackageID` with `core` and `temporal` edges and the `ErrIDContract`
  identity under `ErrPrimitiveContract`. Consumer surgery recorded, not
  performed: bug deletes `internal/core/uuid_value.go` and retargets BugID
  and InvocationID, witness drops `google/uuid` at cli.go RunID minting,
  peachfuzz adopts `id.ULID` when the custody flow lands.
- objectstore to gcsobjects split, 2026-08-08: the authenticated GCS client
  from the entry below pulled the full `cloud.google.com/go/storage` SDK into
  every `objectstore` consumer, including local tools that only use the
  signed-capability surface. The authenticated lifecycle now lives in a new B4
  package, `gcsobjects`, so the SDK is confined to it and base `objectstore` is
  SDK-free again. `Integrity` and the exact-extent `ExactReader` are exported
  from `objectstore` and reused, not reimplemented. The single create-only
  write is replaced by two compiler-selected entry points on a
  served-versus-stored axis: `UploadMedia` (caller content type, optional cache
  directive) and `UploadFile` (application/octet-stream, no cache). Reworking
  the tests surfaced a real defect the single-write API had hidden: a stored
  file carries no cache directive, but the metadata read-back required one, so
  `UploadFile` could never succeed. Absent cache-control is now valid provider
  evidence on both the request and the metadata, proved red-then-green by an
  offline projection test over real provider attributes and error types. Base
  objectstore, gcsobjects, and core are battery-green; pending review and the
  next published revision.

- Hostfacts ambient trio, 2026-08-08: Bug's sweep surfaced the last three
  ambient host asks living in a consumer. `ObserveHostname` reports the
  platform's name bounded and control-free, `UserConfigDirectory` and
  `TemporaryDirectory` admit the standard library's per-user and scratch
  bases as absolute paths, with uniqueness for scratch names deliberately
  left to keygen so no door becomes a hidden entropy source. Oracle-pinned
  proofs and the surface ratchet extended. Same pending revision as the
  Windows observation doors.

- Windows observation doors, 2026-08-08: Bug's migration surfaced the two
  remaining direct `golang.org/x/sys` reaches in a consumer, both Windows
  observations with no POSIX kernel counterpart. `process.ObserveProcesses`
  walks the Toolhelp snapshot as typed sightings, and
  `filestore.ObserveSharing` answers whether another process holds a path
  against opening through a zero-share probing open, with the contention
  refusals as the held answer. POSIX hosts refuse both loudly, because the
  supported spelling there is composing ps or lsof through Process, and a
  lookalike would hide that composition. Windows leaves are cross-compiled
  and shape-ratcheted; executing them on a real Windows kernel joins the
  existing Windows-runner proof debt. Same pending revision as the keygen
  wire-form adoption.

- Keygen wire-form adoption, 2026-08-08: Bug's release material arrives as
  the 64-byte standard-library private key a server contract already speaks,
  and admitting it locally meant crypto/ed25519 size arithmetic in a
  consumer. `AdoptPrivateKey` now owns that admission: exact extent refusal,
  seed-half custody, trailing half re-derived rather than trusted, proven by
  a projection round trip, a forged-trailing-half derivation proof, and an
  extent refusal walk. Products with their own persisted form keep the seed
  and AdoptSigningKey. Pending the next published revision for Bug to pin.

- Authenticated GCS object lifecycle, 2026-08-08: Kernel's private profile
  photo path proved that signed upload capabilities are not the only neutral
  object-storage mechanism products need. `objectstore` now owns official-SDK
  client construction through a closed ADC/service-account selection, typed
  bucket, object, destructive-prefix, cache, and generation values,
  generation-zero create-only writes, SHA-256-bound bounded streaming reads,
  structured provider time metadata, exact-object deletion, and bounded
  generation-matched prefix deletion with a second observation as absence
  proof. Permanent deletion refuses any bucket
  whose soft-delete retention is active; product namespace, retention intent,
  retry, and reconciliation remain downstream. The serious admission surfaces
  have hostile tables over valid, rejected, and exact boundary values. A real
  GCS proof created and read both nonempty and zero-byte objects, observed a
  provider conflict, rejected short-source and wrong-digest writes without
  leaving objects, deleted two exact generations, proved both absent, and
  independently refused a bucket retaining soft-deleted objects.

- Process group-survivor sweep, 2026-08-08: Peachfuzz's migration onto
  Process proved the one supervision moment the doors could not reach: a
  group-contained tree whose reaped leader leaves survivors, either because
  cancellation raced member exits or because WaitDelay released the wait
  while a descendant held an inherited pipe. `Execution.Sweep` now owns that
  moment: group containment only, one hard stop to the group address, legal
  before or after reap, with ESRCH and EPERM as successful terminal outcomes
  because neither can be repaired by retrying. The direct child stays
  refused, because its stored number after reap is the recycled-identity
  delivery ProcessIdentity forbids, and the residual recycled-group window is
  documented as the reason a supervisor sweeps on evidence of survivors
  rather than as routine hygiene. Proof drives a real blocked descendant
  through the WaitDelay path, watches the sweep end it, holds the door shut
  for direct children and unstarted executions, and tolerates sweeping a
  group that is already gone. Published in v2026.0.38 so Peachfuzz can pin
  it.

- Keygen seed-custody round trip, 2026-08-08: the same migration proved the
  adopt-back door could not be fed. Keygen handed out only the 64-byte
  standard-library private key while accepting back only the RFC 8032 seed,
  so a product persisting a key bridged the asymmetry with crypto/ed25519
  size arithmetic of its own, exactly the reach-past the layering law
  forbids. `SigningKey.Seed` now projects a caller-owned copy of the live
  seed and `SeedSize` names the extent, completing the round trip: generate,
  persist the minimal secret, adopt it back. Custody rules unchanged; unset
  and destroyed keys refuse. Proof: full round trip preserves the public
  identity and the seed bytes, the size constant is pinned to the contract
  extent, and both refusal edges are held. Published in v2026.0.38 beside
  the Process sweep.

- Path ingress and canonical location, 2026-08-08: the Peachfuzz sweep found
  the last two path-shaped reach-pasts every consumer repeats.
  `AbsolutePath.ResolveText` admits operator-supplied text against a caller
  base with exactly lexical resolution, replacing the filepath.Abs-and-reparse
  shape whose hidden working-directory ask belongs to `process`; climbs clamp
  at the root, empty text refuses, and the hostile table walks both sides of
  the component-size and component-count boundaries. `filestore.Canonicalize`
  reports where an existing name really leads with every link resolved, the
  one answer an integrity comparison over two spellings needs, refusing
  absent paths and link loops with the native cause preserved. Surface
  ratchet extended; both proofs green. Published in v2026.0.38 beside the Process
  sweep and the Keygen seed round trip.

- Process self identity, 2026-08-08: lock-file diagnostics name their writer,
  and the only spelling of "who am I" was os.Getpid from a product.
  `process.Self` now answers with a validated ProcessIdentity, an observation
  of this process that grants no capability over it; ownership stays with the
  advisory mechanism guarding the record, because identifiers are reused.
  Proof pins the platform's own report as oracle and composes with `Alive`.
  Surface ratchet extended. Published in v2026.0.38 with the rest of today's
  doors.

- Ambient process facts, 2026-08-08: the last two calling-process asks left
  in a consumer were the binary's own path (for the supervisor unit an
  operator installs) and the inherited environment (filtered before a child
  request). `process.Executable` and `process.AmbientEnvironment` now own
  them; the whole-environment selector stays banned everywhere except the
  one named ambient leaf, following the signal-leaf pattern. Proofs pin the
  platform oracles, the typed admission, and the exact Strings round trip a
  filtering consumer performs. Published in v2026.0.38 with the rest of today's
  doors.

- Exchange standard client, 2026-08-08: NewClient demanded a *http.Client
  the package never produced, so every consumer wanting exactly the standard
  transport imported net/http to write an empty literal. `NewStandardClient`
  now produces that admitted shape; customized transports still enter
  through NewClient. Proof: the produced client validates and matches the
  hand-admitted literal's contract. Published in v2026.0.38 with the rest
  of today's doors.

- Canonical external-analyzer closure, 2026-08-03: Witness revision
  `v0.0.0-20260803211814-57582de85018` recognizes Primitive Process as the
  exact owner of the standard-library execution leaf and admits RFC 3161
  ESSCertID-v1 SHA-1 only through Timeproof's exact receiverless certificate
  identifier helper. The SHA-1 rule loses admission when the package, function
  signature, input expression, body shape, receiver, or weak-call count drifts;
  no waiver or consumer-owned capability lookalike exists. The published
  analyzer, rather than a local worktree build, reports zero findings. The
  complete canonical gate passes: ordinary and race suites, Vet, Staticcheck,
  Errcheck, NilAway, Witness-lint, production complexity, constant ownership,
  field alignment, security, vulnerability, enforced dead-code admissions,
  all fifty-one discovered benchmarks, and all thirty discovered fuzz targets.
  Process is therefore `DONE`; Upgrade remains dependency-blocked only on its
  separately recorded live-provider proof.

- Consumer text and linker-contract closure, 2026-08-03: Bug and Witness
  migration proved that Core's owned SHA-256 digest and Ed25519 public-key
  values need the standard `encoding.TextUnmarshaler` boundary, while release
  builders need compiler-owned linker symbols for Primitive's private embedded
  build-identity variables. Both text receivers accept only exact canonical
  lowercase hexadecimal, preserve the receiver on every rejection, refuse nil
  receivers through `ErrPrimitiveContract`, and reject wrong extents before
  converting caller input. CRC32C now applies the same pre-conversion extent
  gate. Release owns the package path, private variable-name tokens, and four
  exported `-X` symbols once; a structural ratchet fails when a new injected
  variable lacks a symbol or any symbol drifts from the compiler package path.
  Focused Core and Release tests, the complete ordinary suite, touched-package
  race proof, Vet, Staticcheck, Fieldalignment, Errcheck, and production
  complexity are green. The user approved this complete Primitive tree for
  publication in the current checkpoint.

- RFC 3161 authority-registry closure, 2026-08-03: Witness's product promise
  to race independent timestamp authorities exposed that Timeproof admitted
  only FreeTSA. Timeproof now owns a closed FreeTSA and DigiCert authority
  registry, each binding one typed authority, exact endpoint, policy OID, and
  embedded pinned root. The DigiCert root is verified against its reviewed
  SHA-256 fingerprint, serial, validity interval, and self-signature before it
  can enter verification. A live DigiCert response exposed the standards-valid
  CMS form that declares `rsaEncryption` beside a separate SHA-256 digest;
  Timeproof now maps only that explicit SHA-256/SHA-384/SHA-512 combination to
  Go's `crypto/x509` algorithms and continues to refuse SHA-1 and unknown
  combinations. The authentic captured response proves nonce, imprint, policy,
  chain, canonical JSON, and hostile cross-authority refusal. Witness consumes
  this registry rather than copying endpoints or trust facts and races the two
  bounded calls, accepts only the first fully verified proof, cancels and joins
  the loser, and retains both attempt records in deterministic authority order.
  Focused ordinary and race tests, Vet, Staticcheck, Fieldalignment, Errcheck,
  Gosec, Witness-lint without waivers, production complexity, and the complete
  Primitive ordinary suite are green. The user approved this reopening for
  publication in the current checkpoint.

- Standard-library signing-capability closure, 2026-08-03: Witness's external
  key-custody migration proved that Attest's raw `ed25519.PrivateKey` request
  excluded KMS and HSM implementations even though Go already owns the exact
  abstraction in `crypto.Signer`. `SignRequest` now accepts that standard
  interface as a clean break; ordinary Ed25519 private keys still traverse the
  same path and are defensively copied and cleared. One internal capability
  owner validates an Ed25519 public identity, canonicalizes and frames the
  body, gives the untrusted signer a fixed-extent copy of the frame, preserves
  provider errors, contains provider panics, validates the returned signature,
  and post-verifies against Primitive's original frame. Hostile proof rejects
  wrong public-key types and extents, mismatched identities, nil/short/long or
  corrupt signatures, callback panics, and a provider that mutates its input;
  the successful external-signer case round-trips through the real verifier.
  The compiler-owned crypto-effect and data-flow inventories include the new
  owner. Attest ordinary and race tests, Vet, Staticcheck, Fieldalignment,
  Errcheck, Gosec, production complexity, and the complete Primitive ordinary
  suite are green. The user approved this reopening for publication in the
  current checkpoint.

- Filestore special-file read closure, 2026-08-02: Bug's migration from a
  product-owned bounded reader to `filestore.Read` exposed that opening a FIFO
  before inspecting its mode can block forever. Filestore now uses the real
  rooted `os.Root.Stat` boundary to refuse an existing non-regular source
  before `Open`, then retains the handle-level `File.Stat` check before any
  bytes stream. A Darwin/Linux subprocess proof constructs a real FIFO and
  requires `ErrFilestoreSource` plus `fs.ErrInvalid`; its deadline is only a
  deadlock backstop for the previously wedged path. Focused race, vet,
  staticcheck, errcheck, field alignment, gosec, production complexity,
  Windows compilation, and strict current Witness-lint are clean.
- Consumer-driven HTTP fact closure, 2026-08-02: OGS migration exposed copied
  standard HTTP facts after Core's unused-export sweep. Exchange now owns
  closed `StandardHeader` and `StandardMediaType` domains for Authorization,
  Cache-Control, Retry-After, JSON, and plain text; Objectstore consumes the
  Authorization projection instead of retaining its own string. Timeproof now
  owns its RFC 3161 request and response `MediaType` domain so consumers can
  compose Exchange without copying protocol media strings. Every new domain
  exhausts the complete `uint8` space, returns its package's typed error and a
  zero projection for every invalid or future value, rejects projection
  collisions, proves `OffWireEnum`, and reaches the real `net/http.Header` or
  `mime.ParseMediaType` handoff. The canonical gate records 74 passing gates;
  only the already-recorded external Witness-lint Process boundary remains
  nonzero. All 51 benchmarks and all 30 semantic fuzz targets passed, including
  the two 10,000-execution real Filestore targets.
- Proof-machine and substrate-error closure, 2026-08-02: the canonical gate no
  longer carries a hand-maintained target list that stopped after Garble. It
  discovers the compiler's complete landed
  benchmark and fuzz inventories, records both as hashed evidence, ratchets
  their current minimum counts, runs every benchmark package serially, and
  gives every fuzz target one explicit serial budget. Parser and verifier
  targets receive 100,000 executions; Filestore's two real rooted-I/O targets
  receive 10,000 executions through a reasoned, exact, two-entry override
  ratchet after a live run proved the uniform budget exceeded their explicit
  test timeout. Gate failures are retained while later independent evidence
  phases continue, and the gate
  returns the first failure only after the complete manifest is written.
  Timeproof now joins every `encoding/asn1`, `crypto/x509`, and signature
  substrate error into `ErrTimeProofInvalid`; a public verifier test proves an
  `asn1.SyntaxError` remains reachable through `errors.As`. Exchange preserves
  method, media-type, and content-coding parser causes through its typed request
  and response identities. PLAN's single-purpose column is now a
  compiler-owned catalog projection with a synthetic drift ratchet for every
  package. Core's sixty-two self-projection admissions retain their fixed
  maximum and compiler witnesses and now carry one of two typed per-entry
  reasons: architecture catalog or external test-isolation analyzer ABI.
- Exchange product-protocol removal, 2026-08-02:
  the house API envelope, product failure codes, request-ID normalization,
  operator message, and remediation-tip protocol are deleted from Primitive.
  Repository tracing found no Primitive production consumer and extensive
  ownership in Kernel's API and transport packages. No shim or alias remains;
  the later Kernel upgrade must move that product protocol to its actual owner.
  Exchange now owns only bounded client and server policy over Go's real
  `net/http` boundary, as stated by the compiler catalog and PLAN projection.
- Shared reader-progress and committed-upgrade closure, 2026-08-02: Core
  reopens with one shared `ReaderConsecutiveEmptyReadMaximum`, proved by
  Exchange, Filestore, Hostfacts,
  and Process consumers. Each funnels repeated `(0, nil)` reads to Go's
  `io.ErrNoProgress` without replacing standard-library copy or framing
  ownership. Upgrade validates Objectstore provider/target binding before
  creating a slot or trial receipt, and after a durable selector commit it
  resolves the committed primary through a cleanup context so caller
  cancellation cannot turn committed success into a false failure. Release's
  clock bounds derive from Temporal's compiler constants rather than copied
  nanosecond arithmetic.
- Peachfuzz release-identity admission, 2026-08-02: Core's single closed
  `Offering` authority now admits `OfferingPeachfuzz` with canonical token
  `peachfuzz`. The full byte-domain ratchet proves that only Bug, Witness, and
  Peachfuzz are valid and round-trip through both text and JSON. A complete
  Peachfuzz `BuildIdentity` round trip and Release's signed artifact, manifest,
  Latest, and verification pipeline prove the new product identity reaches
  every Primitive-owned release boundary unchanged; Upgrade separately proves
  a newer same-platform Peachfuzz artifact pair is admissible. No consumer-side
  enum, alias, or compatibility path is admitted.
- Canonical local evidence, 2026-08-02: the post-Peachfuzz gate manifest at
  `.artifacts/gates/2026-08-02-world-class-audit-peachfuzz` records seventy-four
  passing gates and one blocked gate across 879 summed command-seconds. The
  ordinary and race suites, vet, staticcheck, errcheck, nilaway, complexity,
  exact constant and dead-code admissions, all-struct field alignment, gosec,
  govulncheck, all fifty-one benchmarks, and all thirty fuzz targets passed.
  Twenty-eight parser and protocol fuzz targets ran 100,000 executions each;
  Filestore's two rooted durable-I/O targets ran their ratcheted 10,000
  executions each. The sole nonzero row is Witness's two Process findings
  described below. Admission-path normalization is proved for Windows drive
  letters and separators, and the three-platform workflow ceiling is ninety
  minutes against this approximately fifteen-minute local manifest.
- External analyzer blocker, 2026-08-02: the pinned Witness analyzer still
  reports Process's sole real `exec.CommandContext` ownership boundary. Its
  accepted capability declaration is hard-coded to
  `github.com/offGridSoft/witness/internal/core`, which Go forbids Primitive
  from importing. Process therefore returns to `BLOCKED_BY_DEPENDENCY` instead
  of claiming `DONE`; no inaccessible import, ceremonial local lookalike, or
  waiver is introduced. Upgrade likewise remains `BLOCKED_BY_DEPENDENCY` until
  caller-supplied live provider capabilities permit PLAN's required remote
  proof.

- Prior checkpoint, 2026-08-02: the reviewed doctrine and fail-closed protocol
  sweeps were committed through `6b4acfc` and pushed to `main`. The current
  proof-machine and Peachfuzz Core reopening are a later slice.
  Entries below that say `uncommitted` record the state when their proof was
  captured, before this checkpoint authorization.

- Fail-closed protocol checkpoint, 2026-08-02: Objectstore now proves that each
  provider-labelled signed capability targets a vendor-controlled HTTPS data
  host, uses only the standard port, declares a bounded canonical signed-header
  set, signs every request field it sends, and sends every field the signature
  declares. Upload decode rejects missing provider checksum and create-only
  fields before a capability can cross the package boundary. Real TLS loopback
  tests preserve the production host boundary by redirecting transport dialing,
  not by admitting test hosts. Objectstore's copied status numbers now use
  `net/http`; two fuzz targets each passed 100,000 executions.
  PackageIdentity and PackageKind no longer route text through order-dependent
  switch partitions: compiler-sized tables own every canonical package and kind
  projection, and validation refuses any empty row before JSON or import-path
  output. The same immutable row-aware closure now covers Objectstore, Receipt,
  Timeproof RFC status/code facts, Release, Process, Shutdown, Contextstate,
  Cloudidentity, Hostfacts, Gate, Upgrade, Filestore, and shared Core platform
  identities. `core.UnknownEnumDiagnostic` is the one cross-package invalid
  enum projection; copied package constants are gone. Tiny `sync.OnceValues`
  caches around compiler constants were removed from Cloudidentity and
  Hostfacts; heavy certificate parsing, buffer reuse, synchronization, and the
  explicitly bounded strict-JSON type cache retain their distinct proved
  ownership. Filestore's concurrency proofs now share a ten-minute liveness
  backstop and keep a one-minute post-cancellation termination bound, avoiding
  a false performance assertion when the 10,000-writer proof runs under heavy
  repository load. Full ordinary and race suites pass when run as their
  canonical sequential phases; vet, Staticcheck, Errcheck, Nilaway, Gosec,
  Goconst, Fieldalignment, production gocyclo <= 10, and diff checks pass.
  Witness-lint is clean except for the two previously recorded findings at
  Process's real `exec.Command` ownership boundary; no shim, invented
  capability, or waiver was added. The user explicitly authorized committing
  and pushing this checkpoint; no tag or release was authorized.

- Repository transition: Primitive was copied from the complete Foundation
  worktree at `9ae5b28010b90a140cfaac0ee567034fd84a69b0` plus its local
  2026-07-28 substrate-pattern plan note. Its canonical module identity is now
  `github.com/deliri/primitive/v2026`. Historical evidence remains unchanged
  and therefore retains its original Foundation module paths. `_docs/primitive_policy.md` is a
  tracked architecture projection: Core parses its exact production and
  test-only import table and fails on catalog drift.
- Phase: all 21 admitted Primitive production packages are reviewed, accepted,
  and published. Receipt's admitted surface is one authenticated
  accepted-evidence fact plus one fixed-size monotonic watermark; it owns no
  transport, persistence, retry, provider, payment, pagination, scheduler, or
  customer-rendering behavior. Gate is the CLI-side new-work enforcement boundary
  over one authentic Lease assessment; current and continuity states permit,
  while not-yet-valid, expired, refused, and revoked states return a typed denial.
  OGS's control-plane authentication, route protection, authoritative
  operation authorization, and policy issuance remain separate server-owned
  gates. Existing-record inspection, registration, check-in, recovery,
  persistence, product command mapping, rendering, and customer-facing copy
  remain outside Primitive Gate. Upgrade was reviewed and published at
  `a74ad9a8b84c80f1027e930440ca2e96b8bbdb1f`. Process was reviewed,
  committed, and published at
  `15d64adc1ea5f6c932bcb1a587ecc4d4ed68d4d8`; Lease was reviewed, committed,
  and published at `c86bca9c93c2bbde390fb82291a7fa38db691201`;
  Fuzzfinder was reviewed, committed, and published at
  `6c38ac1623232a95f3d150b2aff414b516599ec5`.
  Shutdown was reviewed, committed, and published at
  `adaa7b49eb950c289625c64824e8d10fcfd03662`.
  Hostfacts, Cloudidentity, Objectstore, `timeproof`, `exchange`, `temporal`,
  and `filestore` are `DONE`; the remaining Process consumer now uses the typed
  Testserial declaration through a cataloged test-only edge.
- Witness-lint polish, 2026-08-01 (awaiting review, uncommitted): the tool pin
  is upgraded to `v0.0.0-20260802013301-1e3f30fb10c2`. Core, Filestore,
  Fuzzfinder, Garble, and Hostfacts closed enums now carry the complete
  `IsValid`, `String`, and `core.OffWireEnum` contract instead of acquiring
  invented JSON protocols. Compiler-sized label tables make a newly added
  member without a diagnostic fail closed. Exhaustive hostile ratchets walk
  every `uint8` value, prove typed rejection identity, unique admitted labels,
  one safe invalid label, and absence of JSON marshal/unmarshal methods.
  Process and Testserial test-protocol findings were corrected through typed
  assertions, context-first helper shape, actionable failure facts, and
  `testserial.Declare`; no error string match or serial waiver remains.
- Core doctrine re-review, 2026-08-01 through 2026-08-02 (awaiting review,
  uncommitted): PLAN's
  admission rule exposed two self-justifying contracts. The governance-document
  subsystem had no non-Core consumer and shipped 160 lines of production policy,
  four stable errors, and 434 lines of tests solely to verify its own testing
  document. It is removed; the project-local testing protocol remains the test
  doctrine, not a runtime Primitive capability. The attempted Process execution
  capability was also removed because its sole typed witness was ceremonial and
  the pinned analyzer did not recognize it. PLAN's exact graph is now a
  tracked, parsed projection including admitted test-only imports; synthetic
  missing and extra production/test edges prove the matcher red. Core's three
  remaining fuzz targets have canonical gate budgets, and Core now names its schema
  `LayerTriad`. Strict JSON's reflective field-name optimization is classified
  and bounded to 128 retained types and 1,024 derived field names per type;
  admission exhaustion falls back to the same standard-library reflection walk
  and cannot change correctness. A live AST ownership ratchet counts direct
  selectors and exactly one compiler-visible type-dependency hop from a directly
  consumed declaration. Explicitly typed enum members inherit their named
  domain's consumers because accepting the enum admits every member; a hostile
  projection test proves untyped facts and arbitrary dependencies inherit none.
  It rejects every new zero- or one-consumer non-error export, with no deferral
  mechanism or allowlist remaining. Fixed-size admission inventories bind
  PLAN's architecture catalog and the external test-isolation ABI to live Go
  identifiers instead of source filenames. Stable-error admission is limited to
  `ErrorIdentity` constants and requires both a named producer and an
  `errors.Is` caller-decision path; the earlier `go/types` implements-error and
  concrete-constructor exemptions are removed. Hard orphans are
  removed or internalized: the unused IP network subsystem,
  timestamp/CRL/cache-control helpers, unused HTTP/JSON and release bounds,
  public internal parsers, linker-symbol strings, and the unconsumed Peachfuzz
  offering. `ErrorIdentity.Matches` now uses a compiler-sized closed-domain
  stack with enqueue-time deduplication, and an independent recursive hostile
  oracle exhausts every identity pair. Stable error text is one
  `[errorIdentityLimit]string` compiler projection; the hand-ranged dispatch
  tree and its duplicate dead branch are gone. `ByteLength` explicitly owns the
  non-negative range through `math.MaxInt64`, matching Go's signed file, stream,
  and HTTP size domain. Construction and JSON ingress reject the first
  unsigned-only value; every Process, Filestore, Exchange, Objectstore,
  Hostfacts, Upgrade, and Receipt call site handles the fallible constructor
  without a shim. `ByteLength.Int64` validates that bound once and then performs
  the proved conversion directly; it no longer repeats the same range decision
  through `CheckedInt64FromUint64`. Stage and disk pressure policies delegate to the real bound
  instead of returning unconditional success, and stream completion preserves
  both the operation failure and a length overflow. Its bidirectional
  marshaler/validator witness inventory now proves a non-vacuous contract.
  Relative and absolute path admission share one validator, and
  component count and byte-size checks now strip the same standard-library
  volume prefix. Core remains standard-library-only. Ordinary and race Core
  tests pass at 87.7% statement coverage, with focused vet, staticcheck, errcheck,
  nilaway, Witness, goconst, fieldalignment, gosec, and formatting clean. The
  last one-consumer exports were removed during their owning package passes;
  no fake consumer or copied helper was introduced to satisfy the ratchet.
- Package-layer and Shutdown inventory closure, 2026-08-02 (awaiting review,
  uncommitted): Release, Upgrade, Shutdown, Currency, Garble, Keygen, and
  Contextstate now expose grep-visible `LayerTriad` proofs by naming existing
  hostile tests that exercise their owned layer boundaries. Shutdown now has a
  bidirectional compiler-typed inventory for all sixteen production structs;
  adding an unclassified struct or retaining a deleted classification fails.
  The testing protocol no longer describes the retired package-free design
  phase, and `AGENTS.md` now names tracked `_docs/primitive_policy.md` as implementation law.
- JSON witness closure, 2026-08-02 (awaiting review, uncommitted): witness
  completeness is no longer a Core-only filename convention. One repository
  ratchet scans every production file in all twenty-two catalog packages and
  requires each `MarshalJSON` receiver to own `Validate` and a compiler
  assertion to `core.ValidatedJSONMarshaler`; it also rejects a witness after
  its marshaler disappears. The red pass found forty-six missing assertions in
  Attest, Garble, Hostfacts, Fuzzfinder, Receipt, Release, Timeproof, and
  Upgrade. All are now typed assertions, and the prior duplicate Currency
  witness is removed.
- Release error ownership, 2026-08-02 (awaiting review, uncommitted): the typed
  offering mismatch detail and its constructor moved from Core to their sole
  Release owner. `OfferingMismatchError` still exposes exact observed and
  expected offerings, unwraps to `core.ErrReleaseVerification`, rejects invalid
  or equal facts, and remains reachable through `errors.As`; Core no longer
  carries either one-consumer export or the obsolete data-flow classification.
- Release proof-projection correction, 2026-08-02 (awaiting review,
  uncommitted): `VerifiedManifest` and `VerifiedLatest` validate their private
  authentication seal once at operation ingress and expose immutable value
  projections without a second unreachable error channel. All Release and
  Upgrade call sites moved in the same clean cut; the prior mixture of ignored
  and checked accessor errors is gone. `AvailableSummary.Validate` reconstructs
  the claimed artifact through `NewArtifact` and compares the constructor-derived
  identity instead of setting Artifact's private construction bit itself.
- Release request-ownership correction, 2026-08-02 (awaiting review,
  uncommitted): all ten exported Release request types now own `Validate` and
  carry compiler assertions to `core.Validatable`. Evaluate, assessment,
  advance, artifact, manifest, signing, and verification entry points funnel
  through those receiver-owned checks instead of maintaining independent
  field-validation prologues. Attest remains the sole owner of private-key and
  trusted-key closure; Release delegates those fields through Attest's typed
  request contracts. Release, Upgrade, race, vet, staticcheck, errcheck,
  nilaway, gosec, field-alignment, and the 100,000-execution Latest-document
  semantic fuzz budget pass. Focused Release statement coverage is 82.8%.
- Fuzzfinder top-down recheck, 2026-08-02 (awaiting review, uncommitted): the
  package remains one rooted, batched directory observation that retains a
  fixed canonical prefix and never descends or claims custody. Go's `os.Root`
  directory path resolves authoritative entry metadata through the standard
  library, including filesystems whose native directory entries omit type
  information. Artifact kind validation, diagnostics, and parsing now project
  from one compiler-sized token table, so adding a member without its contract
  fails closed rather than silently emitting `unknown`. Ordinary, race, static
  analysis, and the 100,000-execution generated-name semantic fuzz budget pass
  at 97.1% statement coverage. The real-directory benchmark measures 335,388
  ns/op and 48,197 B/op for 128 entries; a one-iteration 8,192-entry pressure
  run measures 24,515,000 ns/op and 2,982,480 B/op while retained storage stays
  fixed at the caller's bounded prefix.
- Timeproof canonical-CMS correction, 2026-08-02 (awaiting review,
  uncommitted): a red-first hostile ESSCertIDv2 case proved that the verifier
  accepted an explicitly encoded SHA-256 `hashAlgorithm` even though RFC 5035
  declares SHA-256 as the field default and DER requires default-valued
  sequence components to be omitted. The verifier now rejects that alternate
  spelling. The certificate-conflict comment now states the verified-chain
  scope the loop actually proves, while signer selection remains the separate
  embedded-certificate ambiguity gate. The X.509 GeneralName directoryName
  choice index is a named protocol constant rather than a bare `4`. Ordinary,
  race, vet, staticcheck, errcheck, nilaway, gosec, field-alignment, production
  cyclomatic, and the 100,000-execution authentic-response semantic fuzz budget
  pass. Focused Timeproof statement coverage is 79.0%.
- Lease top-down recheck, 2026-08-02 (awaiting review, uncommitted): the live
  package already closes the reconstruction's two admission blockers. Verify
  authenticates against caller-selected Attest keys and binds the exact
  expected product, entitlement, and device subject; Evaluate merges the
  consumer's durable high-water with the signed issuance floor and Go's real
  monotonic elapsed observation before classifying work. No Lease clock,
  persistence, transport, or product state machine was introduced. State,
  contact, advance, and signing-domain diagnostics now project from
  compiler-sized tables, so an added member without its contract fails closed.
  Clock contradiction compares the already validated nonnegative Temporal
  duration directly against the positive compiler constant; the prior
  construction of that same constant through a fallible API and its
  unreachable error branch are gone. Lease, Gate, race, vet, and staticcheck
  pass; the remainder of the package sweep continues after this checkpoint.
- Lease typed-failure ownership, 2026-08-02 (awaiting review, uncommitted):
  `ScopeMismatch` and `ClockContradiction` are sealed Lease capabilities rather
  than caller-constructible public structs. Package-owned constructors prove
  two valid unequal subjects or a valid wall rollback strictly beyond the
  five-minute tolerance before the specialized Core identity can escape.
  Invalid constructors and even package-internal zero implementations carry
  only `ErrLeaseContract`; authentic reports preserve their exact facts through
  typed `Subjects` and `Instants` projections. A red-first hostile test captured
  the former forgery, including an eight-microsecond value that falsely claimed
  a five-minute clock contradiction. Lease ordinary, race, vet, staticcheck,
  Witness, and production-complexity proof pass.
- Process bounded-write convergence, 2026-08-02 (awaiting review, uncommitted):
  the child-output limiter and public truncating writer retain their distinct
  policies—cancel on overflow versus consume and count dropped bytes—but now
  funnel all admitted-prefix forwarding through one package-private rule. A
  red-first hostile destination proved the child limiter called caller code
  with an empty slice after its bound was exactly full, allowing a legal empty
  write error to replace `ErrProcessOutputLimit` and cancel for the wrong
  reason. Empty prefixes no longer reach caller code; invalid and short counts
  have one implementation. The divergent addressability guard and the
  post-forward count check were semantically unreachable and are removed.
- Process top-down completion, 2026-08-02 (awaiting review, uncommitted): the
  effective-environment projection no longer rebuilds `os/exec`'s
  last-value-wins rule with an O(n-squared) suffix scan. Primitive first rejects
  malformed and NUL-containing caller entries, then delegates canonicalization
  and OS-specific name handling to `exec.Cmd.Environ` before raising the result
  into typed variables. A red-first writer test also proved an impossible count
  discarded a simultaneous native error; the shared forwarder now preserves
  both `io.ErrShortWrite` and the caller cause, matching the reader boundary.
  `Failure`, `StreamFailure`, and `OutputLimitExceeded` are sealed
  Process-produced reports with typed fact projections. Invalid constructors
  and internal zero implementations carry only `ErrProcessContract`; the former
  zero `Failure` can no longer silently identify itself as a wait failure.
  Output policies are bounded by `math.MaxInt64`, the same compiler-owned
  `ByteLength` domain used by every returned count, so a successful observation
  is always representable. Stdin and dropped-byte accounting reject before
  leaving that domain, and redundant checked conversions are removed. All
  three closed Process enums project diagnostics and failure identities from
  compiler-sized immutable tables. Real-child ordinary and race suites pass at
  90.9% statement coverage; vet, staticcheck, formatting, and production
  cyclomatic complexity are clean. Witness continues to report only the two
  previously recorded analyzer capability findings at Process's legitimate
  `os/exec` ownership boundary; no shim or waiver was added.
- Contextstate and Release dead-path correction, 2026-08-02 (awaiting review,
  uncommitted): `classifyContextTerminal` now only compares a known non-nil
  error against Go's two exact terminal sentinels. Its unreachable nil arm and
  impossible comparison panic recovery are removed; the reachable recovery
  remains solely around caller-controlled `Context.Err`. Hostile test names now
  describe rejection without custom identity traversal rather than claiming a
  panic occurred. Release's SHA-256 identity framing documents the standard
  library's infallible `hash.Hash.Write` contract and the fixed NUL-free domain
  separator that makes discarded write errors safe. Focused ordinary and race
  tests plus vet, staticcheck, and production complexity pass; the full
  repository test remains green.
- Shutdown top-down recheck, 2026-08-02 (awaiting review, uncommitted): the live
  redesign satisfies its interview blockers with five typed phases, per-step
  cooperative budgets clipped by one total budget, fixed sixty-four-step
  custody, structured outcomes, and one joined signal-subscription goroutine.
  A red-first hostile test proved an unauthentic zero `SignalCause` failed
  validation yet still claimed `ErrShutdownSignalReceived`; its private
  authenticity seal now governs `Error` and `Unwrap`, so only controller-made
  causes carry that identity. Watch performs its hostile context ingress once
  instead of re-reading caller behavior after signal registration. All seven
  enum domains, internal diagnostics, and step-outcome identities project from
  keyed compiler-sized functions rather than mutable positional tables.
  Grace expiry no longer constructs its own raw timer: immediately after the
  first signal, the sole controller goroutine creates a detached
  `temporal.WithTimeout` context and selects on its `Done` channel, preserving
  the parent-cancellation and second-signal branches without another goroutine.
  The exact-import and forbidden-selector ratchets now reject raw timer
  ownership in Shutdown. Ordinary and race suites pass at 92.2% statement
  coverage; vet, staticcheck, formatting, and production complexity are clean.
- Fail-closed internal discriminator correction, 2026-08-02 (awaiting review,
  uncommitted): Hostfacts' cgroup-level memory declaration no longer assigns
  meaningful observed absence to the Go zero value. Its state domain now names
  and rejects an explicit unknown zero; absent, finite, and unlimited are
  distinct admitted members with compiler-sized diagnostics and a typed
  off-wire witness. Core's strict-JSON container kind likewise names unknown
  and limit sentinels instead of leaving an anonymous `iota + 1` hole. Stack
  admission validates the kind, and value completion switches explicitly over
  object and array before rejecting every other state rather than defaulting to
  array item accounting. Red-first hostile tests captured both former defaults,
  and exhaustive 256-value ratchets prove the closed domains. Focused ordinary,
  race, vet, and staticcheck proof passes for Core, Hostfacts, and Shutdown.
- Receipt top-down recheck, 2026-08-02 (awaiting review, uncommitted): the live
  package remains the deliberately reduced accepted-evidence and pure
  watermark contract; none of the archived Store, payment, pagination,
  transport, or persistence world was restored. A red-first hostile proof
  exposed that Receipt's two private sealed rejection implementations failed
  validation at zero yet still claimed the specialized scope or conflict
  identity. `Error` and `Unwrap` now expose those identities only after the
  typed field or reason validates; invalid internal values carry only
  `ErrReceiptContract`. Revision, scope-field, advance-state, and conflict-reason
  diagnostics now come from keyed compiler-sized functions rather than mutable
  package-global positional arrays. Focused ordinary and race suites pass at
  93.3% statement coverage; vet, staticcheck, errcheck, nilaway, Witness,
  gosec, goconst, field alignment, formatting, production complexity, and the
  100,000-execution signed-document semantic fuzz budget are clean.
- Filestore fail-closed discriminator correction, 2026-08-02 (awaiting review,
  uncommitted): the two internal execution enums no longer leave zero unnamed.
  `streamDestination` validates before streaming and its error classifier
  switches explicitly between caller destination and file activation; an
  unknown value carries only the Filestore contract plus the native cause.
  `directoryPosition` validates before touching the rooted namespace, so an
  unset value cannot silently choose intermediate-directory mode checking over
  final-directory mode synchronization. Both domains have named unknown and
  limit members, compiler-sized diagnostics, typed validation, and exhaustive
  256-value hostile proof. The unrelated `mkdir` operation token moved outside
  the enum declaration. Exported `WalkDirective` and `WalkOrder` now name their
  invalid zero while preserving the same admitted values and off-wire behavior.
  Focused ordinary, race, vet, staticcheck, and Witness proof passes.
- Core ownership closure, 2026-08-02 (awaiting review, uncommitted): the
  one-consumer deferral count is zero and the deferral machinery itself is
  deleted. HTTP method parsing, its closed method/replay lattice, content
  coding, declared body extent, JSON media type, and Exchange-only HTTP facts
  now belong to Exchange. Objectstore owns its vendor-only header names;
  Temporal owns signed instant persistence parsing. Core's CRC32C text decoder
  is an `encoding.TextUnmarshaler` boundary rather than a second exported
  parser. Core's path bounds are private; Filestore no longer restates Core's
  component boundary, and Hostfacts sizes `GetFinalPathNameByHandle` storage
  from the Windows API's returned requirement instead of coupling an OS buffer
  to lexical path policy. Shared Platform, Offering, ReleaseVersion,
  BuildCommit, and BuildIdentity contracts remain in Core for Release and
  Upgrade. Only the one-consumer embedded link-time observation moved to
  Release, where canonical strings enter through the owning Core types'
  standard `encoding.TextUnmarshaler` boundaries.
  Runtime-platform fixture API and the redundant Platform constructor were
  deleted because neither had a production caller. Exchange's HTTP method now
  owns exact JSON projection, hostile closed-domain proof, and the migrated
  declared-body-length fuzz target. Ordinary and race repository tests, vet,
  staticcheck, errcheck, and nilaway pass. Repository Witness analysis now
  reports only Process's two previously recorded external-capability findings;
  it no longer reports any enum, error-format, or shadowing finding from this
  slice.
- Attest top-down recheck, 2026-08-01 through 2026-08-02 (awaiting review,
  uncommitted): the
  package remains one bounded Ed25519 sealing/verification mechanic over a
  consumer-owned streaming canonical body. Production has no mutable global,
  map, reflection, whole-body read, goroutine, clock, filesystem, network, or
  crypto implementation beside the direct standard-library Ed25519/SHA-256
  leaves. Envelope, domain, and signature JSON now funnel through Core's single
  standard-library canonical encoder/string-token contracts, making
  `MarshalCanonicalJSONDocument` genuinely shared by Attest and Exchange and
  removing its Core ownership deferral. `CanonicalObject` now admits the field
  name and remaining object extent before invoking a nested owner or encoding a
  string. Arbitrarily large string input therefore cannot allocate before the
  one-mebibyte contract rejects it, and no encoded member can grow the retained
  object beyond that ceiling. Signed and unsigned decimal members use fixed
  20-byte stack storage and the reused-scalar benchmark remains zero-allocation.
  SHA-256 finalization appends into fixed digest storage instead of allocating a
  second sum, and envelope JSON-limit construction propagates its typed error
  rather than silently substituting an invalid zero policy. The canonical-object
  mutation fuzzer has a canonical 100,000-execution gate budget; a fresh
  100,000-execution run passes. Ordinary and race tests pass at 90.4% statement coverage;
  vet, staticcheck, errcheck, nilaway, Witness, goconst, fieldalignment, gosec,
  formatting, and production `gocyclo <= 10` are clean.
- Process implementation proof, 2026-07-30: the public contract is one typed
  command request over caller-owned `io.Reader` and `io.Writer` streams. It
  lowers exact argv and environment values only at the `os/exec` boundary,
  forwards stdout and stderr under independent byte bounds, retains only
  fixed-size counters and typed failures, and returns nonzero child exit as an
  observation rather than an infrastructure error. Structural ratchets reject
  production maps, goroutines, retained byte slices, whole-output helpers,
  raw syscall/signal paths, and functions with four or more loose parameters.
  A real-child hostile stream test found that an invalid `io.Reader` count
  could panic inside `os/exec` and terminate the program; Process now rejects
  impossible reader and writer counts as typed stream failures without false
  byte accounting. Process has no fuzz target: it owns no parser or wire
  boundary, its closed enum backing spaces are exhausted, and its variable
  stream boundary is pressured through real child processes.
- Process hostile re-review, 2026-07-30: a second pass under
  `_docs/testing_protocol.md` fixed two production defects and closed five
  untested contract surfaces.
  - Native wait error destroyed on cancellation. When this package's kill
    signaled the direct child, `waitCommand` reported only `parent.Err()` and
    discarded the `os/exec` wait error, so a caller could not reach the native
    `*exec.ExitError` and could not observe how the child actually terminated.
    That contradicts the section 1 and section 7 requirement that native errors
    remain reachable. The cause is now `errors.Join(parent.Err(), waitErr)`, so
    the context cause and the native error are both reachable. Proved red by
    reverting the join with the new assertion in place.
  - Silent error discard at bounded-writer construction. `newBoundedWriter`
    resolved the output bound with `maximum, _ := limit.Uint64()`, discarding a
    validation error and caching a duplicate of a fact `core.ByteCount` already
    owns. The bound is now resolved from the owning type inside `Write` and
    reported as a stream failure if it ever fails, which deleted the duplicate
    field, the `boundedWriterRequest` carrier, and its constructor. The shared
    output mutex now has one documented owner in `newCommandStreams`.
  - Untested contracts now proved with real children: exact argv lowering
    across 20 hostile shapes including empty, quoted, shell-metacharacter,
    glob, redirection, flag-shaped, invalid UTF-8, and every admitted byte
    value; ambient inheritance versus exact-environment withholding; the exit
    code domain from 0 through 255; stdin streamed to one mebibyte with exact
    accounting; independent stdout and stderr bounds including simultaneous
    breach; one writer shared by both output streams; and `WaitDelay` bounding
    a lingering descendant with the native `exec.ErrWaitDelay` reachable.
  - Structural ratchets strengthened. The import blocklist became an exact
    allowlist over the section 3 frontier plus the standard library actually
    used, so any new edge fails rather than only the edges already imagined.
    The forbidden-selector scan became package-qualified, which removed a
    false positive on the typed `Command` field and added `exec.Command`,
    `exec.LookPath`, ambient environment reads, and raw process and signal
    paths. That matcher now carries its own synthetic red-state table.
  - Test defects removed: loops that produced duplicate generated subtest
    names instead of naming their boundary; a terminal-context test that
    asserted refusal without proving no child ran; a cancellation test whose
    child lifetime equaled its own deadlock backstop; a `zeroReader` fixture
    that actually emitted `'x'`; and diagnostic assertions that checked only
    for a nonempty string rather than the operator-facing detail.
  - One claim in the first proof was wrong and is corrected here: on the
    cancellation path `os/exec` gives the process `ExitError` precedence over
    the copy error, so `exec.ErrWaitDelay` is not nameable there. The delay
    still bounds the wait, and the lingering-descendant test asserts
    termination rather than an identity the substrate does not provide.
  Focused tests, full race, shuffled sweeps, vet, staticcheck, errcheck,
  fieldalignment, and production gocyclo (maximum 9) pass. Statement coverage
  is 93.9 percent; every remaining uncovered statement is a defensive guard on
  a state unconstructible from outside the module. Benchmarks on Apple M1 Max
  at 30 iterations report stdout 64 KiB at 3,706,253 ns/op, 114,128 B/op, 81
  allocs/op and 1 MiB at 4,211,589 ns/op, 113,529 B/op, 80 allocs/op; stdin
  64 KiB at 3,673,936 ns/op, 113,654 B/op, 80 allocs/op and 1 MiB at
  4,157,682 ns/op, 113,282 B/op, 80 allocs/op. Allocation is flat on both the
  output and the newly measured input path while streamed extent grows 16
  times.
- Process serial-helper correction, 2026-08-01: `TestProcessHelper` remains the
  child-process entry point and cannot call `t.Parallel` before its selected
  `os.Exit` path. It now calls `testserial.Declare` as its first statement with
  the Core-owned process-output/package-process declaration. Core's architecture
  catalog admits `process -> testserial` only as a test-source edge; the
  production frontier and coupling coefficient remain unchanged.
- Process Witness boundary, 2026-08-01: the upgraded analyzer clears every
  prior enum and Testserial finding but still reports its two execution rules
  at the one
  `exec.CommandContext` leaf because capability recognition is hardcoded to
  `github.com/offGridSoft/witness/internal/core`, an internal package Primitive
  cannot legally import. Constructing `exec.Cmd` manually would duplicate and
  bypass the standard library; importing Foundation, adding a waiver, or
  copying a ceremonial capability would violate the clean break. The remaining
  pair is therefore one upstream analyzer contract defect, not a Primitive
  execution defect. Witness remains outside this Primitive-only slice.
- Release contract, 2026-07-30: Core now owns the currently consumed closed Bug
  and Witness offering domain; exact three-`uint32` release ordering; canonical
  SHA-1/SHA-256 build commits; immutable offering/version/commit/platform build
  identity. Release reads installed
  identity only from Core's link-injected binary facts. A Manifest signs
  exactly four ordered artifacts for Windows amd64, Darwin arm64, Linux amd64,
  and Linux arm64, including complete byte integrity and a checked signed total
  extent. Latest signs the exact nested Manifest document, generation, issue
  instant, validity interval, and stable offering stream identity. Pure
  selection absorbs the former Update purpose and yields current, available,
  refresh-required, or reassess-at. Only a freshly current
  `PreparedRelease` carries authority into the later Upgrade package; value
  summaries are validated but non-authoritative.
- Release archive review: all six concrete defects or ownership failures were
  verified against the preserved implementation. Equal-generation advance
  compared only the Latest fact and could accept a differently signed
  document; higher generations did not ratchet all signed timeline bounds; the
  alleged per-target authorization set did not name a target; the specification
  promised total artifact extent that was neither signed nor checked; Update
  accepted caller-constructed installed identity as provenance; and the archive
  retained build-tool/authentication workflow with no current Release or
  Upgrade reader. The new contract compares complete signed documents, ratchets
  issue/from/until before version, signs checked total extent, obtains installed
  identity from the binary, and deletes the unused authorization and build
  workflow surfaces. Identity and offering conflicts precede generation and
  version order. Signer rotation is accepted only through a strictly later
  document where the complete retained contract permits it; it is not treated
  as an equal-generation replay.
- Release acceptance review: four additional live findings were verified and
  corrected. `ArtifactSet.UnmarshalJSON` decoded through a fixed Go array and
  silently discarded trailing signed artifacts; it now decodes a bounded
  slice and requires exact `TargetCount` cardinality. Manifest and Latest
  offering mismatches previously wrapped a nil cause and produced no useful
  distinction from malformed documents; Core now owns the typed
  `ReleaseOfferingMismatchError` with exact observed and expected offerings,
  reachable through both `errors.As` and `ErrReleaseVerification`.
  `newArtifactSetUnchecked` duplicated set closure, used untyped diagnostics,
  and discarded extent failure; it is deleted in favor of the single
  `artifactSetTotalExtent` owner, and validation names a signed-total mismatch
  directly. `ManifestFact.Validate` repeatedly called `ArtifactSet.At`, which
  revalidated and rehashed the complete set once per slot immediately after
  the set had already been validated; validated slots are now traversed
  directly.
- Release acceptance design: `VerifiedManifest` and `VerifiedLatest` are
  private-witness values with no external constructor, decoder, mutator, or
  exported field. Real document validation, offering closure, Attest
  verification, and document-digest construction run once before a private
  typed seal is issued. Their public `Validate` methods now check that seal in
  O(1) instead of re-marshaling and re-hashing immutable authenticated state on
  every accessor. A benchmark ratchet requires zero allocations on the
  assessment path. The per-wire empirical JSON field/depth numbers are
  deleted; every Release decoder uses one package-owned admission contract
  over Core's compiler-owned global structure limits, Release's 64 KiB
  document bound, and exact four-item array bound. `Artifact` now carries an
  explicit constructor bit rather than depending on its zero digest to reject
  the zero value. The public `Evaluate` entry is exercised against a real
  unstamped test binary and returns `ErrReleaseConflict`; successful
  link-stamped selection remains consumer integration proof.
- Release proof: focused statement coverage is 81.8 percent; Core is 88.1
  percent. Exhaustive byte-domain enum tests, numeric generation extremes,
  exact 24-hour lifetime and five-minute correction edges, every selection and
  preparation boundary, signed-document identity under signer rotation,
  authority separation, strict JSON mutation and receiver preservation,
  artifact closure, overflow, zero-value capabilities, compiler-owned embedded
  identity provenance, and complete proof rebinding pass. Full tests, full
  race, ten shuffled focused repetitions, vet, staticcheck, errcheck, nilaway,
  fieldalignment, goconst, gosec, govulncheck, actionlint, production
  `gocyclo <= 10`, formatting, module tidiness, and diff checks pass. The
  signed Latest JSON fuzz boundary completed 92,271 executions in 30 seconds
  without failure after its corpus expanded during review. Windows amd64 and
  Linux amd64/arm64 test binaries
  cross-compile; Darwin arm64 is the native test platform. Direct Release
  Witness analysis is clean with no waiver. The canonical gate passes through
  full tests, full race, vet, staticcheck, errcheck, and nilaway, then stops at
  the recorded repository-wide Witness enum, Process, and Testserial baseline.
- Release M1 Max median benchmarks across five acceptance runs: complete
  Latest verification is 1,272,646 ns/op, 524,260 B/op, and 11,067 allocs/op;
  pure Latest assessment is 370.5 ns/op, zero B/op, and zero allocations; and
  strict Latest document JSON is 489,936 ns/op, 215,380 B/op, and 4,348
  allocations/op. The original multi-millisecond, tens-of-thousands-of-
  allocations assessment numbers were not merely honest fixed-size cost: they
  exposed repeated quadratic-style closure work across nested accessors. The
  corrected numbers measure the shipped paths over the fixed 64 KiB document
  ceiling and make no throughput claim. The package has no variable-size
  stream or I/O effect and retains only fixed arrays and bounded canonical
  documents, so its memory bound is constant in admitted input extent.
- Shutdown review state, 2026-07-30: the preserved implementation's useful
  mechanics were retained only where they ride directly on Go. The archive's
  unphased LIFO list could not express lifecycle order; its one step budget
  duplicated distinct consumer cleanup bounds; and its synchronous force
  callback made `Controller.Close` permanently unjoinable when arbitrary Go
  code ignored cancellation. The new Plan has five closed phases:
  stop-admission, drain, persist, flush, and release. Phases execute in order,
  steps execute LIFO within their phase, every step owns its own cooperative
  Temporal budget, and one total budget clips the complete run. A fixed
  64-entry array owns registration and another fixed 64-entry array owns the
  immutable report. No callback runs in a helper goroutine.
- Shutdown signal design: one Controller owns one real `os/signal`
  subscription, one fixed two-event channel, and one goroutine. The first
  supported signal cancels its context with an authentic typed SignalCause.
  A second supported signal or real Go timer may publish one sealed Escalation
  fact, but Shutdown never runs force work, exits a process, or invents a
  process supervisor. Signal release completes before escalation publication,
  so the composition root receives policy authority only after Primitive has
  stopped observation. Parent cancellation and concurrent Close always join
  the owned goroutine. Stable step, total-timeout, signal-source, and
  signal-received identities are Core-owned.
- Shutdown hostile proof: all seven closed enum domains exhaust all 256 backing
  values and are compiler-marked off-wire. Plan tests pressure zero,
  one-nanosecond, maximum-duration, longer-than-total clipping, all five
  phases, mixed registration order, duplicate identities, 64 concurrent
  registrations, one-over capacity, callback reentry, terminal-parent
  detachment, panic containment, step timeout, total timeout, and accounting
  for every skipped step. Controller tests pressure invalid policy
  cross-products, unknown signals, closed sources, parent cancellation, first
  and second signals, exact release-before-escalation ordering, seven-minute
  grace expiry under Go `testing/synctest`, 64 concurrent Close calls, forged
  values, and typed nils. Real child processes prove SIGINT, SIGTERM, and
  SIGHUP through public Watch on Darwin. Windows amd64 and Linux amd64/arm64
  test binaries cross-compile. Focused statement coverage is 92.9 percent.
- Shutdown review corrections, 2026-07-30: five defects were found and fixed
  under review. A started step that ran and then failed reported only the
  budget fact, discarding the callback's native error, so Core's total-timeout
  identity was returned for work that was not skipped; every classification
  branch now joins what the callback actually produced, and a contained panic
  now outranks the clock observations because it is an unambiguous fact about
  the callback. A contained panic discarded its value outright; the value is
  now preserved, and a panic carrying an error stays reachable through
  `errors.Is`. `Report.Validate` proved only its own seal flag and count, so a
  report could certify observations that failed their own contract; it now
  proves every retained result, while `Result` performs only constant-time
  seal, capacity, and index checks so walking a report stays linear rather than
  quadratic. Every rejection in
  the package returned one diagnostic-free `core.ErrShutdownContract`, leaving
  a full plan, a duplicate identity, and a closed plan indistinguishable to an
  operator; `errors.go` now names each rejection while preserving the Core
  identity for `errors.Is`. `Plan.Run` returned its parent-context rejection
  without the Shutdown identity at all, the one rejection in the package a
  caller could not classify; it is now wrapped like every other. Core's
  total-timeout doc line was corrected to cover work stopped as well as
  skipped. Tests: the panic table asserted containment but never the
  preservation its name claimed, and the boundary table asserted only the bare
  sentinel for six distinct rejections; both were upgraded to two-tier
  assertions that prove the typed class first and the operator diagnostic
  second. `StepResult.Validate` and `Report.Validate` had no direct coverage
  and now carry hostile matrices over every outcome/identity pairing.
- Shutdown independent hardening found two more defects and one duplicated
  contract in the reviewed correction. Rendering a recovered panic with
  `fmt.Sprint` could invoke hostile caller-owned `Error` or `String` methods,
  panic again outside containment, and allocate in proportion to an arbitrary
  byte payload. Panic facts now use a typed `StepPanicError`: error-valued
  panics remain reachable without rendering the native error; non-error
  diagnostics stream into a fixed 256-rune bound; byte payloads are decoded
  without a whole-payload string copy; invalid UTF-8 is normalized; and
  arbitrary values expose only their Go type. The new rejection reasons were
  duplicated string literals rather than a compiler-owned domain; one closed
  `uint8` diagnostic enum now owns every detail and tests recover the exact
  typed value with `errors.As`. Finally, impossible retained Plan and Report
  counts could still index beyond their fixed arrays; validation, registration,
  execution, and report reads now reject those corrupt seals before indexing.
  Direct Shutdown Witness analysis is clean with no waiver.
- Shutdown benchmark proof on Apple M1 Max: the actual maximum 64-step no-op
  construction, registration, phased run, timeout ownership, and report path
  allocates exactly 8,592 B and 137 allocations per operation across three
  final runs. Wall time ranged from 34.9 to 35.1 microseconds; this is a local
  observation, not a latency promise. Allocation is fixed by the
  compiler-owned 64-step ceiling rather than external workload or process
  lifetime.
- Shutdown verification: focused coverage is 92.9 percent; five shuffled
  focused race repetitions, full tests, full race, and a full shuffled
  repository run pass. Vet, staticcheck, errcheck, nilaway, production-only
  fieldalignment, goconst, gosec, govulncheck, actionlint, production
  `gocyclo <= 10`, formatting, module tidiness, and diff checks pass. Windows
  amd64 and Linux amd64/arm64 test binaries cross-compile. The canonical gate
  passes through full tests, full race, vet, staticcheck, errcheck, and
  nilaway, then stops at the recorded repository-wide Witness enum, Process,
  and Testserial baseline.
- Shutdown limitation: Go cannot preempt a non-cooperative in-process cleanup
  callback. Plan therefore remains synchronously joined and may be delayed by
  caller code that ignores its context. Guaranteed termination, process exit,
  worker discovery, and hard kill remain explicit composition-root or
  supervisor policy. Native Windows Control-C/Control-Break execution is not
  proved on the Darwin host; only the exact Windows projection and cross-build
  are local evidence.
- Lease review sweep, 2026-07-30: a hostile re-review of the package under
  `_docs/testing_protocol.md` found and fixed two production defects and closed
  the surfaces that hid them.
  - `Evaluate` anchored monotonic elapsed time to the floor (the later of the
    signed issuance and the consumer's durable high-water) instead of to the
    start observation's own wall reading. That counted the interval between the
    start observation and the committed high-water twice, so the effective
    clock ran fast whenever the high-water was fed back, and once a process's
    uptime passed `ClockRollbackToleranceNanoseconds` the process rejected its
    own committed high-water as a `ClockContradiction`. The documented consumer
    loop therefore self-destructed after five minutes of uptime. Trusted
    progress is now `max(floor, startWall + monotonicElapsed)` under one
    contradiction check against the current wall reading; `checkedBaseline` is
    deleted. Every previously expected rejection still holds.
  - `Advance` decided generation order before lease identity, so a decision
    naming a different subject returned `ErrLeaseRollback` when its generation
    was lower and `ErrLeaseConflict` when it was equal or higher. A consumer
    that retries on rollback and stops on conflict would silently discard a
    decision belonging to another installation. Identity is now decided first
    through `requireSameLeaseIdentity`; every cross-subject or cross-revision
    pair conflicts regardless of generation.
  - Test upgrade: statement coverage 80.3 percent to 92.0 percent after the
    final contract deletion. Added
    hostile strict-JSON tables for `Subject`, `Grant`, `Refusal`, and
    `Revocation`; nil-receiver and zero-value marshal matrices for every wire
    type; exhaustive 256-value label and validity proofs for every enum and
    off-wire state; signing-domain text round trip; typed-failure identity,
    `errors.As`, and field recovery; unset-carrier projection refusal for
    `Verified`, `Assessment`, `AdvanceResult`, and `Decision`; `Header`
    validation pressure; an `Evaluate` ingress matrix; a high-water fixed-point
    and long-uptime regression; `Assessment` tampered-projection refusal; and
    out-of-domain outcome refusal on every switch. Canonical maxima are now
    proven exactly attained per type rather than only for the grant decision,
    and both fuzz oracles now assert canonical idempotence, declared bounds,
    and, for documents, that verification either authenticates the exact signed
    fixture or fails with a typed identity.
  - Contract reduction after user review: recoverable refusal now carries only
    OGS's signed `ContactAfter` boundary. The deleted `RefusalReason` enum
    copied payment, entitlement, and clock policy into Primitive even though
    Lease had no reader for it. The continuing sequence identity is exactly
    `(Revision, Subject)`; `Subject` remains the product, entitlement, and
    registered-installation tuple. No unused lease ID, account, organization,
    parent, seat, or device-key custody was admitted.
  - Protocol ratchets now pin the exact revision-v1 signed decision body with a
    typed SHA-256 vector, require all wire members explicitly, and keep semantic
    field order independent of field-alignment pressure. JSON structure limits
    are passed as one inventoried typed contract instead of four loose helper
    parameters. Unchanged advance is proved with two different trusted signing
    keys, and real `temporal.Observe` values traverse Lease under Go's
    deterministic monotonic clock.
  - Temporal follow-through: `Observation` now keeps Go's real `time.Time`
    monotonic carrier separate from its exact wall projection. The typed
    `WithWall` value operation can model an OS wall correction without
    replacing Go's elapsed-time substrate or creating a clock framework. Lease
    now proves that seven minutes of real monotonic progress paired with a
    six-minute backward wall correction is rejected through `ErrLeaseClock`
    with the exact observed and trusted instants recoverable by `errors.As`.
  - Remaining defensive branches with no reachable caller, reported and not
    faked: the `Verified.Validate` envelope-domain check (`Document.Validate`
    already rejects that pair and `attest.Verified` cannot be constructed
    outside Attest), the `enumFact` "has no contract" leg, and the
    `jsonLimits`, `marshalEnum`, `maximumInstant`, and `requireBefore` error
    legs whose inputs are already closed by an earlier `Validate`.
- Review gate: no external reviewing agent was initialized. The user reviewed
  Objectstore and Exchange, then explicitly authorized their commit and push at
  `c65460d7b54a74f665cdb30bdf627adf20f191fe`. No tag or release is
  authorized. Cloudidentity was subsequently reviewed, committed, and pushed
  at `44b297a97c02e417a8cda84c28cb177847ededed`. The user independently
  published subsequent Exchange/Core work through
  `021373fb01141b51ed84e755f9961f1676875661`; Hostfacts was kept isolated while
  that base advanced. The user reviewed Hostfacts, supplied concrete cgroup,
  filesystem, error-ownership, and proof findings, accepted the corrected
  package, and explicitly authorized its commit and push. No tag or release is
  authorized. Fuzzfinder was subsequently reviewed and published at the
  revision above. Lease was subsequently reviewed and explicitly authorized
  for commit and push at the revision above. Release was subsequently reviewed,
  supplied four concrete implementation findings and three contract notes,
  accepted after correction, and explicitly authorized for commit and push.
  Shutdown was subsequently reviewed, corrected, accepted, and explicitly
  authorized for publication at the revision above. Upgrade was subsequently
  reviewed, corrected, accepted, and explicitly authorized for publication at
  the revision above. Gate was subsequently reviewed, corrected, accepted, and
  explicitly authorized for publication by the commit carrying this ledger
  entry.
- Production packages: 20 of 20 accepted.
- Test-support packages: 0 of 1 complete and 1 in consumer surgery.
- Deferred consumer surgery: Witness must adopt the exact typed
  `testserial.Declare` contract before Testserial can become `DONE`, after all
  Primitive production packages are complete.
- Canonical local gate: `PASS` on every phase in
  `evidence/attest-accepted/gate-results.tsv` for the accepted Attest
  worktree. This includes ordinary and race tests, vet, static analysis,
  zero-waiver witness-lint, `gocyclo <= 10`, security and vulnerability checks,
  dead-code analysis, Core JSON benchmarks, two Attest signing benchmarks, two
  100,000-execution Core fuzz phases, and two 100,000-execution Attest fuzz
  phases over the widened bounded JSON input space.
- Durable gate scope: the recorded evidence is local `darwin/arm64` evidence.
  It does not certify the GitHub Actions matrix. Multi-platform evidence remains
  unavailable until an authorized revision runs on Linux, macOS, and Windows;
  N27 is open rather than falsely closed.
- Public typed strict-JSON fuzz proof: 100,000 executions passed. Each arbitrary
  mutation reaches the public decoder and an independent typed-error/zero-value
  or validate/encode/round-trip oracle. The former checksum-derived legal path
  and four-value surrogate fixture were removed from the callback; their real
  boundary cases remain in hostile public-path tables.
- Shared JSON-string fuzz proof: 100,000 executions passed. Each arbitrary byte
  slice reaches the real shared marshaler; invalid UTF-8 must return
  `ErrJSONContract` and nil output, while valid UTF-8 must produce valid JSON
  whose independently decoded string is byte-for-byte equal to the input.
- Encode benchmark evidence: N9 remains disclosed rather than claimed as
  stdlib-equivalent cost. The canonical durable run is 2,252 ns/op, 5,445 B/op,
  and 27 allocs/op versus 323.2 ns/op, 192 B/op, and 5 allocs/op for
  `json.Marshal`.
  The enforced ratchets are at most 6,000 B/op and 32 allocs/op. Time remains
  measured and disclosed rather than hard-bounded across platforms. N16's
  increase from the earlier N9 measurement of 2,618 B/op is a real roughly
  twofold cost, not a benchmark-shape artifact: one shared JSON-correct,
  HTML-neutral string marshaler plus strict token scan, typed decode, validation,
  re-encode, and stability comparison all execute on the encode path.
- Maximum strict-JSON evidence: the globally supported limits are now the
  proven default envelope: 1 MiB per document, depth 64, 256 fields per object,
  and 1,024 items per array. A 1,047,997-byte public `DecodeStrictJSON`
  composition using those maxima completes here in 61.5 ms with 28,383,332
  B/op and 1,192,374 allocs/op. The benchmark enforces at most 40 MiB
  and 1,500,000 allocations. `StrictJSONLimits` documents that it bounds input
  and Core's standard-library token scan, not resource use inside consumer
  `UnmarshalJSON` or `Validate` methods.
- Review findings: N1 through N6 remain closed. N7 is closed by one shared
  HTML-neutral JSON-string marshaler used by every string-backed Core JSON type,
  direct `MarshalJSON` validity tests, and hostile legal control-byte paths. N8
  is closed by accepting valid JSON string escape choices independently of Go
  HTML escaping, direct marshaler-to-public-decoder parity, and ten explicit raw
  or escaped external path documents. N9 remains disclosed, with measured
  encode cost plus allocation-count and B/op ratchets, and is not claimed
  closed or stdlib-equivalent.
  N10 is closed by encode-side documentation and encode-owned diagnostics for
  the package-wide case-insensitive duplicate-key rule. N11 is closed by moving
  the structural helper into test-only code. N12 is closed on target form.
  N13 is closed at the shared JSON-string boundary by rejecting unpaired
  surrogate escapes before stdlib repair and rejecting invalid UTF-8 before
  encoding; the public path, direct typed methods, object keys, object values,
  and the shared marshaler are hostile-tested. N14 remains addressed by folding
  keys once, sorting once per completed object, and scanning adjacent keys,
  with no map or reflection. N19 is addressed without a second JSON parser:
  Core retains `json.Decoder.Token`, lowers every global maximum to the proven
  default envelope, and holds the public maximum case with the resource
  ratchets above. N15 remains closed by the independent mutation-input oracle.
  N16 is answered by the encode measurements above. N17's encode order remains
  held by its typed producer-order test; its decode diagnostic is now held at
  the public consumer extension point, without claiming Core types bypass their
  own unmarshal validation. N18 remains closed for durable local evidence and
  honest whole-second timing; it does not imply N27's multi-platform proof.
- Fifth-pass findings: N20 selects the noncanonical-input contract. Public docs
  state that strict decode is not injective, accepts insignificant whitespace
  and equivalent JSON escapes, and requires byte-signing protocols to
  authenticate original bytes before decode. N21 is closed by recover guards
  around consumer `UnmarshalJSON` and `Validate`, with external public-path
  zero-value/typed-error tests. N22 is held by a nine-row public diagnostic
  matrix whose load-bearing assertion is `errors.Is(ErrJSONContract)` and whose
  second tier checks the operator detail. N23 is recorded at its actual encode
  and consumer-decode surfaces. N24's redundant mandatory fuzz half is removed.
  N25 is closed by documenting invalid UTF-8 and surrogate rejection on both
  public entry points. N26 is answered above. N27 remains open as disclosed.
- Sixth-pass findings: B1 invalidated the prior N7 record. The faulty
  post-marshal byte rewriter is deleted; the shared string primitive now uses
  `json.Encoder` with HTML escaping disabled and a guarded terminator removal.
  A twelve-row exact-wire table proves literal `&`, `<`, and `>`, preserved
  backslash/Unicode-escape text, controls, JSON validity, and an independent
  `strconv.Unquote` semantic round trip. B2 adds escaped-backslash public-decode
  seeds and a mutation-driven encode fuzz target whose semantic oracle is
  independent of the encoder implementation. F4 removes production validation
  code reachable only from tests and asserts constructor output directly. F5
  adds a compile-time guard binding the closed error domain to the two-word
  traversal capacity. F7 removes four speculative arithmetic exports; they
  remain deferred until named consumers prove their signatures. F8 documents
  the value/pointer decode symmetry that `EncodeValidatedJSON` enforces at
  runtime.
- Attest standard-library boundary: `crypto/ed25519` owns every signature and
  verification, `crypto/sha256` owns canonical-body hashing, `encoding/binary`
  owns the bounded frame scalars, and `encoding/json` plus Core's strict decoder
  owns envelope projection and admission. Attest adds typed domain separation,
  bounded streaming, fixed owned values, caller-selected trust, validation,
  stable Core error identity, and proof-carrying verification. It does not own
  key generation, entropy, persistence, trust selection, time, storage,
  transport, product policy, or a generic signing callback.
- Attest public contract: consumer-owned closed domains and canonical bodies
  enter `SignRequest[D]` and `VerifyRequest[D]`; standard
  `ed25519.PrivateKey` enters only the signing boundary and is copied and
  checked against its derived public suffix before any consumer callback.
  `Signature`, `TrustedKeys`, `Envelope[D]`, and `Verified[D]` close mutable or
  zero-value states. Canonical domains are 1..64 bytes, canonical bodies are
  1..1 MiB, trust sets are 1..16 keys, the maximum signed frame is 166 bytes,
  canonical envelope JSON is at most 405 bytes, and admitted envelope
  documents are at most 1,429 bytes including insignificant whitespace.
- Attest frame and wire: Ed25519 signs the frame directly. The frame is the
  generation `foundation-attestation-2026`, a zero separator, big-endian
  uint16 domain length, canonical domain bytes, 32-byte signer, big-endian
  uint64 body length, and 32-byte SHA-256 body digest. Canonical envelope order
  is `domain`, `signer`, `body_length_bytes`, `body_sha256`, `signature`, which
  mirrors the signed frame and appends the detached signature last. Strict input
  accepts harmless whitespace, member reordering, and equivalent JSON escapes,
  then normalizes on emission.
- Attest third-pass audit correction: the accepted-document bound and the
  canonical-output bound were one constant, so an envelope Attest itself
  produced at maximum extent, 405 bytes, became undecodable after a single
  space. `EnvelopeCanonicalJSONMaximumBytes` now owns the exact canonical
  extent that `MarshalJSON` enforces, and `EnvelopeJSONMaximumBytes` is that
  extent plus a bounded 1024-byte insignificant-whitespace allowance, so an
  indented envelope decodes and renormalizes. Observed red first as a
  nonpositive whitespace allowance. `newAttestationFrame` additionally rejects
  an empty domain and an over-extent frame instead of letting `append`
  reallocate off its fixed array, which the extent matrix proves red in both
  drift directions. The final protocol audit added the missing one-below case
  to the accepted-document allowance matrix, so that numeric boundary is held
  below, exactly at, and one above its threshold.
- Attest red/green evidence: the fixed-vector test was deliberately observed
  red with placeholder frame and signature values before the direct
  standard-library oracle was pinned. The canonical-order test was observed
  red when field alignment reordered the active encoder, then green after the
  wire projection was made order-stable without disabling field alignment.
  The post-protocol audit produced two further behavioral reds: forged negative
  `TrustedKeys.count` and `domainToken.length` values panicked on negative slice
  bounds. Their owning `Validate` methods now reject all nonpositive values
  with `ErrAttestContract`. Sentinel's source matchers carry synthetic missing,
  extra, duplicate, unknown, alias, and raw-effect red fixtures through the
  same matchers used on the live tree.
- Attest contract-ratchet slices: canonical-body, private-key, domain, trust,
  request-validation, verification, error-hierarchy, JSON, receiver-safety,
  panic-containment, closed-writer, proof-zero, and fixed-storage matrices are
  ratchets over the newly approved contract rather than claims about a prior
  production regression. Each serious public validator/parser/verifier has at
  least ten accepting and ten rejecting cases or exhausts its smaller complete
  state space; exact boundaries include one below, exact, and one above.
  Producer, envelope-schema, JSON-projection, and verifier responsibilities
  each have a named positive/negative/neutral `LayerTriad`.
- Attest independent proof: the fixed vector reconstructs the complete frame
  independently and proves byte-for-byte parity with direct `ed25519.Sign` and
  `ed25519.Verify`. The signed-field fuzzer independently mutates domain,
  signer, body length, body digest, signature, and canonical body bytes, then
  uses direct standard-library cryptography or SHA-256 as its oracle. The JSON
  fuzzer requires typed rejection with receiver preservation or validated,
  bounded, stable semantic closure.
- Attest streaming evidence: the accepted canonical 64 KiB signing benchmark
  is 126,935 ns/op, 8,877 B/op, and 15 allocs/op. The exact 1 MiB maximum is
  546,593 ns/op, 8,878 B/op, and 15 allocs/op. Equal allocation counts and
  effectively equal allocated bytes across a sixteenfold body-size increase
  hold the O(1) allocation shape; wall-clock time remains measured rather than
  platform-ratcheted.
- Sentinel activation: Attest is the first live non-Core edge. The live audit
  proves the on-disk package exists and has exactly `attest -> core`, including
  test imports; the coupling violation inventory is empty. Synthetic red/green
  fixtures exercise missing, extra, undeclared, nested, duplicate, test-only,
  and aliased edges. PLAN's graph table and README's Mermaid graph are parsed
  through typed fixed inventories and match Core's compiler-owned catalog.
  Export-to-consumer ownership is enforced with exact typed deferrals for the
  existing one-consumer migration tier; new orphan or single-owner Core facts
  fail immediately.
- Attest architecture proof: every production struct has a compiler-visible
  data-flow role; the public surface, imports, import aliases, and raw
  cryptographic effect owners are exact AST ratchets with synthetic red/green
  fixtures. Production contains no maps, reflection, loose protocol payloads,
  whole-stream reads, compatibility shims, product imports, or sibling imports
  beyond Core.
- Attest protocol-audit correction: the first green gate bundle was produced
  before the review-enforced testing rules were fully audited and is retained
  only as `evidence/attest-pre-protocol-audit/`. It is not canonical evidence.
  The superseded second-pass bundle follows the rewrite of shared mutable
  fixtures, validator matrices, signature acceptance boundaries, setup failure
  paths, per-layer triads, synthetic structural fixtures, and private
  fixed-storage hostility. The completed third pass proved the user review
  corrections. The accepted canonical bundle is `evidence/attest-accepted/`
  after the final below/exact/above audit and restoration of the reviewed
  100,000-execution per-target Attest fuzz budget.
- Attest consumer surgery deferral: no Witness, Bug, Peachfuzz, or Kernel
  retirement is claimed. Exact migration and durable-byte equivalence remain a
  future consumer-owned proof; no old signing engine is changed in this slice.
  `crypto.Signer` and KMS/HSM-held private keys are also a named deferral:
  Attest accepts `ed25519.PrivateKey` only until a real Release or Lease
  consumer proves that a remote-key boundary is required. No speculative
  signing callback enters the accepted API.
- Gate portability: the witness tools remain pinned to commit
  `133471a26114024d1bb17e629408eb8bce96c584`, now spelled as its immutable Go
  pseudo-version so public-proxy installs do not require Git metadata
  resolution.
- Contextstate top-down recheck, 2026-08-01 (awaiting review, uncommitted): the
  prior reconciliation correctly kept `Context.Err` as the sole terminal truth
  and added `ObserveAfterDone`, but its private 128-node `Unwrap` graph engine
  reimplemented `errors.Is`, its unused public `Classify(error)` had no landed
  consumer, and its thousand-line repository AST policy forbade other packages
  from using Go's error semantics directly. Those parallel inventions are
  removed. Contextstate now reads `Err` exactly once and admits only the exact
  `context.Canceled` and `context.DeadlineExceeded` sentinels promised by the
  standard-library interface. Nil, panic, noncomparable, wrapped, joined,
  cyclic, or otherwise nonstandard results become the zero State with
  `ErrContextObservation`; no custom `Is`, `Unwrap`, graph, budget, goroutine,
  reflection, or second precedence source exists. The invalid zero enum member
  is internalized. Production remains exactly `context` plus Core, with no
  mutable state or effects.
- Contextstate archive exclusions remain deliberate: State JSON, `Parse`, and
  their fuzzer were not brought forward because no named consumer persists the
  state. Restoring them would add a second wire protocol rather than make
  `context.Context` easier to use. Doctrine plumbing, package-local error
  labels, and shared error traversal likewise remain rejected.
  This is a clean reconciliation, not an archive transplant.
- Contextstate proof: the external State test exhausts all 256 values of the
  enum's underlying domain. Public `Validate` and `ObserveAfterDone` matrices
  use real standard contexts for normal behavior and synthetic implementations
  only for malformed interface ingress. They prove exact standard-sentinel
  acceptance, rejection of custom/wrapped/joined/cyclic terminal errors,
  noncomparable-error and panic containment,
  future-deadline admission based solely on `Err`, typed-nil terminal-error
  rejection, panic containment, active-after-Done rejection, exactly one
  `Context.Err` call, and an exact standard-interface seam whose `Deadline`
  and `Done` methods panic if either is observed instead of `Err`. A
  package-local AST ratchet separately rejects production calls to `Deadline`,
  `Done`, `Value`, or `Cause`, so the complete `doc.go` claim is
  compiler-visible without a witness-lint waiver for `Context.Value`'s
  interface return.
  Typed-nil context handling is containment, not detection: a typed nil whose
  `Err` panics is contained, and a typed nil whose `Err` is nil-safe reports no
  terminal state and is admitted as usable. Distinguishing the second case
  needs reflection, which the package excludes, so `Validate` documents the
  limit instead of implying detection.
- Contextstate architecture proof: the exact public surface rejects JSON,
  parsers, aliases, `Classify`, an exported invalid sentinel, and extra exports.
  The package-local import ratchet proves production imports exactly the
  standard `context` substrate and Core. A focused AST ratchet proves production
  does not call `Deadline`, `Done`, `Value`, or `Cause`; package owners remain
  free to use standard-library error classification for their own results.
- Contextstate focused reconciliation evidence: `go test -count=1`,
  `go test -race -count=1`, vet, staticcheck, witness-lint, errcheck, nilaway,
  fieldalignment, goconst, gosec, and production-plus-test `gocyclo -over 10`
  are clean for `./contextstate`. The fresh whole-repository gate also passed
  every phase in `evidence/contextstate-sixth-pass/`; the evidence remains
  local `darwin/arm64` proof and does not close N27.
- Contextstate's historical sixth-pass evidence predates the top-down
  simplification and is retained only as historical proof; its global AST
  ownership matcher and private graph mechanism no longer describe production.
- Contextstate fuzz, benchmark, native, and live proof are not applicable.
  State's complete byte-sized domain is exhaustively tested; `error` and
  `context.Context` interface graphs have no honest byte-mutation oracle; the
  package has no I/O, platform branch, clock, goroutine, or effect leaf. The canonical gate
  still reruns Core's benchmarks and all four Core fuzz targets at 100,000
  executions each.
- Waiver state: direct Core and Contextstate `witness-lint` pass with zero
  waivers. The repository-wide analyzer still stops on Process's two recorded
  `exec.Command` capability findings because the pinned analyzer recognizes
  only Witness's inaccessible internal capability path; Primitive adds no shim,
  duplicate execution API, or waiver. The inherited Core string-search waiver
  was removed without dropping N22's second-tier
  diagnostic proof: `jsonContractError` now carries a private typed diagnostic
  detail, and the existing hostile table uses `errors.As` before comparing its
  compiler-owned detail.
- Consumer surgery deferral: `contextstate` performs no consumer surgery in
  Kernel, Witness, Bug, or Peachfuzz. Its interview names superseded
  implementations only in `temporal`, `exchange`, and `shutdown`, which are not
  built. Under section 4's deferral rule, a low-level primitive with no
  consumer-owned separable implementation defers surgery to the first package in
  that chain that delivers a complete capability. Transplanting the primitive
  before its own catalog consumers exercise it would pin four repositories to
  signatures that no landed consumer has yet proved.
- Commit and push authorization: granted by the user for the accepted combined
  depth-2 slice after completing their review.
- Currency top-down recheck, 2026-08-02 (awaiting review, uncommitted): the
  package is a typed exact-minor-unit layer over integer arithmetic, `strconv`,
  Core's standard-library JSON boundary, and independent `math/big` test oracles.
  Its closed twelve-code domain owns fraction digits, exact decimal projection,
  checked same-code arithmetic, ordering, and strict typed JSON. The prior wire
  type duplicated the substrate by manually walking `json.Decoder` tokens and
  assembling object punctuation. It now remains only a typed struct with its
  owned `Validate`; Core's strict decoder rejects unknown, duplicate, and
  case-folded field names before standard-library structure decoding, while
  Core's canonical encoder owns the output bytes. Minor-unit string tokens use
  those same shared JSON boundaries. The official ISO 4217 Maintenance Agency
  amendment 180, dated 2025-09-22 and effective 2026-01-01, confirms the source
  revision and the selected EUR minor-unit exponent. Ordinary and race tests,
  vet, staticcheck, errcheck, nilaway, Witness, fieldalignment, formatting, and
  production `gocyclo <= 10` pass. Fresh decimal and amount-JSON semantic fuzz
  runs each pass 100,000 executions.
- Garble top-down recheck, 2026-08-02 (awaiting review, uncommitted): the
  package is a typed layer over standard Base64, HKDF-SHA256, and the pinned
  upstream Garble executable. It owns one exact
  eight-byte Garble seed, Core-owned 64-byte custody, deterministic derivation,
  and a streaming typed build-argument intent. It does not own command
  execution, generic CLI parsing, persistence, environment policy, or a
  replacement Garble protocol. The complete preserved implementation was mined
  for upstream mechanics and rejected where it accepted lossy seed forms,
  exposed loose string arguments, or invented wider ownership.
  Seed JSON no longer carries a private wrapper type and second
  `encoding/json` decode path: its bounded raw document enters through Core's
  shared standard-library string-token contract and its canonical output uses
  the matching encoder. The obsolete wire inventory and production JSON import
  are deleted. Ordinary and race tests, vet, staticcheck, errcheck, nilaway,
  Witness, fieldalignment, formatting, and the production cyclomatic bound pass;
  a fresh semantic Seed JSON fuzz run passes 100,000 executions.
- Keygen top-down recheck, 2026-08-02 (awaiting review, uncommitted): the package
  remains only friendly typed convenience over
  `crypto/rand.Read`, `ed25519.GenerateKey`, and `ed25519.NewKeyFromSeed`.
  Generic secret sizes exhaust the complete Core-owned 16-through-64-byte
  interval in deterministic tests; Ed25519 keys are copied into owned
  redaction-safe values, validated against the standard-library relation,
  projected through caller-owned copies, and destructible through Core secret
  ownership. Entropy providers, algorithms, derivation, signing policy,
  persistence, formats, KMS/HSM behavior, and command surfaces remain excluded.
  Source reinspection found no second implementation to remove: production on
  Go 1.26.5 uses the protected nil-source Ed25519 generation path and direct
  `rand.Read`, while hostile source injection remains private test reachability.
  The two public operations, exact imports/effects, zero maps, absence of local
  secret-content predicates, shared destruction, and generic-format redaction
  are ratcheted. Ordinary and race tests, vet, staticcheck, errcheck, nilaway,
  Witness, fieldalignment, formatting, and the production cyclomatic bound pass.
- Testserial review state: Primitive owns only a typed declaration
  `Declare(*testing.T, core.TestIsolationDeclaration)` plus the Core-owned
  hazard/scope/identifier contract. It owns no lock, scheduler, mutual
  exclusion, reason string, JSON protocol, or compatibility `Serial` API.
  Hostile tests use real `testing.T` execution to prove invalid declarations
  stop following behavior. The corresponding Witness analyzer migration is
  consumer surgery and cannot be published before the new Primitive Core
  contract is published.
- Filestore review state: the package is the smallest typed layer over
  `os.Root`, `*os.File`, `io.Reader`, `io.Writer`, `io/fs`, and the native
  namespace. It owns rooted relative paths, bounded streaming, durable stage
  receipts, create-only link activation, replacement rename activation,
  recovery handoff, caller-owned append handles, physical append rotation, and
  nonrecursive single-entry removal. It owns no virtual filesystem, reader or
  writer wrapper, lock, scheduler, retry framework, transaction engine, or
  recursive walk.
- Filestore top-down correction, 2026-08-02 (awaiting review, uncommitted):
  lexical `Walk` previously admitted every positive `uint32` directory maximum,
  converted it to `int`, and requested that many entries plus one from
  `os.File.ReadDir`. A hostile maximum could therefore overflow on 32-bit or
  demand a multi-billion-entry allocation on 64-bit, contradicting the fixed
  bounded-memory claim. `DirectoryEntryMaximumLimit` now compiler-owns the
  65,536-entry ceiling already required by real Witness scans; constructor and
  validation reject the next value and `math.MaxUint32` before filesystem work.
  Focused hostile/public-surface tests, the full Filestore race suite, vet,
  staticcheck, errcheck, nilaway, Witness, formatting, and a Linux 386
  cross-build pass.
- Filestore consumer-review hardening: Witness recovery proved that the former
  implicit create-or-open append behavior could durably fabricate a zero-byte
  segment after a stat/open race. `AppendMode` now makes exclusive create,
  existing-only, and ordinary create-or-open intent compiler-visible.
  `RotationRequest` admits only exclusive creation for the incoming generation.
  Rotation ownership transfers only after context and request validation;
  rejected validation or cancellation leaves the outgoing real `*os.File`
  caller-owned, while every accepted rotation closes it even when the cutover
  fails. Dedicated real-filesystem layer triads prove all three append intents,
  native conflict/not-exist reachability, absence of ghost files, preservation
  of existing identity and mode, and both sides of the rotation ownership
  boundary.
- Filestore seam proof: dedicated positive, negative, and neutral triads cover
  typed ingress, context ingress, rooted confinement, directory creation,
  bounded source reads, destination writes, stage synchronization, receipt
  identity, create and replace activation, failed-Write ownership handoff,
  cleanup, recovery, append open and rotation, single-entry removal, parent
  synchronization, distinct-target concurrency, and same-target contention.
  Every production function executes; package statement coverage is recorded
  only as an 83.8% zero-function ratchet, not as the quality target.
- Filestore current proof: ordinary repository tests, race tests with shuffle
  and two repetitions, vet, staticcheck, errcheck, nilaway, production
  `gocyclo <= 10`, goconst, fieldalignment, gosec, govulncheck, actionlint, and
  two fresh exactly-10,000-execution Filestore fuzz targets pass. The canonical
  gate passes through nilaway and then stops at the pinned Witness analyzer's
  known stale off-wire enum JSON/String doctrine and retired Testserial
  convention. Primitive does not add compatibility or wire behavior to
  satisfy those findings.
- Filestoretest retirement proof, 2026-08-01 (awaiting review, uncommitted):
  the archived package is not admitted. Current Filestore delegates reader
  pressure to `testing/iotest` and tests destination behavior at the real
  `io.Writer` seam. One local production-path ratchet now proves a destination
  returning an accepted prefix plus `syscall.ENOSPC`: Filestore reports the
  exact accepted extent and bytes, retains `ErrFilestoreDestination` and the
  native errno, and does not invent an `io.ErrShortWrite` identity when the
  writer already supplied a real error. No capacity model, filesystem
  simulator, shared writer wrapper, concurrency promise, or duplicate
  observation state enters Primitive.
- Filestore append-intent proof: fresh ordinary repository tests and
  race/shuffle tests with two repetitions pass. Vet, staticcheck, errcheck,
  nilaway, production `gocyclo <= 10`, gosec, govulncheck, actionlint, gofmt,
  module tidiness, and diff checks pass. The canonical gate passes through
  nilaway and stops only at the already classified stale Witness enum-wire and
  retired Testserial doctrines; the one new test-diagnostic finding was
  corrected and does not recur. Fieldalignment reports only existing test
  fixture layouts outside the new append-intent file.
- Temporal accepted state: the package is a typed exact-nanosecond layer over
  Go's real `time`, `context`, and arithmetic primitives. `Instant` owns the
  complete signed int64 Unix-nanosecond domain and distinguishes an unset zero
  value from the epoch. `Duration` owns nonnegative bounded elapsed time.
  `AggregateDuration` is an unsigned 128-bit accumulator with fixed-storage,
  canonical decimal persistence. `Observation` preserves the standard
  library's monotonic reading, and `Interval` closes start/end/elapsed
  arithmetic. Timeout, deadline, wait, and ticker effects validate typed
  requests and return or use real standard-library capabilities. Temporal owns
  no clock interface, fake clock, scheduler, worker, retry engine, state
  machine, humanization policy, provider persistence shape, or custom timer or
  ticker wrapper.
- Temporal supporting contracts: Core now owns the one closed `Comparison`
  vocabulary used by Currency and Temporal. Currency's duplicate `Order`
  domain is deleted without an alias or shim. `ErrTemporalOverflow` is
  Core-owned and remains reachable as both `ErrTemporalContract` and
  `ErrNumericOverflow`. Contextstate now exposes `Observe`, allowing Temporal
  to consume the existing typed context-state boundary rather than copying
  terminal-context classification. Core also owns the positive `OffWireEnum`
  marker shared by Comparison, Contextstate State, and Temporal Precision;
  package method-set tests remain the negative proof that those types implement
  no wire interfaces.
- Temporal review corrections: Comparison and Precision now project from
  complete limit-sized fact tables, so adding an enum member without its fact
  row fails compilation. Aggregate widening validates Duration rather than
  silently mapping an internally negative value to zero, and the checked error
  flows through `Duration.Aggregate` and `AddDuration`. The Instant difference
  overflow guard checks sign before bounded arithmetic. `NewInstant` now uses
  the standard library's `UnixNano` projection and a `time.Unix` round trip to
  reject out-of-domain values; the exact minimum no longer depends on two
  wrapping signed arithmetic operations canceling each other. Unit constructors
  project their guarded product directly rather than repeating the same bound
  through a second conversion helper. String-form temporal JSON re-encodes with
  `encoding/json` and rejects escaped alternate spellings such as `"\u0037"`
  while preserving the explicitly bounded outer-whitespace policy.
  Compile-time witnesses now pin every Temporal function and method signature and all public request
  layouts, plus the touched Contextstate and Currency signatures; the existing
  AST ratchets continue to reject added or removed public names.
- Temporal hostile proof: signed instant construction and arithmetic exercise
  both int64 extremes, epoch crossings, full-span overflow, reverse ordering,
  and negative truncation toward the preceding boundary. Every duration unit
  constructor is held at zero, one, the largest exact value, one above, and
  maximum uint64. Aggregate arithmetic crosses the uint64 limb, carries into
  the high limb, reaches the uint128 maximum, rejects maximum-plus-one, and
  proves narrowing. Canonical JSON tests reject malformed, noncanonical,
  wrong-type, over-bound, signed/unsigned-domain, and mutation-on-error cases.
  Observation, interval, timeout, deadline, wait, and ticker seams each have
  exact positive, negative, and neutral proof over Go's real primitives;
  `testing/synctest` controls the standard clock without a production clock
  abstraction.
- Temporal current proof: focused ordinary and race/shuffle tests, full
  ordinary and race/shuffle repository tests, vet, staticcheck, errcheck,
  nilaway, production `gocyclo <= 10`, Temporal goconst and fieldalignment,
  gosec, govulncheck, actionlint, formatting, module-tidiness, and diff checks
  pass. Fresh Aggregate, signed Temporal, numeric Instant, and numeric Duration
  fuzz runs each passed 100,000 executions. Temporal statement coverage is
  86.5% and is used only as a
  zero-function ratchet; `Precision.OffWireEnum` is intentionally an empty
  compiler marker with no executable statement. Direct Witness analysis of
  Temporal is clean. The canonical gate passes through nilaway and then stops
  only at the already classified stale Witness off-wire-enum JSON/String
  doctrine and retired Testserial convention.
- Temporal numeric projection, 2026-08-01 (awaiting review, uncommitted):
  `NumericInstant` and `NumericDuration` close the gap the temporal interview
  already specified at `_docs/interviews/temporal.md:122` and `:130`, where wire
  and persistence formats are compiler-owned Primitive contracts and preserved
  mechanics include numeric persistence projections. `Instant` and `Duration`
  marshal exact nanoseconds as a JSON string; a consumer whose wire has always
  carried those nanoseconds as a JSON integer previously had to hand-roll a
  wrapper. Three consumers prove the demand: Kernel wraps
  `foundationcore.UnixNanoTime` (`:107`), Witness `VerifierBudget` uses raw
  durations under `_ns` JSON names (`:320`), and Peachfuzz wrote
  `protocol/wire_time.go` this session for exactly this reason, including a
  second copy of `canonicalSignedDecimal`.
  - Both types are placed directly in a wire struct with a json tag. That is the
    point: append and parse helpers alone would leave every consumer writing the
    same wrapper, which is the duplication being removed.
  - The projection wraps the owning typed value rather than a raw `int64`, so
    value semantics stay `Instant`'s and `Duration`'s and there is no second
    home for the nanosecond fact. `NumericInstant.Instant` returns an error
    because its Go zero value carries no instant; `NumericDuration.Duration`
    cannot fail because its zero value is an exact zero duration, which
    `Duration` admits. No always-nil error is invented for symmetry.
  - Encoding is deliberately separate from range policy. The types admit the
    complete signed Unix-nanosecond domain including pre-epoch instants. Kernel's
    `PositiveUnixNanoTime`/`OptionalUnixNanoTime` and Peachfuzz's post-epoch
    evidence rule are a distinct shared need with a second real consumer; that
    contract is named here and deliberately not folded in. It needs its own
    interview pass and its own slice.
  - Decode admits no insignificant whitespace, unlike the string projection.
    `encoding/json` hands a member's exact literal bytes to `UnmarshalJSON`, so
    one value keeps exactly one accepted encoding. `decodeNumericNanoseconds`
    delegates to the existing `parseSignedNanoseconds`, so the canonical-decimal
    rule keeps one owner rather than gaining a byte-slice twin.
  - Red proof: four production mutations were each observed red and reverted.
    Emitting the string form failed the bare-number tables and the wire-struct
    ratchet; dropping the canonical-decimal gate admitted `+1`, `-0`, `01`, and
    `-01`; assigning the receiver before admission failed every
    mutation-on-rejection assertion; and removing the nonnegative gate admitted
    `-1` and `-1000000000`.
  - Proof: focused tests, race with shuffle, vet, staticcheck, errcheck,
    fieldalignment, and production `gocyclo <= 10` are clean for `./temporal`.
    `FuzzNumericInstantJSON` completed 5,733,868 executions and
    `FuzzNumericDurationJSON` 6,744,037 executions in 30 seconds each; both
    oracles require an accepted document to re-encode to exactly the accepted
    bytes and a rejection to carry a stable typed identity with an untouched
    receiver. The repository-wide delivery result is recorded below.
- Core canonical integer ownership, 2026-08-01: `parseCanonicalUint64JSON` was
  a Core-private fact used by three Core files while Temporal separately carried
  its own hand-written `canonicalSignedDecimal` grammar. Two grammars for one
  rule is the duplication class Primitive exists to remove. Core now exports
  `ParseCanonicalUint64JSON` and adds `ParseCanonicalInt64JSON`; Temporal's
  `parseSignedNanoseconds` projects from that owner and its local grammar is
  deleted. The rule is parse, re-encode, require byte equality, so the accepted
  grammar is the encoder's own output and cannot drift from it. This admission
  satisfies PLAN section 2 without an exception: Core itself and Temporal are two
  named Primitive packages. Temporal's existing string-projection tests and the
  new numeric tables both pass unchanged across the substitution, which is the
  evidence that the two grammars were semantically identical.
  `canonicalUnsignedDecimal` is deliberately retained: AggregateDuration's
  39-digit unsigned-128 decimal exceeds uint64 and cannot route through
  `strconv.ParseUint`.
- Attest canonical object, 2026-08-01 (delivery review complete):
  `CanonicalObject` closes the canonical JSON field-append gap. Placement in
  Attest rather than Core was re-verified from source before any code was
  written, because reversing it would fail the gate silently: `lease` and
  `receipt` build their canonical JSON from ordered pointer-field wire structs
  through `json.Marshal` (`lease/decision.go:436`, `receipt/evidence.go:461`,
  `receipt/evidence.go:506`), not from an append mechanic. A Core placement
  would therefore have zero Primitive consumers, failing PLAN section 2 and
  Sentinel's fewer-than-two-named-packages rule. `CanonicalBody.WriteCanonical`
  is what creates the need, so Attest owns it.
  - The mined Peachfuzz version (`protocol/canonical_json.go`) routed every
    value through `json.Marshal` on a generic `T any`, which is reflection on
    the signing path. The Primitive version is a closed set of typed member
    methods, `String`, `Int64`, `Uint64`, `Bool`, plus `Value` for a nested
    `core.ValidatedJSONMarshaler`. That covers every observed Peachfuzz call
    site, keeps reflection off the signing path, and satisfies
    `protocol/typed-boundary`: the one interface is a narrow behavior interface,
    not a payload bag.
  - Two properties the mined version lacked are now enforced. Member names are
    deduplicated, so a body cannot be signed as bytes that Core's strict decoder
    would later refuse. A nested member owner runs under the existing
    `guardedCall` panic guard and its output must be non-null valid UTF-8 JSON,
    so a hostile or defective member cannot escape as signed bytes or replace
    Attest's error identities.
  - Errors are threaded and the first failure is retained, so `End` returns nil
    bytes on any rejection. A partially built document is never returned.
  - Core admission: `marshalJSONString` was a Core-private fact and Attest needs
    the same rule for member names and string values. It is now exported as
    `core.MarshalCanonicalJSONString`, the encoding counterpart of
    `DecodeJSONStringToken`. Core and Attest are two named Primitive packages, so
    this satisfies PLAN section 2 without an exception, exactly as the canonical
    integer owner did. A second string-escaping grammar would let two owners
    disagree about the bytes a signature covers.
  - Red proof: six production mutations were each observed red and reverted.
    Dropping the duplicate-name check, dropping the null-member rejection,
    admitting uppercase in member names, returning the buffer despite a threaded
    error, dropping the empty-object refusal, and switching string encoding to
    HTML-escaping `json.Marshal` each failed the tables. The public-surface
    ratchet was separately observed red by removing one new method.
  - Proof: focused tests, race with shuffle at `-count=2`, vet, staticcheck,
    errcheck, fieldalignment, production `gocyclo <= 10`, and `witness-lint` are
    clean for `./core` and `./attest`; the full repository suite passes.
    `FuzzCanonicalObjectEmitsStrictlyDecodableAndStableDocuments` completed
    1,513,520 executions in 30 seconds. Its oracle requires an accepted document
    to decode under Core's strict JSON contract, preserve every typed member
    exactly, and rebuild to identical bytes, and a rejection to carry
    `core.ErrAttestContract` with no bytes returned. The repository-wide
    delivery result is recorded below.
- Exchange envelope read/write split, 2026-08-01 (delivery review complete): `APIEnvelope`
  now constrains its body to `core.Validatable`, and emission moved to
  `MarshalAPIEnvelope`, whose own constraint stays `core.ValidatedJSONMarshaler`.
  The split came from a real consumer proof rather than a preference. Peachfuzz
  decodes an upload grant whose capability is `objectstore.UploadCapability`,
  which is deliberately decode-only: it never retains the received URL text, so
  re-serializing the bearer is structurally impossible. Under the old single
  constraint that response could not be an envelope body at all, which would
  have forced Peachfuzz to keep a second hand-written envelope, the exact
  duplication this contract was added to retire. A producer must own an explicit
  JSON representation; a consumer that will never re-emit the document must not
  be forced to invent one, least of all for a bearer credential.
  The arms became unexported in the same slice, because relaxing the constraint
  without that would have opened a silent hole: a reflected encode of a body
  owning no `MarshalJSON` emits an empty object, and the envelope would have
  passed every validation on the way there. Ingress is now
  `NewAPISuccessEnvelope`, `NewAPIFailureEnvelope`, and `UnmarshalJSON`; egress
  is `MarshalAPIEnvelope`; reading is `Outcome`, `Payload`, `Failure`, and
  `RequestID`. `MarshalText` remains only to refuse, so a value that merely
  contains an envelope fails loudly instead of emitting arms the encoder cannot
  see, without accidentally making every envelope satisfy
  `core.ValidatedJSONMarshaler`. Review caught that a refusal-only `MarshalJSON`
  erased this compiler distinction; the text-marshaler refusal preserves both
  the loud reflected-encoding failure and the stricter emission constraint.
  `TestAPIEnvelopeAdmitsDecodeOnlyBodies` now proves the motivating consumer can
  decode while neither its body nor its envelope claims an emission contract.
  Two-armed, armless, identifier-less, and invalid-payload envelopes are no
  longer constructible, so those cases moved from struct literals to the decode
  boundary, which is the only place they can still arrive.
  Proof: 7 mutations of the new paths, 6 RED. The survivor removes the emitter's
  `Validate`; it is masked by `APIRequestID.MarshalJSON`, which validates the
  only unproven envelope Go can build, the zero value. Removing both gates
  together is RED, so the masking gate is itself ratcheted. Three earlier
  survivors were real gaps and are now closed by
  `TestAPIEnvelopeDecodeLeavesTheReceiverUntouchedOnRejection`,
  `TestAPIEnvelopeEmitterRefusesAnUnprovenValue`, and
  `TestAPIEnvelopeNilReceiverIsRefusedRatherThanPanicking`;
  `TestAPIEnvelopeRefusesReflectedEncoding` covers the refusal contract and
  asserts the envelope does not implement `json.Marshaler`. The
  data-flow struct inventory ratchet fired for `apiEnvelopeWire` and was
  classified rather than widened. gofmt, vet, staticcheck, errcheck,
  fieldalignment, gocyclo, and witness-lint are clean for `./exchange`; the full
  repository suite and `-race -shuffle=on -count=2` are green;
  `FuzzAPIEnvelopeDecodeAcceptsOnlyStableSingleArmDocuments` completed 140,760
  executions with no crashers.
- Exchange API envelope, 2026-08-01 (delivery review complete): `APIEnvelope`
  closes the typed API response gap that Kernel (`kernel/core/api_contracts.go`)
  and Peachfuzz (`protocol/api_envelope.go`) currently carry as two
  near-identical hand-written copies. Placement in Exchange was re-verified from
  source before any code was written: a repository-wide search for an envelope,
  request-identifier, or failure-code concept across every Primitive package
  found none, so a Core placement would have had zero Primitive consumers and
  failed PLAN section 2 and Sentinel's fewer-than-two-named-packages rule.
  Exchange already owns `JSONCall` (`exchange/client.go:43`),
  `decodeJSONResponse` (`exchange/client.go:377`), and both server ingress and
  egress, so the envelope is the shape those already imply and no exception is
  required. Correction to the handoff note: Witness owns no API envelope. Its
  only match is `internal/updatecmd/update_test.go`, so the named consumers are
  Kernel and Peachfuzz, not three.
  - Surface: `APICode` (8 closed members plus exact tokens), `APIRequestID`,
    `APIErrorBody`, `APINoBody`, and `APIEnvelope[Body]`.
  - `APIBody` was deliberately not absorbed. Both consumers own a marker
    interface whose implementers are product response types, so it decides which
    types a given product's API may return. That is local policy; Primitive
    cannot own a marker whose closed set is a product's. The generic constraint
    carries the load instead.
  - The constraint is `core.ValidatedJSONMarshaler`, not `core.Validatable` as
    both mined copies had. Exchange already demands an explicit JSON
    representation of every JSON request body, `core.EncodeValidatedJSON` cannot
    accept a value without one, and Core's own doc gives the reason: it prevents
    an opaque typed value from silently encoding as an empty object, which is
    exactly the failure every validation in the envelope would otherwise miss.
  - The absent arm is omitted rather than emitted as null, matching the deployed
    producer (Kernel), which uses `omitempty`. Peachfuzz emits explicit null but
    is the client on this wire and its decoder accepts both, so no consumer
    breaks and the live bytes do not move.
  - Encoding is canonical with HTML escaping off, through
    `core.MarshalCanonicalJSONString` for tokens and one small unescaped
    document encoder for the two wire structs. Both mined copies used
    `json.Marshal`, which escapes `<`, `>`, and `&`; that is a second string
    grammar, the exact duplication the 2b Core admission removed.
  - `validateAPIText` is the single admission rule for every operator-facing
    token, and it is the strict union of the two mined copies: present, valid
    UTF-8, no control runes, no replacement rune, no surrounding whitespace, and
    bounded in runes rather than bytes. Kernel admitted U+FFFD and Peachfuzz
    admitted untrimmed text; both are now refused, since a literal U+FFFD is
    always the residue of a lossy decode upstream and two separately mangled
    identifiers would otherwise correlate as one.
  - `MarshalJSON` validates first on all four wire types, so a both-arm or
    invalid envelope can never become bytes. Neither mined copy did this.
  - Deliberate exclusion: `APIHeaderXRequestID`. It has one consumer (Kernel),
    zero Primitive consumers, and Exchange cannot construct a
    `core.HTTPHeaderName` without a Core admission that PLAN section 2 forbids
    at one consumer. It stays Kernel-local product routing policy.
  - Red proof: ten production mutations were each observed red and reverted.
    Emitting the absent arm as null, resolving both-arms by data precedence,
    admitting surrounding whitespace, admitting the replacement rune, counting
    the rune bound in bytes, leaving HTML escaping on, marshalling without
    validating, matching failure tokens case-insensitively, admitting an unset
    request identifier, and marshalling an out-of-domain code each failed the
    tables. The data-flow struct inventory ratchet was separately observed red
    naming all four new structs.
  - Correction to the handoff note: Exchange has no public-surface ratchet, so
    the applicable structural ratchet is its data-flow struct inventory, which
    is what was updated. Adding a public-surface ratchet to Exchange is a
    separate slice and is not claimed here.
  - Proof: 165 subtests, focused tests, race with shuffle at `-count=2`, vet,
    staticcheck, errcheck, fieldalignment, production `gocyclo <= 10`, and
    `witness-lint` are clean for `./exchange`; the full repository suite passes.
    `FuzzAPIEnvelopeDecodeAcceptsOnlyStableSingleArmDocuments` completed 460,758
    executions and `FuzzNewAPIRequestIDAlwaysProducesOneCanonicalIdentifier`
    1,427,321 executions in 30 seconds each, with no crashers written. The
    repository-wide delivery result is recorded below.
- Objectstore upload capability, 2026-08-01 (delivery review complete):
  `UploadCapability` closes the received-capability gap. The boundary was
  re-verified against the objectstore interview before any code was written,
  because the interview excludes "grants" from this package
  (`_docs/interviews/objectstore.md:79`) and assigns grant issuance and
  target-to-authorization binding to the issuing protocol (`:567`). Reading
  `peachfuzz/protocol/upload_capability.go` against
  `peachfuzz/protocol/run_evidence_upload.go:328` settles it: the grant is
  `RunEvidenceUploadGrant`, which binds a descriptor, a schema, and a
  `ValidateRequest` replay check, and it stays in Peachfuzz. What moves is only
  its capability fields, which are the wire projection of values this package
  already owns: `SignedURL`, `SignedHeaders`, `Provider`, and the target expiry.
  That is squarely inside the interview's proposed contract (`:549-557`).
  - The gap exists because those values are deliberately off the wire.
    `SignedURL` has no string accessor and redacts under every formatting verb
    (`objectstore/values.go:128`, `:158`), so a capability received as JSON
    cannot be projected by any package that cannot validate a signed URL. This
    is the only package that can.
  - It decodes only. `UploadCapability` implements `json.Unmarshaler` and
    deliberately not `json.Marshaler`, because emitting a capability is issuing
    one, which the package doc excludes alongside buckets, credentials, and
    signed-URL creation. It also never retains the received URL text: the bytes
    are parsed into an opaque `SignedURL` and dropped, so re-serializing the
    bearer is structurally impossible rather than merely discouraged. The mined
    Peachfuzz type kept `value string` and marshalled it, which would have
    carried that leak into Primitive. A structural ratchet asserts the absence
    of `json.Marshaler`, `fmt.Stringer`, and `encoding.TextMarshaler`.
    Consequence for consumers: a test double acting as the issuing server builds
    the document as JSON text rather than marshalling the type.
  - The provider token is already general. `Provider` covers Amazon S3, Google
    Cloud Storage, and Cloudflare Images, so no GCS-only enum was shipped; the
    accepted token is `Provider.String()` itself, so the wire vocabulary and the
    execution domain cannot drift into two tables that disagree. `Provider`
    keeps `OffWireEnum`: it is never a JSON field, the capability owns its own
    decoding, and it stays inside the ratcheted execution-enum family that
    `TestOffWireEnumExhaustiveDomains` enumerates. Naming this as the one
    reversible call in the slice: making `Provider` a wire enum outright would
    be simpler but breaks that reviewed family.
  - No second method enum was created. The wire `method` token is checked
    against `Spec(provider)` and then discarded, because the vendor
    specification already decides how the transfer runs. The token is the
    issuer's assertion about what its signature covers, so a disagreement means
    the capability would be spent on a request the vendor rejects. That turns a
    redundant field into a real integrity check instead of a second source of
    truth.
  - Bounds reuse the owned ones: `SignedHeaderMaximumCount`,
    `SignedHeaderMaximumBytes`, and one new URL and document bound. Decoding
    runs `core.DecodeStrictJSONStructure` into a private temporary and writes
    the receiver only after the completed value validates, which is the exact
    sequence Core documents for typed boundary projection.
  - Red proof: fourteen production mutations were run. Ten were observed red and
    reverted: admitting absent wire members, skipping the vendor method check,
    matching the provider token case-insensitively, dropping the URL extent
    bound, skipping revalidation before the value escapes, admitting absent
    header members, letting a multipart vendor accept the signed-put token,
    replacing redaction with a real rendering, and raising or lowering the
    document byte bound by any amount. The data-flow struct inventory ratchet
    was separately observed red naming all three new structs.
  - Raising the document bound initially survived, which was a real table gap:
    every oversize case also tripped a sub-bound. It is closed by a case that
    pads a valid document with insignificant whitespace, isolating the document
    bound from the URL and header-set bounds, and the bound is now ratcheted on
    both sides.
  - Three surviving mutations are reported as behavior-equivalent rather than as
    closed gaps, because each is masked by a downstream gate that is itself
    ratcheted: admitting the zero provider as a token still fails at
    `Spec(ProviderUnknown)`; building the signed-header set without
    `NewSignedHeaders` still fails when the completed value revalidates; and
    dropping the unset check still fails at `Provider.Validate`. Mutation 5 in
    that list, removing the revalidation those two depend on, was observed red,
    so the layering is proved rather than assumed.
  - One contract question the tables answered: an empty signed header value is
    admitted. The mined Peachfuzz copy refused it, but an empty field value is
    legal HTTP and is what the already-landed `SignedHeader` owner admits.
    Refusing it only in the decoder would have been a second grammar.
  - `witness-lint` rejected a bare error-string search in the disclosure test.
    It was restructured rather than waived, and the restructure added the
    missing typed tier: the rejection is now proved by `errors.Is` first, and
    the rendering check is explicitly the second-tier operator-facing proof.
  - Proof: 67 subtests, focused tests, race with shuffle at `-count=2`, vet,
    staticcheck, errcheck, fieldalignment, production `gocyclo <= 10`, and
    `witness-lint` are clean for `./objectstore`; the full repository suite
    passes. `FuzzUploadCapabilityAdmitsOnlyTransferableCapabilities` completed
    252,080 executions in 30 seconds with no crashers written. Its oracle
    requires every rejection to carry this package's identity with an unset
    receiver, every acceptance to project onto a target the transfer entry point
    would itself admit, and no formatting verb to render anything but redacted
    text. The repository-wide delivery result is recorded below.
- Doctrine audit of capabilities 3 and 4, 2026-08-01: the two slices were
  re-read against the single-source-of-truth and compiler-owned rules. Three
  violations were found and fixed, and three findings are named below as open
  judgment calls rather than silently accepted.
  - Fixed, duplicated mechanic: `exchange` had its own copy of Core's canonical
    JSON encoder, including a second terminator constant. It was the same
    twelve lines as `core.MarshalCanonicalJSONString`, which is exactly the
    duplication class the 2b entry above says a second grammar creates. Core now
    owns `MarshalCanonicalJSONDocument`, `MarshalCanonicalJSONString` projects
    from it and keeps only its string-specific UTF-8 gate, and Exchange calls the
    Core owner and restates the failure under its own identity. Core and
    Exchange are two named Primitive packages, so PLAN section 2 is satisfied.
  - Fixed, duplicated constants in tests: three tables restated `256` and `1024`
    instead of binding to `APIRequestIDMaximumRunes`,
    `APIErrorMessageMaximumRunes`, and `APIErrorTipMaximumRunes`. The message
    and tip bounds are two separate contracts that are equal today; one shared
    literal would have hidden the day they diverge, so they are now bound
    separately.
  - Fixed, untested error identity: moving the encoder to Core left
    Exchange's identity restatement unproved. A mutation dropping it survived.
    `apiRefusingBody` is a payload that validates and then refuses to encode,
    which is the only way Core's encoder fails on the envelope path; the
    mutation now fires red and the test also proves the Core and originating
    identities stay reachable through `errors.Is`.
  - Mutation-harness correction: an earlier surviving mutation was a harness
    defect, not a test gap. Its anchor also matched an identical snippet in
    `APICode.MarshalJSON`, so the first-occurrence replacement mutated a
    different function. Re-run against a unique anchor it fires red. The two
    remaining survivors on that path are genuinely equivalent: Core returns nil
    bytes on failure, so returning them alongside the error changes nothing.
  - Fixed, restated vendor grammar in tests: the capability tests restated
    `X-Goog-Signature`, `X-Amz-Signature`, the signed-header query names, the
    HTTPS scheme, and the Cloudflare upload host that production owns as
    unexported constants. Exporting the grammar would be wrong, since a caller
    never constructs a signed URL, so both test files are now `package
    objectstore`, as `provider_validation_test.go` already was, and every vendor
    token binds to `queryGCSSignature`, `queryS3Signature`,
    `queryGCSSignedHeaders`, `queryS3SignedHeaders`, `httpsScheme`, and
    `cloudflareImagesUploadHost`. Zero vendor literals remain restated. The
    external-view proof is unaffected: interface satisfaction is identical from
    inside the package, and `capabilityCarrier` still decodes through
    `core.DecodeStrictJSON`, which is the real consumer path.
  - Fixed, chosen constant: `uploadCapabilityFramingBytes` was an 8 KiB
    allowance. Every term of `UploadCapabilityJSONMaximumBytes` is now derived:
    the URL bound, the owned signed-header aggregate,
    `SignedHeaderMaximumCount` times the exact JSON punctuation one header
    object adds, and the exact punctuation and member names of the widest
    document, both spelled as `len` of a real literal so the compiler computes
    them. The bound moved from 49,152 to 41,813 and is ratcheted on both sides.
  - Fixed, informal protocol: the envelope's arm was read as pointer nullity,
    a convention every consumer would repeat at each call site. `APIOutcome` is
    now a closed off-wire reading with `Outcome()`, and `Payload()` and
    `Failure()` extract the arm so a caller cannot dereference without checking.
    `ValidateSuccess` and `ValidateFailure` were removed rather than kept
    alongside them: the extracting accessors subsume both, and two ways to
    assert one contract is the duplication this audit exists to remove. This
    changes what Kernel and Peachfuzz call, which is the point.
  - Post-audit red proof: fifteen production mutations were each observed red
    and reverted, ten in `objectstore` and five in `exchange`, including the
    derived document bound moved in either direction, `Outcome` disagreeing with
    the arm it reads, `Payload` and `Failure` returning a zero body instead of
    refusing, `Payload` skipping validation, and `APIOutcome` admitting its zero
    reading. `APIOutcome` also has an exhaustive 256-value domain walk that pins
    it as off-wire.
  - Post-audit proof: `gofmt`, vet, staticcheck, errcheck, fieldalignment,
    production `gocyclo <= 10`, and `witness-lint` are clean for `./exchange`
    and `./objectstore`; `./core` carries only the sanctioned baseline. The full
    repository suite passes, as does `-race -shuffle=on -count=2`. Fuzz re-ran
    against the changed code: 504,263 and 233,501 executions in 30 seconds each,
    no crashers.
  - `witness-lint ./core/` reports 10 findings, all the stale off-wire-enum
    doctrine in `governance_contracts.go` and `test_isolation_contract.go`,
    which are untouched at HEAD and are already named in the sanctioned
    baseline. Zero findings are in any file this work modified. Correction to
    the handoff note, which claimed `./core/` was clean: it is not, and its own
    expected-findings list says so.
  - `filestore` `TestCreateOnlyWritersResolveRealNamespaceContentionWithoutPrimitiveLocks`
    failed once at 62.9 seconds when the full suite was run concurrently with a
    race-and-shuffle run on the same machine. It passes isolated and on a clean
    full run. The load was self-inflicted, but the test's wall-clock deadline is
    doing double duty as both a deadlock backstop and a throughput assertion,
    which `test/sync/no-sleep` warns against. Named for Ase; it is not in this
    slice and was not changed.
- Delivery review of capabilities 2b, 3, and 4, 2026-08-01: source review found
  and closed four gaps before publication.
  - Attest had rejected only the exact bytes `null`; a nested owner could emit
    whitespace-wrapped null. Nested output now passes Core's strict structural
    decoder, so exact and case-folded duplicate nested names are also refused,
    and all three hostile cases are ratcheted. Duplicate top-level name checks
    now compare byte spans directly without temporary string projections.
  - Exchange's `APINoBody` documentation said it existed only to instantiate a
    failure envelope, but its former always-valid value could become a success
    data arm encoded as `{}`. Its owned invariant now refuses that use, while a
    failure envelope remains valid because the absent data arm is not read.
  - Core's newly exported canonical document encoder now has a direct local
    success/error-identity ratchet in addition to its Exchange consumer proof,
    and the last raw `256` in the envelope round-trip test now binds to
    `APIRequestIDMaximumRunes`.
  - Objectstore now proves a rejected replacement decode preserves a previously
    valid receiver. Its private wire comment also states the actual typed rule:
    provider, method, URL, and expiry are required; absent and empty headers
    both project to the one empty `SignedHeaders` value.
  - Requested delivery proof passed: `go fix ./...` (no rewrite), `go vet`,
    `fieldalignment`, production `gocyclo -over 10`, `goconst`, `nilaway`,
    `errcheck`, `staticcheck`, `deadcode`, `deadcode -test`, `govulncheck`,
    `gosec`, `go test ./...`, and `go test -race -shuffle=on -count=2 ./...`.
    Direct `witness-lint ./attest ./exchange ./objectstore` is clean. The exact
    repository-wide `witness-lint ./...` and canonical gate stop only at the
    already-recorded enum, Process, and Testserial baseline after every prior
    gate stage passes; no finding names a file added or modified by this slice.
- Peachfuzz half, 2026-08-01 (not started, named so it is not lost): Peachfuzz
  repins and rewires only after capabilities 2b, 3, and 4 are reviewed,
  committed, and pushed. Its scope is the go.mod repin, retiring
  `protocol/wire_time.go`, `protocol/canonical_json.go`,
  `protocol/api_envelope.go`, and the absorbed part of
  `protocol/upload_capability.go`, moving `MachineEvidenceIdentity` out of the
  wire package under `internal/professor`, the door-to-door call-site rewire
  that `_docs/peachfuzz_policy.md:198-200` forbids doing mechanically, hostile tables
  for `protocol/` at the testing-protocol floor, and complete Foundation removal
  from all 80 importing files.
- Exchange review state: the package is a typed policy layer over the caller's
  real `*http.Client`, `net/http`, `io.Reader`, `io.Writer`, Go runtime, and OS
  network stack. It owns bounded aggregate JSON/byte operations, exact bounded
  streaming, replay eligibility, finite retry and server hints, total and
  per-attempt budgets, rejected or same-origin redirects, typed response
  metadata, and typed/native error reachability. It owns no DNS, TLS, proxy,
  connection pool, framing, socket, queue, worker, transport, copy engine,
  persistence, custody, or Objectstore policy.
- Exchange review corrections: HTTP field-value grammar now belongs to
  `Header.Validate`, including captured projections, after direct Go source
  inspection and a real TCP probe disproved the claimed reader/writer
  asymmetry. Upload preserves both `StatusError` and an oversized diagnostic
  drain failure. Response observation cannot return a silent zero success and
  does not fabricate response identity over a native dial failure. Retry-After
  parsing, every request-family validator, header collection bounds, closed
  modes, and the transport/response precedence path now have hostile proof.
  One single-behavior regression test no longer carries a false `LayerTriad`
  name. `JSONWriteCall` intentionally has no context because one bounded
  `ResponseWriter.Write` offers no honest mid-write interruption point.
- Exchange Core-export ownership deferral: `core.HTTPMethodCount` and
  `core.HTTPEndpoint.SameOrigin` presently have exactly one production
  consumer, Exchange, so a future export-to-consumer Sentinel will report both
  under PLAN section 5 until another named package consumes them. This is a
  deliberate, named deferral rather than drift. Keeping the method-domain size
  compiler-owned eliminates a latent replay-table index panic; keeping origin
  comparison on `HTTPEndpoint` prevents Exchange from copying HTTP schemes,
  default ports, and endpoint normalization. Private Exchange copies would
  violate section 1's single-owner law, so they are not introduced merely to
  silence the future ownership finding.
- Exchange current proof: focused ordinary and shuffled race tests pass at
  79.3% statement coverage; `parseRetryAfter` is 85% and
  `observedAggregateResponse` is 100%. Fresh full repository ordinary and race
  tests pass. Vet, staticcheck, errcheck, nilaway, production `gocyclo <= 10`,
  goconst, fieldalignment, gosec, govulncheck, actionlint, formatting, module
  tidiness, and diff checks pass. Direct Witness analysis of Exchange is clean.
  The canonical gate passes through nilaway and then stops only at the already
  classified stale Witness off-wire-enum JSON/String doctrine and retired
  Testserial convention. The measured M1 Max loopback benchmarks are
  3.626 GB/s, 111,523 B/op, and 129 allocs/op for upload and 3.643 GB/s,
  109,726 B/op, and 137 allocs/op for download; these are local measurements,
  not network promises.
- Exchange buffer-reuse proof: the fixed `sync.Pool` path is
  race-clean and preserves O(1) input-extent memory while reducing the measured
  10 MiB loopback download to approximately 42.2 to 43.6 KiB/op and 134 to 135
  allocations/op. Release scrubs the fixed extent before returning it, and an
  AST ratchet proves that order without reading memory after its ownership has
  passed back to the shared pool.
- Exchange JSON-boundary benchmark proof: exact 128-byte, 1-KiB, and 8-KiB
  encoded documents cross real Exchange client/server TCP loopback paths in
  serial and parallel forms. Separate request-construction and configured-limit
  controls prevent transport cost and benchmark setup from being confused.
  Result validation is non-vacuous and aggregate request-plus-response byte
  reporting is exact.
- Timeproof review state: the package owns RFC 3161 request construction,
  bounded response verification, and evidence replay. It owns no HTTP, TLS,
  retry, authority server, certificate acquisition, CRL service, clock,
  scheduler, state machine, or custody store. Refusals preserve
  `core.ErrTimeProofRefused` through `errors.Is` while exposing the authority's
  typed status and failure codes through `errors.As`. CMS and TSTInfo
  structure, versions, digest binding, signer-certificate attributes, TSA
  identity, pinned trust, nonce, serial, policy, generation time, and optional
  accuracy are verified before evidence is admitted.
- Timeproof current proof: focused ordinary, race, and tenfold shuffled tests
  pass at 78.8% statement coverage. A fresh 200,000-execution response fuzz run
  completed without a crasher. Vet, staticcheck, errcheck, nilaway, production
  `gocyclo <= 10`, goconst, fieldalignment, gosec, govulncheck, actionlint,
  formatting, module tidiness, and full ordinary and race repository tests
  pass. Direct Witness analysis has no unwaived finding; its two narrow waivers
  cover only RFC 3161's protocol-mandated ESSCertID v1 SHA-1 compatibility
  path. The canonical gate passes through nilaway and stops at the already
  recorded stale Witness off-wire-enum and retired Testserial doctrines.
- Timeproof benchmark evidence on the M1 Max: Prepare is 4,436 ns/op,
  2,725 B/op, and 73 allocs/op; authentic Verify is 699,644 ns/op,
  105,490 B/op, and 666 allocs/op; oversized-response rejection is 1,681 ns/op,
  1,312 B/op, and 37 allocs/op; and replay is 283,584 ns/op, 189,389 B/op, and
  1,304 allocs/op. The bounded response and evidence limits, rather than input
  extent, cap memory.
- Fuzz scope for this slice is limited to externally controlled text or bytes.
  Fresh 100,000-execution differential runs pass for Currency decimal text,
  Currency Amount JSON, and Garble Seed JSON. Keygen has no external parser and
  exhausts its complete 49-value valid size interval plus hostile effect-result
  matrices. Testserial has a finite typed enum and AST domain. Neither package
  receives a ceremonial fuzz target.
- Focused proof state for the accepted 2026-07-28 slice remains historical.
  The 2026-08-01 Witness polish supersedes its analyzer baseline: the upgraded
  pin recognizes `testserial.Declare` and off-wire enums. Fresh full proof for
  the uncommitted polish slice is recorded separately after review.
- Witness publication dependency: the upgraded analyzer now enforces the exact
  first-statement `testserial.Declare` shape and distinguishes off-wire enums.
  Its remaining publication gap is external capability ownership: process
  execution recognizes only Witness's own `internal/core` capability path and
  cannot yet recognize Primitive Core's typed equivalent. Primitive will not
  add a compatibility import, duplicated capability, manual `exec.Cmd`
  construction, or waiver to silence that upstream mismatch.
- Gate portability correction: `file_sha256` now selects `sha256sum` or
  `shasum`, extracts the digest without positional-parameter mutation, and
  validates the exact 64-hex-character shape. This addresses the observed
  Windows runner failure without changing evidence semantics.
- Objectstore scope authority: the user expanded the original GCS-only
  interview boundary to the official Amazon S3 whole-object PUT/GET, Google
  Cloud Storage XML PUT/GET, and Cloudflare Images direct creator upload
  contracts. Object naming and browser-side naming hashes remain above the
  package. Each call owns one provider and one stream; no fan-out, bucket,
  credential, signed-URL, draft-record, retry, scheduler, queue, state-machine,
  or recovery ownership entered the package.
- Objectstore operation selection is compiler-owned at the public boundary:
  `UploadS3`, `UploadGCS`, `UploadCloudflareImages`, `DownloadS3`, and
  `DownloadGCS` are the only transfer entry points. Targets carry no runtime
  provider selector, and the retired generic `Upload` and `Download` paths have
  no wrapper or compatibility alias.
- Objectstore proof: real TLS loopback paths pass for all three uploads and
  both whole-object downloads, including exact zero/short/exact/long streams,
  create-only and checksum headers, Cloudflare multipart framing, provider
  version projection, S3 full-object versus composite checksum semantics,
  provider checksum contradictions, conflict/absence, cancellation, native I/O
  errors, secret redaction, and commitment boundaries. Coverage is 88.0%; the signed
  URL fuzz boundary passed 100,000 executions; focused shuffle and race proof,
  full repository tests and race tests, vet, staticcheck, errcheck, nilaway,
  fieldalignment, production `gocyclo <= 10`, goconst, gosec, govulncheck,
  deadcode, actionlint, formatting, module-tidy, Linux build, and Windows build
  pass. Direct Objectstore witness-lint is clean with zero waivers. The
  canonical gate reaches only the recorded stale off-wire-enum and retired
  Testserial analyzer baseline.
- Objectstore M1 Max streaming benchmark: the public Objectstore-plus-Exchange
  pipeline holds exactly 91 allocations for both 1 KiB and 10 MiB inputs.
  The serial five-run 10 MiB range is 1.697 to 1.735 GB/s with approximately
  39.2 to 39.4 KiB/op. This proves source-size-independent allocation shape across a
  10,240-fold input increase; it is not a remote-provider throughput claim.
- Hostfacts scope: five explicit structure-to-structure operations own
  caller-available held-root disk capacity, the Go `Sys - HeapReleased`
  managed-memory metric, Linux cgroup current-plus-ancestor effective memory
  ceiling, held-root logical regular-file tree extent, and exact Go OOM-banner
  presence. There is no runtime observation selector, filesystem model,
  watcher, cache, scheduler, worker, fake clock, compatibility path, or process
  termination claim. Percent and regular-file count remain package-owned
  because no second named Primitive package currently requires them.
- Hostfacts proof: focused coverage is 85.4%; five shuffled race repetitions
  and the full repository test/race suites pass. The OOM fuzz boundary passed
  1,370,342 executions without a failure. Vet, staticcheck, errcheck, nilaway,
  fieldalignment, gosec, govulncheck, production `gocyclo <= 10`, formatting,
  module tidy, and diff checks pass. Linux amd64/arm64, Windows amd64, and
  FreeBSD amd64 test binaries cross-compile; Linux and Windows staticcheck
  pass. The canonical gate passes through nilaway and stops at the recorded
  stale Witness enum/Testserial doctrine. Direct Hostfacts findings are only
  demands for JSON and String methods on six intentionally off-wire enums;
  Hostfacts has no waiver.
- Hostfacts M1 Max bounded-stream benchmarks: OOM classification is
  8,205 ns/op for 1 KiB and 4,395,141 ns/op for 1 MiB, with 41,008 B/op and
  two allocations at both extents. Cgroup mountinfo scanning is 4,939 ns/op
  for 1 KiB and 204,544 ns/op for 1 MiB, with 69,704 B/op and five
  allocations at both extents. The 2026-08-02 recheck extended that benchmark
  through the actual mountinfo parser: 7,336 ns/op for 1 KiB and 2,324,590
  ns/op for 1 MiB, with 69,704--69,715 B/op and exactly five allocations at
  both extents. The parser now projects fields through `bytes.FieldsSeq`
  without constructing a per-line token slice or string. Tree traversal uses
  64-entry read batches and a
  path-component-bounded descriptor stack; its 1,000-file benchmark is
  14,060,135 ns/op, 166,585 B/op, and 4,152 allocations. Total traversal
  allocations necessarily reflect Go/OS directory entries and are not a
  retained-working-set claim.
- Hostfacts native proof covers real Darwin/APFS disk, tree, memory, and OOM
  paths plus real Linux cgroup v2 execution. A standard Colima host proved the
  cgroup-root `memory.max` absence and an unlimited current hierarchy; a
  Docker container with a 64 MiB memory limit proved the exact finite
  67,108,864-byte result through the public operation. Windows
  NTFS/ReFS/quota behavior remains cross-build/static proof, not native
  certification. Windows derives the capacity query path from the held
  directory handle and revalidates the volume serial afterward; it does not
  claim a direct handle-only capacity API.
- Hostfacts review corrections make a missing cgroup interface a typed
  no-declaration level, preserve the closest declared interface for unlimited
  results, saturate mount ambiguity instead of allowing count wraparound,
  reject unclassified tree entries, deterministically close refused
  directories, separate ingress-contract failures from observation failures,
  keep one public `Failure` owner per effect boundary, and keep nil out of the
  multi-error unwrap result when a `Failure` carries only its stable identity,
  as required by Go's `errors` contract. The public Go-memory operation
  discloses the stop-the-world cost of `runtime.ReadMemStats`. Mountinfo escape
  decoding projects its four admitted kernel spellings directly; the
  unreachable `strconv.ParseUint` failure branch after that closed admission
  check is gone.
- Fuzzfinder scope: `Find` accepts one validated `filestore.Location`, one
  explicit Go cache format, and one caller-selected retention limit. It opens,
  streams, and closes the real rooted directory through Go's `os.Root` and
  `fs.ReadDirFile`, retains the canonical lowest names in a fixed 128-entry
  array, and separately counts ignored directories, non-regular entries,
  over-limit observations, and unsupported regular files. Returned names are
  observations, not payload custody.
- Fuzzfinder format decision: `CacheFormatGo1_26` binds the exact
  16-lowercase-hex generated filename emitted by Go 1.26.5's
  `internal/fuzz.writeToCorpus` path. The archive's invented 8..64-character
  compatibility range is retired. An unknown regular filename returns a
  validated `ObservationUnsupportedFormat` plus
  `ErrFuzzFinderFormat`; partial directory reads retain observed facts and
  preserve both `ErrFuzzFinderObservation` and the native source error.
- Fuzzfinder proof: focused coverage is 97.4%; focused shuffled race proof,
  full repository tests and race tests, vet, staticcheck, errcheck, nilaway,
  fieldalignment, gosec, goconst, govulncheck, actionlint, production
  `gocyclo <= 10`, formatting, module tidiness, and diff checks pass. Linux,
  Windows, and FreeBSD amd64 test binaries cross-compile. The independent
  semantic generated-name fuzzer completed 12,988,618 review executions and a
  fresh 100,000-execution post-review run without failure.
  Direct Witness analysis has no waiver and reports only stale `String`
  demands on the two compiler-declared off-wire enums. The canonical gate
  passes through nilaway and stops at the recorded repository-wide enum and
  Testserial analyzer baseline.
- Fuzzfinder review corrections: `ArtifactKind` now inherits Core's one public
  JSON string-token admission rule, classification is mandatory ingress and is
  carried through every result, generated-name parsing derives its width from
  the declared format, impossible over-limit accounting is rejected, and the
  zero-information `EntryCount.Validate` ceremony is removed. Tests classify
  ordinary entry kinds and multi-batch traversal on real rooted directories;
  the non-native `fs.ReadDirFile` seam remains only for nil-entry,
  zero-progress, mid-read failure, and duplicate-name states that a real
  directory cannot produce in-process.
- Fuzzfinder M1 Max real-directory benchmarks: 128 entries complete in
  237,251 ns/op, 48,194 B/op, and 539 allocs/op; 8,192 entries complete in
  18,918,915 ns/op, 2,982,499 B/op, and 33,929 allocs/op. These numbers include
  `os.Root` open/stat/close and Go/OS directory-entry materialization. Total
  allocation is linear in directory entries; Fuzzfinder's retained working set
  remains the fixed 128-name array and 64-entry read batch.
- Lease contract: OGS signs exactly one `Grant`, `Refusal`, or `Revocation`
  decision for one opaque product, entitlement, and registered-installation
  subject. Every variant shares one revision, generation, and issuance time.
  Grant carries exact `NotBefore`, `ContactAfter`, `NotAfter`, and `GoodUntil`
  instants; recoverable refusal carries only the next-contact instant;
  revocation carries only one closed for-cause reason. Expected
  subject binding is mandatory during real Attest verification.
- Lease time decision: evaluation is pure and fixed-size. The caller supplies
  a real Temporal start/current observation and a durably committed
  high-water. Lease advances the signed issuance floor with Go's monotonic
  elapsed time, selects the maximum trusted instant, and rejects a wall
  contradiction beyond the compiler-visible five-minute tolerance. The caller
  persists the returned high-water before starting paid work. Lease performs
  no clock read, filesystem operation, network call, retry, goroutine, or
  background scheduling.
- Lease proof: focused coverage is 92.0%; focused shuffled race proof, full
  repository tests, doubled shuffled tests, and isolated full race tests pass.
  Vet, staticcheck, errcheck, nilaway, fieldalignment, gosec, goconst,
  govulncheck, actionlint, production `gocyclo <= 10`, formatting, module
  tidiness, and diff checks pass. Linux amd64/arm64 plus Windows and FreeBSD
  amd64 test binaries cross-compile. The strict signed-decision fuzz boundary
  completed 651,773 executions and the signed-document boundary completed
  754,047 executions in 30 seconds each without failure. Direct Witness
  analysis of Lease is clean with no waiver; the canonical gate passes through
  nilaway and stops only at the recorded repository-wide enum and Testserial
  analyzer baseline.
- Lease M1 Max median benchmarks across five runs: pure evaluation completes in
  929.6 ns/op, 128 B/op, and 4 allocs/op; real Ed25519/Attest verification
  completes in 58,736 ns/op, 2,002 B/op, and 52 allocs/op; canonical decision
  JSON completes in 4,326 ns/op, 1,361 B/op, and 40 allocs/op. Lease has no
  variable-size data plane: exact schema maxima bound every accepted document,
  so these are fixed-size cost measurements rather than a false asymptotic
  streaming claim.
- Lease legal-copy follow-up: on the OGS
  `legal/licensing-units-peachfuzz-coverage` branch, `legal/ogs_contract.md`
  was amended first and `alfred/website/agreement.go` was then brought into
  exact agreement. The copy now places enforcement at new-work boundaries,
  keeps existing-record inspection outside licensing state, makes revocation
  prohibit later contact under that decision, promises continued support for
  authentic binaries' compiled wire revision, describes signed continuity and
  offline absolute boundaries, and makes payment recovery automatic at the
  next permitted operation or check-in. OGS's Markdown/Go sync test passes;
  legal review remains external.
- Upgrade contract: Release remains the sole signed release authority.
  Upgrade accepts `release.PreparedRelease`, streams the exact artifact through
  Objectstore into the fixed unselected slot, applies a caller-owned Hostfacts
  free-space reserve, and verifies exact extent, SHA-256, and CRC32C through a
  bounded Filestore read. `TrialTarget.Command` is the compiler-owned absolute
  path products pass directly to Process; products retain ownership of argv,
  automated test meaning, old-data sandboxing, user trials, customer copy,
  telemetry, and ticket submission.
- Upgrade selection decision: one canonical typed selector names revision,
  slot, and exact Release artifact. A passing product-owned `TrialReport`
  creates a sealed `Promotion`; promotion re-reads the prior selector,
  re-verifies candidate bytes, atomically replaces only the selector, resolves
  the new primary, and then removes the former exact binary and empty slot.
  Cleanup failure returns the already-selected `Primary` with a typed
  `ErrUpgradeCleanup`; it never reports the old primary after selector commit.
  A failed or abandoned trial uses `DiscardTrial`, which verifies the exact
  candidate before removing it and refuses stale selection authority.
- Upgrade hostile findings: the first staging design could lose exclusive
  creation and then delete another attempt's candidate, while bootstrap
  collision cleanup could delete a pre-existing primary. Both now use
  compiler-owned in-call ownership receipts, so cleanup touches only bytes the
  failing call created. Effect recovery and owned cleanup use
  `context.WithoutCancel` only after filesystem state exists, preventing
  cancellation from stranding an owned partial file. Staging never
  pre-emptively deletes an occupied trial slot. A consumer-path review also
  removed the need for products to reconstruct executable paths with copied
  `filepath.Join` conventions.
- Upgrade review corrections, `2026-07-31`: an instrumented probe against the
  submitted package produced red evidence for six defects, all fixed.
  1. Staging was not crash-recoverable. Any interrupted attempt left bytes in
     the unselected slot, and the exclusive-create download then failed with a
     namespace conflict forever, including when the occupant was the exact
     authenticated candidate. `TrialTarget` is unconstructable outside `Stage`,
     so no public call could reclaim the slot and the ledger's own
     "a restart restages the candidate" claim was false. `Stage` now reclaims
     through one durable typed trial receipt written before candidate bytes.
     `reclaimCandidateSlot` adopts an occupant that verifies as the exact
     receipt-bound artifact and removes only partial bytes owned by that same
     attempt. A receipt for a different live candidate conflicts without
     deleting it. The slot is provably not the primary, because
     `stageAuthority` already matched the on-disk selector to the prior
     selection and the candidate slot is its opposite.
  2. Failing to create the candidate slot was reported as
     `FailurePhaseCleanup` with `ErrUpgradeCleanup`, telling an operator the
     failure came after bytes landed. It is now persistence.
  3. `Promote`'s post-commit removal of the former slot used the caller's
     cancellable context, so a cancelled caller stranded the old binary and
     wedged the next candidate's slot. It now settles through
     `recoveryContext`, as does bootstrap's post-write cleanup and the
     verification-failure removal.
  4. `writeBootstrapArtifact` returned an unowned receipt when Filestore
     recovery failed, which is exactly the case where the target name may
     exist. That state is now owned and cleaned.
  5. `mustByteCount` silently returned zero for an unrepresentable extent,
     which is a swallowed error on a verification path.
  6. Upgrade bounded its own selection document with Release's exported
     document maximum and `release.TargetCount`, an informal cross-package
     contract for a document Release does not own. Upgrade now owns
     `selectionDocumentMaximumBytes` and `selectionArrayItemMaximum`, and the
     Release export widening was reverted, leaving Release untouched.
  `absoluteBinaryPath` also gained the explicit slot gate that every other path
  builder already had, and the three enums moved to package-level label tables
  with the house compile-time limit guards.
- Upgrade final hostile corrections, `2026-07-31`: reviewing the review found
  three more live authority and crash failures.
  1. The first reclamation correction still treated every nonmatching occupant
     as crash debris. Every version of one offering uses the same fixed binary
     path, so a sequential Stage for v3 deleted a v2 `TrialTarget` that a caller
     was still testing. Red proof observed a nil reclaim error and missing v2
     bytes. The durable `trialDocument` now binds the exact prior selector and
     candidate before download. Same-candidate restart adopts authentic bytes
     or replaces only its own partial bytes; a different candidate returns
     `ErrUpgradeConflict` and preserves the active trial.
  2. A stale in-memory `TrialTarget` could Promote or Discard after the durable
     trial owner changed, including when two signed builds had identical
     bytes. Promote and Discard now re-read and require the exact typed receipt
     before verification or mutation. Promotion removes the consumed receipt
     after selector commitment, and all exact cleanup removes fixed receipt
     names without recursion.
  3. A process crash leaving `.primitive-primary.next` wedged the next selector
     write for the same exclusive-create reason as the original candidate bug.
     `writeSelection` now settles only Upgrade's fixed temporary before
     starting the next atomic Filestore write.
  `AttemptError.Error` now names the bounded offering, version, platform, and
  commit while continuing not to render a signed download capability; the
  native cause and stable Core identity remain reachable through `Unwrap`.
  Direct Witness analysis also found the test suite was searching rendered
  error text even though Upgrade diagnostics are typed enums in the error
  graph. Every diagnostic assertion now uses `errors.Is`; no waiver was added.
- Upgrade review test corrections, `2026-07-31`: the zero-value ingress table
  asserted only `ErrUpgradeContract`, which every Upgrade identity parents to,
  so it could not separate an unset primary from an unset root; it is now
  two-tier. The selection-decoder and trial-report rejection loops were
  index-named with no typed assertion; both are now named hostile tables, the
  decoder at thirty rows. The verification table had no passing row at all, so
  no fixture proved the accept path. New coverage: `validateUpgradePair`, the
  diagnostic table's totality and distinctness, slot projections across the
  whole byte domain, and the failure-phase honesty ratchet. The production
  "no world model" scan was a raw substring search over file bytes that could
  not tell code from prose and missed `go\tf()`; it is now an AST match with a
  synthetic-source proof that the matcher is not vacuous, plus a named-exception
  ratchet that every settled removal carries `recoveryContext`.
- Upgrade processing proof: candidate download and verification are streaming
  with fixed hash state; selector processing is bounded by Upgrade's own
  8 KiB selection-document maximum and trial-receipt processing by its own
  16 KiB maximum. Cleanup names one binary, two fixed receipt names, and one
  empty directory and never recurses. Production contains no goroutine, map,
  scheduler, queue, worker, generic state machine, command execution, or
  retained artifact buffer. The package's six sibling imports exactly match
  Core's existing Upgrade frontier, and production `gocyclo <= 10`.
- Upgrade hostile evidence: focused coverage is 65.7%, used as disclosure
  rather than a quality proxy. Real rooted-filesystem tests prove canonical
  selection and trial-receipt persistence, fixed-temporary recovery, exact
  hashing at short/equal/long extent
  contradictions, stale-promotion refusal, post-commit cleanup truth,
  occupied-slot preservation, exact-candidate restart, different-live-trial
  preservation, stale-target promotion/discard refusal, bootstrap collision
  preservation,
  cancellation-resistant owned cleanup, rejected-trial discard, and the
  direct Process command projection. The selection fuzzer completed 2,380,986
  executions and the trial-receipt fuzzer completed 1,665,491 executions,
  each in 30 seconds with no failure. The public authenticated
  `Stage`/`Bootstrap` success composition is not fabricated inside Upgrade
  tests because constructing Release and Objectstore witnesses would add
  forbidden Attest and Exchange package edges; Release/Objectstore own those
  proofs, and a live-provider Upgrade proof remains pending caller-supplied
  signed S3 or GCS authority.
- Upgrade verification: focused shuffled race tests, full repository tests and
  race/shuffle tests, vet, staticcheck, errcheck, nilaway, gosec, goconst,
  govulncheck, actionlint, fieldalignment on touched packages, direct
  Witness analysis, formatting, module tidiness, and diff checks pass.
  Linux amd64/arm64, Windows amd64, and FreeBSD amd64 Upgrade test binaries
  cross-compile. The canonical gate passes through nilaway and stops at the
  existing repository-wide Witness enum/Testserial/Process baseline; Upgrade
  itself has no Witness finding or waiver. The repository-wide fieldalignment
  command continues to report only two pre-existing Shutdown test fixtures.
- Upgrade M1 Max fixed-selector benchmark: the actual shipped 4 KiB
  `ResolvePrimary` path, including selector read/strict decode and complete
  artifact re-hash, has a five-run median of 234,136 ns/op, 143,458 B/op, and
  1,937 allocs/op. The byte path is streaming and memory is bounded, but these
  costs are disclosed without calling the operation cheap or allocation-free.
- Gate contract: Primitive Gate is only the authentic CLI-side new-work
  boundary. `AuthorizeNewWork` accepts one proof-carrying `lease.Assessment`;
  its private `NewWorkPermit` closes only over current and continuity, while
  `DenialError` retains the exact rejected assessment for typed state and
  contact recovery. Products own exhaustive command-to-new-work mapping and
  bypass Gate entirely for permanent existing-record inspection, check-in,
  registration, and recovery. OGS owns a separate hard server boundary through
  its route/auth gates and handler-specific authority; a client permit never
  authenticates an OGS request, and OGS authentication never substitutes for
  the signed Lease assessed by the CLI.
- Gate archive reconciliation: the preserved signed action-policy package was
  rejected rather than repaired. Its second signed policy duplicated the
  settled OGS Grant/Refusal/Revocation authority, its seventeen actions copied
  product workflow, its bare observed time could roll authority backward, and
  its standalone verification did not bind expected subject. Gate therefore
  imports only Core and Lease and creates no signing domain, action vocabulary,
  policy document, clock, persistence, transport, scheduler, callback, worker,
  or state machine.
- Core reopening for declared test edges: `_docs/primitive_policy.md` section 5 names an
  undeclared test edge as a Sentinel failure and a term of the coupling
  coefficient, but the catalog modelled only production edges and the landed
  scan audited production and test sources against one conflated allowlist.
  Gate is the first package to prove the gap, because a valid
  `lease.Assessment` is deliberately unconstructable without a real Attest
  signature. Core now owns `DirectTestImportContract` and
  `PrimitiveDirectTestImportCount`, and admits exactly `gate -> attest` and
  `gate -> temporal`. A declared test edge grants no production dependency,
  counts against the same `PrimitiveMaximumDirectImports` ceiling, may not
  duplicate a production edge, and is rejected when no test source spends it.
  The landed scan now audits production sources against the production
  frontier alone, which also closes the prior hole where a declared production
  edge could be satisfied by a test-only import.
- Gate proof: the Lease-state disposition test exhausts all 256 underlying
  values; only current and continuity permit, every other admitted state
  denies, and every unknown or future value fails closed with both Gate and
  Lease identity. The external suite drives the real path end to end: an
  Ed25519-signed grant, refusal, and revocation are verified through Attest and
  Lease, evaluated at one exact instant, and authorized. It pressures both
  sides of every grant boundary, including the exact `NotBefore`, `NotAfter`,
  and `GoodUntil` instants, and proves the permit carries the caller's exact
  assessment while each denial carries the exact state and contact posture.
  The internal suite forges both capabilities over the wrong authentic state
  and proves each rejects itself. Zero request, zero permit, zero denial, the
  complete diagnostic-boundary domain, denial-versus-contract identity
  separation, exact operator-facing text, exact public function surface,
  production struct data-flow roles, and the exact production and test-only
  import frontiers projected from the Core catalog are ratcheted. Focused
  statement coverage is 89.6%; the remainder is the fail-closed arms that no
  admitted state can currently reach.
- Gate defects corrected in review: `AuthorizeNewWork` returned a populated
  `NewWorkPermit` alongside the validation error when a permit failed its own
  gate, so a caller could hold a capability and a failure at once; it now
  returns the zero permit on every error path. `DenialError` was a pointer
  whose `Error` and `Unwrap` answered on a nil receiver, so a nil denial
  carried Gate's denial identity with no assessment behind it; the reviewed
  correction first changed it to a value and deleted the test that blessed the
  nil receiver, before the independent hardening below closed the remaining
  zero-value form. Permit and denial validation conflated a domain error with a
  wrong-disposition result and discarded the distinction; each now returns its
  own named boundary. The package architecture test restated the import
  frontier instead of projecting it from Core, and the struct inventory listed
  names without classifying roles.
- Gate independent hardening: the reviewed change from pointer to value removed
  the nil receiver but left `DenialError{}` externally constructable as an
  `error` that unwrapped to `ErrGateDenied` without an assessment; a
  `*DenialError` typed nil also inherited the value-receiver methods and could
  panic through the error interface. The permanent zero-denial ratchet was
  observed red. `DenialError` is now a sealed interface whose unexported method
  can be implemented only by Gate; the package returns one private value
  implementation after validation. The zero interface is nil and cannot claim
  denial identity, while authentic denials remain recoverable through
  `errors.As`. The two unreachable wrong-disposition guards now use their
  compiler-owned `ContractBoundary` rather than untyped local error strings.
  Core's test-edge contract matrix was also extended across unset and future
  endpoints, Core/self edges, duplicates, production overlap, a Testserial
  target, and the combined production-plus-test coupling ceiling. Per-package
  import and violation inventories now derive their exact capacities from the
  package catalog instead of the unrelated repository-wide production-edge
  count.
- Gate canonical gate: module tidiness, workflow, formatting, full ordinary and
  race tests, vet, staticcheck, errcheck, and nilaway pass. The canonical gate
  then stops exactly at the existing repository-wide Witness enum, Process,
  and Testserial baseline. Direct Gate Witness analysis is clean with no
  waiver. Five focused shuffled race repetitions pass; Gate coverage remains
  89.6 percent, with only compiler markers and fail-closed arms unreachable
  from the admitted state domain. Production `gocyclo <= 10`, Gate-scoped
  goconst, touched-package fieldalignment, full gosec and govulncheck, full
  build, and Linux amd64/arm64, Windows amd64, and FreeBSD amd64 Gate
  cross-builds pass. A post-baseline repository-wide goconst run reports only
  seven existing cross-package literal pairs outside Gate; no compatibility
  constant or copied literal was added to silence them.
- Receipt contract, `2026-07-31`: the archive was reduced rather than restored.
  Receipt signs one typed `EvidencePayload` through Attest: receipt, account,
  offering, revision, occurrence instant, submission, object, extent, SHA-256,
  and CRC32C. Verification authenticates before comparing caller intent and
  reports the exact mismatched field through typed `ScopeMismatch`; malformed
  evidence cannot borrow scope diagnostics. Receipt owns nominal 16-byte
  account, offering, submission, and object identities, with canonical
  lowercase hexadecimal persistence and distinct non-convertible Go types.
  Core retains only their stable error identity.
  The compiler-owned graph admits exactly `receipt -> core, attest, temporal`,
  bringing the catalog to 21 production packages plus Testserial and 50
  production edges while retaining the six-edge ceiling and zero undeclared
  coupling.
- Receipt watermark design: `Watermark` is one fixed-size durable fact over
  revision, account/offering scope, positive generation, remote-cursor digest,
  and accepted-chain hash. `AdvanceWatermark` is a pure comparison, not a
  state-machine or persistence framework. Identity and scope decide before
  generation; lower generations roll back, equal generations require exact
  replay, and a higher generation must replace both closures. Callers own the
  durable write and transaction boundary. The archived Payment, Page, Summary,
  Store, provider-version field, derived watermark identity, and duplicated
  body digest were rejected because no current independent reader justified
  them or because they rebuilt transport/persistence state above existing
  Primitive owners.
- Receipt hostile corrections found during implementation: `fieldalignment`
  proved that using exported struct declaration order as canonical JSON order
  was an implicit wire contract—alignment reordered `EvidenceBody` and
  `Watermark` while all length checks remained green. Dedicated pointer-only
  projection structs now own canonical member order independently of memory
  layout. Gosec also found a signed-to-unsigned JSON-bound conversion; the
  conversion now crosses Core's checked numeric boundary before constructing a
  byte count. Both are retained as structural and tool ratchets.
- Receipt defects corrected in review, `2026-07-31`: the projection correction
  above was incomplete and unproven. Only `EvidenceBody` and `Watermark` had
  received a projection; `Header`, `EvidencePayload`, `EvidenceDocument`, and
  `Scope` still reached the encoder through a local alias of their own exported
  type, so the two structures inside the signed payload still derived signed
  member order from memory layout. Nothing tested member order at all, so the
  original correction could not have failed. Every canonical structure now has
  one pointer-only wire projection used in both directions, the duplicate
  `evidenceBodyProjection` and `watermarkProjection` twins were deleted, and
  three ratchets were added: exact member order for every signed and durable
  structure, exact signed payload bytes for a determined fixture, and a
  structural proof that every wire structure is pointer-only.
  Five further defects were corrected. `AdvanceWatermark` returned a populated
  result alongside its error on both success paths, the shape Gate's review
  already removed. `ScopeMismatch` was an exported struct with an exported
  field, so any caller could build a value carrying `ErrReceiptScope` with no
  authenticated fact behind it, and a test asserted that this worked; it is now
  a sealed interface over a package-private value, matching `gate.DenialError`.
  `AdvanceResult.State`, `AdvanceResult.Watermark`, `VerifiedEvidence.Document`,
  `VerifiedEvidence.Header`, and `VerifiedEvidence.Body` returned silent zeros
  for an unset receiver, which a caller cannot distinguish from a real answer;
  all five now return an error, and a test blessing the silent zeros was
  replaced by one requiring refusal. `EvidenceBody.Validate` enforced the
  empty-stream integrity rule in one direction only, admitting a nonzero extent
  that claimed the digest of zero bytes; the rule is now biconditional. Watermark
  conflict rejection collapsed its distinct causes into one opaque sentinel
  with no diagnostic; `ConflictReason` and a sealed `WatermarkConflict` now name
  the four behaviorally reachable conflict invariants, with cursor-before-chain
  precedence proved. Rollback remains its own typed rejection.
  The lifecycle decoder formerly in Core carried the wrong identities: a
  malformed JSON token returned `ErrLifecycleIdentityContract` without
  `ErrJSONContract`, while an invalid identity returned `ErrJSONContract`, so a
  caller classifying wire failures missed every malformed lifecycle token.
  `IssueEvidenceRequest` built the signed payload twice, once in `Validate` and
  once in `IssueEvidence`; one `payload` projection now feeds both. The
  unexplained `(7 - 3)` in the envelope bound is now two named constants.
- Receipt weak tests replaced in review: two tables derived their expected
  outcome from their own case names, through `strings.Contains(tc.name,
  "admitted")` and an equality check against one case name, so any rename
  silently changed the assertion; both now carry a typed `wantErr`. The closed
  enum test proved only that admitted values had nonempty text, which cannot
  see two swapped labels, so exact label content and cross-label distinctness
  are now proved. The evidence-body, watermark-advance, and sealed-result
  tables were below the `test/boundaries` floor and are now at or above it,
  with both sides of every generation, extent, and closure boundary. New proofs
  cover strict scope JSON, the nominal closure constructors, `Watermark`
  validation branch by branch, forged sealed rejections, and non-sensitive
  diagnostics. Four mutations were run against the new proofs: reverting the
  integrity rule to one-sided, swapping conflict precedence, reordering
  `headerWire`, and restoring a silent-zero accessor each produced targeted red,
  and production was restored byte-exact afterward.
- Receipt post-review ownership and dead-path correction, `2026-07-31`: the four
  lifecycle identities were non-error facts exported by Core for exactly one
  landed consumer, violating `PLAN` sections 2 and 5. They now live directly in
  Receipt; no alias, wrapper, compatibility type, or copied Core definition
  remains. A compiler-red move exposed every old call site before the real
  callers and inventories were updated. The stable lifecycle error remains in
  Core, as required for shared error classification. Receipt's canonical
  decoder now preserves native `encoding/hex` errors through `errors.As`; the
  previous formatted wrapper retained prose but destroyed that typed cause. A
  hostile red probe proved the loss before correction. The advertised
  `ConflictReasonRevision` was also dead: both watermarks must validate before
  comparison and the only admitted revision is `RevisionV1`, so two valid
  revisions cannot differ. A totality ratchet failed red for the unreachable
  enum member; the member and branch were deleted instead of preserving a
  speculative path.
- Receipt proof: focused statement coverage is 93.5 percent. Tests exhaust all
  256 values of every closed enum, press zero and maximum identities,
  generations, instants, and extents, prove canonical empty integrity, mutate
  every signed and expected field, preserve receivers across strict JSON
  failure, attain the exact payload/document/watermark maxima, retain native
  writer errors, validate sealed projections, and inventory every production
  struct by compiler-visible data-flow role. The post-review signed-document
  fuzz pass completed 3,559,238 executions in 30 seconds; its oracle either
  authenticates the exact signed fixture or requires a typed verification
  failure, and every accepted document round-trips canonically. Apple M1 Max
  five-run medians are 66,380 ns/op, 2,924 B/op, and 50 allocs/op for complete
  evidence verification, and 203.1 ns/op, zero B/op, and zero allocations for
  watermark advance. Full ordinary and shuffled race tests,
  vet, staticcheck, errcheck, nilaway, fieldalignment on touched packages,
  production gocyclo at or below 10, Receipt-scoped goconst, full gosec and
  govulncheck, actionlint, formatting, module tidiness, diff checks, direct
  Receipt Witness lint, and Linux amd64/arm64, Windows amd64, and FreeBSD amd64
  cross-builds pass. The canonical gate passes through nilaway and then stops
  at the recorded repository-wide Witness enum, Process, and Testserial
  baseline; Receipt itself has no Witness finding or waiver.
- Receipt review-proof correction: direct nilaway and Witness lint initially
  disproved the review report's clean-tool claim. The canonical-member-order
  walker could slice an empty stack on malformed structure, one failure omitted
  its observed error, and one diagnostic test searched error text despite
  already proving exact sealed output. The walker now rejects decoder and
  delimiter faults, the failure carries got/want context, and the redundant
  substring search is gone. Direct Receipt Witness lint and repository nilaway
  are green after those test corrections. The canonical gate passes module,
  workflow, formatting, ordinary tests, race, vet, staticcheck, errcheck, and
  nilaway, then stops at the existing repository-wide Witness enum, Process,
  and Testserial baseline; it reports no Receipt finding or waiver. Full
  repository goconst still reports the prior seven literal pairs plus two
  Receipt-era semantic coincidences (`replay` and `crc32c`); Receipt-scoped
  goconst is clean, and unrelated enum domains were not coupled through a
  manufactured Core constant.

Next:

1. Begin the consumer-upgrade phase. Select the first Witness package surgery
   from its interview evidence, pin the published Primitive revision, delete
   the superseded implementation, and preserve direct typed ownership.
2. Keep Controlstate deferred. Its complete interview finds no real consumer
   requiring the archived nine-domain atomic aggregate, and its 30-minute
   expiry conflicts with demonstrated offline Lease authority. Re-entry needs
   new source-cited consumer proof and explicit approval under `PLAN` section 3.
3. Run disposable Objectstore live-provider proof when caller-supplied S3/GCS signed URLs
   and a Cloudflare Images one-time upload URL are available; local tests do not
   claim remote-provider proof.
4. Preserve the deferred Witness Testserial/analyzer consumer surgery without
   compatibility paths or analyzer waivers.

## Packages

State vocabulary: `NEXT`, `BLOCKED_BY_DEPENDENCY`, `NOT_STARTED`, `RED`,
`IMPLEMENTING`, `CONSUMER_SURGERY`, `REVIEW`, `DONE`.

| Package            | State                   |
| ------------------ | ----------------------- |
| `core`             | `DONE`                  |
| `attest`           | `DONE`                  |
| `contextstate`     | `DONE`                  |
| `currency`         | `DONE`                  |
| `garble`           | `DONE`                  |
| `keygen`           | `DONE`                  |
| `testserial`       | `CONSUMER_SURGERY`       |
| `filestore`        | `DONE`                  |
| `hostfacts`        | `DONE`                  |
| `temporal`         | `DONE`                  |
| `exchange`         | `DONE`                  |
| `fuzzfinder`       | `DONE`                  |
| `lease`            | `DONE`                  |
| `gate`             | `DONE`                  |
| `receipt`          | `DONE`                  |
| `process`          | `DONE`                  |
| `release`          | `DONE`                  |
| `shutdown`         | `DONE`                  |
| `objectstore`      | `DONE`                  |
| `timeproof`        | `DONE`                  |
| `cloudidentity`    | `DONE`                  |
| `upgrade`          | `BLOCKED_BY_DEPENDENCY` |

Consumer state is recorded under the active package only. There is no separate
all-consumer phase.

`contextstate` remains `DONE` after the 2026-08-01 top-down simplification.
`Validate` is the usable-now admission gate, `ObserveAfterDone` carries the
post-event precondition in its name, and `Observe` returns the current exact
standard terminal state. State JSON, token parsing, arbitrary-error
classification, graph traversal, a standalone nil-check API, and the global
no-clone source policy are rejected as unproved ceremony or duplicate
standard-library ownership. `String` remains diagnostic-only. The no-wire
decision is proved by asserting that neither the value nor pointer receiver
implements a standard marshaling interface; `OffWireEnum` is the positive
compiler contract and the interface-absence test is the negative proof.

## Closed facts

- `2026-07-27`: The historical implementation was preserved on NAS and at
  `/Users/d/code/foundation_back_up_july_27th_2026`.
- `2026-07-27T03:35:41-04:00`: Archive commit
  `d046f7b675fcb797398d7cdc87b5504f43978056`.
- `2026-07-27T16:59:16-04:00`: Published new-repository root commit
  `989ebc3dab4e1d3a76afc8dc7ca305b780ba665e` to
  `git@github.com:offGridSoft/foundation.git`.
- `2026-07-27`: Recon completed for 33 archived Go packages and the
  specification-only Unleash surface. The 34 interviews remain under
  `_docs/interviews/`.
- `2026-07-27`: The user approved the 20-package, 47-edge initial graph.
- `2026-07-27`: The schema, review-verifier, digest, packet, and permanent
  citation-checker approach was rejected and removed.
- `2026-07-27`: Pre-implementation graph synthesis returned `PASS` and the
  governance synthesis returned `NO BLOCKERS`.
- `2026-07-27`: The isolated implementation review returned four blockers and
  nineteen total findings. Its `23:05 EDT` second pass marked fourteen of the
  original nineteen closed, H6 and M13 partial, M10 open, and added N1 through
  N6. The temporary review file was intentionally deleted before the accepted
  Contextstate reconciliation revision.
- `2026-07-28`: Fourth-pass findings N13 through N18 were corrected and the
  complete durable Core gate passed. N9 remains an explicit performance
  disclosure rather than a stdlib-equivalence claim.
- `2026-07-28`: The user accepted Attest after two red-first corrections split
  canonical-output and admitted-document bounds and closed fixed-frame extent
  failure. The fresh accepted canonical gate passed and publication was
  authorized. A proposed million-execution local fuzz budget was explicitly
  returned to 100,000 per target after its corpus-management cost proved
  disproportionate; the interrupted bundle was deleted and is not evidence.
- `2026-07-28`: The Currency, Garble, Keygen, and Testserial review pass made
  three red-first corrections and closed four unprovable invariants. Decimal
  parsing now reports `ErrCurrencyOverflow` and therefore `ErrNumericOverflow`
  for the int64 minor-unit domain instead of `ErrCurrencyDecimal`, so one
  caller question has one answer; the signed domain moved into `signedValue`
  as its single owner, which retired two unreachable bounds and the dead
  `decimalOverflowDetail` text. `ParseTestIsolationHazardGoIdentifier` and
  `ParseTestIsolationScopeGoIdentifier` reject the empty identifier explicitly
  rather than depending on every enum member carrying a switch case. New
  proofs cover the invalid-value half of both isolation identifier domains,
  the `Argument` kind-to-seed relation and closed `Text` switch through a new
  garble internal test, consumer stop at every position of the build-argument
  sequence, every zero-value garble boundary, and forged non-Ed25519-width
  signing custody. Testserial stopped declaring a runtime-allocation hazard it
  does not have. `currency/decimal.go` and `core/test_isolation_contract.go`
  now carry no unexercised statement.
- `2026-07-28`: The accepted review was independently checked before
  publication. The nine decimal overflow identity failures were reproduced
  against reconstructed pre-fix behavior. Both empty analyzer identifiers
  were proved to resolve silently to a future admitted enum value when the
  explicit gates were removed. Testserial's process-output hazard was confirmed
  directly from its `testing.RunTests` path. Garble's runtime-impossible
  canonical JSON extent branch was deleted while the external canonical-extent
  test retained the ratchet. Generic Keygen entropy now delegates the all-zero
  rule to `core.NewSecretMaterial` and only classifies the Core-owned rejection.
  Testserial's accepted-declaration test now truthfully exercises the
  process-output hazard it declares. Currency's exact-case token router remains
  because `encoding/json` deliberately case-folds struct field names;
  `Declare(*testing.T, ...)` remains test-only by contract; public-key temporary
  clearing remains harmless ownership cleanup; and `fmt.Appendf` modernization
  is not a contract correction. The testing protocol's retired `Serial` prose
  was updated to the typed `Declare` contract.
- `2026-07-28`: Witness consumer surgery exposed Filestore removal's missing-
  parent neutral case. `Remove` had correctly classified an absent target as
  idempotent but then attempted to synchronize a parent whose namespace had
  not changed and might itself be absent. A red-first extension of
  `TestRemovalLayerTriad` reproduced the native `ENOENT`; `Remove` now returns
  immediately on `fs.ErrNotExist`, while successful removals still synchronize
  the real parent and other failures retain typed and native cleanup identity.
- `2026-07-29`: Peachfuzz consumer surgery exposed Filestore's rejected
  target-late crossing. A content digest cannot name its final directory until
  after bounded staging completes, while the old `CommitRequest` admitted only
  a same-directory target. A red-first cross-directory activation triad now
  proves create and replace through one real `os.Root`, missing-parent native
  failure without stage loss, and idempotent recovery of already-landed link
  and rename effects. Create synchronizes the target parent before removing
  the stage name and then synchronizes the stage parent; replace and recovery
  synchronize both affected parents. The OS still owns namespace arbitration,
  and no copy, scheduler, lock, transaction model, or filesystem wrapper was
  introduced.
- `2026-07-29`: The user reviewed Timeproof, supplied concrete RFC 3161/CMS
  hardening findings, accepted their corrected implementation, and explicitly
  authorized commit and push together with the completed Exchange JSON
  boundary benchmarks. No tag or release was authorized.
- `2026-07-29`: Cloudidentity was reconstructed as one bounded outbound bearer
  with explicit `AcquireGoogleCloud` and `AcquireAmazonWebServices` functions.
  Google explicitly requests the standard service-account token format. AWS
  accepts only an already query-signed regional STS capability, binds every
  consumed XML element to the requested API namespace, rejects ambiguous
  element multiplicity or token markup, and structurally redacts capability
  construction and acquisition failures. The two providers derive distinct
  bare-token and token-plus-envelope response bounds. Cloudidentity does not
  discover credentials or implement SigV4. Real loopback HTTP, cancellation,
  redirect, native-error, response-bound, XML, redaction, race, shuffle, fuzz,
  allocation-benchmark, and architecture proofs pass at 92.1% statement
  coverage. Latest Apple M1 Max loopback benchmarks: Google 1 KiB
  56,164 ns/op and 12,679 B/op; Google 16 KiB 89,825 ns/op and 60,200 B/op;
  AWS 1 KiB 73,162 ns/op and 24,644 B/op; AWS 16 KiB 250,005 ns/op and
  169,628 B/op. Direct Witness-lint analysis is clean with no waivers. The
  canonical gate stops only at the recorded repository-wide enum and
  Testserial analyzer baseline. No live Google metadata or AWS STS call is
  claimed. User review and publication authorization remain pending.
- `2026-08-03`: The accepted Objectstore upload-capability projection exposes
  the receiver's exact bounded wire contract without exposing its private wire
  type. The projection is encode-only, validates the closed provider and
  method domains, preserves signed headers, and redacts capability material.
  Core canonical JSON now uses the standard library's compact HTML-escaped
  spelling, making direct custom-marshaler output a fixed point when embedded
  through `encoding/json`; Objectstore, Attest, Currency, and HTTP endpoint
  proofs cover that boundary. Focused and repository-wide tests, race tests,
  vet, Staticcheck, Errcheck, Nilaway, Witness-lint, Gosec, Govulncheck,
  field-alignment, and production complexity gates passed. The canonical gate
  reached its final Deadcode step, where the analyzer remained idle without a
  child process and was interrupted; no Deadcode result is claimed. The user
  reviewed the findings and explicitly authorized commit, push, and the next
  Primitive tag.
- `2026-08-03`: Filestore read acquisition now opens a rooted Unix descriptor
  with `O_NONBLOCK`, proves regular-file identity on that acquired handle, and
  restores ordinary blocking reads through `SyscallConn`; this removes the
  stat-then-open FIFO race without adding coordination or a filesystem model.
  The write/read fuzzer independently checks the OS-visible committed bytes.
  Linux physical-memory scaling now rejects a zero kernel unit before its
  overflow division while preserving the Core-owned host-observation identity;
  zero-unit and overflow details remain diagnostics, not new caller decisions.
  A five-second subprocess startup assertion failed under the full
  race suite while 20 focused race repetitions completed correctly, so both
  FIFO subprocess tests now use a 30-second deadlock backstop. Focused package
  tests and race tests, Vet, Staticcheck, Errcheck, Nilaway, Witness-lint,
  Gosec, Govulncheck, field alignment, production complexity, and Linux,
  FreeBSD, AIX, and Windows cross-compilation pass; the Linux physical-memory
  boundary table also passed 20 executions on a real Linux/arm64 Go runtime.
  The user required a fully
  clean Primitive worktree and authorized publication as `v2026.0.9` after the
  repository-wide gates and exact diff review.
- `2026-08-03`: Objectstore now derives one domain-separated
  `UploadCapabilityCommitment` from the exact bounded canonical capability
  document on both the encode-only issuer projection and the decode-only
  receiver. Higher protocols can sign the non-secret commitment beside an
  opaque bearer without acquiring a URL accessor or a second capability wire
  grammar. Hostile proof covers the real enclosing JSON shape, an independent
  digest oracle, URL, expiry, signed-header, and provider substitution,
  malformed commitment input, zero values, and non-mutating rejection. The
  ratchet failed when canonical capability bytes were removed from the digest.
  Objectstore tests, race/shuffle, Vet, Staticcheck, Errcheck, Nilaway,
  Witness-lint, Gosec, field alignment, and changed-file production complexity
  pass. The repository-wide canonical gate passed through Govulncheck; its
  PTY-bound Deadcode process became idle, while the same exact command passed
  immediately without a PTY. A clean non-PTY gate rerun was invalidated by
  concurrent uncommitted Release work introducing a new `release -> garble`
  architecture edge; no Objectstore failure was observed and the Release files
  were preserved outside this slice.
- `2026-08-04`: Witness migration proved that repository release admission was
  independently reimplemented by Witness, Bug, and Peachfuzz. Release now owns
  one product-neutral `VerifyRepository` boundary over the real Git executable
  and Primitive Process: it binds canonical HEAD to the requested build commit,
  rejects tracked, staged, deleted, and untracked worktree state, refuses
  ambient or `GIT_`-controlled environments, and returns proof-carrying clean
  repository facts. Refusing `GIT_` variables is not by itself sufficient: the
  configuration those variables would steer is also reachable through `HOME`,
  so every Git invocation now carries its own policy on the command line and
  neutralizes `core.excludesFile`, `core.attributesFile`, `core.fsmonitor`, and
  optional index locks. Without it a machine-wide ignore rule reports a dirty
  worktree as clean, which the hostile suite proves as a red state.
  Cleanliness is detected by a first-byte refusing writer,
  so arbitrarily large status output is neither retained nor world-built.
  Typed mismatch and dirty errors preserve `ErrReleaseContract`, while native
  process failures remain reachable. Real-repository hostile tests, including
  status beyond the process capture ceiling, targeted race, Vet, Staticcheck,
  Witness-lint, production complexity, and diff checks pass. Full canonical
  gate, consumer migrations, user review, commit, and publication remain
  pending.
- `2026-08-05`: Registration migration proved that the control-wire scalars were
  independently reimplemented by OGS and Peachfuzz. The revision, the request
  nonce, and the one-time registration token are the three values the server and
  the customer binary must agree on byte for byte before either can say anything
  else, and each end held a private copy. Nothing broke the build when one
  drifted; the drift surfaced as a refused request in the field. Controlwire now
  owns all three plus the one-way verifier a control plane persists. Core
  rejected them first, correctly: its two-named-Primitive-consumers rule found
  zero, because both consumers are outside the module. Placement is a new
  order-5 package over Core, Keygen, and Exchange. Revision refuses an
  unrecognised token rather than assuming forward compatibility; RequestNonce
  and the verifier both refuse their impossible all-zero value, the verifier
  because a blank or truncated persisted record decodes to exactly that and two
  such records compared Equal before the floor was added; RegistrationToken
  keeps its secret in Core SecretMaterial, redacts every formatting verb fmt
  will route, and decides canonicality on encoded bytes so no unwipeable string
  copy of an unspent secret is ever built. Core owns the hexadecimal grammar for
  both public scalars; a second decoder is how two owners come to disagree about
  the same bytes. Hostile tables, layer triads, a data-flow inventory, and four
  fuzz targets with real oracles pass, along with targeted race, shuffle, Vet,
  gofmt, and fieldalignment. Consumer migration in OGS and Peachfuzz, which
  delete their copies and vendor the golden fixtures that prove no wire byte
  moved, remains pending.
- `2026-08-05`: The control-plane documents joined the scalars. The wire test is
  that both ends of the exchange run this same stack, so anything crossing the
  wire has exactly one owner and that owner is Primitive; a type the authority
  publishes and each product mirrors is duplication that goldens can only
  observe, never prevent. The registration request, the installation
  certificate, the signed registration payload and document, the response header
  with its binding to one exact request, the commercial product status, the
  per-installation usage watermark, and the closed signing-domain set were all
  held privately by OGS and re-implemented in Peachfuzz. They could not join
  Controlwire: the response header alone needs Temporal, Receipt, and Lease,
  which would put Controlwire at seven direct imports against a ceiling of six.
  Placement is a new order-6 `controlplane` package over Core, Controlwire,
  Attest, Lease, Temporal, and Receipt, exactly at the ceiling. Controlwire
  keeps the scalars and stays at three. Controlwire also gained PolicyCursor,
  whose Crockford base32 revision identifier refuses lowercase and the I, L, and
  O aliases: an installation echoes the identifier back on every later exchange,
  so a leniently parsed value would re-encode to different bytes than arrived
  and break the echo rather than tolerate a variant. Product status admits a
  grant only under active and payment-retry, payment retry included because a
  failed charge inside grace is still a paying customer. RegistrationRequest
  splits its machine layout from the protocol's field order and converts with
  unkeyed literals, so drift on either side stops compiling; the residual
  fieldalignment finding sits on the wire struct, whose order the authority owns
  and which must not be optimised. Both real OGS fixtures round-trip byte exact,
  including the 2727-byte signed response with its nested certificate, lease,
  watermark, header, and two attestation envelopes. Full Primitive suite,
  gocyclo, and Vet pass. The receipt Watermark overlap is recorded rather than
  resolved: it is the same mechanic scoped to an account and offering instead of
  an installation, and unifying them would redesign durable bytes. Consumer
  migration in OGS, Peachfuzz, Bug, and Witness, plus package hostile tables and
  fuzz targets for the new documents, remain pending.
- `2026-08-06`: The check-in became one document. It was the only shape in
  `controlplane` that named a product: `PeachfuzzCheckInPayload`,
  `PeachfuzzUsageWindow`, and the closed command and outcome enums, with a
  validator comparing the offering against a constant, so an authority had to
  know which tool was asking before it could read the request. Registration was
  already the passport shape; the check-in now is too, and the offering is read
  off the build identity the payload already carried. The coupling was
  concentrated in the window, which carried ten fields of one product's
  vocabulary. An authority cannot validate "candidates at most sightings"
  without learning what a candidate is, so counts now travel over opaque class
  ordinals it validates for range, ordering, and arithmetic and never
  interprets. That also replaced a weaker rule with a stronger one: outcomes
  used to have to account for a separate slice count while the command counts
  were tied to nothing, and now the two totals must agree, which is what caught
  that counting CLI invocations as work units was never implementable, since a
  report produces no outcome to balance against. Two defects surfaced. The
  list-length guard could never fire, because classes are closed and must
  strictly ascend, so entry thirty three cannot hold an admissible class; it was
  deleted and the real bound is now discovered by a test rather than asserted in
  a comment. The whole surface had no test at all, and both check-in goldens
  were committed and read by nothing. There is now a hostile table over every
  window rule, exhaustive proofs of both class constructors, a decoder fuzz
  target, tampering tables over the facts each signature binds, the authority
  half of the exchange end to end, the exhaustive status and outcome matrix, and
  a proof that Bug, Witness, and Peachfuzz travel through identical types.
  Statement coverage moved from 70.7 to 79.5 percent; what remains uncovered is
  the interface-sealing marker, an error message, and a generic witness.
  Peachfuzz pulled `v2026.0.31` in and now owns the projection from its own
  closed vocabulary into the wire ordinals. One bound fact is deliberately
  untested: Controlwire publishes exactly one revision, so the revision arm of
  the response binding cannot disagree until a second one exists. Every new rule
  was checked by reverting it and watching the proof go red. OGS still imports
  this package in zero files, which is the alarm the both-ends test exists to
  raise, and the window builder and payload assembly remain pending before a
  check-in can be produced end to end.
- `2026-08-07`: A day of Witness-migration doors landed without their ledger
  records; this entry closes the gap. Registration now holds its signed
  document to the same `ValidateOutcome` rule the check-in response obeys, with
  hostile tables over the outcome and status cross product. Hostfacts observes
  terminal attachment and column geometry through one `TIOCGWINSZ` leaf on
  POSIX and `GetConsoleScreenBufferInfo` on Windows, proved against real
  pseudo terminals on Darwin and Linux. Filestore's `Inspect` reports allocated
  storage from the `Stat_t` blocks the standard library already returned, with
  a sparse-file native proof. Process owns child containment, the running
  `Execution` handle, liveness, and the working directory, and a fully zero
  `Containment` fills to the direct-kill default. Keygen owns `RandomUint64`
  and the bounded public `RandomToken` at the entropy boundary. Core gained
  `SHA256Of` for whole buffers and `DigestWriter.Digest` and `Reset` for
  mid-stream peeks and pooled reuse.
- `2026-08-07`: A whole-repository review against `_docs/primitive_policy.md`
  swept the week's doors and the standing packages; five independent reviewers
  reported and every fix below was landed red first with focused, race, vet,
  formatting, field-alignment, and cross-compilation proof on the touched
  packages. The user reviewed the reported slices and authorized this commit
  and push with a version bump.
  - Degenerate receivers no longer crash: `core.DigestWriter`,
    `process.Execution`, and `shutdown.Controller` all refused only the nil
    pointer while `new(T)` held nil internals and panicked in `Write`, `Digest`,
    `Reset`, `Deliver`, `Wait`, and `Close`; every door now refuses an
    unconstructed receiver with its package contract identity, and the one
    constructor is named in each refusal.
  - The process platform leaves read the three-state `Isolation` enum as a
    boolean, so an unknown isolation silently rode the direct arm;
    `applyContainment` and `deliverSignal` now switch explicitly and refuse
    outside the admitted domain, the same fail-closed discriminator contract
    the 2026-08-02 sweep installed elsewhere.
  - Windows accepted `IsolationGroup` while `deliverSignal` could only ever
    kill the direct child, a containment nobody could enforce; the request is
    now refused before the child exists, and `CAPABILITIES.md` plus `doc.go`
    scope whole-tree cancellation to POSIX hosts.
  - The zero-`Containment` default had three independent derivation sites and
    public prose that still promised the package owned no hidden default; the
    default now resolves once at `beginValidated` ingress and the `Containment`
    and package documentation state the deliberate zero contract.
  - A live pseudo terminal reporting zero width was recorded as "not a
    terminal", a detachment nobody observed; the attachment domain gained
    `TerminalAttachmentTerminalWithoutGeometry`, both platform leaves return
    it, `Columns` still refuses, and the pty proof pins the new member plus
    the 65535 ceiling on a real terminal.
  - Filestore's Unix allocation leaf converted garbage `st_blocks` (negative
    or unrepresentable) into the vacuously satisfied "unreported" answer on
    the exact reserve question the door decides; it now refuses through
    `sourceError` like the sibling size fact, and the `Inspect` assembly
    propagates the refusal.
  - The filestore Windows test binary no longer compiled (`syscall.Mkfifo` in
    an untagged test file), breaking the canonical gate's cross-compile
    ratchet; the FIFO helper moved to a unix leaf with an `errors.ErrUnsupported`
    twin, and Windows and Linux test binaries cross-compile again.
  - `process/execution_native_test.go` encoded unix signal facts with no build
    tag and was red on Windows; it is now unix-tagged, and the Windows
    counterpart proofs are named below as an open surface.
  - Controlplane's `AdmitsOutcome` survived c8a7e98 as a second, weaker,
    exported authority for the outcome question with zero production callers;
    it is deleted, `ValidateOutcome` is the one rule, and the two incidental
    test uses moved to the surviving contracts.
  - `CAPABILITIES.md` rows were trued: hostfacts names all nine doors, the
    two nonce rows no longer contradict each other on zero semantics,
    `RandomUint64` is scoped to full-width draws, and the group-isolation row
    carries the Windows refusal.
  Three findings were then ruled on and executed the same day.
  `ProductStatus.ValidateOutcome` is offering blind: the read-only refusal is
  admitted for every product alike, `offeringAdmitsReadOnly` and the offering
  parameter are deleted, and what read-only means to a product moves to the
  issuer and the product during OGS consumer surgery. The usage watermark
  gained its third domain constant, so genesis, window, and chain digests can
  never be presented as one another, and `AdvanceUsageWatermark` now takes the
  typed `UsageWindow` and derives the one canonical form itself instead of
  accepting raw bytes. Exchange gained the `keygen` production edge in the
  catalog, the policy table, and the README projection together;
  `randomJitter` draws through `keygen.RandomUint64` and an entropy failure
  keeps keygen's identity instead of masquerading as transport.
  The sweep continued past the rulings: `core.DecodeCanonicalHex` is now the
  one exported hex admission rule, with the native `hex` cause preserved and a
  hostile spelling table, and the seven restatements in attest, timeproof
  (nonce and serial), cloudidentity, lease, receipt, and core's build commit
  all delegate to it. Four of the nine hand-rolled sha256 sites moved to the
  core doors (`SHA256Of` for lease device identity and release artifact
  integrity, `DigestWriter` as the copy destination for release build-tool and
  metadata-asset streaming). Execution supervision now refuses `Deliver` and
  `Terminate` once `Wait` has returned, proved against a real reaped child, so
  a stored number can never address a recycled process or group. The begin
  path that refuses an invalid child identity now reaps the started child
  instead of orphaning it. The six new process types joined the witness block
  with `Validatable` and `OffWireEnum` declarations, and the unix liveness
  leaf states why the x/sys null-signal probe is taken over `os.FindProcess`.
  A second pass the same evening closed the rest of the reviewers' surfaces.
  `filestore.Inspect` now answers `<file>/deeper/child` as unreachable through
  an `errnoSaysNotADirectory` read in the one syscall-naming leaf, with the
  two-below-file boundary row proved red first. The sha256 unification is
  complete: the three domain-framed release identity digests collapsed into
  one `framedDigest` over `core.SHA256Of` with identical bytes (the signed
  golden vectors are the proof), objectstore's dual-digest stream peeks
  through `DigestWriter.Digest` and its upload-capability commitment frames
  through `SHA256Of`, and release's artifact inspection streams into a
  `DigestWriter`; timeproof's five remaining `sha256.Sum256` calls are raw
  protocol-byte comparisons inside its CMS verifier leaf and stay direct
  substrate use by design. `RandomToken` returns a sealed `Token` whose zero
  value refuses, with the surface and inventory ratchets extended. The usage
  window's interval now marshals through a controlplane-owned bounds wire
  projection with snake_case members, the check-in golden was regenerated from
  the producer, and the fuzz seeds moved with it. `UsageDisposition` has an
  exhaustive byte-domain walk including the watermark answer per member, all
  twelve document ceilings refuse an oversized input in one table, and the
  check-in one-shape proof derives its offerings by walking the closed byte
  domain instead of pinning three names. Hostfacts and process gained
  x/sys-confinement ratchets that name their exact platform leaf files and go
  red when a leaf rots vacuous. The group-reap proof now watches the announced
  descendant die through `process.Alive`, the reaped-child liveness proof
  polls to gone instead of accepting either answer, a lingering child under a
  fully zero containment is proved killed on cancellation, `observedAllocation`
  has a direct boundary table over a real `Stat_t` carrier including both
  refusal identities and the exact x512 constant, the dense-file native proof
  writes an incompressible payload, the token table covers the maximum
  representable count, and the darwin pty helper verifies the opened slave's
  minor against the master per run. `errorIdentityParents` split by family
  before its next member could cross the complexity line. Deploy's happy-path
  publication now crosses a real TLS client and server exchange through
  redirected dialing, with the fabricated transport retained only for failure
  and zero-request shaping.
  The Windows counterpart proofs now exist as windows-tagged native tests in
  process and hostfacts covering exactly those facts: containment Windows
  cannot deliver is refused before the child exists, a killed child reports an
  exit and no signal with its identity polling to gone, and pipes, files, and
  closed handles classify honestly. The Darwin host cross-compiles both test
  binaries; executing them on a real Windows kernel is the one remaining
  proof, and it needs a Windows runner.
  The full battery then ran clean end to end: all fifty-one benchmarks with
  allocation flat across input extents (attest 64KiB to 1MiB at nineteen
  allocations, process and objectstore streams flat to 10MiB, hostfacts
  parsers byte-identical from 1KiB to 1MiB, exchange allocating by content
  rather than by limit, three zero-allocation ratchets holding), then go fix,
  vet, fieldalignment, production gocyclo, goconst, nilaway, errcheck,
  staticcheck, both deadcode passes, govulncheck, gosec, witness-lint, the
  ordinary suite, and the doubled shuffled race suite, every one at zero
  findings. Closing that battery surfaced and fixed its own tail: the
  isolation refusal diagnostic became one shared constant, nilaway-visible
  inline guards replaced the helper-hidden ones with a real winsize nil guard
  gained in the unix terminal leaf, the opaque usage classes own canonical
  decimal JSON and diagnostics, RouteFamily and TerminalColumns declared
  off-wire, route and patience domains gained the walks that reach every
  door deadcode found unreached, two err shadows were renamed, two G115
  conversions carry their bounds justification, and the token redaction proof
  searches the rendering rather than the error contract.
