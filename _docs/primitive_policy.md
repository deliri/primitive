# Primitive

<!-- SHIP STATUS: identical in every policy document. Change all of them together, on the day it stops being true. -->

## Status: not shipped

**As of 2026-08-06, nothing here is shipped.** No customer holds an installed
binary, a released artifact, a persisted bundle, or a signed document produced
by this code.

While that is true, every fix is a clean break:

- change the real contract and delete the superseded one in the same commit;
- no compatibility layer, wrapper, alias, adapter, dual format, fallback reader,
  or migration path;
- rename freely, move a type to its correct owner freely, reshape a persisted
  format or a wire document freely;
- a rewrite that is correct beats a patch that is compatible with something
  nobody is running.

This is the cheapest this work will ever be. Section 11 forbids compatibility
debt permanently; today obeying it costs nothing, because there is no installed
population to be compatible with. Deferring a correct redesign now does not save
the cost, it only moves it to the side of the line where it is expensive.

**This section is deliberately revocable, and revoking it is not automatic.**

The day a customer holds a released artifact, whoever shipped it rewrites this
section, dates it, and names exactly what became load-bearing: which formats now
exist on disk in the field, which documents have been signed and are held by
someone else, and which routes an installed client will call. From that day the
rules tighten. A persisted format, a signed document, and a wire contract in the
field can no longer be changed by deleting the old one, and every change must
state what it does to the installed population.

Until that day: clean break, every time.

<!-- END SHIP STATUS -->

## Objectives

Read this section before anything else. It is here, at the top of the law rather
than in a second file, because a second file does not get read.

**Primitive is the product-neutral Go primitive layer used to build reliable command-line tools and services. It makes Go's standard library, operating-system primitives, documented protocols, and official SDKs typed, validated, bounded, streaming, and easier to compose.**

### What this owns

- every mechanism by which a consumer touches the real world: filesystem,
  process, network, clock, locks, entropy, signals, and host facts;
- product-neutral execution of consumer-owned typed policy through bounded
  scans, streaming transforms, incremental commitments, durable writes,
  process and network calls, and other real-world effects;
- every contract that crosses a wire, because both ends run this identical
  stack and a wire type has exactly one home; and
- the obligation to be reachable: a door that exists but a caller cannot reach
  is a defect equal to a missing one.

The real substrate still does the work. Primitive adds types, validation, and
bounds around it.

### What this deliberately does not do

- own product policy. Business rules, thresholds, vocabulary, and workflow
  belong to consumers;
- own a consumer's record schema, event vocabulary, lifecycle, ordering,
  rotation or retention decision, replay interpretation, fold, or result
  meaning;
- decide which product evidence is eligible, choose a transfer destination,
  disclose transfer contents to a customer, obtain confirmation, interpret a
  product-owned `--yes` flag, render CLI tables or progress bars, or write
  customer language. Primitive supplies the validated declarations, progress
  observations, custody chits, payment receipts, and retrieval agreements that
  let the owning product make and present those decisions;
- classify hidden `build` or `release` commands, obtain operator confirmation,
  choose diagnostic arguments or a live-data sandbox, authorize a release,
  render build and publication output, or own a product's direct control-plane
  adapter. Primitive supplies the clean-repository proof, exact build/process
  plans, publication agreements, and upgrade mechanics those commands use;
- replace, imitate, or hide the substrate;
- import Kernel, Witness, Bug, or Peachfuzz, ever; or
- demand a value it will not produce. If Primitive requires it, Primitive
  supplies a way to make one.

### Where this meets the real world

Primitive *is* this layer. It rides the Go standard library and reaches the
operating system only through it, using `golang.org/x/sys` where the standard
library genuinely cannot reach. Section 0.1 has no exceptions.

Every entry above names a Primitive door. A real-world capability with no named
door is either a Primitive gap to close or a reach-past to delete. That is the
layering law of section 0.1 made auditable at the product level.

`CAPABILITIES.md` is the index consumers read before writing anything that
touches the real world. Keeping it accurate is part of this package's job: a
capability nobody can find is a capability somebody rebuilds.

<!-- OBJECTIVES ABOVE. SHARED CONTRACT BELOW: byte-identical in every repository that rides Primitive. Do not edit one copy. -->

**Status:** Normative  
**Audience:** Human maintainers, reviewers, and implementation LLMs  
**Applies to:** Primitive production packages, tests, tooling, architecture enforcement, releases, and later consumer migrations  
**Progress record:** `LEDGER.md` only

## How to use this contract

For any implementation task:

1. apply sections 0 through 13 as universal law;
2. locate the package's exact row in section 15;
3. apply any closed scope in sections 16 and 17;
4. preserve the compiler-enforced graph in section 18;
5. execute the package loop in section 20 without skipping a step;
6. use section 21 as the acceptance checklist;
7. obey every stop condition in section 22; and
8. do not commit until the user explicitly approves the exact reviewed diff.

Do not summarize away a requirement. Every **MUST** and **MUST NOT** remains
active unless this contract explicitly limits it to an applicable package or
boundary.

## 0. Mission and the layering law

Primitive is a set of Unix-like, single-purpose Go packages.

Primitive makes Go's real primitives typed, validated, bounded, composable,
and pleasant to use.

**SIMPLE. AS. FUCK.**

Simple scales. Complex collapses.

Primitive improves the substrate. It does not replace, imitate, or hide the
substrate.

### 0.1 The stack, with zero exceptions

```text
OS
  -> Go standard library
       (golang.org/x/... only where the standard library genuinely cannot
        reach the operation)
    -> Primitive
      -> consumer policy (Witness, Bug, Peachfuzz, Kernel, and its distros)
```

Each layer rides the one below it and never reaches around it. This rule has no
exceptions and takes none.

- **Primitive rides the Go standard library.** It does not reach past the
  standard library to the operating system.
- Where the standard library cannot express an operation at all, the escape is
  the official extended library -- `golang.org/x/sys/unix`,
  `golang.org/x/sys/windows` -- which is still Go's own substrate rather than a
  hand-rolled one. That escape belongs to Primitive and to no consumer.
- Hand-written syscall numbers, assembly thunks, and `unsafe` are forbidden
  outright. "The standard library made this awkward" is not a reason; it is
  precisely why the `x/` packages exist.
- Reading a value the standard library already returned is riding it, not
  bypassing it. A type assertion such as `info.Sys().(*syscall.Stat_t)` is
  admissible for exactly that reason: it inspects what `Lstat` handed back and
  asks the kernel for nothing.

#### Policy enters; execution leaves

```text
consumer-owned typed policy
    -> Primitive-owned typed execution contract
        -> Go standard library
            -> operating system or documented external service
        <- typed execution observation
    <- consumer-owned interpretation
```

The consumer owns what a fact means and which operation it wants. Primitive
owns the product-neutral mechanics that execute the validated request with
explicit bounds. Primitive returns typed observations; it does not reinterpret
them as consumer state, workflow, success, or failure.

Reuse moves blind mechanics downward. It never moves product vocabulary,
record schema, lifecycle, ordering, replay acceptance, folds, or terminal
meaning downward.

### 0.2 The blink model

Primitive is layered the way the internet is layered: each layer answers one
question, hands its payload to the layer below, and does not reimplement what is
beneath it. The layers are fixed. New packages slot into an existing layer;
nothing renumbers. That stability is why the model scales, and it is why each
layer is defined by a **question** rather than by whatever currently occupies it.

| | layer | the question it answers |
| --- | --- | --- |
| **B0** | Substrate | what actually exists: silicon, disk, network, kernel |
| **B1** | Standard library | how does Go name the substrate (`golang.org/x/sys` only where the standard library cannot reach) |
| **B2** | Value | what is a legal value? no effect |
| **B3** | Mechanism | how do I perform exactly one effect, bounded and correct? |
| **B4** | Capability | how do I complete one whole useful operation? |
| **B5** | Agreement | what must both ends agree on, byte for byte? |
| **B6** | Decision | given agreed facts, what am I allowed to do? |
| **B7** | Policy | what does a consumer want, and why? not Primitive |

```text
B2  core currency contextstate testserial
B3  temporal keygen attest filestore filelock process exchange shutdown hostfacts id lineio manual
B4  objectstore gcsobjects cloudidentity secretstore timeproof fuzzfinder release deploy upgrade
B5  controlwire lease receipt controlplane distribution
B6  gate
```

Primitive occupies B2 through B6 and never B7. The table is a snapshot of the
graph in section 15, not a second authority: the catalog owns the edges.

#### The one structural rule

> **Import sideways or down. Never up.**

Not "never sideways". Same-layer composition is how the real stack works, and
`filestore` -> `temporal`, `deploy` -> `objectstore`, and `controlplane` ->
`controlwire` are all healthy. What must never happen is a lower layer reaching
upward.

`exchange` is the transport every layer above rides. A package above B3 that
speaks `net/http` directly has skipped a layer and duplicated the retry,
backoff, redirect confinement, and bounded-body policy `exchange` already owns.

#### Primitive has no peers, and the contract has one shape

Primitive is the floor both ends stand on, not a participant standing beside
one. The installed tool and the control plane are peers; Primitive is underneath
both.

**Neither end may know of the other.** A government issues a passport. The agent
at the border reads it and decides. The agent does not know the traveller's
name, their dog, or where they are going, and does not keep one reader per
country: there is one passport shape, and nationality is a field written on it.

Applied here:

- There is **one** shape of each document. The product is a **field** --
  `core.Offering` -- never a type name, never a function name, never a package.
- The control plane must not be able to tell which tool is asking except by
  reading that field. A per-product request type, or a handler that switches on
  the product before validating the document, is the coupling this rule exists
  to forbid.
- Counters follow the same rule. One bounded set of typed work-unit classes and
  counts over one exact window. The control plane validates bounds and window
  and applies volume and abuse policy; it never learns what a class *means*.
  Meaning stays with the product that owns it.

Because the contract carries no product in its shape, it is product neutral by
construction, and belongs in Primitive with every other product-neutral
mechanic. A contract that needs per-product types is not a contract yet; it is a
coupled design, and moving it to another module relocates the coupling rather
than removing it.

`core.Offering` carrying one consumer-owned canonical identity is therefore
correct. It is the nationality field, but Primitive does not own a catalog of
nationalities and never interprets the value. A Primitive-named product
constant, per-product payload type, derived product identity, or product switch
would reintroduce the coupling this contract forbids.

#### The documents

Travel uses several documents because they carry genuinely different facts, and
each one has exactly one shape:

| travel | here | what it establishes |
| --- | --- | --- |
| passport | installation certificate | who this installation is, bound to a device key it never transmits |
| visa | `lease` | permission for a period: not before, not after, contact after, good until |
| boarding pass | issued capability | one transfer, one scope, expiring, create only |
| gate agent | `gate` | reads the assessed visa and decides whether new work may begin |
| stamp | `receipt` | authenticated proof that the accepted thing happened |

Collapsing these into one document with a mode field would be the naive
simplification: every field becomes optional, every validation becomes
conditional, and the result is the bool protocol section 4.2 forbids. Separate
documents, one shape each.

#### Obliviousness

Each layer carries what it is handed without understanding it.

The internet does not know whether it is moving a video call, a payment, or a
model's instructions. That is not a limitation of the internet; it is the
property that let it scale to carry things its designers never imagined.

The same rule holds at every layer here:

- `exchange` moves a bounded body and does not know it is a check-in.
- `attest` signs and verifies bytes and does not know what they assert.
- `objectstore` moves one stream under an issued or authenticated provider
  capability and does not know what the bytes are evidence of.
- the control plane reads a document, validates it, and applies its own policy
  without knowing which tool produced it.
- the tool obeys an authentic decision without knowing how the policy that
  produced it was reached.

**Every step is a struct or an enum.** A typed document is written, travels,
is read, and a typed document is written back. No prose, no map, no string
blob, no shape that names a product, at any step in that chain.

This is why the design is worth getting right before the code: **when each
layer is oblivious to what it carries, coupling does not have to be policed,
because there is nowhere for it to attach.**

### 0.3 What this obliges Primitive to do

Because consumers may not reach past Primitive, every door a consumer needs must
exist here and must be reachable.

- A capability two or more consumers need is Primitive's, even when only one
  needs it today.
- A door that exists but cannot be reached is a defect equal to a missing one.
  The question to ask is "why can't the caller reach it?", not "does it exist?".
- A type Primitive demands but will not produce is the same defect wearing a
  different hat. If Primitive requires a value, Primitive supplies a way to make
  one.
- Consumer-specific product policy, vocabulary, thresholds, and business rules
  never enter Primitive. Evidence eligibility, exact pre-transfer disclosure,
  confirmation and `--yes` behavior, destination choice, CLI rendering, and
  customer language therefore remain product-owned. Primitive carries only
  the typed facts and agreements those decisions consume. Hidden build/release
  command classification, operator confirmation, diagnostic arguments,
  live-data sandbox policy, release authorization, display, and the direct
  control-plane adapter likewise remain product-owned. `CAPABILITIES.md` is the
  index consumers read before writing anything that crosses the wire or touches
  the real world.

## 1. Normative language and decision order

The words **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY** are
requirements, not suggestions.

When two instructions appear to conflict, apply this order:

1. **The Go compiler and typed Go source own executable contracts.** Public
   types, nominal values, closed enums, constants, errors, interfaces,
   validation methods, and function signatures are the authority for code.
2. **`core` owns shared compiler-visible contracts.** Its typed architecture
   catalog is the executable authority for Primitive package identities and
   allowed dependency edges.
3. **This document owns architecture, package scope, delivery law, testing law,
   and review law.** The package graph in this document must remain identical
   to the `core` catalog and its generated or checked projections.
4. **`LEDGER.md` owns live progress only.** It records what happened, what is
   deferred, and what comes next. It does not define public contracts or alter
   architecture.
5. **Interviews and backups are evidence, not authority.** They preserve
   research, proven mechanics, defects, and rejected approaches. They cannot
   override the compiler-owned design.
6. **README diagrams and tables are projections, not parallel authorities.**
   Sentinel must fail when they drift from the catalog and this document.

There must never be two active authorities for the same fact.

## 2. The prime directive

> If the compiler cannot see a requirement, enforce it, or break the build when
> it changes, it is not a real contract.

For every important requirement:

1. move the requirement to its real owner;
2. represent it with a typed struct, nominal type, closed `iota` enum, typed
   error, typed interface, validation method, or compiler-visible constant;
3. validate it at every ownership boundary where invalid state could enter;
4. test it through compiler-owned contracts; and
5. add or strengthen a ratchet so the requirement cannot silently regress.

Comments and prose may explain a contract. They may not be the only place the
contract exists.

## 3. Canonical implementation path

Every package MUST follow one path:

```text
typed execution request or external input
    -> pure typed Decode / New / Validate / Prepare
    -> one owner-only projection
    -> exact Go standard-library, documented protocol, or official-SDK primitive
    -> O(1) streaming execution
    -> validated typed result
```

Each stage has one job.

### 3.1 Typed request or external input

- Known inputs MUST enter through typed request structs, nominal values, closed
  enums, standard interfaces, or explicitly typed external capabilities.
- External bytes, text, headers, paths, environment values, and provider
  documents MAY be raw at the outermost boundary only.
- Raw external representations MUST be decoded immediately into the owning
  typed form before package logic uses them.
- `map[string]any`, open-ended option bags, string blobs, bool protocols, and
  implicit field conventions are forbidden.

### 3.2 Pure typed construction

- `Decode`, `New`, `Validate`, and `Prepare` operations SHOULD be deterministic
  and side-effect free unless their name and owning type explicitly describe an
  effect.
- Constructors MUST establish or reject invariants. They MUST NOT return a
  value that is known to be invalid.
- Decoders MUST parse into typed structures, reject unsupported forms, and
  invoke the owning validation rules.
- Preparation MUST produce a typed, bounded execution plan. It MUST NOT smuggle
  effects through hidden globals, callbacks, or raw strings.

### 3.3 One owner-only projection

- A typed value MAY be projected into a raw standard-library, protocol, OS, or
  official-SDK representation only inside the package that owns that
  projection.
- The projection MUST be small, direct, inventoried, and testable.
- Other packages MUST NOT reproduce the projection, copy its constants, or
  infer its conventions.
- Internal logic MUST remain structure-to-structure. Raw representation is a
  boundary detail, never an internal protocol.

### 3.4 Real substrate execution

- Effects MUST use the exact Go standard library, documented protocol, real OS,
  or official SDK named by the package contract.
- Primitive MUST NOT insert a lookalike filesystem, scheduler, transport,
  process model, encoder, transaction engine, worker system, or provider client
  between typed intent and the real substrate.

### 3.5 Streaming execution

- Variable-sized data MUST move through `io.Reader`, `io.Writer`, iterators,
  hashers, encoders, decoders, and bounded buffers.
- Production memory MUST be O(1) in the extent of one input unless a fixed,
  explicit, validated bound is proved.
- Total memory MAY scale with the number of active operations when each
  operation has a fixed, owned bound. It MUST NOT scale with total dataset size
  by accident.

### 3.6 Typed result

- Public results MUST be typed structs, nominal values, closed enums, or typed
  errors.
- Results MUST be validated before crossing a package boundary, being
  persisted, or being emitted externally.
- Native and provider error identity MUST remain reachable.

## 4. Compiler-owned data and contract rules

### 4.1 Required representations

| Important fact | Required representation | Forbidden representation |
| --- | --- | --- |
| Request or result | Named struct with typed fields | Loose map, positional slice, string blob |
| Identifier, path, revision, token, amount, digest, mode, or status | Nominal type with owner validation | Bare repeated string, integer, or byte slice |
| Closed choice | Typed `iota` enum with exhaustive validation | String switch, integer magic value, multiple booleans |
| Repeated protocol value | Owner package constant; `core` constant when shared | Copied literal or comment-only convention |
| Cross-package invariant | Focused typed contract in `core` | Sibling import added merely to borrow a rule |
| Failure identity | Stable `core` identity plus typed local context when useful | Error-string matching or copied error text |
| External payload with known schema | Typed document struct and validated codec | `map[string]any` or ad hoc concatenation |
| Variable-sized bytes | Reader/writer/iterator plus fixed buffer policy | Whole-dataset read or unbounded accumulation |
| Lint or architecture requirement | Typed witness or compiler-visible catalog entry | Ceremonial import, comment, filename convention |

### 4.2 Structure-to-structure only

Public package operations MUST accept and return structures, nominal values,
standard data-plane interfaces, or typed errors.

The following are forbidden as internal APIs:

- `map[string]any`;
- `map[string]string` used as an unbounded option or protocol bag;
- JSON, YAML, SQL, shell, HTTP, or provider documents passed around as opaque
  strings;
- delimiter-based mini-languages;
- mode or status encoded by booleans;
- multiple arguments whose ordering or combination forms an undocumented
  protocol; and
- untyped callbacks whose legal behavior exists only in prose.

A map MAY exist only when the domain itself is a map and all of the following
are true:

1. the map has a named domain type;
2. keys and values are typed;
3. size and legal contents are validated;
4. iteration-order assumptions are absent or explicitly encoded; and
5. the map is not being used to avoid defining a real struct.

A `bool` MAY describe a genuinely local binary fact. It MUST NOT encode a mode,
phase, policy, ownership decision, status machine, or multi-state protocol.

### 4.3 No hidden contracts

The following are not contracts and MUST NOT be relied upon:

- copied filenames or directory names;
- copied header names, methods, status values, or protocol revisions;
- copied error strings;
- magic numeric limits;
- magic environment variable names;
- function names discovered or invoked by informal convention;
- undocumented ordering requirements;
- reflection-based discovery used to avoid a typed interface;
- package initialization order used as protocol; and
- comments that describe requirements not enforced by types, validation, tests,
  or the canonical gate.

Move every repeated or cross-package requirement to its typed owner.

## 5. Validation law

### 5.1 Ownership

Validation belongs to the type that owns the rule.

- A request validates request invariants.
- A nominal path type validates path invariants.
- An enum validates legal enum values.
- A persisted document validates its schema and semantic invariants.
- A result validates the guarantees promised to callers.
- A package coordinator may validate composition rules only when that package
  owns the composition.

No caller may reimplement another type's validation with copied conditionals,
regular expressions, literals, or string checks.

### 5.2 Required validation boundaries

The owning `Validate()` method, or an equally explicit typed validation method,
MUST run at every applicable boundary:

1. **Ingress:** immediately after external input is decoded.
2. **Construction:** before a constructor returns a value.
3. **Package crossing:** before a value is accepted from or returned to another
   package.
4. **Persistence:** before writing and immediately after reading.
5. **Execution:** immediately before an effect uses the value.
6. **Recovery:** before recovered state is trusted or resumed.
7. **External output:** before a document, response, artifact, or capability is
   emitted.

Validation at one boundary does not excuse skipping another boundary when data
may have been persisted, reconstructed, corrupted, or supplied by a different
owner.

### 5.3 Validation behavior

Validation MUST:

- be deterministic for the same value;
- reject every undefined enum value, including zero unless zero is deliberately
  valid;
- enforce bounds before allocation or execution;
- preserve stable error identity;
- compose by calling child types' validation methods;
- return early on failure;
- avoid mutation unless the method is explicitly a normalization constructor;
  and
- avoid I/O unless the method's contract explicitly owns external validation.

Validation MUST NOT:

- silently fill required fields;
- treat an empty value as a hidden default;
- duplicate a rule owned by another type;
- depend on error text;
- accept unknown fields or modes merely for possible future compatibility; or
- convert an invalid state into a vaguely “best effort” state.

## 6. `core`: shared contracts, not a junk drawer

### 6.1 Admission rules

`core` MUST begin deliberately small. A contract enters `core` only when current
evidence proves one of the following:

1. it is a package identity or architecture-catalog fact;
2. it is a non-error, cross-cutting fact independently required by at least two
   named consuming Primitive packages; or
3. it is a stable typed error identity with a named producer and a real caller
   decision.

“Shared” means independently required by at least two named consuming Primitive
packages. The package that first discovers or defines a fact does not count as
a second consumer merely because it owns the code. A fact needed by only one
consumer stays with its natural owner.

A coherent domain contract does not move into `core` merely because another
package consumes it. The domain package continues to own its request and result
types, enums, validation, and operations; the consumer imports that package only
when the exact graph lists the capability edge. For example, `controlwire` owns
the control-wire domain even though `controlplane` consumes it.

No speculative, orphaned, future-proof, or “probably reusable” contract may
enter `core`.

### 6.2 What belongs in `core`

When the admission rule is satisfied, `core` owns the shared contract for:

- package identities and exact dependency edges;
- cross-cutting paths and path rules independently needed by multiple consumers;
- cross-cutting protocol revisions and repeated protocol values;
- HTTP constants independently used by multiple Primitive packages;
- shared numeric, encoding, and size contracts;
- repeated function-name or witness contracts;
- cross-cutting validation rules that no coherent domain type naturally owns;
- stable error identities;
- typed architecture metadata; and
- any other product-neutral invariant that at least two named consuming
  Primitive packages must interpret identically.

`core` does not absorb a coherent domain package's API. A listed sibling import
is correct when the consumer uses that sibling's actual owned capability. A
sibling import is forbidden when its only purpose is to borrow an incidental
constant, copied path, error text, validation helper, or naming convention.

Use focused files such as:

```text
core/error_identity.go
core/path_contracts.go
core/http_constants.go
core/protocol_contracts.go
core/architecture_catalog.go
```

Each file MUST have one narrow ownership reason. `core` MUST NOT become a
miscellaneous utilities package.

### 6.3 Import rule

- `core` imports only the Go standard library.
- `core` imports no Primitive package.
- Every current non-Core package in the exact graph MUST import `core` and use
  the declared edge semantically.
- A Primitive package imports another sibling only when section 15 lists that
  exact capability edge and the consumer uses the sibling's owned domain.
- Primitive sibling packages MUST NOT import one another merely to share a
  constant, path, error string, validation rule, or naming convention.
- Ceremonial imports are forbidden. Every required import edge must be used
  semantically and witnessed where Sentinel requires it.

### 6.4 Reopening `core`

`core` is expected to reopen.

When a later package proves a missing shared contract:

1. stop that package;
2. name the producer, every current consumer, and the caller decision;
3. add the smallest typed contract to `core`;
4. add hostile boundary tests and structural enforcement;
5. run the canonical gate;
6. present the exact `core` diff for explicit user review;
7. commit and publish `core` only after approval; and
8. resume the dependent package against the published contract.

Never copy the missing contract locally to keep moving. Starting too small is
cheap to correct. Starting too large couples every dependent to an abstraction
that evidence did not justify.

## 7. Error identity law

### 7.1 Stable identity

Stable error identity is compiler-owned and lives in `core`.

Every stable identity MUST have:

- a named producer;
- at least one real caller decision;
- a typed or sentinel identity that `errors.Is` or `errors.As` can recognize;
- tests proving identity survives wrapping; and
- no duplicate text-based identity elsewhere.

### 7.2 Local context and native causes

A package MAY add typed local context, operation names, safe field values, or a
native/provider cause. It MUST preserve all identities callers need.

- Wrap with `%w`, `Unwrap`, or another standard mechanism that preserves the
  chain.
- Native OS, standard-library, protocol, and provider errors MUST remain
  reachable through `errors.Is` or `errors.As` when callers may need them.
- Redaction rules MUST be typed and applied before external disclosure.

### 7.3 Caller and test behavior

Callers and tests MUST use:

- `errors.Is` for stable identity;
- `errors.As` for typed detail or native/provider error types; and
- typed fields for structured decisions.

Callers and tests MUST NOT use:

- exact string equality;
- substring matching;
- regular-expression matching against error text;
- copied error messages;
- assumptions about wrap depth; or
- assumptions about punctuation or formatting.

Error text is for humans. Error identity is for programs.

## 8. Real substrate and effect ownership

### 8.1 Standard interfaces remain the data plane

Use Go's real interfaces directly, including:

- `context.Context`;
- `io.Reader` and `io.Writer`;
- `fs.FS` and real OS handles;
- `http.RoundTripper` and `net/http`;
- `os/exec`;
- standard encoders and decoders; and
- official provider SDK types when the package contract selects an SDK.

Primitive MUST add typed execution contracts, validation, and bounds around
these primitives. It MUST NOT replace them with Primitive-shaped lookalikes or
interpret a consumer's product policy.

### 8.2 One effect leaf

Every effect MUST terminate in one small, inventoried leaf that directly uses
the selected standard-library, OS, documented-protocol, or official-SDK
primitive.

The effect leaf MUST:

- receive validated typed intent;
- own only the final projection and effect;
- expose native/provider failures;
- contain no hidden retry or global coordination;
- be small enough for direct review; and
- have a clear owner and cleanup path.

Hidden effect paths are forbidden.

### 8.3 Single ownership of execution policy

Here, execution policy means the mechanics of performing an effect: retry,
backoff, timeout, buffering, commitment, and cleanup. The consumer still owns
which operation to request, the values it selects, and the product meaning of
the returned observation.

Exactly one layer owns each of the following for one operation:

- retry;
- backoff;
- jitter;
- timeout;
- redirect behavior;
- buffering;
- concurrency or admission, when Primitive owns it at all;
- integrity commitment;
- final persistence or publication commitment; and
- cleanup or rollback.

No package may add a second layer “for safety.” Duplicate policy produces
unbounded behavior, contradictory timing, and invisible coupling.

### 8.4 Forbidden substrate replacements

Production code MUST NOT introduce:

- a raw syscall, hand-written syscall number, or assembly thunk in place of the
  standard library or `golang.org/x/sys`;
- raw sockets in place of `net/http` or the selected official protocol client;
- `unsafe`;
- copied provider behavior;
- a private scheduler;
- a generic worker pool added merely to look scalable;
- an alternate filesystem abstraction;
- a hand-built transaction engine;
- a generic state machine that models a world the substrate already owns;
- a lookalike clock, context, reader, writer, transport, process, encoder, or
  provider client; or
- a duplicate buffer hierarchy.

When the standard library genuinely cannot reach an operation, the only escape
is `golang.org/x/sys`. There is no third option and no local exception. Where a
package must scope such an import, it does so in one build-tagged leaf file that
a structural ratchet names, so the exception stays visible rather than
spreading.

### 8.5 Feature rejection test

Reject a package or feature when any of the following is true:

1. direct substrate code is equally clear and preserves all Primitive value;
2. Primitive would duplicate substrate behavior;
3. the package has no single product-neutral owner;
4. the package needs a world model to appear correct;
5. the effect cannot be reduced to one direct inventoried leaf;
6. resource ownership or bounds cannot be stated precisely; or
7. the API is not simpler than repeatedly writing the direct, correct substrate
   operation.

## 9. Streaming, memory, concurrency, and lifetime bounds

### 9.1 Streaming law

Variable-sized input MUST be streamed whenever the substrate permits it.

Preferred tools are:

- readers and writers;
- iterators;
- incremental hashers;
- streaming encoders and decoders;
- bounded scanners;
- fixed-size scratch buffers; and
- explicit reopenable-source contracts when an operation must be replayed.

Do not load an entire file, object, response body, artifact set, repository,
manifest collection, or dataset into memory unless the package contract
explicitly requires it and proves a fixed maximum bound before allocation.

### 9.2 O(1) meaning

“O(1) memory” means memory is constant in the extent of each input stream.

It does not mean zero allocation. It means:

- buffer sizes are fixed and typed;
- bounds are checked before allocation;
- one operation does not accumulate all prior input;
- total memory is explainable from fixed per-operation costs and the number of
  active operations; and
- no hidden queue or fan-out multiplies work or buffers.

### 9.3 Lifetime ownership

Every goroutine, handle, stream, retry, cleanup, timer, ticker, lock, process,
detached operation, and temporary artifact MUST have:

1. one named owner;
2. a finite lifetime;
3. a cancellation or completion condition;
4. a bounded resource cost;
5. an explicit cleanup path; and
6. tests for abnormal termination where applicable.

No fire-and-forget production work. No leaked handles. No orphaned goroutines.
No cleanup delegated to hope.

### 9.4 Concurrency ownership

Primitive MUST use Go's concurrency and the real substrate directly. It MUST
NOT invent global coordination where host limits already provide the honest
boundary.

Shared mutable state MUST be avoided or explicitly synchronized. Race proof is
part of completion when concurrency exists.

## 10. Complexity, clarity, and ratchets

### 10.1 Cyclomatic complexity

Every production function MUST satisfy:

```text
gocyclo <= 10
```

Do not suppress, waive, or raise the threshold to land a change.

### 10.2 Function design

Production functions MUST be:

- single-purpose;
- named for one decision or transformation;
- flat rather than deeply nested;
- built with early returns;
- split into typed helpers when complexity approaches the limit; and
- readable when used in an expression or condition.

Split functions aggressively. A large function is not “simpler” because its
helpers were inlined.

### 10.3 Local and global ratchets

Every accepted improvement raises the minimum standard.

- A **local ratchet** protects one package's typed API, validation, bounds,
  tests, complexity, and effect inventory.
- A **global ratchet** protects repository-wide architecture, imports, Core
  ownership, testing law, formatting, static analysis, race safety, and the
  canonical gate.

A later change MUST NOT weaken a ratchet, delete a proof, widen an API, restore
an untyped path, raise a limit, or add an exception merely to make the build
pass.

Clean builds only. No ignored failures. No “temporary” red gate on the main
worktree.

## 11. No compatibility debt

Primitive performs clean upgrades only.

The following are forbidden:

- compatibility wrappers;
- shims;
- aliases for retired APIs;
- dual read or write formats;
- wrapper functions preserving old signatures;
- temporary adapters;
- deprecated paths kept “for now”;
- local `replace` directives for consumer migration;
- unpublished Primitive revisions in consumers;
- copied implementations during transition;
- two authorities for the same operation; and
- dead code retained as rollback folklore.

When a contract changes:

1. update the real typed contract;
2. update every real call site in the approved migration scope;
3. delete the superseded implementation;
4. run focused and full proof;
5. review the exact diff; and
6. land the change atomically.

No backwards-compatibility debt. No half-migration. No dead path.

## 12. Witnesses, lint contracts, and compiler visibility

Typed witness declarations and lint-required interfaces are compiler-owned
contracts. They MUST remain when an analyzer, Sentinel rule, or API contract
requires them.

They MUST be:

- typed;
- minimal;
- direct;
- semantically connected to the implementation; and
- tested or structurally checked where appropriate.

They MUST NOT be replaced by:

- comments;
- reflection;
- copied names;
- ceremonial imports;
- generated strings;
- blank identifiers that prove nothing; or
- tests coupled to an incidental implementation detail.

Do not confuse typed witnesses with the external **Witness** consumer
repository. Primitive never imports the Witness repository. Commands and
consumer workflow coordinators remain leaves.

## 13. Testing law

### 13.1 Canonical protocol

All tests MUST conform to:

```text
/_docs/testing_protocol.md
```

No package-specific test convention may weaken or bypass that protocol.

### 13.2 Compiler-owned test inputs and assertions

Tests MUST use only compiler-owned contracts:

- typed request and result structs;
- nominal values;
- closed enums;
- `Validate()` and other owner methods;
- `core` constants;
- typed architecture entries;
- `errors.Is`;
- `errors.As`; and
- real standard-library, OS, documented-protocol, or official-SDK behavior.

Tests MUST NOT use:

- raw string matching for error identity;
- duplicated constants;
- copied paths or protocol values;
- hidden wrapper assumptions;
- brittle private-field coupling;
- implementation-order assumptions not guaranteed by the contract;
- raw blobs in place of typed documents; or
- a test-only protocol that production does not use.

### 13.3 Hostile tests

Tests are hostile. Their job is to break production behavior before production
does.

For every expected boundary, test both sides:

- smallest valid value;
- largest valid value;
- value immediately below the valid range;
- value immediately above the valid range;
- zero value;
- undefined enum values;
- truncated input;
- malformed input;
- duplicated input;
- reordered input where order matters;
- cancellation before, during, and after commitment;
- native and provider failures;
- recovery from partial state;
- cleanup after failure;
- concurrency and race behavior; and
- streaming behavior at consumer-scale input.

Tests pressure real valid and invalid boundaries. They do not invent a security
problem unrelated to the package's actual ownership or threat surface.

### 13.4 Real package, real substrate

The following are forbidden in package design, implementation, and tests:

- stubs;
- fakes;
- mocks;
- simulators;
- surrogate packages;
- Primitive-shaped test doubles;
- temporary type-check trees; and
- lookalike standard-library or provider components.

Tests run in the real package against the real substrate. When a live or native
proof is required, run it rather than replacing it with a story.

### 13.5 Required proof classes

Run every proof class applicable to the package:

| Proof | Required purpose |
| --- | --- |
| Focused unit | Typed transformations and owner validation |
| Boundary | Exact minimum, maximum, malformed, and transition behavior |
| Failure | Native, provider, cancellation, cleanup, and commitment failures |
| Fuzz | Decoder, parser, validator, and invariant pressure |
| Race | Shared-state and concurrent-operation safety |
| Native | Real OS, filesystem, process, signal, clock, or network behavior |
| Crash/recovery | Durable state, activation, rollback, and restart truth |
| Live | Real provider or end-to-end behavior when the contract requires it |
| Structural | Exact imports, no cycles, Core ownership, witness use, projections |
| Canonical gate | Repository-wide ratchets through `bash scripts/gate.sh` |

Meaningful red tests SHOULD precede implementation. A placeholder failure does
not count as a red test.

## 14. Core-first bootstrap

Core is the first production package and the base of every later package.

Core's first milestone MUST include:

- package identities;
- the typed dependency catalog;
- currently proven shared facts;
- currently proven stable error identities;
- focused owner validation;
- hostile boundary tests;
- fuzzable validation;
- landed-package structural enforcement;
- the canonical gate; and
- explicit user review.

Sentinel begins with `attest`, the first non-Core package that creates a real
compiler-visible consumer edge. Sentinel MUST NOT block Core on a relationship
the current graph cannot observe or decide.

After approval, Core is committed and published before any dependent package
starts.

## 15. Exact package graph

The catalog contains **41 production packages** plus test-only `testserial` and
`controlplanetest`.
Every listed production import is required and MUST be used semantically. Every
unlisted Primitive sibling import is forbidden.

A package may start only when its complete dependency frontier is published.
The order is dependency depth, not a command to build every package in a row.

| Order | Package | Single owned purpose | Required direct imports | Required test-only imports |
| ---: | --- | --- | --- | --- |
| 1 | `core` | Shared nominal values, errors, paths, protocol facts, numeric and encoding contracts | none | none |
| 2 | `attest` | Canonical Ed25519 envelopes and proof-carrying verification | `core` | none |
| 2 | `contextstate` | Nil-safe context ingress and terminal observation | `core` | none |
| 2 | `currency` | Exact minor-unit values, arithmetic, ordering, and decimal projection | `core` | none |
| 2 | `keygen` | Exact secret and Ed25519 key generation | `core` | none |
| 2 | `testserial` | Test-only isolation declaration and analyzer contract | `core` | none |
| 2 | `wiring` | Bounded immutable runtime component graphs with exact Primitive-door declarations | `core` | none |
| 3 | `filelock` | One advisory whole-file lock on one already-open file | `core`, `contextstate` | none |
| 3 | `filestore` | Rooted OS handles, confinement, inspection, durability, activation, append rotation, rename, and recovery | `core`, `contextstate`, `temporal` | `filelock` |
| 3 | `hostfacts` | Host disk, memory, cgroup, tree, and OOM observations | `core`, `contextstate` | none |
| 3 | `temporal` | Time, duration, arithmetic, persistence, waits, and tickers | `core`, `contextstate` | none |
| 3 | `lineio` | Bounded line scanning over one `io.Reader` through Go `bufio.Scanner` and `bufio.ScanLines` | `core` | `filestore` |
| 3 | `manual` | Bounded validated human text and stable machine JSON manuals from one product-owned typed book | `core` | none |
| 4 | `exchange` | Bounded client and server boundary policy over `net/http` | `core`, `contextstate`, `keygen`, `temporal` | none |
| 4 | `fuzzfinder` | Bounded classification and observation of Go-generated fuzz artifacts | `core`, `filestore` | none |
| 4 | `id` | Canonical UUIDv7 and ULID time-ordered identifiers from one observed instant and caller-supplied entropy | `core`, `temporal` | none |
| 4 | `lease` | Signed lease timeline, assessment, renewal, and monotonic advance | `core`, `temporal`, `attest` | none |
| 4 | `process` | Argv, environment, containment, bounded output, exit, and reaping over `os/exec` | `core`, `contextstate`, `temporal` | `testserial` |
| 5 | `release` | Clean repository binding, verified Go builds and process plans, bounded maintainer material exchange, executable inspection, signed tool and metadata provenance, immutable artifacts, manifests, Latest, and selection | `core`, `temporal`, `attest`, `filestore`, `controlwire`, `keygen`, `process` | `testserial` |
| 4 | `shutdown` | Signal observation and phased bounded cleanup | `core`, `contextstate`, `temporal` | none |
| 5 | `gate` | Pure CLI-side new-work authorization over one authentic Lease assessment | `core`, `lease` | `attest`, `temporal` |
| 5 | `receipt` | Authenticated accepted-evidence facts and fixed-size monotonic watermarks | `core`, `attest`, `temporal` | none |
| 5 | `controlwire` | Shared control-wire facts and paired authenticated socket with request-owner body limits | `core`, `keygen`, `exchange`, `temporal` | `controlplane`, `controlplanetest` |
| 5 | `objectstore` | Bounded vendor-specified S3, GCS, or Cloudflare Images transfers through issued HTTPS capabilities, with integrity and provider evidence | `core`, `contextstate`, `temporal`, `exchange` | none |
| 5 | `timeproof` | RFC 3161 request construction, response verification, and replay | `core`, `temporal`, `keygen` | none |
| 5 | `cloudidentity` | Bounded Google Cloud identity-token and OAuth access-token or AWS identity-token acquisition with redacted disclosure | `core`, `temporal`, `exchange` | none |
| 4 | `secretstore` | Bounded exact-version secret access through official provider SDKs | `core`, `contextstate` | `process` |
| 5 | `taskmanager` | Blind bounded task-management wire and paired HTTP socket | `core`, `exchange`, `id`, `temporal` | none |
| 6 | `controlplane` | Signed control-plane request and response documents, their binding to one exact request, product status, and usage watermark | `core`, `controlwire`, `attest`, `lease`, `temporal`, `receipt` | none |
| 6 | `submission` | Authenticated evidence declarations, authority upload grants, and device-signed provider completion evidence bound to one exact request | `attest`, `chit`, `controlwire`, `core`, `id`, `objectstore`, `receipt`, `temporal` | `exchange` |
| 7 | `submissionauth` | Installation-certificate binding, device authentication, and authority reconciliation for evidence submissions | `core`, `attest`, `chit`, `controlplane`, `controlwire`, `objectstore`, `receipt`, `submission` | `controlplanetest`, `exchange` |
| 7 | `controlplanetest` | Real authority-signed installation certificate fixtures for hostile control-plane tests | `core`, `controlplane`, `controlwire`, `lease`, `receipt`, `temporal` | none |
| 6 | `deploy` | Exact create-only GCS publication of one authenticated release and its metadata | `core`, `objectstore`, `release` | `attest`, `exchange`, `temporal` |
| 6 | `upgrade` | Crash-recoverable installation, activation, startup truth, rollback, and recovery | `core`, `filestore`, `hostfacts`, `objectstore`, `release`, `temporal` | `exchange` |
| 7 | `distribution` | Signed product-neutral release publication, update discovery, and exact upgrade-download agreements | `attest`, `controlwire`, `core`, `deploy`, `objectstore`, `release`, `temporal`, `upgrade` | `exchange` |
| 7 | `distributionauth` | Authenticated release-material responses plus installation-certificate binding and device authentication for publication, update, and upgrade requests | `attest`, `controlplane`, `controlwire`, `core`, `distribution`, `release` | `controlplanetest`, `deploy`, `exchange`, `objectstore` |
| 6 | `gcsobjects` | Authenticated Google Cloud Storage bucket provisioning, typed logical namespace composition, create-only writes, IAM-signed short-lived upload capabilities, exact-generation observation, digest-bound reads, and generation-matched permanent deletion through official SDKs | `core`, `contextstate`, `temporal`, `objectstore` | `exchange`, `testserial` |
| 5 | `chit` | Authority-signed immutable custody tickets, streaming manifest closure, bounded catalogs, and device-signed catalog queries | `attest`, `controlwire`, `core`, `id`, `receipt`, `temporal` | none |
| 7 | `chitauth` | Installation-certificate binding and device authentication for one chit catalog query | `attest`, `chit`, `controlplane`, `controlwire`, `core` | `controlplanetest`, `receipt` |
| 6 | `retrieval` | Device-signed exact-object requests, authority-signed expiring download capabilities bound to authenticated chit manifests, and atomic exact-file retrieval execution | `attest`, `chit`, `controlwire`, `core`, `filestore`, `objectstore`, `temporal` | `exchange`, `receipt` |
| 7 | `retrievalauth` | Installation-certificate binding and device authentication for one evidence-retrieval request | `attest`, `controlplane`, `controlwire`, `core`, `retrieval` | `controlplanetest` |
| 6 | `payment` | Authority-signed exact payment receipts, bounded catalogs, and device-signed catalog queries | `attest`, `controlwire`, `core`, `currency`, `id`, `receipt`, `temporal` | none |
| 7 | `paymentauth` | Installation-certificate binding and device authentication for one payment catalog query | `attest`, `controlplane`, `controlwire`, `core`, `payment` | `controlplanetest`, `currency`, `receipt`, `temporal` |

Graph-wide rules:

- `core` imports no Primitive package.
- Production never imports test support.
- Primitive never imports Kernel, Witness, Bug, or Peachfuzz.
- Commands and consumer workflow coordinators are leaves.
- Every listed edge is required and semantically used.
- Every unlisted sibling edge is forbidden.
- A package does not start until every required dependency is published.

## 16. Closed package scopes

### 16.1 `objectstore`

`objectstore` owns three closed provider contracts:

1. Amazon S3 whole-object PUT/GET;
2. Google Cloud Storage XML API whole-object PUT/GET; and
3. Cloudflare Images one-time direct upload.

It composes `exchange` against caller-supplied, expiring HTTPS capabilities
without duplicating transport, retry, buffering, or provider behavior. It owns
the exact-extent streaming reader and the dual SHA-256 and CRC32C integrity
proof that the authenticated `gcsobjects` package reuses; the official Cloud
Storage SDK is not imported here.

Each transfer call owns:

- one provider target; and
- one stream.

Multi-provider replication is explicit caller composition over independently
reopenable sources. It is never a hidden fan-out engine.

`objectstore` does not:

- create buckets;
- mint or persist credentials;
- create signed URLs;
- create Cloudflare draft records;
- implement resumable sessions;
- implement multipart object-upload protocols;
- perform authenticated provider operations, which belong to `gcsobjects`;
- hide multi-provider replication; or
- duplicate SDK, provider, or `exchange` retry behavior.

The authenticated Google Cloud Storage lifecycle is `gcsobjects`, a separate
B4 package that composes the official provider SDK behind a Primitive-owned
client and reuses this package's `Integrity` and `ExactReader`. Its closed
scope is declared in its own objectives policy, `gcsobjects/gcsobjects.md`, per
section 16.3.

### 16.2 `cloudidentity`

`cloudidentity` owns distinct opaque, redacted outbound identity and OAuth
access bearers through three explicit entry points:

1. Google Cloud metadata identity-token acquisition; and
2. Google Cloud metadata OAuth access-token acquisition with its positive
   provider-declared lifetime; and
3. Amazon Web Services regional STS `GetWebIdentityToken` acquisition.

Google Cloud and AWS retain distinct typed request contracts. There is no
runtime provider selector and no generic dispatch path.

AWS authorization arrives as an already-signed exact request capability.

`cloudidentity` does not:

- discover credentials;
- implement Signature Version 4;
- parse or verify token claims;
- cache or refresh tokens;
- execute provider tools; or
- authorize a consumer operation.

### 16.3 The package objectives policy

Each package declares its own objectives policy in its package document,
`<package>/<name>.md` (see section 24.1).

The objectives policy owns **what the package is for**: its single purpose, the
exact capabilities it owns, the decisions it makes, the neighbouring things it
deliberately does not do, and the conditions under which it refuses. Sections
16.1 and 16.2 are that declaration written inline for the two packages whose
scope had to be closed before they were built.

It does not restate typed contracts. Signatures, invariants, enums, errors, and
validation live in Go, where the compiler owns them. An objectives policy that
starts describing what the code must do has become the shadow specification
section 24 forbids; that requirement belonged in a type.

Where the objectives policy meets the real world it names the exact substrate:
the standard-library call, the `golang.org/x/sys` escape, the documented
protocol, or the official SDK that carries the capability, and the one leaf that
owns it. That entry is what makes the layering law of section 0.1 auditable at
the package level.

```text
objectives policy  ->  what this package is for, and what it decides
the effect leaf    ->  how it touches anything outside itself
```

Consumers get the same division one layer up: their objectives are their own,
and every real-world touch routes through Primitive.

## 17. Excluded archive surfaces

| Decision | Surfaces |
| --- | --- |
| Absorbed | `update` into `release` |
| Added from consumer evidence | `process`, `deploy` |
| Deferred | `callbudget`, `cmd/keygen`, `controlstate`, `filestoretest`, `rate`, `redactiontest`, `register`, `status` |
| Retired by clean redesign | `unleash` |
| Retired | `cmd/capabilityinventory`, `probe` |

Deferred and retired surfaces receive no:

- placeholder;
- package;
- constant;
- error;
- adapter;
- alias;
- stub;
- compatibility path; or
- speculative catalog entry.

Re-entry requires all of the following:

1. current consumer demand;
2. one product-neutral owner;
3. a clear real substrate;
4. a complete dependency frontier;
5. a typed bounded design; and
6. explicit user approval.

## 18. Compiler-enforced architecture and Sentinel

### 18.1 Typed architecture catalog

`core` owns one small typed architecture catalog. It is the executable owner of:

- Primitive package identities;
- production dependency edges;
- test-only dependency edges; and
- package ownership declarations needed for structural enforcement.

Core structural tests compare the catalog against:

- compiler build information;
- every landed package's production imports;
- every declared test-only import;
- the package table in this document; and
- the README architecture projection.

### 18.2 Sentinel activation

Sentinel implementation begins with `attest`, when a non-Core package first
makes a catalog edge compiler-visible.

The export-to-consumer ownership check activates only when its named consumers
land and become observable in compiler build information. Before that point:

- interview evidence and explicit user review govern admission;
- the deferral is recorded in `LEDGER.md`; and
- Sentinel MUST NOT claim it decided an unobservable consumer mapping.

### 18.3 Required zero-coupling invariant

Define the coupling coefficient as:

```text
coupling coefficient
    = undeclared production edges
    + undeclared test edges
    + consumer imports
    + deferred-package imports
```

Every term is nonnegative.

The required coupling coefficient is exactly:

```text
0
```

No exceptions. No tolerated drift.

### 18.4 Sentinel failure conditions

Sentinel MUST fail on:

- a missing required production edge;
- an extra production edge;
- a cycle;
- an undeclared test edge;
- production reachability to test support;
- a consumer import;
- a deferred-package import;
- a Core export used by fewer than two named Primitive packages, except a
  stable typed error identity with a named producer and caller decision;
- a ceremonial required import with no typed semantic witness;
- drift between the catalog and this document's package table; or
- drift between the catalog and the README projection.

### 18.5 What Sentinel does not decide

Sentinel checks compiler-visible graph and ownership declarations. It does not
pretend to decide semantic questions the graph cannot prove.

User review must inspect:

- semantic duplication;
- copied provider behavior;
- copied validation logic;
- hidden world models;
- poor API judgment;
- prose quality;
- whether Primitive is actually clearer than direct substrate use; and
- whether a package owns a product-neutral primitive worth keeping.

Semantic review is not replaced by a green Sentinel result.

## 19. Vertical delivery strategy

After Core, build useful capabilities from their published dependency
frontiers.

Consumer repositories remain untouched until the user has accepted all
selected Primitive packages. Then Witness, Bug, and Peachfuzz receive the
complete compiler-owned upgrade one Primitive package at a time.

Select a Primitive package only when:

1. its complete dependency frontier is published;
2. its interview proves current consumer value;
3. its single owner is clear;
4. its real substrate is clear;
5. its typed API and validation ownership can be stated;
6. its resource and lifetime bounds can be stated;
7. its effect leaf is clear; and
8. its later consumer migration boundary is understood.

Build, review, commit, and publish the Primitive package. Record every named
consumer implementation and exact migration boundary without changing the
consumer yet.

When no consumer owns a separable implementation of a low-level primitive,
defer surgery to the first package in that chain that delivers a complete
capability. Record the deferral in `LEDGER.md`.

A deferred consumer surgery does not block another Primitive package whose
complete dependency frontier is already published. Package choice, later
consumer order, and deferrals are evidence-driven ledger decisions, not fixed
examples.

## 20. Mandatory package implementation loop

An implementation LLM or human MUST execute these steps in order for every
package.

### Step 1: Gather the actual evidence

Read:

- the package interview;
- the exact package row in section 15;
- the preserved pre-rebuild package in the backup recorded by `LEDGER.md`;
- the applicable parts of this contract; and
- `/_docs/testing_protocol.md`.

Mine the backup for:

- proven mechanics worth retaining;
- known defects;
- rejected approaches; and
- consumer behavior that establishes a real contract.

The backup is evidence, not authority. Retain only typed convenience layered
directly on Go's real standard-library, OS, documented-protocol, or official-SDK
substrate.

### Step 2: Write the ownership inventory before code

Identify, explicitly:

1. the package's single purpose;
2. the package's single owner boundary;
3. the real substrate;
4. every required production import;
5. every required test-only import;
6. every forbidden sibling import;
7. request and result types;
8. nominal values and closed enums;
9. every validation boundary;
10. every shared contract that belongs in `core`;
11. stable and native error identities;
12. memory, concurrency, and lifetime bounds;
13. the one effect leaf;
14. the one owner for retry, timeout, buffering, and commitment; and
15. the exact consumer paths that will later be deleted.

If any item cannot be stated precisely, implementation has not started. Fix
ownership or scope first.

### Step 3: Design the exact typed API in the real package

Create the real implementation surface, including as applicable:

- `doc.go`;
- exported request and result structs;
- unexported implementation types;
- nominal values;
- closed `iota` enums;
- `Validate()` methods;
- `core`-owned stable error identities;
- package-local typed context errors;
- typed witness declarations; and
- exact function and method signatures.

Do not create a stub, fake, mock, surrogate package, temporary type-check tree,
or speculative compatibility layer.

A separate pre-implementation review pause is required only when the work
exposes:

- a new dependency edge;
- an ownership conflict;
- a new shared contract that must enter `core`; or
- a material scope decision requiring user authority.

Do not invent an answer to any of those four conditions.

### Step 4: Add meaningful hostile red tests

Work directly in the real package on the main worktree.

Add tests that fail because the intended real behavior is absent, not because a
placeholder says `TODO` or returns a forced error.

The tests MUST obey section 13 and `/_docs/testing_protocol.md`.

### Step 5: Implement the smallest direct typed path

Implement only the path required by the current contract:

```text
typed intent
    -> owner validation
    -> owner projection
    -> real substrate effect
    -> typed validated result
```

No speculative surface. No generic engine. No hidden extension point. No copied
behavior.

### Step 6: Run proof

Run, as applicable:

- focused tests;
- boundary and failure tests;
- fuzz proof;
- race proof;
- native proof;
- crash and recovery proof;
- live provider proof;
- Sentinel and structural checks;
- `gocyclo`;
- all local ratchets; and
- `bash scripts/gate.sh`.

A failure is a blocker. Do not explain away a red gate.

### Step 7: Present the exact review package

Before any commit, present:

- the complete typed API;
- the implementation;
- tests;
- exact diff;
- dependency changes;
- Core changes;
- error identities;
- validation boundaries;
- resource bounds;
- effect inventory;
- proof commands; and
- proof results.

Call out every residual uncertainty. Do not call an unverified property proven.

### Step 8: Fix every verified blocker

Fix the implementation or contract. Then rerun every affected proof and the
canonical gate.

Do not weaken a test, delete a witness, raise a bound, suppress a linter, or add
an exception merely to turn the gate green.

### Step 9: Obtain explicit user approval

Only explicit user review and approval authorizes a commit.

Silence, an earlier approval for a different diff, a green gate, or an LLM's own
confidence is not approval.

### Step 10: Commit and publish Primitive

After approval:

- commit the exact reviewed diff;
- publish the Primitive revision; and
- verify the published dependency frontier.

Do not include unreviewed changes in the commit.

### Step 11: Record later consumer surgery

Record the exact consumer code that the accepted Primitive package will replace.
Do not start consumer surgery until all selected Primitive packages are
accepted.

### Step 12: Update `LEDGER.md`

Record:

- accepted scope;
- published revision;
- proof completed;
- deferred decisions;
- later consumer surgery; and
- the next eligible package frontier.

Do not put live progress anywhere else.

## 21. Package definition of done

A package is done only when every statement below is true.

### 21.1 Contract

- [ ] Public request and result paths are structure-to-structure.
- [ ] Every important fact is compiler-owned.
- [ ] Every closed choice is a typed enum.
- [ ] Every repeated or shared value has one compiler-visible owner.
- [ ] Every cross-cutting invariant independently required by at least two named
      consuming packages is in `core`, except an admitted stable error identity.
- [ ] Every coherent domain invariant remains with its domain owner.
- [ ] Every package-local invariant remains package-local.
- [ ] Every public value has the required validation method.
- [ ] Validation runs at every applicable ownership boundary.
- [ ] No raw string, loose map, magic value, bool protocol, copied literal, or
      hidden convention remains.

### 21.2 Architecture

- [ ] The package owns exactly one purpose.
- [ ] The real substrate is named and used directly.
- [ ] There is one small inventoried effect leaf.
- [ ] Every required import is present and used semantically.
- [ ] No unlisted sibling import exists.
- [ ] No consumer, deferred package, or production test-support import exists.
- [ ] Sentinel reports coupling coefficient `0`.
- [ ] The architecture catalog, this document, and README projection agree.

### 21.3 Errors

- [ ] Stable error identities are `core`-owned.
- [ ] Every stable identity has a named producer and caller decision.
- [ ] Local typed context wraps rather than replaces stable identity.
- [ ] Native and provider errors remain reachable.
- [ ] Callers and tests use `errors.Is` and `errors.As` only.
- [ ] No error-string matching or copied error text exists.

### 21.4 Resources and effects

- [ ] Variable-sized input streams.
- [ ] Memory is O(1) in input extent or a fixed bound is proved before
      allocation.
- [ ] Every buffer is fixed, typed, and owned.
- [ ] Every goroutine, handle, retry, timer, cleanup, and detached operation has
      a finite owner and lifetime.
- [ ] Exactly one layer owns retry, timeout, redirect, buffering, and
      commitment.
- [ ] No world model, hidden queue, private scheduler, duplicate buffer
      hierarchy, or copied provider behavior exists.

### 21.5 Code quality

- [ ] Every production function satisfies `gocyclo <= 10`.
- [ ] Control flow is flat and uses early returns.
- [ ] Functions are single-purpose and typed helpers are used aggressively.
- [ ] No shim, alias, wrapper, dual format, dead path, or compatibility layer
      exists.
- [ ] Required typed witnesses and lint interfaces remain intact.
- [ ] Clean build only; no suppressed or ignored blocker remains.

### 21.6 Tests and review

- [ ] Tests conform to `/_docs/testing_protocol.md`.
- [ ] Tests use compiler-owned contracts only.
- [ ] Tests are hostile at every real boundary.
- [ ] No stub, fake, mock, simulator, surrogate, or Primitive-shaped test double
      exists.
- [ ] Applicable failure, boundary, fuzz, race, native, crash, recovery, live,
      and structural proofs pass.
- [ ] Local and global ratchets pass.
- [ ] `bash scripts/gate.sh` passes.
- [ ] Independent semantic review accepts the design.
- [ ] The user explicitly approves the exact diff.
- [ ] Only then is the package committed and published.

Any unchecked item means the package is not done.

## 22. Immediate stop conditions

Stop implementation and correct ownership before continuing when any of the
following appears:

- a new undeclared dependency edge;
- a cycle;
- a missing dependency frontier;
- a copied cross-package contract;
- a Core candidate with fewer than two named package consumers and no stable
  error-identity exception;
- a duplicated substrate behavior;
- a second retry, timeout, redirect, buffer, or commitment owner;
- an unbounded input, allocation, queue, goroutine, handle, or cleanup path;
- an untyped request, result, mode, path, status, error, or protocol value;
- an implicit protocol;
- a need for a generic world model, worker engine, or private scheduler;
- a compatibility shim or dual authority;
- a test that requires a fake Primitive-shaped substrate;
- a production function over `gocyclo 10`;
- Sentinel coupling coefficient above `0`;
- a red local or global ratchet; or
- a request to commit without explicit review and approval.

Do not work around a stop condition. Remove its cause.

## 23. Consumer surgery

Consumer migration begins only after all selected Primitive packages are
accepted. Migration is package-by-package, never one giant cutover.

For each accepted Primitive package:

1. inspect its interviews and accepted migration record;
2. choose the consumer order that gives the clearest first surgery and safest
   dependency progression;
3. skip a consumer when evidence proves it does not own that capability;
4. pin one published Primitive revision;
5. remove the superseded consumer path completely;
6. replace it with the typed Primitive contract;
7. run focused, full, native, crash, and live proof as applicable;
8. present the exact diff for explicit user review;
9. commit and publish that consumer independently after approval; and
10. update `LEDGER.md`.

There is no global consumer order.

Every surgery lands atomically. No local `replace`, unpublished revision, shim,
copy, adapter, dual authority, or dead fallback.

## 24. Documentation ownership

Do not create a per-package planning or specification Markdown file.

The ownership split is:

- typed Go owns public contracts, invariants, errors, witnesses, and executable
  architecture;
- the package interview preserves research and consumer evidence;
- this document preserves architecture and build law;
- `LEDGER.md` preserves live state; and
- README preserves a checked projection for readers.

A new prose file MUST NOT become a shadow specification.
### 24.1 Where documents live

Repository law lives in `_docs/`, organized by kind:

```text
_docs/<name>_policy.md      the build contract: objectives, then shared law
_docs/testing_protocol.md   test doctrine, byte-identical everywhere
_docs/architecture/         how it works: design, specs, reference
_docs/governance/           enforced rules and linter contracts
_docs/performance/          measurements and benchmarks
_docs/history/              dated records: migrations, audits, load tests
_docs/business/             plans and market material
```

The two files at the top are law and are read before work starts. Everything
below them is reference.

A package's own document sits beside the package and is named for it:

```text
<package>/<name>.md
```

That placement is deliberate, not a compromise. A package's document belongs
where a reader already is: someone opening `filestore/` should not have to know
a directory convention to find out what filestore is for. One document does not
need a directory around it.

A package that genuinely owns several documents may add `<package>/_docs/`.
That is the exception. Neither form is created speculatively -- the convention
says where a document goes when one exists, not that every package owes one.

The prohibition in section 24 stands unchanged in both forms: a package document
holds objectives, evidence, measurements, and research. It never holds a
specification, plan, or contract that competes with typed Go. A package document
that starts describing what the code must do has become a shadow specification,
and that requirement belonged in a type.

## 25. Proven substrate pattern

Filestore's first consumer-scale proof establishes the pattern later packages
must preserve:

```text
compiler-owned typed intent
    -> validated bounded stream
    -> Go standard-library concurrency and interfaces
    -> real OS namespace, files, durability, and scheduling
```

The layers reinforce rather than imitate one another.

- Types make ownership, bounds, modes, paths, outcomes, and recovery obligations
  impossible to omit silently.
- Streaming makes memory proportional to active operations, not to the extent
  of the data being moved.
- Go supplies `context.Context`, `io.Reader`, `io.Writer`, goroutines, and real
  handles.
- The OS supplies confinement, namespace arbitration, physical I/O,
  synchronization, and native failures.
- Primitive contributes no alternate filesystem, scheduler, transaction engine,
  worker system, or hand-built state machine between them.

The 2026-07-28 Bug consumer proof drove 10,000 concurrent, distinct, create-only
10 MiB writes through the published Filestore path on an Apple M1 Max. It wrote
97.656 GiB, then streamed every file back and matched every SHA-256 digest to
the real source file.

The profiled one-shot benchmark completed at 448.78 MB/s with approximately
405.3 MB allocated across the complete operation. Of the sampled allocation
space, approximately 322.9 MB was the intentional fixed 32 KiB Filestore buffer
for each simultaneously active stream. CPU samples were dominated by OS
syscalls and Go runtime waits. Bug contributed no flat hot-path CPU and
introduced no mutex, queue, worker pool, or admission bottleneck.

This proves 10,000 as a consumer floor on the measured machine. It does not
establish a hard-coded ceiling or a universal throughput promise. Available
file descriptors, threads, memory, storage, filesystem behavior, and hardware
remain real host limits. Primitive must expose those limits honestly and must
not replace them with global coordination of its own.

Use this result as an architectural ratchet for `exchange` and `objectstore`.

- `exchange` adds typed policy to `net/http` while leaving transport and
  scheduling with Go.
- `objectstore` adds typed bounded transfer and integrity commitment while
  leaving byte movement, provider semantics, and supported retry behavior with
  the official SDK, `exchange`, Go, and the remote service.

If either package needs a world model, duplicate buffer hierarchy, generic state
machine, or private scheduler to look correct, its ownership boundary is wrong.
Stop and redesign it.

## 26. Fast forbidden-pattern replacement table

| Never do this | Do this instead |
| --- | --- |
| Pass `map[string]any` between packages | Define a named request or result struct |
| Pass a raw JSON, path, header, status, or protocol string internally | Decode once into an owner type; project once at the effect leaf |
| Repeat one cross-cutting literal independently in multiple consumers | Prove two named consumers, then move one typed constant to `core` |
| Import a sibling merely for its incidental constant or error text | Move the admitted cross-cutting contract to `core` |
| Consume a sibling's real domain capability | Use the exact typed sibling edge listed in section 15; do not copy or move the whole domain into `core` |
| Encode mode with booleans or strings | Use a closed typed `iota` enum and validate every value |
| Match an error string | Use `errors.Is` or `errors.As` |
| Replace native error identity with friendly text | Wrap typed context while preserving the native cause |
| Revalidate another type with copied logic | Call that type's `Validate()` method |
| Read an entire variable-size input | Stream through a reader/writer/iterator and fixed buffer |
| Add a hidden queue or worker pool | Let Go and the substrate schedule; add only typed bounds actually owned |
| Add retry “just in case” | Name the single retry owner and remove all duplicates |
| Add a compatibility wrapper | Update real call sites and delete the old path |
| Add a fake Primitive substrate in tests | Exercise the real package against the real substrate |
| Keep a linter-required name in a comment | Preserve the typed witness or interface |
| Add a Core helper because it might be useful | Keep it local until two named packages prove the same contract |
| Commit after tests pass | Present the exact diff and wait for explicit user approval |

## 27. Final LLM execution check

Before claiming any task complete, the implementation LLM MUST answer every
question below with **yes** and point to the compiler-visible proof.

1. Is there one clear package owner and one clear purpose?
2. Is the real substrate named and used directly?
3. Is every important input, result, mode, path, status, limit, and protocol fact
   typed?
4. Are all public operations structure-to-structure?
5. Does every owning type validate its own rules?
6. Does validation run at ingress, package crossing, persistence, execution,
   recovery, and external output wherever applicable?
7. Are all cross-cutting facts with at least two named consuming packages in
   focused `core` owners with current evidence?
8. Do coherent domain contracts and package-local facts remain with their
   natural owners?
9. Are stable errors compiler-owned, wrap-safe, and tested with `errors.Is` or
   `errors.As`?
10. Do native and provider errors remain reachable?
11. Is variable-sized work streaming and O(1) in input extent?
12. Does every resource have one finite owner and bound?
13. Is there exactly one effect leaf?
14. Is there exactly one owner for retry, timeout, redirects, buffering, and
    commitment?
15. Does every production function satisfy `gocyclo <= 10`?
16. Are control flow and helpers flat, early-returning, and single-purpose?
17. Is Sentinel's coupling coefficient exactly `0`?
18. Are all typed witness and lint-required contracts intact?
19. Is every compatibility path, alias, shim, duplicate implementation, and dead
    fallback gone?
20. Do hostile tests conform to `/_docs/testing_protocol.md` and use only
    compiler-owned contracts?
21. Do all applicable focused, boundary, failure, fuzz, race, native, crash,
    recovery, live, structural, local-ratchet, global-ratchet, and canonical-gate
    proofs pass?
22. Has the exact diff received explicit user review and approval before commit?

If any answer is **no**, **unknown**, inferred from prose, or supported only by a
string convention, the task is not complete.

Type it. Validate it. Test it. Ratchet it. Or it is not a contract.
