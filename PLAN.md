# Primitive 2026 Build Plan

## GOAL

Unix-like, single-purpose packages.

Primitive makes Go's real primitives typed, validated, bounded, composable,
and pleasant to use.

SIMPLE. AS. FUCK.

Simple scales. Complex collapses.

Live progress belongs only in `LEDGER.md`.

## 1. Implementation law

Every package follows one path:

```text
typed request or external input
    -> pure typed Decode / New / Validate / Prepare
    -> one owner-only projection
    -> exact Go standard-library, documented protocol, or official-SDK primitive
    -> O(1) streaming execution
    -> validated typed result
```

Primitive improves the substrate; it does not replace it.

- Public operations are structure-to-structure.
- Important facts are structs, nominal types, closed `iota` enums, typed
  errors, or package constants.
- Standard interfaces such as `context.Context`, `io.Reader`, `io.Writer`,
  `fs.FS`, and `http.RoundTripper` remain the data plane.
- Validation belongs to the owning type and runs at ingress, package crossing,
  persistence, execution, recovery, and external output.
- Stable error identities live in Core. Context may be wrapped, while
  `errors.Is` and `errors.As` remain valid.
- Shared facts live in focused Core files only when at least two named
  Primitive packages require them. One-package facts stay in that package.
- Every effect has one small inventoried leaf using the standard library,
  documented protocol, or official SDK.
- Exactly one layer owns retry, backoff, jitter, timeout, redirects, buffering,
  and commitment.
- Variable-sized data uses readers, writers, iterators, hashers, encoders,
  decoders, and bounded buffers. Production memory is O(1) in input extent
  unless a fixed bound is proved.
- Every goroutine, handle, retry, cleanup, and detached operation has a finite
  owner and bound.
- Native and provider errors remain reachable.
- Production functions satisfy `gocyclo <= 10`.

Forbidden:

- loose maps, string blobs, bool protocols, copied literals, magic values,
  hidden conventions, or informal cross-package contracts;
- stubs, fakes, mocks, simulators, surrogate packages, temporary type-check
  trees, or Primitive-shaped test doubles; all design, tests, and
  implementation run in the real package against the real substrate;
- lookalike readers, writers, contexts, filesystems, clocks, transports,
  encoders, processes, or provider clients;
- raw syscalls, raw sockets, `unsafe`, hidden effect paths, copied provider
  behavior, or duplicate retry layers; and
- compatibility wrappers, shims, aliases, dual formats, or dead paths.

Reject a package or feature when using Primitive is not clearer than
repeatedly writing the equivalent direct substrate code, when direct substrate
use preserves all of Primitive's value, or when Primitive would duplicate
substrate behavior.

Tests pressure extreme valid and invalid inputs on both sides of each expected
boundary. They do not invent a security problem.

## 2. Core first

Start Core deliberately small. Inspect every admitted interview, but admit only
contracts already justified by current evidence:

- package identities and the typed dependency catalog;
- a non-error fact required by at least two named Primitive packages; and
- a stable typed error identity with a named producer and caller decision.

Use focused files such as `error_identity.go`, `path_contracts.go`,
`http_constants.go`, and similarly narrow owners. Core imports only the Go
standard library and no Primitive package.

Core is not a junk drawer. A non-error fact enters Core only when at least two
named Primitive packages need it. Package-private facts stay with their
package. Every stable error identity names a real producer and a caller
decision. No orphan or speculative contracts.

Core's milestone includes hostile boundary tests, fuzzable validation, the
typed architecture catalog, landed-package structural enforcement, the
canonical gate, and explicit user review. Sentinel begins with `attest`, the
first non-Core package that gives it a real compiler-visible consumer edge to
inspect; it does not block Core with a check the current graph cannot decide.
Then Core is committed and pushed as the base for every later package.

Core is expected to reopen. When a later package proves a missing shared
contract, stop, add it to Core with proof, review and publish Core, then resume.
Never guess the future contract and never copy it locally. Starting too small
is cheap to correct; starting too large couples every dependent to an
unjustified abstraction.

## 3. Exact graph

The catalog is 21 production packages plus test-only `testserial`.
Every listed import is required and must be used semantically. Every unlisted
Primitive sibling import is forbidden.

A package may start only when its complete dependency frontier is published.
The order column is dependency depth, not a mandate to build an entire row.

| Order | Package | Single owned purpose | Required direct imports | Required test-only imports |
| ---: | --- | --- | --- | --- |
| 1 | `core` | Shared nominal values, errors, paths, protocol facts, numeric and encoding contracts | none | none |
| 2 | `attest` | Canonical Ed25519 envelopes and proof-carrying verification | `core` | none |
| 2 | `contextstate` | Nil-safe context ingress and terminal observation | `core` | none |
| 2 | `currency` | Exact minor-unit values, arithmetic, ordering, and decimal projection | `core` | none |
| 2 | `garble` | Tool identity, seed custody and derivation, and typed build intent | `core` | none |
| 2 | `keygen` | Exact secret and Ed25519 key generation | `core` | none |
| 2 | `testserial` | Test-only isolation declaration and analyzer contract | `core` | none |
| 3 | `filestore` | Rooted OS handles, confinement, durability, activation, append rotation, and recovery | `core`, `contextstate` | none |
| 3 | `hostfacts` | Host disk, memory, cgroup, tree, and OOM observations | `core`, `contextstate` | none |
| 3 | `temporal` | Time, duration, arithmetic, persistence, waits, and tickers | `core`, `contextstate` | none |
| 4 | `exchange` | Policy over `net/http`: replay, retry, backoff, jitter, redirects, budgets, and bounded bodies | `core`, `contextstate`, `temporal` | none |
| 4 | `fuzzfinder` | Bounded classification and observation of Go-generated fuzz artifacts | `core`, `filestore` | none |
| 4 | `lease` | Signed lease timeline, assessment, renewal, and monotonic advance | `core`, `temporal`, `attest` | none |
| 4 | `process` | Argv, environment, containment, bounded output, exit, and reaping over `os/exec` | `core`, `contextstate`, `temporal` | `testserial` |
| 4 | `release` | Embedded build identity, immutable artifacts, manifests, Latest, verification, and pure selection | `core`, `temporal`, `attest` | none |
| 4 | `shutdown` | Signal observation and phased bounded cleanup | `core`, `contextstate`, `temporal` | none |
| 5 | `gate` | Pure CLI-side new-work authorization over one authentic Lease assessment | `core`, `lease` | `attest`, `temporal` |
| 5 | `receipt` | Authenticated accepted-evidence facts and fixed-size monotonic watermarks | `core`, `attest`, `temporal` | none |
| 5 | `objectstore` | One bounded vendor-specified S3, GCS, or Cloudflare Images transfer with integrity and commitment | `core`, `contextstate`, `temporal`, `exchange` | none |
| 5 | `timeproof` | RFC 3161 request construction, response verification, and replay | `core`, `temporal`, `keygen` | none |
| 5 | `cloudidentity` | Bounded Google Cloud or AWS outbound identity-token acquisition and redacted disclosure | `core`, `temporal`, `exchange` | none |
| 6 | `upgrade` | Crash-recoverable installation, activation, startup truth, rollback, and recovery | `core`, `filestore`, `hostfacts`, `objectstore`, `release`, `temporal` | none |

Core imports no Primitive package. Production never imports test support.
Primitive never imports Kernel, Witness, Bug, or Peachfuzz. Commands and
consumer workflow coordinators are leaves.

Objectstore starts with three closed vendor contracts: Amazon S3 whole-object
PUT/GET, Google Cloud Storage XML API whole-object PUT/GET, and Cloudflare
Images one-time direct upload. It composes Exchange against caller-supplied
expiring HTTPS capabilities without duplicating transport, retry, buffering,
or provider behavior. Each call owns one provider target and one stream;
multi-provider replication is explicit caller composition over independently
reopenable sources, never a hidden fan-out engine. Objectstore does not create
buckets, credentials, signed URLs, Cloudflare draft records, resumable
sessions, or multipart object-upload protocols.

Cloudidentity owns one common opaque, redacted outbound bearer and two explicit
provider entry points: Google Cloud metadata identity-token acquisition and
Amazon Web Services regional STS `GetWebIdentityToken` acquisition. Google
Cloud and AWS retain distinct typed request contracts; there is no runtime
provider selector or generic dispatch path. AWS authorization arrives as an
already-signed exact request capability. Cloudidentity does not discover
credentials, implement Signature Version 4, parse or verify token claims,
cache or refresh tokens, execute provider tools, or authorize a consumer
operation.

### Excluded archive surfaces

| Decision | Surfaces |
| --- | --- |
| Absorbed | `update` into `release` |
| Added from consumer evidence | `process` |
| Deferred | `callbudget`, `cmd/keygen`, `controlstate`, `filestoretest`, `rate`, `redactiontest`, `register`, `status`, `submission`, `unleash` |
| Retired | `cmd/capabilityinventory`, `probe` |

Deferred and retired surfaces get no placeholder, constant, error, adapter, or
compatibility path. Re-entry needs current demand, one product-neutral owner,
its complete dependency frontier, and explicit user approval.

## 4. Vertical delivery strategy

After Core, build useful capabilities from their published dependency
frontier. Consumer repositories remain untouched until the user has accepted
all selected Primitive packages; then Witness, Bug, and Peachfuzz receive the
complete compiler-owned upgrade package by package.

Select the package only when:

- its complete dependency frontier is published;
- its interview proves current consumer value;
- its substrate and ownership boundary are clear; and
- its consumer migration order is understood.

Build, review, commit, and push that Primitive package. Record every named
consumer implementation and exact migration boundary without changing the
consumer yet. Once the Primitive package set is complete, replace superseded
consumer implementations one repository at a time. Delete the old code, pin
the published Primitive revision, perform the typed surgery, run focused and
full tests, review, commit, and push.

When no consumer owns a separable implementation of a low-level primitive,
defer surgery to the first package in that chain that delivers a complete
capability and record the deferral in `LEDGER.md`.

Deferred consumer surgery does not block another Primitive package whose
complete dependency frontier is already published. Package choice, later
consumer order, and deferrals are evidence-driven ledger decisions, not fixed
examples.

## 5. Compiler-enforced graph

Core owns one small typed architecture catalog. The catalog is the executable
owner of package identities and required imports. Core's structural tests
compare it with compiler build information and every landed package's
production imports.

Sentinel implementation begins with `attest`, when a non-Core package first
makes a catalog edge compiler-visible. It additionally checks the table above
and README diagram as projections. The export-to-consumer ownership check
activates only as its named consumers land and become observable in compiler
build information. Before then, interview evidence and explicit user review
govern admission, and `LEDGER.md` records the deferral; Sentinel must not claim
to have decided an unobservable consumer mapping.

Sentinel fails on:

- a missing required or extra production edge;
- a cycle;
- an undeclared test edge or production reachability to test support;
- a consumer or deferred-package import;
- a Core export used by fewer than two named Primitive packages, except a
  stable typed error identity with a named producer and caller decision; and
- drift in the plan or README projection.

The coupling coefficient is the compiler-visible sum of undeclared production
edges, undeclared test edges, and consumer or deferred imports. Every term is
nonnegative. The required coefficient is exactly zero.

Typed witnesses reject ceremonial imports. Sentinel stays small: it checks the
graph and ownership declarations, not prose or review quality.

Semantic duplication and copied behavior are not graph properties. User review
inspects them directly. Sentinel never reports a semantic property it cannot
decide.

## 6. Vertical package loop

For each package:

1. Read its interview, the package row above, and the preserved pre-rebuild
   package in the backup recorded by `LEDGER.md`. Reconcile all three under
   section 1. Mine the backup for proven mechanics so solved work is not
   reinvented, and for known defects and rejected approaches so they are not
   repeated. The backup is evidence, not authority: retain only typed
   convenience layered directly on Go's real standard-library, OS, protocol,
   or official-SDK substrate.
2. Confirm one owner, one substrate, exact imports, typed API, validation
   boundaries, resource bounds, effect leaf, and consumer paths to retire.
3. Prepare the exact typed API design as part of the real implementation:
   `doc.go`, exported structs and enums, Core-owned error identities, and exact
   function and method signatures. Do not create a stub, fake, mock, surrogate
   package, or temporary type-check tree. A separate pre-implementation review
   pause is not required unless the work exposes a new dependency edge,
   ownership conflict, or material scope decision that needs user authority.
4. Work directly in the real package on the main worktree. Add meaningful red
   tests under `_docs/testing_protocol.md`; never satisfy them with a test
   double or placeholder implementation.
5. Implement the smallest direct typed path over the real substrate.
6. Run focused tests, race and fuzz proof, native or live proof when relevant,
   and `bash scripts/gate.sh`.
7. Present the complete typed API, implementation, tests, exact diff, and
   evidence for fresh explicit user review. Semantic review is not replaced by
   Sentinel.
8. Fix every verified blocker and rerun affected proof.
9. Commit and push Primitive only after explicit approval.
10. Record the exact later consumer surgery proved by the interview; do not
   start it until all selected Primitive packages are accepted.
11. After Primitive completion, review, commit, and push each consumer
   independently.
12. Update `LEDGER.md`.

No per-package planning or specification Markdown file is created. Public
contracts, invariants, errors, and witnesses live in typed Go. The interview
preserves research; this plan preserves architecture; the ledger preserves
state.

Core is the expected exception to package closure: it reopens whenever a later
package proves a missing shared contract.

## 7. Done

A package is done only when:

- its request and result path is structure-to-structure;
- important facts and validation are compiler-owned;
- variable input streams with bounded memory;
- its single effect leaf uses the real substrate;
- native errors remain reachable through `errors.Is` and `errors.As`;
- production `gocyclo <= 10`;
- no compatibility path, copied contract, or hidden convention remains;
- required failure, boundary, fuzz, race, native, crash, and live proofs pass;
- the canonical gate and local ratchets pass; and
- independent and user review accept the exact diff.

Stop and correct ownership before continuing if a new edge, cycle, duplicated
substrate behavior, unbounded resource, or untyped contract appears.

## 8. Consumer surgery

Consumer migration begins only after all selected Primitive packages are
accepted. It is then package-by-package, not one giant final cutover.

For each package, inspect its interviews and choose the consumer order that
gives the clearest first surgery and safest dependency progression. Skip a
consumer when its evidence proves it does not own that capability. There is no
global consumer order.

Each surgery pins one published Primitive revision, removes the superseded
path completely, runs focused, full, native, and live proof as applicable, and
lands atomically. No local `replace`, unpublished revision, shim, copied
implementation, or dual authority.

## 9. Proven substrate pattern

Filestore's first consumer-scale proof establishes the model later packages
should preserve:

```text
compiler-owned typed intent
    -> validated bounded stream
    -> Go standard-library concurrency and interfaces
    -> real OS namespace, files, durability, and scheduling
```

The layers reinforce rather than imitate one another. Types make ownership,
bounds, modes, paths, outcomes, and recovery obligations impossible to omit
silently. Streaming makes memory proportional to active operations, not to the
extent of the data being moved. Go supplies `context.Context`, `io.Reader`,
`io.Writer`, goroutines, and real handles. The OS supplies confinement,
namespace arbitration, physical I/O, synchronization, and native failures.
Primitive contributes no alternate filesystem, scheduler, transaction engine,
worker system, or hand-built state machine between them.

The 2026-07-28 Bug consumer proof drove 10,000 concurrent, distinct, create-only
10 MiB writes through the published Filestore path on an Apple M1 Max. It wrote
97.656 GiB, then streamed every file back and matched every SHA-256 digest to
the real source file. The profiled one-shot benchmark completed at 448.78 MB/s
with approximately 405.3 MB allocated across the complete operation. Of the
sampled allocation space, approximately 322.9 MB was the intentional fixed
32 KiB Filestore buffer for each simultaneously active stream. CPU samples were
dominated by OS syscalls and Go runtime waits; Bug contributed no flat hot-path
CPU and introduced no mutex, queue, worker pool, or admission bottleneck.

This proves 10,000 as a consumer floor on the measured machine, not a
hard-coded ceiling or a universal throughput promise. Available descriptors,
threads, memory, storage, filesystem behavior, and hardware remain real host
limits. Primitive must expose those limits honestly and must not replace them
with its own global coordination.

Use this result as an architectural ratchet for Exchange and Objectstore.
Exchange must add typed policy to `net/http` while leaving transport and
scheduling with Go. Objectstore must add typed bounded transfer and integrity
commitment while leaving byte movement, provider semantics, and supported retry
behavior with the official SDK, Exchange, Go, and the remote service. If either
package needs a world model, a duplicate buffer hierarchy, a generic state
machine, or a private scheduler to look correct, its ownership boundary is
wrong and implementation stops for redesign.
