# Status package recon

Status: `COMPLETE` | Decision: `DEFER`

This report is the sole recon record for archived package `status`. Here,
`status` means an independently signed, product-neutral declaration of one
offering's public service condition. It does not mean Kernel process liveness,
dependency readiness, local boot state, Witness or Bug license standing,
Peachfuzz failure severity, an HTTP response code, or a generic string named
`status`.

## Evidence boundary


The following repositories and exact revisions were inspected read-only:

| Repository | Revision or Primitive pin | Relevant state |
| --- | --- | --- |
| `foundation_back_up_july_27th_2026` | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` | Full archived package |
| archived Status tree | `7629b3e85311cab40577bcc9175618d130554c32` | `HEAD:status` tree |
| initial Status specification | `ae3aa657085be3101c9cf14d79e1e0f89a1316e5` | Authenticated service-status contract introduced |
| Status implementation | `ab617e4cf6a1acec9f08ed0199ab101251e8ffad` | Client, proof, assessment, advance, cache, refresh, and hostile tests introduced |
| Core contract consolidation | `d259789e87bcadb829c5ffac72c6c91ccc604098` | Status constants centralized into Core |
| latest Status-constant change | `251429ce3de8230eaa4195cec2c03bb5736bad61` | Shared JSON-bound relationships changed while Rate was implemented |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | Committed pin `0df2954a2d91`; dirty worktree pin `e8b7172161a4` |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` | Pin `773add8ba0fc` |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` | Pin `388e593231a2` |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` | Pin `3f74d8fc35b4` |

The Status package tree itself has not changed since
`ab617e4cf6a1acec9f08ed0199ab101251e8ffad`. Later commits changed its
Core-owned constants and neighboring packages. The archive worktree has one
unrelated untracked Core test. No archived or consumer source was changed
during this interview.

The archived package is 6,787 lines across its specification, production,
tests, and `core/status_constants.go`. It contains 39 named tests or fuzz
targets.

The focused package gates were rerun:

- `go test ./status` completed successfully.
- `go test -race -shuffle=on -count=2 ./status` completed successfully.
- Production-only `gocyclo -over 10` reported no findings.

The green result proves the archived package on the current Darwin host. It
does not prove an independently deployed Status authority, a real consumer
composition, a scheduler, a production persistence migration, Linux or Windows
execution, or suitability for Primitive 2026.

### Consumer pin and import facts

The committed Primitive pins used by Kernel, Witness, and Bug predate the
Status directory. Kernel's dirty worktree pin `e8b7172161a4` also predates it.
Peachfuzz's pin `3f74d8fc35b4` contains the archived Status package, but
Peachfuzz does not import it.

An exact import scan found:

- no `github.com/deliri/primitive/v2026/status` import in Kernel;
- no such import in Witness;
- no such import in Bug; and
- no such import in Peachfuzz.

The only archived Primitive production dependent is `controlstate`.
Controlstate makes one Status proof mandatory in every issued aggregate
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:17-45`), embeds the concrete Document
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:48-64`), verifies it with independently
supplied Status keys (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/verified.go:97-123`), includes its
validity end in aggregate expiry (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/snapshot.go:244-280`),
and delegates Status progression back to `status.Advance`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/advance.go:363-395`).

That is a real internal dependency, but it is not a consumer cutover. None of
the four consumer repositories imports either archived Status or an aggregate
path that proves its runtime use.

## Capability ownership

The archive says Status does one thing: authenticate, refresh, reconcile, and
evaluate one service-status line for one offering
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:6-9`).

Its truth model deliberately distinguishes:

- an authoritative operational, degraded, maintenance, or outage declaration;
- a current cached declaration;
- an expired declaration;
- an unreachable Status source; and
- a malformed or forged response.

Those states do not collapse into one boolean. In particular, an unreachable
source is not proof of outage, and expired maintenance is not current
maintenance (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:11-21`).

The archive correctly excludes:

- local process health;
- installed version and diagnostics;
- registration, lease, gate, rating, receipt, or update state;
- incident management and on-call workflow;
- endpoint provisioning, DNS, CDN, WAF, and deployment;
- clocks, scheduling, rendering, notifications, telemetry, and analytics; and
- compatibility aliases, wrappers, shims, or fallback decoders
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:778-793`).

The reusable Primitive capability is therefore narrow in meaning:

1. define one signed Status fact and its closed state/timeline vocabulary;
2. verify it against trusted keys and expected offering;
3. assess freshness and effective state at a caller-supplied instant;
4. compare generations without rollback or fork;
5. optionally fetch it through a typed transport boundary; and
6. optionally retain one proof plus bounded refresh-attempt state.

The composition root or authority must own:

- the concrete offering;
- signer custody and rotation;
- authoritative state transitions;
- concrete foreground and background endpoints;
- independent deployment and failure-domain proof;
- observation of wall time;
- scheduling from `NextStep`;
- user-facing rendering;
- status-page or incident-management integration;
- migration of any existing consumer state; and
- how fresh independent Status coexists with an older aggregate.

The archive generally states that ownership honestly. Its implementation,
however, packages the pure proof domain, transport client, cache, durable
throttle, and scheduling advice into one import unit. That coupling is not
justified by a current consumer.

## Archive evidence

### Closed authoritative state

`State` is a closed enum for operational, scheduled maintenance, active
maintenance, degraded, and outage. Unknown is invalid. A refresh transport
failure can never construct `StateOutage`; outage exists only inside a verified
Status document (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:131-146`).

This is the most important semantic rule in the package. It prevents an
ordinary DNS, TLS, timeout, or server failure from being presented as an
authoritative service incident.

The same separation appears in `RefreshResult`: authoritative, source
unreachable, and locally throttled are distinct private unions with
variant-specific payloads (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/client.go:143-239`). Invalid
combinations fail `Validate`.

### Derived nominal identity and monotonic generation

`Identity` privately owns a fixed SHA-256 value. It is derived from a
domain-separated, length-prefixed frame containing revision and Core offering
identity (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/values.go:17-40`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/values.go:94-110`). The caller cannot select it.

`Generation` is a positive private `uint64`, encoded as a canonical quoted
decimal. Decode rejects zero, leading zeroes, overflow, and non-canonical JSON
without mutating the receiver (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/values.go:112-170`).

`Fact.Validate` re-derives Identity and rechecks the complete state/timeline
lattice (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/fact.go:69-100`). This is compiler-visible,
structure-to-structure closure rather than trust in copied wire fields.

### Signed proof carrier

The immutable Fact contains revision, derived identity, offering, generation,
state, timeline, and notice. The signing domain is fixed by the Fact rather
than caller-selected (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/fact.go:24-43`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/fact.go:103-110`).

`Issue` calls the real Attest signer and produces one concrete Document
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/document.go:10-45`). `Verify` independently checks the expected
offering, re-derived identity, trusted key set, envelope, body, and domain, then
returns a privately constructible proof-carrying `Verified`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/verified.go:8-59`).

`Verified.Validate` closes the proof back to the exact Document envelope
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/verified.go:61-75`). Consumers cannot fabricate an
authenticated status by populating exported fields.

### Exact temporal semantics

`Timeline` owns issued, effective, valid-until, and review instants as
`temporal.Instant`, not `time.Time`, integer nanoseconds, or strings
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/timeline.go:10-34`).

Validation enforces:

- issue before validity end;
- issue before review;
- validity end no later than review;
- positive validity;
- an exact maximum validity of 30 minutes; and
- checked Temporal arithmetic
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/timeline.go:48-68`).

The maximum validity and both refresh intervals are constructed through
Temporal from compiler-owned unit counts
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/timeline.go:120-130`). Production imports no `time` package and
reads no clock.

Scheduled maintenance is not a copied second truth. Before its effective
instant it assesses as scheduled; at and after that instant, while current, it
derives active maintenance (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/assessment.go:97-123`).

### Freshness cannot masquerade as current truth

`Assess` accepts one verified proof and one caller-supplied observation instant.
It derives a total `Assessment` containing signed state, effective state,
freshness, timeline, notice, and next step
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/assessment.go:49-95`).

Freshness is a closed not-yet-valid/current/expired enum. `CurrentState` returns
false unless the assessment is current, so a caller cannot obtain expired or
future status through the convenient current-state accessor
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/assessment.go:164-176`).

`NextStep` is a closed union:

- reassess locally at a typed instant;
- refresh from source at a typed instant; or
- refresh from source with no instant known.

The not-yet-valid clock-skew case reassesses the document already held instead
of opening a fleet-correlated network request. Current refreshes at expiry;
expired has no invented completion instant
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/assessment.go:8-47`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/assessment.go:125-136`).

This is a strong scheduling contract. It preserves the semantic difference
between the proof becoming usable and the need to fetch a replacement.

### Honest monotonic advance

`Advance` requires the same derived identity, rejects lower generations,
accepts byte-identical equal-generation replay, rejects divergent
equal-generation forks, and permits higher generations only when revision,
offering, and issue-time progression remain valid
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/advance.go:26-70`).

A later status may shorten validity or review. That is correct: a newer outage
or active-maintenance declaration may need a shorter horizon than an older
operational declaration (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:300-318`).

The selected proof is carried in a validated replay/advanced union
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/advance.go:73-99`). Callers do not compare notices, severity,
arrival order, or local clocks to decide recency.

### Typed transport with no second HTTP path

`ClientRequest` contains one validated Exchange client, two distinct typed
endpoints, trusted keys, Exchange policy, offering, and revision. Construction
validates every boundary and rejects equal endpoint identities
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/client.go:14-38`).

Foreground and background refresh are closed kinds selecting distinct
prevalidated routes. The kind is not transmitted as a caller-asserted class.
That lets the independently deployed host shed scheduled load without making a
foreground incident check indistinguishable
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:320-363`).

The network path uses `exchange.TransmitNoBody` once and then verifies the
returned Document against the client's offering and revision
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/client.go:242-276`). Status owns no HTTP client, redirect,
retry loop, body copier, or media-type parser.

Exchange transport failures and retryable response statuses project into typed
unreachable reasons. Cancellation remains a trusted context error. Malformed,
forged, wrong-offering, or non-retryable responses remain errors and never
become outage or source-unreachable
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/client.go:279-334`).

### Durable per-kind refresh politeness

The archive correctly recognizes that Status cannot use Callbudget: its source
is independently deployed and cannot reconcile lifecycle admission. Instead,
Status owns two local attempt slots with fixed minimum intervals: 30 seconds
for foreground attempts and five minutes for background attempts
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:496-545`).

`Store.Refresh` validates context, client, request, store binding, and offering,
then runs a Store-owned check-and-record transition before the network call
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store.go:96-130`).

Under Filestore root ownership, it reads one bounded record, verifies any
retained proof, checks the selected slot, and durably records the supplied
observation instant before releasing ownership
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store.go:305-342`).

The transition:

- admits a never-attempted kind;
- rejects clock rollback;
- computes the exact next instant with checked Temporal addition;
- returns locally throttled one nanosecond before the boundary; and
- admits exactly at the boundary
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store.go:358-403`).

This is materially better than a process-local mutex, in-memory last-attempt
value, or informal retry instruction. A crash or a second process cannot evade a
successfully written attempt slot.

### Bounded one-record cache

The retained record contains exactly:

- an optional Status Document;
- one foreground attempt slot; and
- one background attempt slot
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store_types.go:144-186`).

The wire is strict, canonical, bounded, and receiver-preserving. Optional
values are explicit private unions, not zero-value conventions or loose maps
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store_wire.go:11-110`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store_wire.go:113-167`).

`Load` treats only `fs.ErrNotExist` as absent. Malformed, oversized, forged,
wrong-offering, unreadable, or invalid retained state fails loudly
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store.go:178-198`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store.go:254-265`).

`Retain` re-verifies the candidate against the Store's own trust roots and
offering, applies `Advance` against any current document, and durably replaces
the one bounded record (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store.go:268-303`).

The bounded sink rejects writes beyond the exact compiler-owned record maximum,
and Filestore owns durable replacement and recovery
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store.go:200-251`).

### Stable typed errors and structural ratchets

Core owns five stable Status identities:

- `core.ErrStatusContract`;
- `core.ErrStatusVerification`;
- `core.ErrStatusRollback`;
- `core.ErrStatusConflict`; and
- `core.ErrStatusPersistence`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:61-65`).

The package wraps those identities while preserving underlying Temporal,
Attest, Exchange, JSON, Context, and Filestore identities for `errors.Is` and
`errors.As` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/errors.go:10-42`).

The architecture suite requires the intended Primitive primitives and forbids
direct `net/http`, `os`, `syscall`, `time`, clock, sleep, and timer use
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/architecture_test.go:17-108`). It also pins the dependency
closure (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/architecture_test.go:136-180`), exact public functions,
types, Store methods, struct inventory, absence of maps, and absence of
production goroutines (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/architecture_test.go:184-311`).

These are useful ratchets. They make a bypass visible at compile/test time.

### Real boundary tests

The suite uses:

- real Ed25519 Issue and Verify paths;
- real `httptest.Server` Exchange requests;
- real temporary files and Filestore;
- exact one-nanosecond temporal boundaries;
- typed error identity assertions;
- strict canonical decode rejection;
- semantic fuzzers; and
- real subprocess contention.

The strongest existing process proof launches two child processes against one
root with explicit stdin barriers. Exactly one reaches the server and the other
returns the exact locally throttled result
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/process_hostile_test.go:25-117`).

The Store layer tests also prove absent/present retention and foreign-authority
rejection through public Store operations
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/network_store_hostile_test.go:126-211`), and prove a forged
retained document blocks both mutation and network execution
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/network_store_hostile_test.go:213-292`).

These are mechanisms worth carrying forward, but they do not satisfy every
proof the specification itself requires.

## Consumer evidence

### Kernel

### Actual use

Kernel has no Primitive Status import. Its colliding `/status`, `/health`,
`/ready`, and `/live` vocabulary describes the local Kernel process and its
Store dependency.

`api.Monitor` owns a local `Prober`, readiness bit, latency source, and
component-recovery goroutine (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/monitor.go:20-45`). Without a Store it
deliberately degrades to liveness-only behavior; with a Store it begins ready
and probes `PendingCount` at readiness boundaries
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/monitor.go:47-81`,
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/monitor.go:215-235`).

The four routes are distinct. `/live` always returns the cached liveness
payload, while health/readiness can return service unavailable after a bounded
Store probe; `/status` returns the Monitor's current local snapshot
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/monitor.go:250-306`).

The service wrapper mounts all four typed routes and fails closed with HTTP 500
when the embedded monitor is absent
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:service/health/health.go:79-118`).

Kernel boot readiness is another separate domain. `ResolveReadiness` projects
validated startup state into failed, needs credentials, not configured, or
active (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:appboot/storeboot/readiness.go:14-29`).

None of those facts is an independently signed public declaration, and none
should be moved into Primitive Status.

### Gems to retain

- Keep liveness independent from dependency readiness. A Store outage should
  stop new traffic, not tell the orchestrator the process is dead.
- Fail closed when the local monitor capability is absent.
- Use typed ingress requests and nominal endpoint viewmodels
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/api_probe_models.go:9-30`,
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/api_probe_models.go:48-66`).
- Cache immutable healthy probe JSON rather than allocate it on every request
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/monitor.go:153-183`).
- Keep recovery-loop lifecycle local to the component it probes.
- Render public signed Status beside local readiness; never let either overwrite
  the other's truth.

### Local defect not to copy

Kernel's shared probe payload stores `Status` and `Component` as unconstrained
strings. Its `Validate` checks only non-empty values
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/api_probe_models.go:32-46`). The call sites currently use Core
constants, but the type itself admits arbitrary strings.

That should be repaired in Kernel with closed local enums. It is not a reason
to broaden Primitive Status into process-health ownership.

### Witness

### Actual use

Witness has no Primitive Status import. Its relevant status output is local
license custody and standing.

Activation explicitly treats an unreachable server as non-authoritative: the
account token and device remain stored so a later check-in can finish
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:120-145`). That is the same semantic lesson as
Status's distinction between an unreachable service and an outage, but it
belongs to license enrollment.

Witness keeps one mutable `license.State` owner for signed server-time
commitment, local check-in rate limiting, clock high water, and lease
progression (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:12-26`). `Validate` closes its
persistence invariants (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:63-89`).

`CheckInDue` separates the signed lease's commercial schedule from a local
retry floor that prevents an offline device from dialing on every command
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:106-124`). Accepted check-ins ratchet time
and lease progression forward (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:146-180`).

### Gems to retain

- Separate transport unreachability from authoritative commercial standing.
- Let signed authority facts own commercial timing.
- Let local durable state own only courtesy back-pressure and rollback floors.
- Keep one state owner and validate at persistence.
- Preserve authenticated historical state when the authority cannot be
  reached.

### Local defect not to copy

The Witness status renderer collapses unreadable device, token, and grant state
into ordinary absence:

- device read error becomes a not-registered result;
- token read error becomes an absent-token result; and
- grant read error becomes an absent-lease result
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:409-435`).

The later trust renderer does distinguish unreadable state
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:438-462`), proving the earlier collapse is not
an unavoidable UX rule.

Primitive Status's explicit absent/present cache result and loud corrupt-state
failure are better. Any consumer composition must preserve that distinction.

### Bug

### Actual use

Bug has no Primitive Status import. Its nearby behavior is license check-in,
activation, local state, and CLI standing.

`performCheckIn` returns a closed local result rather than parsing error text.
Payload, transport-construction, and request failures become unreachable;
response acceptance decides the remaining result
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:195-214`).

Activation dispatch distinguishes granted, refused, unreachable, invalid
grant, and Store fault. The default branch fails loudly if a new result is
added without dispatch support (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:1015-1038`,
`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:1061-1067`).

Unreachable activation says the key remains stored for a later online command,
while an invalid grant says no lease was installed
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:1069-1077`). That is a good authority-versus-transport
distinction.

Bug also separates signed lease schedule from a local retry floor
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:174-192`).

### Gems to retain

- Use a closed result lattice for authority, transport, verification, and
  persistence outcomes.
- Keep unknown/default dispatch loud.
- Do not lose local work merely because a source is temporarily unreachable.
- Keep retry courtesy local and authoritative standing signed.
- Keep the product-specific usage tally and teaching policy outside Status.

### Verified local defects not to copy

`activateCheckIn` treats a State load error as a fresh zero State and continues
the network operation (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:1019-1024`). Corruption therefore
loses the local rollback/check-in high-water facts precisely at an execution
boundary.

The CLI status renderer also collapses read errors into absence for device,
key, and grant, and it ignores device and State read errors during trust
resolution (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:1110-1150`).

Primitive Status must keep its current loud corrupt/unreadable behavior. A
2026 composition must never convert persistence failure to an absent-cache
result or reset a retained throttle/advance record.

### Peachfuzz

### Actual use

Peachfuzz's Primitive pin contains Status, but Peachfuzz has no Status import
or call site. Its relevant code owns local failure classification and archive
availability.

`FailureBoundary` is a closed execution/maintenance/persistence enum, and
`FailureSeverity` is a closed policy result
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/failure_severity.go:13-31`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/failure_severity.go:68-129`).

`ClassifyFailure` shows why Status should report facts rather than universal
policy: the same failure can be maintenance-degraded, target-transient,
execution-expired, cancelled, pause-worthy, or fatal depending on its execution
boundary (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/failure_severity.go:131-175`).

The GCS tests require every non-success status to remain
`ErrArchiveUnavailable`. A broken or unauthorized archive must never look like
an empty archive (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs_test.go:451-490`).

Peachfuzz's daemon owns its own worker and maintenance lifecycle
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/loop.go:22-95`). That is product execution policy, not
Primitive Status scheduling.

### Gems to retain

- Unavailability is not absence.
- Report authenticated Status facts first; let the consuming boundary decide
  whether they pause, degrade, retry, or terminate work.
- Preserve stable typed error identity through transport and persistence.
- Keep product-specific retry and maintenance supervision with the product.

### Local concern not to import into Status

The GCS adapter owns its own retry classification, jitter, and timer
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs.go:551-580`). Whether that should move fully
behind Exchange is a Peachfuzz/archive concern. It does not justify giving
Status a second retry loop or clock.

## Strong mechanics and proof

The archive provides strong authenticated status facts, trusted-time
assessment, closed freshness outcomes, and bounded local scheduling
mechanics. The consumer evidence shows adjacent service and license status
needs but does not establish one current adopter of the full package boundary.

## Defects and blockers

### 1. No real consumer exercises the capability

No inspected consumer imports Status. Peachfuzz could import the package at its
pin but does not. Kernel's similarly named endpoint is a different local-health
domain. Witness and Bug contain useful reachability lessons but no public
service-status flow.

Therefore the archive proves an internally coherent candidate, not a required
2026 public API. There is no evidence yet for:

- which product first needs the capability;
- which offering identity it uses;
- where the independent endpoint is provisioned;
- which trust roots are installed;
- how `NextStep` drives a real scheduler;
- how fresh independent Status is rendered beside local health and license
  standing; or
- how it coexists with Controlstate.

Copying the package now would freeze answers invented by the archive.

### 2. The required independent failure domain is unproved

The specification requires the Status endpoint to avoid the primary lifecycle
API's request-serving and database failure path. It explicitly says a second
hostname on the same process is not independent, and that Primitive cannot
infer independence from a URL (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:46-56`).

Its required-proof section demands an OGS integration where Status remains
reachable while the primary API deployment is unavailable; a second hostname
to the same process must fail the gate
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:751-756`).

No such OGS deployment or integration evidence is present in the archive or
four consumers because the specification deliberately assigns it to an
external OGS integration. `httptest.Server` proves client classification, not
failure-domain independence.

This is an admission blocker, not deferred polish. Until that external gate is
run, the package's central availability claim is unproved.

### 3. The required scheduler oracle is missing

The specification requires an oracle driven only by `NextStep` that proves a
not-yet-valid document performs zero network refreshes and becomes current by
local reassessment (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:730-738`).

The test suite contains a direct assessment table checking the `NextStepKind`
at issue and expiry boundaries
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/contract_hostile_test.go:378-424`). It does not construct a
scheduler or drive the public refresh path from `NextStep`.

An exact scan finds no scheduler test outside the architecture inventory's
description of `NextStep` as a scheduler union
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/architecture_test.go:246-287`).

The direct helper matrix can remain green even if a future composition ignores
the union and refreshes at the wrong boundary. The required end-to-end policy
proof is absent.

### 4. Cross-process cache advance and crash proof are missing

The specification separately requires:

- a crash after attempt transition, followed by a real second process that
  observes the same back-pressure
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:701-707`);
- two-process refresh exclusion
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:711-715`); and
- real cross-process cache contention proving a lower generation cannot
  overwrite a higher one and distinct writers cannot lose an advance
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:751-753`).

The subprocess suite proves only the middle requirement: two processes contend
for one refresh attempt and exactly one reaches the network
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/process_hostile_test.go:25-117`).

The unreachable durability test closes and reopens a Store in the same test
process. It does not crash or launch a second process
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/network_store_hostile_test.go:606-652`).

No subprocess test exercises concurrent `Retain`, lower/higher generation
ordering, or crash-after-attempt restart. The implementation may be correct by
inheritance from Filestore, but the Status-owned transaction promises are not
proved as required.

### 5. Refresh and retention form an unproved two-operation protocol

`Store.Refresh` durably records the attempt and then returns the transport
result. It does not retain an authoritative Document
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store.go:96-115`).

`Store.Retain` is a separate public operation that re-verifies and advances the
cache (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store.go:73-94`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/store.go:268-303`).

That split permits this real sequence:

1. an initially empty Store durably records a refresh attempt;
2. the network returns a valid authoritative proof;
3. the process exits before the caller invokes `Retain`;
4. the next process has no cached document; and
5. the next refresh is locally throttled until the kind-specific boundary.

The archive does not specify a typed composition transaction for that sequence
or test it as a crash boundary. It may be acceptable politeness behavior, but
it cannot remain an implicit caller convention.

A 2026 design must choose one compiler-visible contract:

- `Refresh` owns verified retention before returning authoritative success; or
- a typed result or transaction explicitly represents an authoritative fact
  that is not yet retained, and the composition root proves crash and restart
  behavior.

A comment telling callers to call `Retain` is not sufficient.

### 6. The import unit is broader than the proof domain

Controlstate needs only Status Document, Verify, Verified, Advance,
`AdvanceRequest`, and `Revision`/`Revision2026V1`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/advance.go:385`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:controlstate/revisions.go:43`).
Importing archived `status` also pulls the package's Contextstate, Exchange, and
Filestore dependency closure because proof, transport, and persistence live in
one Go package
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:27-44`).

The package exposes 14 exported functions, 37 exported types, and four Store
methods, all pinned by one architecture inventory
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/architecture_test.go:230-311`).

That is a coherent implementation, but it is not the smallest DAG for a pure
aggregate verifier. A network-free verifier should not acquire storage and
transport dependencies merely because another composition wants refresh.

### 7. Core owns Status-private implementation facts

The archive says Core owns every shared Status token and invariant, which is
correct. It goes further: `core/status_constants.go` owns Status-only enum
tokens, notice limits, refresh intervals, cache filename prefix/suffix, and
every private wire-size formula
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/status_constants.go:5-95`).

The architecture test then rejects every Status-local constant outside
`enums.go` and requires every Status JSON field to appear in a Core-owned list
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/architecture_test.go:356-425`).

This turns package-private implementation details into global coupling. A
cache filename or private attempt-wire maximum belongs in the package that
owns that record unless another package semantically imports it.

For 2026:

- stable cross-package error identities, shared offering identity, shared JSON
  field names, and genuinely cross-package protocol facts belong in Core;
- Status signing/identity domains, revision tokens, state tokens, local bounds,
  refresh intervals, cache namespace, private wire fields, and AST inventories
  belong with their owning Status capability; and
- no second package may copy them.

Single source of truth does not mean putting every constant in Core. It means one
typed owner at the narrowest correct boundary.

### 8. Review status is internally inconsistent

The specification header says `Draft permanent contract`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:status/SPEC.md:1-4`). The package index calls both implementation and
specification reviewed (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:29`).

The missing scheduler, crash, cross-process cache, and OGS deployment proofs
show why those labels cannot both be treated as authoritative. Archive status
labels are not admission evidence; exact tests and deployment artifacts are.

### 9. No explicit no-alias/no-shim ratchet

The architecture test pins the exact current public inventory and catches many
additive wrappers indirectly, but it does not explicitly reject:

- exported type aliases;
- compatibility packages;
- forwarding methods with the same inventory shape;
- deprecated compatibility constructors; or
- permissive legacy decoders outside the scanned directory.

The 2026 testing protocol requires clean upgrades with no shims or aliases.
The rebuilt capability needs explicit repository-wide structural ratchets, not
only a package-local inventory.

### 10. No authority-side semantic proof

The archive proves client Issue using a local signing key and client Verify
using test trusted keys. It does not contain the production authority that:

- decides operational/degraded/maintenance/outage;
- guarantees monotonic generation;
- publishes foreground and background routes;
- rotates Status signing keys;
- makes CDN/cache behavior preserve validity and canonical bytes;
- keeps Status independent from lifecycle deployment; or
- prevents stale or forked publication across regions.

Primitive should define the shared proof contract, not pretend that a client
library proves the issuer. Admission needs an authority adapter and
deployment-level proof owned outside the pure domain package.

## Primitive 2026 ownership and DAG

### Candidate Status proof domain

The smallest reusable domain owns:

- `Revision`;
- `State`;
- `Freshness`;
- `Generation`;
- nominal derived `Identity`;
- bounded `Notice`;
- typed `Timeline`;
- immutable `Fact`;
- concrete signed `Document`;
- Status signing domain;
- `Verified`;
- `Issue` and `Verify`;
- `Assessment` and `NextStep`;
- `Advance` and its replay/advanced union; and
- Status-domain typed errors.

It depends only on:

- Core shared identities and stable error roots;
- Temporal;
- Attest; and
- the standard library.

It must not depend on Contextstate, Exchange, Filestore, HTTP, filesystem,
schedulers, renderers, or products.

This is the layer Controlstate may import if a real aggregate still needs
Status.

### Candidate Status transport capability

A transport layer owns:

- foreground and background refresh kinds;
- two distinct typed endpoints;
- one reviewed Exchange policy;
- request execution through Exchange;
- expected offering and revision binding;
- authoritative/source-unreachable result;
- closed unreachable reason; and
- bounded typed retry guidance.

It depends on Core, Contextstate, Exchange, Temporal, Attest trust capability,
and the Status proof domain.

It owns no cache path, durable attempt state, wall clock, scheduler, renderer,
or Callbudget reservation.

### Candidate Status cache and throttle capability

A persistence layer owns:

- one injective typed cache target;
- absent/present cached proof;
- foreground and background attempt slots;
- fixed compiler-owned refresh intervals;
- monotonic supplied-observation guard;
- exact next-eligible instant;
- Store lifecycle;
- durable check-and-record;
- verified retain and monotonic advance; and
- crash/process proof.

It depends on Core, Contextstate, Filestore, Temporal, and the Status proof
domain.

Whether transport execution is a Store method must be decided from the first
consumer transaction. If it remains combined, authoritative success and cache
retention must have one explicit typed state machine. The Store must not accept
an untyped callback or loose transport interface to simulate a split.

### Candidate composition root

The product composition root owns:

- concrete offering and trusted keys;
- independently provisioned endpoints;
- observation instants;
- scheduler execution from `NextStep`;
- foreground command versus background job binding;
- rendering beside local liveness/readiness and license state;
- reconciliation with any cached Controlstate document;
- process shutdown; and
- consumer-specific persistence migration.

The composition must prove that:

- local readiness can be degraded while signed Status is operational;
- signed Status can report outage while the local process is live;
- Status source unreachable never changes authoritative state;
- an expired proof never supplies current state;
- not-yet-valid causes local reassessment, not network traffic;
- corruption is never absence;
- throttling is never source unreachability; and
- a new independent Status proof cannot silently mutate an older signed
  Controlstate aggregate.

### Candidate authority and deployment owner

The authority adapter owns:

- authoritative state decision;
- generation transaction;
- Status signing-key custody and rotation;
- exact canonical publication;
- separate foreground/background serving policy;
- cache-control and CDN semantics;
- deployment topology;
- multi-region fork prevention; and
- independent-failure-domain integration proof.

That adapter may share the proof-domain types. It must not import the local
Filestore cache or client throttle.

### Core candidates

Core should retain only facts shared across package boundaries:

- `ErrStatusContract` and stable child identities that have real producers;
- offering identity;
- common HTTP and JSON primitives;
- common path and byte-count primitives; and
- JSON field names shared by multiple admitted protocols.

The Status owner should retain:

- Status revision and state tokens;
- Status signing and identity domains;
- Status-only labels;
- Notice and validity bounds;
- refresh intervals;
- cache filename projection;
- private retained-wire field names;
- response-size relationships; and
- exact implementation inventories.

Other packages import the Status types; they do not copy these facts into
parallel constants.

### Resulting DAG

Copying the archive produces:

```text
core
  -> temporal / attest / contextstate / exchange / filestore
  -> status
  -> controlstate
  -> future consumers
```

That forces every Status proof consumer through the transport and filesystem
closure.

A narrower consumer-driven DAG is:

```text
core -> temporal -> attest
core + temporal + attest -> status-proof
status-proof + contextstate + exchange -> status-transport
status-proof + contextstate + filestore -> status-cache
status-proof + status-transport + status-cache -> product composition root
```

If Controlstate is admitted, it imports `status-proof`, never transport or
cache. Product code joins proof, transport, cache, local health, and rendering.
The OGS authority is a separate downstream implementation of the same proof
contract.

Package names should be chosen during the consumer design; this report names
capabilities, not a pre-approved directory layout.

## Decision rationale and conditions

### Required implementation proof

Before any Primitive 2026 Status production implementation begins:

1. Name the first real consumer and the exact user operation requiring public
   signed service status.
2. Define the offering, trust-root installation, foreground path, background
   path, and renderer through typed composition structures.
3. Provide an OGS deployment test proving Status remains reachable while the
   primary lifecycle deployment is unavailable.
4. Make a same-process second hostname fail that deployment gate.
5. Preserve the closed authoritative State, Freshness, RefreshResult, and
   `NextStep` unions.
6. Preserve the rule that source unreachability never creates outage.
7. Preserve caller-supplied `temporal.Instant`; no package clock, callback,
   raw nanosecond, or `time.Time` escape hatch.
8. Prove scheduled maintenance and validity at one nanosecond before, exactly
   at, and after every boundary.
9. Build a scheduler oracle using only public `Assessment.NextStep`.
10. Prove not-yet-valid performs zero refreshes and becomes current only through
    local reassessment.
11. Decide and type the authoritative-refresh-to-retention transaction.
12. Prove crash after durable attempt and before network in a real second
    process.
13. Prove crash after authoritative response and before/after retention.
14. Prove two real processes admit exactly one refresh with no sleeps.
15. Prove two real cache writers cannot lose a higher generation or install a
    lower one.
16. Prove corrupt, forged, oversized, wrong-offering, and unreadable cache state
    is never absence.
17. Prove foreground and background attempt slots remain independent at exact
    interval boundaries.
18. Use Exchange as the sole HTTP executor and Filestore as the sole durable
    cache owner.
19. Keep every persisted and wire extent compiler-bounded and O(1) memory.
20. Keep stable error identity Core-owned and assert it with `errors.Is` and
    `errors.As`.
21. Move Status-private constants out of Core while preserving one typed owner.
22. Add repository-wide no-alias, no-wrapper, no-shim, and no-compatibility
    ratchets.
23. Run native Darwin, Linux, and Windows proof for the admitted cache and
    transport closure.
24. Migrate one real consumer and land Primitive, authority, and consumer
    pins atomically only after explicit user review.

Tests must follow `foundation@working-tree:_docs/testing_protocol.md:1-12`. Behavioral red proof
for the missing scheduler and crash/cache transactions comes before
implementation or structural ratchets.

### Current rationale

Do not copy the archived Status package wholesale into the initial Primitive
2026 scaffold.

Preserve as design evidence:

- authoritative service state separate from transport observation;
- signed product-neutral Fact and proof-carrying Verified;
- derived nominal identity and monotonic generation;
- exact Temporal timeline and caller-supplied observation;
- current/not-yet-valid/expired assessment;
- scheduled-to-active derivation;
- closed `NextStep` scheduling meaning;
- lower/equal-fork/equal-replay/higher Advance;
- typed authoritative/unreachable/throttled refresh outcomes;
- exact bounded retry guidance;
- durable per-kind attempt-before-network throttling;
- loud corrupt-state handling;
- one bounded cache record;
- real Exchange, Filestore, Attest, and subprocess proof; and
- stable typed Core error identity.

Reject or redesign before admission:

- the unproved independent deployment claim;
- zero real consumer imports;
- the monolithic proof/transport/cache import closure;
- the implicit Refresh-then-Retain protocol;
- the missing scheduler oracle;
- the missing crash/restart attempt proof;
- the missing cross-process cache advance proof;
- Status-private constants forced into Core;
- internally contradictory review status; and
- absence of explicit repository-wide no-shim ratchets.

Kernel, Witness, Bug, and Peachfuzz supply valuable constraints, not a reason
to merge their domains:

- Kernel proves local liveness and readiness must remain separate.
- Witness proves transport failure must not erase signed commercial standing.
- Bug proves closed result dispatch is valuable and corrupt state must not be
  reset to zero.
- Peachfuzz proves unavailability is not absence and consumer boundaries own
  operational severity.

The strongest 2026 direction is a small pure Status proof domain, admitted only
when one consumer names the need, followed by separately owned transport and
cache capabilities if that consumer actually requires them. Independent OGS
deployment proof is mandatory before the feature can claim the resilience its
name promises.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
