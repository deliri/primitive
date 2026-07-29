# Exchange package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole recon report for archived package `exchange`. Primary archive,
Primitive-internal, and consumer evidence is integrated.

## Evidence boundary

- Primitive archive: `d046f7b675fcb797398d7cdc87b5504f43978056`.
- Kernel HEAD: `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`;
  committed Primitive pin `0df2954a2d91`, dirty pin `e8b7172161a4`.
- Witness HEAD: `b9629af57b7058b68982be5d3b282be440b1e76e`;
  pin `773add8ba0fc`.
- Bug HEAD: `39ce96242240d7174d562c90bb255860946595dc`;
  pin `388e593231a2`.
- Peachfuzz HEAD: `2b2d080c455edaadf88502c1c253845605a4336a`;
  pin `3f74d8fc35b4`.

Kernel's committed code uses an older exchange surface while its dirty
Primitive pin contains the newer architecture. Witness and Bug do not yet
consume the archived package directly. Peachfuzz contains both older and newer
patterns. These states are kept distinct; no dirty tree is represented as a
released consumer.

## Capability ownership

`exchange` owns typed HTTP transmission and reception:

- strict validated JSON request and response flow;
- explicitly bounded byte requests and responses;
- O(1)-memory upload and download;
- body-presence lifecycle encoded in request types;
- retry, backoff, jitter, `Retry-After`, and attempt deadlines;
- idempotency and replay contracts;
- same-origin redirect enforcement;
- server receive/projection/respond symmetry; and
- stable error identities for request, response, status, transport, body,
  cancellation, redirect, retry, and write failures.

It composes `net/http`; it does not replace DNS, TLS, pooling, HTTP framing, or
proxy behavior. It does not own object stores, authentication, product routes,
payload schemas, persistence, or logging
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/SPEC.md:22`).

## Archive evidence

The archived specification and implementation provide the strongest existing
product-neutral transport baseline. The mechanics and their proof are assessed
below.

## Consumer evidence

### Kernel

Kernel's local bridge composes a total workflow budget and performs useful
budget preflight before retry attempts. It does not perform that check before
the first attempt. The total-budget concept is still a requirement the archived
`exchange.Policy` does not yet represent.

The local bridge also demonstrates what not to retain: unchecked duration
multiplication, raw header and route strings, broad status retry, and default
redirect behavior. The total-budget idea is valuable; the informal contracts
are not.

### Witness

Witness's custody flows preserve typed upload/download integrity across a
multi-provider operation. It also applies a higher-level multi-TSA policy above
individual HTTP attempts. That separation is a gem: `exchange` owns one
operation's transmission mechanics, while the consumer owns provider quorum
and workflow policy.

### Bug

Bug stages updates and verifies digests around transport. Staging, artifact
identity, and activation belong to update/filestore composition, not exchange.
Its raw upload code shows the migration need for streaming typed targets and
stable transfer errors.

### Peachfuzz

Peachfuzz has two levels of retry: individual HTTP transmission and
evidence-level/provider workflow retry. Its GCS paths correctly reconstruct
stream sources and destinations for replay. These must remain separate typed
policies; one generic retry loop would blur ownership and double-retry.

## Strong mechanics and proof

### Body lifecycle is structural

JSON body, no-body, bounded bytes, streaming upload, and streaming download
have separate request types. Empty JSON and zero bytes remain bodies; absence
is not inferred from length. This eliminates the common implicit protocol where
method, nil pointer, and length jointly decide request shape
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/SPEC.md:165`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/SPEC.md:175`).

Primitive-owned fields such as content length, content type, accept encoding,
and idempotency are derived from typed contracts. Caller headers cannot
silently override them.

### Validation crosses real boundaries

Open request and policy structs validate at ingress. Strict JSON validates
outgoing structure and incoming decoded structure. Projection occurs into a
temporary value before the immutable result escapes. The package does not
revalidate immutable internal wrappers merely because data moved between
functions (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/SPEC.md:197`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/SPEC.md:207`).

### Attempt and replay mechanics

Each attempt receives a typed `temporal.Duration` deadline. Retry permission is
not inferred solely from method strings: body replayability and idempotency are
explicit contracts. Retry waits and `Retry-After` remain context-aware.

Streaming retries reconstruct the source or destination rather than reusing a
partially consumed stream. This is essential for object-store and evidence
transfers. The package retains neither an entire transfer nor an unbounded
response in memory.

### Redirect confinement

Redirect policy is applied to a shallow client copy so the caller's
`*http.Client` is not mutated. Redirects are restricted to the permitted
origin/semantics rather than accepting the standard client's broad default
behavior.

### Primitive-internal consumers

Archived internal dependents include:

- `objectstore` for bounded and streaming object operations;
- `rate` for typed remote policy exchange;
- `register` for registration protocol transport;
- `status` and `submission` for service exchange;
- `timeproof` for bounded timestamp-provider attempts; and
- `workloadidentity` for identity exchange.

Those dependents demonstrate that the transport primitive must remain
product-neutral and that shared HTTP facts belong in `core`, not copied into
each protocol package.

## Defects and blockers

1. **No total operation timeout.** `Policy` and `StreamPolicy` own
   `AttemptTimeout`, but the package does not bound the complete call across
   attempts and waits (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/SPEC.md:226`). Parent context
   is an ingress boundary, not a compiler-owned guarantee that a caller applied
   the required total budget. Primitive 2026 needs a typed total timeout and a
   derived operation context.

2. **Some wire facts remain raw strings.** `Target.String`, header/query
   fields, and `HeaderSelection` permit protocol meaning to escape typed,
   validated ownership. Shared field names and canonical tokens belong in
   focused `core` contracts.

3. **Some external boundary errors escape through joins.** Body close/read,
   destination/stream write, and `http.ResponseWriter` paths join their
   original error directly
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/client.go:521`,
   `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/bounded.go:395`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/bounded.go:423`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/bounded.go:875`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/bounded.go:896`,
   `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/server.go:288`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/server.go:337`). These errors come from standard-library
   or trusting-caller capabilities, not directly from wire-controlled Go error
   implementations. Even so, stable identity and bounded diagnostic policy
   need an explicit owner.

   Transport and retry-wait errors already have a stronger baseline:
   `trustedTransportCause` allowlists standard identities and
   `trustedURLError`/`trustedTerminalContext` contain panics
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/client.go:701`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/client.go:715`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/client.go:734`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/client.go:748`); production retry waiting
   returns Temporal-owned identity. Preserve that containment and extend a
   precisely scoped rule to the remaining close/read/write boundaries.

4. **Redirect proof is incomplete.** The intended same-origin policy is sound,
   but the proof matrix does not yet exhaust scheme/host/port normalization,
   credential stripping, relative redirects, redirect loops, hostile location
   values, and client immutability across every request family.

5. **Completion was overclaimed.** The archived SPEC labels implementation and
   release gates complete, but the total-budget and hostile-error gaps are
   contract blockers. Passing ordinary gates did not establish the advertised
   lifecycle.

6. **Cross-package timeout ownership needs one rule.** `callbudget` owns
   admission capacity; `exchange` owns total and per-attempt transport limits;
   `temporal` owns arithmetic and waiting; consumers own larger workflows.
   These values must be related by typed validation instead of repeated
   duration literals.

## Primitive 2026 ownership and DAG

- `core` owns HTTP methods, statuses, media types, canonical header/query
  identities, endpoint/path contracts, limits, and stable error identities.
- `temporal` owns durations, deadlines, ordering, checked arithmetic, and
  context-aware waits.
- `contextstate` owns hostile-safe context ingress and terminal observation.
- `exchange` owns HTTP operation execution, total and attempt budgets, retries,
  replay, redirect confinement, bounded bodies, and streaming.
- Provider and product packages own typed targets, payloads, authentication,
  accepted result semantics, and multi-provider workflow policy.

The API should preserve separate structural request families. It should not
introduce a generic transport interface, loose option map, untyped header bag,
or body-kind switch.

## Decision rationale and conditions

The archive is a strong architectural starting point, especially its typed
body lifecycle, replay contracts, bounded streaming, and client/server
symmetry. It is not safe to copy unchanged. A compiler-owned total operation
budget, fully typed wire facts, hostile external-error containment, and a
complete redirect matrix are admission blockers.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
