# Primitive 2026 Ledger

Last updated: `2026-07-29`

## Current

- Repository transition: Primitive was copied from the complete Foundation
  worktree at `9ae5b28010b90a140cfaac0ee567034fd84a69b0` plus its local
  2026-07-28 substrate-pattern plan note. Its canonical module identity is now
  `github.com/deliri/primitive/v2026`. Historical evidence remains unchanged
  and therefore retains its original Foundation module paths. `PLAN.md` is
  local-only and excluded from the public repository.
- Phase: the Exchange implementation slice is accepted. It cleanly reopened
  Core only for typed HTTP endpoint, content-coding, field-value, method-domain,
  media-type, header, strict-JSON projection, and checked-I/O contracts.
  `exchange`, `temporal`, and `filestore` are `DONE`; `testserial` remains in
  its required `CONSUMER_SURGERY`.
- Review gate: no external reviewing agent was initialized. The user's two
  Exchange reviews were independently checked against real `net/http`, TCP,
  filesystem, context, and stream paths; every reproduced defect was corrected
  and incorrect findings were rejected with direct evidence. The user
  explicitly authorized commit and push. No tag or release is authorized.
- Production packages: 9 of 19 accepted.
- Test-support packages: 0 of 1 complete and 1 in consumer surgery.
- Active-package consumer surgery: Witness must adopt the exact typed
  `testserial.Declare` contract before Testserial can become `DONE`.
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
  Export-to-consumer ownership remains honestly deferred until named consumers
  land and become observable.
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
- Contextstate backup reconciliation: the published package was compared
  directly with
  `/Users/d/code/foundation_back_up_july_27th_2026/contextstate` at archive
  commit `d046f7b675fcb797398d7cdc87b5504f43978056`, including its production
  files, specification, hostile tests, four relevant commits, archived
  Core error-graph primitive, and real Primitive call sites. The upgrade
  restores the archive's exact production import/effect ratchet, its active
  future-deadline boundary, typed-nil terminal-error rejection, hostile custom
  cancellation normalization, and the design rule that `Context.Err` is the
  sole terminal truth and is called directly rather than behind a potentially
  leaking defensive goroutine.
- Contextstate is stronger than the archived implementation: archived public
  `Classify` discarded its internal observability boolean and could report an
  unsafe graph as `StateNone`; the 2026 API returns `(State, error)` and
  preserves `ErrContextObservation`. Archived traversal lived in Core, used
  reflection, and its package specification contradicted its own cycle proof;
  the 2026 traversal is reflection-free, package-local until a second consumer
  exists, and has exact 128-node depth and width ratchets. The new
  `ObserveAfterDone` closes the repeated Temporal/Exchange post-event rule that
  the archive lacked. State proof now exhausts all 256 underlying values, and
  the synthetic-plus-live no-clone audit supersedes the archive's directory
  absence check. The restored import ratchet now holds the smaller production
  surface to exactly `context` and `core`; the archive also needed JSON,
  formatting, and parsing imports.
- Contextstate archive exclusions remain deliberate: State JSON, `Parse`, and
  their fuzzer were not brought forward because no named consumer persists the
  state. Restoring them would add a second wire protocol rather than make
  `context.Context` easier to use. Doctrine plumbing, package-local error
  labels, and the reflection-based shared traversal likewise remain rejected.
  This is a clean reconciliation, not an archive transplant.
- Contextstate proof: the external State test exhausts all 256 values of the
  enum's underlying domain. The 28-row public `Classify` matrix proves standard
  identities, both precedence orders, wrapping, joining, non-comparable errors,
  panicking methods, hostile ordering, and bounded cycles. A separate internal
  seven-row mechanism ratchet holds depth and width at one below, exactly at,
  and one above the private 128-node maximum without claiming public proof from
  private access.
- Context boundary proof: the 23-row `Validate` matrix and 18-row
  `ObserveAfterDone` matrix use real standard contexts for normal behavior and
  synthetic implementations only for malformed interface ingress. They prove
  exact standard-sentinel normalization, hostile custom-match non-escape,
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
  parsers, aliases, and extra exports. One shared AST matcher is proved against
  synthetic fixtures and then scans every landed catalog package for retired
  `contextcheck` imports, copied direct terminal classification, repeated
  context ingress logic, and pure compatibility aliases or forwarders. Direct
  classification now covers `errors.Is` against a terminal sentinel, direct
  `==` or `!=` identity comparison in either operand order, and terminal
  sentinels named in switch case clauses, including dot-imported forms.
  Identity comparisons and switch cases were previously invisible, and because
  they need no `errors` import the audit also returned early before inspecting a
  single node. Today
  Core does not import `context` or `contextstate`, and Contextstate is the
  exempt owner, so the live scan has no possible semantic violation; it is a
  forward ratchet for later consumers. Its independently derived coverage
  assertion is presently load-bearing and proves that every on-disk catalog
  directory and at least one production file were actually scanned. A restored
  package-local import ratchet separately proves that Contextstate production
  imports exactly the standard `context` substrate and Core, so clock,
  reflection, I/O, or sibling-package dependencies cannot enter silently.
- Contextstate focused reconciliation evidence: `go test -count=1`,
  `go test -race -count=1`, vet, staticcheck, witness-lint, errcheck, nilaway,
  fieldalignment, goconst, gosec, and production-plus-test `gocyclo -over 10`
  are clean for `./contextstate`. The fresh whole-repository gate also passed
  every phase in `evidence/contextstate-sixth-pass/`; the evidence remains
  local `darwin/arm64` proof and does not close N27.
- Contextstate sixth-pass closure: the set-based production import ratchet, the
  package-local `Deadline`/`Done`/`Value`/`Cause` AST ratchet, and direct
  identity classification through `errors.Is`, equality, inequality, reversed
  operands, dot imports, and switch case clauses are all included in the
  sixth-pass bundle. The rejected `Value(any) any` runtime seam was removed
  rather than waived after witness-lint correctly identified its interface
  return. Two switch-case rows were observed red before the matcher arm landed,
  while the non-terminal `context.TODO` switch row remained green.
- Contextstate fuzz, benchmark, native, and live proof are not applicable.
  State's complete byte-sized domain is exhaustively tested; `error` and
  `context.Context` interface graphs have no honest byte-mutation oracle; the
  package has no I/O, platform branch, clock, goroutine, or effect leaf; and
  traversal is held by the exact 128-node mechanism ratchet. The canonical gate
  still reruns Core's benchmarks and two 100,000-execution fuzz targets.
- Waiver state: `witness-lint` passes with zero waivers. The inherited Core
  string-search waiver was removed without dropping N22's second-tier
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
- Currency review state: the package is a typed exact-minor-unit layer over
  integer arithmetic, `strconv`, `encoding/json`, and `math/big` test oracles.
  Its closed twelve-code domain owns fraction digits, exact decimal projection,
  checked same-code arithmetic, ordering, and strict typed JSON. A
  standard-library differential fuzz oracle exposed that `encoding/json`
  otherwise admitted case-folded field names; the wire owner now decodes exact
  compiler-owned field names through `json.Decoder`. The promoted hostile case
  remains in the deterministic matrix.
- Garble review state: the package is a typed layer over standard Base64,
  HKDF-SHA256, and the pinned upstream Garble executable. It owns one exact
  eight-byte Garble seed, Core-owned 64-byte custody, deterministic derivation,
  and a streaming typed build-argument intent. It does not own command
  execution, generic CLI parsing, persistence, environment policy, or a
  replacement Garble protocol. The complete preserved implementation was mined
  for upstream mechanics and rejected where it accepted lossy seed forms,
  exposed loose string arguments, or invented wider ownership.
- Keygen review state: the package is only friendly typed convenience over
  `crypto/rand.Read`, `ed25519.GenerateKey`, and `ed25519.NewKeyFromSeed`.
  Generic secret sizes exhaust the complete Core-owned 16-through-64-byte
  interval in deterministic tests; Ed25519 keys are copied into owned
  redaction-safe values, validated against the standard-library relation,
  projected through caller-owned copies, and destructible through Core secret
  ownership. Entropy providers, algorithms, derivation, signing policy,
  persistence, formats, KMS/HSM behavior, and command surfaces remain excluded.
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
  overflow guard checks sign before bounded arithmetic. Compile-time witnesses
  now pin every Temporal function and method signature and all public request
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
  pass. Fresh Aggregate and signed Temporal fuzz runs each crossed 10,000
  executions. Temporal statement coverage is 86.3% and is used only as a
  zero-function ratchet; `Precision.OffWireEnum` is intentionally an empty
  compiler marker with no executable statement. Direct Witness analysis of
  Temporal is clean. The canonical gate passes through nilaway and then stops
  only at the already classified stale Witness off-wire-enum JSON/String
  doctrine and retired Testserial convention.
- Exchange review state: the package is a typed policy layer over the caller's
  real `*http.Client`, `net/http`, `io.Reader`, `io.Writer`, Go runtime, and OS
  network stack. It owns bounded aggregate JSON/byte operations, exact bounded
  streaming, replay eligibility, finite retry and server hints, total and
  per-attempt budgets, rejected or same-origin redirects, typed response
  metadata, and typed/native error reachability. It owns no DNS, TLS, proxy,
  connection, pool, framing, socket, queue, worker, transport, copy engine,
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
- Fuzz scope for this slice is limited to externally controlled text or bytes.
  Fresh 100,000-execution differential runs pass for Currency decimal text,
  Currency Amount JSON, and Garble Seed JSON. Keygen has no external parser and
  exhausts its complete 49-value valid size interval plus hostile effect-result
  matrices. Testserial has a finite typed enum and AST domain. Neither package
  receives a ceremonial fuzz target.
- Focused proof state: fresh repository tests, race tests with shuffle and two
  repetitions, vet, staticcheck, errcheck, nilaway, production
  `gocyclo <= 10`, goconst, fieldalignment, gosec, govulncheck, actionlint, and
  the three package benchmark phases pass. The canonical evidence-writing gate
  has not been run because the pinned Witness revision predictably rejects the
  clean-break Testserial contract and over-applies JSON doctrine to off-wire
  enums.
- Witness publication dependency: the currently pinned analyzer recognizes
  only the retired `testserial.Serial` convention and treats every enum as a
  wire protocol. Primitive will not add a compatibility call, JSON methods,
  or waivers to satisfy those stale assumptions. After explicit review and
  publication of this Primitive slice, Witness must pin the new Core
  contracts, require the exact first-statement `Declare` shape and reject
  contradictory `t.Parallel`, narrow JSON doctrine to actual wire enums,
  publish its revision, and then be repinned here before the canonical gate can
  close Testserial.
- Gate portability correction: `file_sha256` now selects `sha256sum` or
  `shasum`, extracts the digest without positional-parameter mutation, and
  validates the exact 64-hex-character shape. This addresses the observed
  Windows runner failure without changing evidence semantics.

Next:

1. Publish the accepted Exchange revision without creating a tag or release.
2. Reassess the next package or consumer surgery only after Exchange
   publication; do not broaden this slice during publication.
3. Reassess Timeproof now that Exchange supplies its required typed evidence.

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
| `hostresource`     | `BLOCKED_BY_DEPENDENCY` |
| `temporal`         | `DONE`                  |
| `exchange`         | `DONE`                  |
| `fuzz`             | `BLOCKED_BY_DEPENDENCY` |
| `lease`            | `BLOCKED_BY_DEPENDENCY` |
| `process`          | `BLOCKED_BY_DEPENDENCY` |
| `release`          | `BLOCKED_BY_DEPENDENCY` |
| `shutdown`         | `BLOCKED_BY_DEPENDENCY` |
| `objectstore`      | `BLOCKED_BY_DEPENDENCY` |
| `timeproof`        | `BLOCKED_BY_DEPENDENCY` |
| `workloadidentity` | `BLOCKED_BY_DEPENDENCY` |
| `upgrade`          | `BLOCKED_BY_DEPENDENCY` |

Consumer state is recorded under the active package only. There is no separate
all-consumer phase.

`contextstate` is `DONE` after accepted reconciliation against the preserved
backup. Bounded external-error traversal is package-local, the archive's
dropped observability boolean is replaced with `(State, error)`, and State
JSON, token parsing, and a standalone nil-check API remain rejected as unproved
ceremony. `Validate` is the usable-now admission gate; `ObserveAfterDone`
carries the post-event precondition in its name; and `String` remains
diagnostic-only. The no-wire decision is proved by asserting that neither the
value nor the pointer receiver implements any standard marshaling interface.
`OffWireEnum` is a shared compiler-owned positive declaration; the
interface-absence test, not the marker, is the negative proof. The
synthetic-plus-live no-clone AST ratchet is active, and the live scan now
asserts its own coverage against the catalog directories present on disk, so a
drifted path derivation fails instead of silently auditing nothing.

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
