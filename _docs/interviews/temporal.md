# Temporal package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This document is comparative evidence for the product-neutral capability that
may become Primitive `temporal`. It is not a package specification, an
admission decision, or permission to copy an archived implementation.

The interview order is:

1. Kernel;
2. Witness;
3. Bug;
4. Peachfuzz;
5. archived Primitive `temporal` and `timeproof`;
6. recon synthesis evidence for the later design phase.

Search counts are navigation facts only. Conclusions below are backed by
located production, contract, or test paths.

## Evidence boundary


- Archive: `d046f7b675fcb797398d7cdc87b5504f43978056`.
- Kernel HEAD: `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`;
  committed pin `0df2954a2d91`, dirty pin `e8b7172161a4`.
- Witness HEAD: `b9629af57b7058b68982be5d3b282be440b1e76e`;
  pin `773add8ba0fc`.
- Bug HEAD: `39ce96242240d7174d562c90bb255860946595dc`;
  pin `388e593231a2`.
- Peachfuzz HEAD: `2b2d080c455edaadf88502c1c253845605a4336a`;
  pin `3f74d8fc35b4`.

Kernel's dirty pin and materially dirty tree are separated from committed HEAD
evidence below. Witness and Bug carry older time designs. Peachfuzz consumes an
older Primitive pin. Search counts are navigation only; located paths are the
evidence.

## Capability ownership

Temporal owns exact instants, bounded durations, wide aggregate duration,
wall plus monotonic observation, intervals, checked arithmetic, ordering,
canonical machine projections, and bounded deadline, wait, and ticker
mechanics. It does not own consumer TTL values, retry eligibility, transport
attempt policy, signed authority, durable high water, lease progression,
display policy, or deadline blame.

## Archive evidence

The July 27 archive contains the strongest coherent primitive implementation,
including private validated values, checked arithmetic, persistence projection,
wall plus monotonic observation, and bounded wait and ticker mechanics. Its
detailed implementation evidence, defects, and missing proof follow after the
consumer comparison so the strongest consumer requirements can be evaluated
against the archive rather than inferred from it.

## Consumer evidence

### Kernel

#### Source boundary

- Committed Kernel HEAD:
  `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`.
- HEAD Primitive pin:
  `v2026.0.1-0.20260723012501-0df2954a2d91`, resolving to
  `0df2954a2d911a5d7d775691d023d569affa2c20`
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76`).
- Dirty working-tree Primitive pin:
  `v2026.0.1-0.20260727043347-e8b7172161a4`, resolving to
  `e8b7172161a4994efcb7f092113e23c28928da43`
  (`kernel@working-tree:go.mod:76`).
- Kernel has a materially dirty tree. Its staged Peachfuzz-related deletions,
  unstaged changes, and untracked governance files are not committed-head
  evidence.
- HEAD contains 152 production Go files and 305 Go test files that directly
  import `time`. These counts describe scale; the behavioral inventory below
  is the evidence.
- No committed Kernel code imports Primitive `temporal` or `timeproof`.
  Committed Kernel consumes time-related Primitive contracts through
  `foundation/core` and `foundation/shutdown`.
- The dirty Primitive pin adds the newer temporal stack while removing
  `foundationcore.UnixNanoTime`. Kernel has not migrated its call sites, and
  the dirty pin does not currently compile Kernel Core.

#### Existing declared boundary

Kernel's architecture already says Core is pure, never calls `time.Now`, and
receives observed time from callers
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:_docs/architecture/architecture.md:1442`,
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:_docs/architecture/architecture.md:4531`). Its governance also
calls for one UTC wall-clock source, monotonic elapsed measurement, and one
shared deadline budget
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:_docs/governance/blink_architecture.md:790`).

The implementation has not completed that transition. At HEAD, production
contains 127 direct `time.Now()` calls across 55 files and 42 raw timeout,
timer, or ticker sites. Clock injection exists in several packages, but it is
implemented repeatedly as unrelated `func() time.Time` fields, setters, and
fallbacks. Production contains no direct `time.Sleep`; cancellation-aware
timers are already the stronger local convention.

#### Located production use cases

| Concern | Kernel evidence | Observed behavior and boundary lesson |
| --- | --- | --- |
| Wire and persistence instants | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/time_contracts.go:16`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/sql/helpers.go:26`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/helpers.go:69` | Kernel wraps `foundationcore.UnixNanoTime`, requires present audit/creation/expiry values to be post-epoch, and uses integer zero as historical absence. The new capability needs explicit typed presence and must not preserve zero-as-unset. |
| Domain timestamps | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/user/model.go:384`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/order_contracts.go:376`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/outbox/outbox.go:803`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/ledger/event.go:19` | User, order, outbox, ledger, cursor, waiver, lock, and projection facts mix raw `time.Time`, raw integers, and Primitive values. Domain meaning remains Kernel-owned; representation, validation, and arithmetic are Primitive mechanics. |
| Clock acquisition | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/token.go:20`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/gate.go:88`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/starter/service.go:73`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:nonce/nonce.go:89`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:webauthn/webauthn.go:149`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:identity/identity.go:22`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:breaker/breaker.go:173`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/sql/store.go:29`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/store.go:73` | Many packages independently own optional callbacks and silent `time.Now` fallbacks. A compiler-owned observation capability is missing. One operation should capture one validated observation at its boundary. |
| Authentication expiry | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/duration.go:7`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/auth_flow_contracts.go:5`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/token.go:182`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/cookies.go:130`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/starter/token.go:168` | Kernel owns access, refresh, pre-auth, WebAuthn, action-code, lockout, refresh-window, and skew policy. JWT issuance can derive `iat`, `exp`, and cookie age from one supplied instant. Starter tokens instead read the wall clock and retain a seconds-versus-nanoseconds compatibility heuristic that must be retired. |
| Nonce and TOTP windows | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:nonce/nonce.go:509`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/nonce_contracts.go:96`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:professor/totp.go:30` | Nonces embed Unix nanoseconds with a 24-hour TTL and 60-second future-skew policy. TOTP is more coherent: a caller supplies the instant and Kernel applies its RFC 6238 window policy. |
| WebAuthn expiry and grace | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:webauthn/session.go:17`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:webauthn/webauthn.go:370`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/webauthn_contracts.go:13` | WebAuthn uses a required caller clock, five-minute session TTL, ten-second replay grace, nanosecond persistence, and single-use enforcement. The TTL values remain Kernel policy; checked instant arithmetic and explicit equality semantics are generic. |
| Refresh-family transitions | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/family/advance.go:98`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/family/advance.go:229`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/family/model.go:66` | `Advance` is a pure structure-to-structure decision receiving `Now` and `MinRefreshInterval`. `LastRefreshUnix` actually stores nanoseconds, and rollback is treated as not rate-limited. The pure input model is valuable; the loose unit and rollback decision are not. |
| Monotonic identifiers and state | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:identity/identity.go:75`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:payapi/payapi.go:415` | ULID milliseconds clamp to the prior generated millisecond and advance after entropy exhaustion. Order transitions use `previous + 1ns` when wall time does not advance. These are valuable hostile-clock mechanics that need checked arithmetic and explicit ownership. |
| Canonical ledger precision | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/ledger/event.go:464`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/ledger/chain.go:136` | Ledger timestamps are required, normalized to UTC, and truncated to Firestore microseconds before hashing and persistence. Canonical hashing repeats the rule. Primitive should own one typed precision projection so two implementations cannot drift. |
| Outbox, leases, and caches | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/outbox/outbox.go:803`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/outbox/outbox.go:857`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/outbox.go:395`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/sql/outbox.go:84`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/ledger.go:51` | Outbox timing includes creation, sent, retry, bounded exponential backoff, jitter, a 60-second claim lease, and expired-lease reclamation. Firestore also has 30-second and 15-minute visibility caches. Cadence values and notification policy remain Kernel-owned; safe arithmetic, expiry comparison, and timer mechanics are generic. |
| HTTP retry and total budgets | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:bridge/client.go:45`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:bridge/client.go:136`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:bridge/client.go:184`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:bridge/client.go:436`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:bridge/client.go:474` | Requests combine per-attempt timeout, parent deadline, total retry budget, backoff, jitter, `Retry-After`, and cancellation-aware sleep. `Timeout * (MaxRetries+1)` is unchecked. The mechanism belongs in Primitive `exchange` over typed temporal contracts; Kernel chooses values and retry eligibility. |
| Breakers, rate limits, and dedup | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:breaker/breaker.go:82`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:breaker/breaker.go:214`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:wall/bastion.go:192`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:wall/bastion.go:365`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:telemetry/siren/siren.go:325` | Rolling windows, cooldowns, half-open probes, refill buckets, cleanup cadence, dedup windows, and retry delays repeatedly rebuild similar temporal mechanics. Primitive should provide typed windows, observations, waits, and checked arithmetic, not Kernel's thresholds or actions. |
| Server and route deadlines | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/server_contracts.go:61`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/route_budget.go:12`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:server/server.go:72`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:server/server.go:261`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:switchboard/switchboard.go:516`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:wall/wall.go:126` | Kernel owns server timeout values and route classes. Route deadlines are compiler-generated from typed intents, but `RouteBudget` is duplicated between Core and Wall. Primitive should own deadline derivation and remaining-budget mechanics; Kernel owns the route policy. |
| Background timers and shutdown | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/monitor.go:355`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:ceremony/boot.go:293`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:appboot/telemetry.go:852`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/runner.go:212` | Recovery, animation, telemetry dumps, limiter cleanup, retry, graceful shutdown, and process reaping use timers and tickers. New mechanics must make cancellation, stop/drain ownership, and detached-but-bounded work explicit. |
| Elapsed measurement | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:appboot/ceremony.go:15`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:telemetry/horizon/middleware.go:102`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/build.go:871`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:exodus/migrate.go:572`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:scout/scout.go:441` | Boot, HTTP, build, migration, and scan durations use `time.Now`/`time.Since`. Go's monotonic reading survives only while the original `time.Time` stays in memory. Wall-clock instants and monotonic elapsed observations must be distinct contracts. |
| Parsing, formatting, and zones | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:compass/compass.go:748`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:bridge/client.go:474`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:appboot/ceremony.go:43`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/compile/assets.go:202`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/deploy.go:596`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:html/error500.go:259`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:prettyprint/panel.go:512` | Environment durations, HTTP dates, RFC3339 values, filenames, deployment stamps, reports, and localized display are decentralized. Wire and persistence formats can be compiler-owned Primitive contracts; product display-zone choices remain downstream. |
| Scout cache and locks | `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:scout/scout.go:273`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:scout/ledger.go:26`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:scout/ledger.go:171`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:scout/waiver.go:15` | Scout combines typed timestamps, 24-hour cache TTL, deadline-stamped lock files, stage/total timeouts, duration strings, and expiring waivers. Its freshness check accepts future timestamps because negative age remains below the TTL. |

#### Strong mechanics worth preserving

Kernel supplies evidence for these product-neutral capabilities:

- a validated, private-representation `Instant` with explicit optional
  semantics, numeric persistence projections, UTC normalization, precision
  conversion, total comparison, and checked arithmetic;
- a non-negative typed `Duration`, plus validated request types for TTL,
  timeout, interval, grace, and total budget where those distinctions affect
  behavior;
- one compiler-owned observation contract that separates wall projection from
  monotonic elapsed measurement;
- system, fixed, and concurrency-safe manual observations capable of forward,
  frozen, and backward wall movement;
- context-aware waits, timers, and tickers with explicit stop and cancellation
  ownership;
- checked duration multiplication and instant-plus-duration operations with
  stable typed overflow identity;
- explicit expiry rules that make equality, grace, rollback, and future-skew
  behavior visible in the request type;
- typed deadline selection that respects the earlier of parent deadline and
  local budget;
- typed precision projections, especially exact nanoseconds and canonical
  microsecond truncation;
- bounded exponential backoff and jitter mechanics with deterministic test
  inputs;
- caller-driven pure temporal decisions, as demonstrated by TOTP, WebAuthn,
  and refresh-family transitions;
- monotonic-under-rollback mechanics for identifier and ordered-event
  generation; and
- virtual-time test support built on standard `testing/synctest`, without a
  custom assertion framework.

#### Hostile proof and failure lessons

Kernel tests demonstrate useful proof patterns:

- negative/unset/optional instant behavior:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/time_contracts_test.go:11`;
- clock reversal, nanosecond-unit drift, and precision loss:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/family/regression_test.go:11`;
- frozen and backward clocks over 5,000 ULIDs:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:identity/clock_regression_hostile_test.go:13`;
- expiry equality across point and list APIs:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/store_test.go:1506`;
- captured `Retry-After` without real sleep:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:bridge/client_test.go:282`;
- virtual-time concurrent delivery and fresh timestamps:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:service/notify/regression_test.go:17`,
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:service/relay/regression_test.go:41`;
- exact TTL, grace, cooldown, and timeout ordering boundaries:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:webauthn/adversarial_test.go:165`,
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:telemetry/siren/hostile_test.go:78`,
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:server/server_test.go:601`;
- skew-window rejection:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:professor/hostile_test.go:679`; and
- timing as hostile fuzz input:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/family/fuzz_test.go:10`,
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/outbox/fuzz_test.go:12`,
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:server/fuzz_test.go:72`.

The following tests are warnings, not reusable proof:

- `TestBug338_RefreshRateLimitEviction` never enters its polling loop because
  `count` begins at zero while the condition is `count > 10`
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/gate_test.go:2989`);
- the ceremony panic-drain test observes an unrelated goroutine rather than
  the production drain
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:ceremony/boot_test.go:360`);
- parallel ceremony tests mutate one package global for less than the full test
  lifetime
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:ceremony/boot_test.go:410`);
- concurrency tests still use scheduling sleeps
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/regression_test.go:278`,
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:service/policy/regression_test.go:84`);
- `server.waitForAddr` busy-spins
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:server/server_test.go:322`);
- the bridge budget proof depends on real elapsed time and 100 milliseconds of
  slack (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:bridge/client_test.go:963`); and
- multiple tests grep source text instead of proving behavior, including
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/gate_test.go:2741`,
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:service/notify/notify_test.go:1389`,
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/sql/admin_test.go:436`, and
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:scout/scout_test.go:583`.

Overflow remains materially under-proved. Kernel does not prove overflow of
retry-budget multiplication or nanosecond expiry-plus-grace arithmetic.

Kernel also contains compiler-visible but duplicated timing truths:

- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/duration.go:22` and `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/server_contracts.go:62` independently define
  identical website server timeout values;
- SQL and Firestore independently implement the same Unix-nanosecond
  conversion helpers;
- store packages independently define `SetClock` and `time.Now().UTC`
  defaults; and
- `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/compass_contracts.go:66` expresses duration bounds as raw nanosecond
  integers beside a composed `time.Second` default.

These are migration defects to remove, not patterns to reproduce in
Primitive.

#### Kernel-owned policy

The following facts must remain in Kernel Core or the owning Kernel package:

- token, session, code, lockout, and refresh TTL values;
- route classes and route-budget values;
- server read, write, idle, and shutdown timeout values;
- notification retry cadence and retry eligibility;
- outbox claim duration and cache-retention choices;
- breaker, limiter, cooldown, deduplication, and spend-window thresholds;
- product actions resulting from an active, grace, expired, or locked state;
- display timezone and human-formatting policy; and
- domain state transitions that consume typed temporal facts.

Primitive may own their typed representation, validation, safe arithmetic,
observation, deadline, waiting, precision, and persistence mechanics. It must
not own Kernel's values or decisions.

#### Dirty-tree-only evidence

The dirty Kernel tree deletes
`static/js/shared/kernel_peachfuzz_liveproof.js` and its test. At committed
HEAD, that feature has an 8-second request deadline, 200-millisecond delayed
loading state, 400-millisecond minimum display duration, relative-time
rendering, Unix-nanosecond-to-millisecond conversion, and injected
scheduler/cancel/now functions
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:static/js/shared/kernel_peachfuzz_liveproof.js:7`,
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:static/js/shared/kernel_peachfuzz_liveproof.js:145`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:static/js/shared/kernel_peachfuzz_liveproof.js:187`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:static/js/shared/kernel_peachfuzz_liveproof.js:239`). These remain historical consumer requirements even
though the working tree removes the feature.

#### Kernel conclusion

Kernel strongly supports admitting a product-neutral temporal capability, but
not by copying its local wrappers. The new capability should replace raw clock
callbacks, zero-as-absence integers, unchecked arithmetic, repeated expiry
comparisons, and package-local timer conventions. Kernel's timing values and
domain decisions remain downstream.

No final package contract is inferred until Witness, Bug, Peachfuzz, and the
archived Primitive implementations have been interviewed.

### Witness

#### Source boundary and age

- Committed Witness HEAD:
  `b9629af57b7058b68982be5d3b282be440b1e76e`.
- The only working-tree item is untracked `.ledger_pending.md`; there are no
  tracked source or module deltas.
- Committed and live Primitive pin:
  `v2026.0.1-0.20260722113053-773add8ba0fc`, resolving to
  `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9`
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17`, `witness@working-tree:vendor/modules.txt:26`).
- The active pin predates the July 27 archive. It contains the old
  `core.UnixNanoTime`, `core.NanosecondsDuration`, and `core.BackoffPolicy`,
  but not the later `temporal`, `exchange`, `currency`, or `garble` packages.
- `protocol/_foundation_source/core` is a second, still older archaeological
  snapshot based on `8c6328da729bd17ee743c1be28f6c575655749af`.
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/FOUNDATION_EXTRACTION.md:9` explicitly describes it as preserved
  extraction evidence rather than a final API.
- Tracked Witness source already imports newer missing Primitive packages.
  Consequently `protocol/custody`, `protocol/license`, and
  `protocol/release` do not currently build in vendor mode. This is a
  documented incomplete migration boundary, not evidence that the old pin is
  desirable (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/FOUNDATION_EXTRACTION.md:47`).

Witness is therefore consumer archaeology only. A mechanic qualifies as a gem
only if it represents a real requirement not already superseded by the July 27
archive or a stronger consumer implementation.

#### Existing declared boundary

Witness doctrine says shells observe hostile wall time and Core receives time
as data (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:_docs/governance/go_doctrine.md:1307`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:_docs/governance/go_doctrine.md:2009`). It distinguishes monotonic elapsed measurement from wall time
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:_docs/governance/go_doctrine.md:2170`), requires child deadlines to remain within the parent budget
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:_docs/governance/go_doctrine.md:2965`), and requires bounded retry time, idempotency, capped backoff, and
jitter (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:_docs/governance/go_doctrine.md:4670`).

Witness execution policy is intentionally downstream. It owns deadline blame,
retry disposition, profile budgets, ledger ordering, custody bindings,
retention, subscription policy, and human projections. Primitive should own
the product-neutral values and mechanisms those policies consume.

#### Located production use cases

| Concern | Witness evidence | Observed behavior and boundary lesson |
| --- | --- | --- |
| Local sealed time scalars | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/types.go:108`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/types.go:143` | `EpochNs` and `DurationNs` seal record fields, but duplicate old Primitive scalars. Zero means epoch, absence, or a real zero duration depending on the record, and reversed instants silently become zero duration. Witness may retain schema-specific `_ns` projections, but computation needs one Primitive owner and explicit presence. |
| Sealed duration representations | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/types.go:1637`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/types.go:5790`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/verifier_budget.go:15` | Sealed records mix `DurationNs`, raw `int64` fields ending in `Ns`, raw `time.Duration`, and a duration-valued `DeadlineNs` name that resembles an instant. Compiler-visible duration ownership is incomplete even before considering the old Primitive wrapper. |
| Sealed timing invariants | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/types.go:1031`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/run_record.go:97`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/budget_deadline.go:12` | Execution records require duration to equal finished minus started and centrally derive budget overrun/deadline evidence. The invariant is valuable; the duplicate scalar types are not. |
| Monotonic execution measurement | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/run/run.go:817`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/run/run.go:8629` | The default run path preserves Go monotonic elapsed time, then normalizes finish precision before sealing. Injected-clock and later execution paths still subtract independent wall instants. New contracts must keep observation/elapsed mechanics distinct from serializable instants. |
| Deadline ownership and blame | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/failure_ownership.go:69`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/types.go:5785`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/validate.go:1426` | Witness seals which actor owned a deadline, the fired deadline, overrun, and retry disposition. These meanings remain Witness-owned; typed deadline derivation and waiting remain Primitive mechanics. |
| Budget derivation | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/run_record.go:2357`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/profile/profile.go:85`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/plan/plan.go:160`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/verifier_budget.go:9` | Profiles derive effective budgets and stream verifier evidence under byte/time ceilings. Numeric values, retry permission, and blame remain Witness policy. `VerifierBudget` still uses raw `time.Duration` fields with `_ns` JSON names. |
| Signed server-time commitment | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/checkin_time_commitment.go:15`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/store.go:233`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:190`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/gate.go:160` | Server time is signed and bound to product schema, device, lease, generation, and request nonce. Durable state rejects regression/conflicting equal-time commitments and uses the maximum trusted floor rather than raw wall time. This is Witness's strongest gem, likely belonging across `timeproof` and product-neutral lease contracts rather than raw `temporal`. |
| Lease and calendar boundaries | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/lease.go:483`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/offer.go:56`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/gate.go:46` | Lease evaluation distinguishes warning, write-grace, and expiry. Calendar-month addition clamps month ends and leap years. Product plans and durations remain Witness policy; signed timeline ordering and calendar-period arithmetic may be generic. |
| RFC 3161 verification | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:225`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:399`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:526`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:716`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:1342`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:2028` | The mature notary implementation injects clock/sleeper, creates fresh nonce and verification time per request, races authorities while deterministically ordering evidence, handles retry/backoff/`Retry-After`, verifies freshness/revocation, and validates chains at TSA `genTime`. These mechanics belong in one Primitive `timeproof` capability if the newer archive lacks them. |
| RFC 3161 total budget | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/run/run.go:8493`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/timestamp_client.go:16`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:324` | One timestamp operation stacks an unnamed 20-second outer context over independent 15-second client/attempt limits and retry behavior. This is a concrete budget-divergence defect; one typed total budget must bound all attempts without copied literals. |
| Weaker custody timestamp path | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/timestamp_client.go:43`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/timestamp_client.go:79`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/timestamp_client.go:143`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/timestamp.go:191` | A second client stamps `TimestampedAt` from local `Now` and verifies only structural token/response correspondence. It does not prove imprint, `genTime`, signer chain, nonce, or policy. It must be retired, not promoted. Authoritative proof time and local observation must be compiler-distinct. |
| Retry and waiting | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:716`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:769`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:869`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/client.go:159` | Witness has local retry/backoff/jitter beside old Primitive `BackoffPolicy`, while extracted clients partially move to `exchange`. Direct newer `temporal` use merely converts raw budgets for attempt policies and later converts retry delays back to `time.Duration`. The mechanism needs one Primitive owner and typed values must survive package crossings. |
| Host-suspend detection | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/host_suspend.go:5`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/host_suspend_monitor.go:11` | The pure detector is an O(1) state machine and the monitor owns one prior sample and one ticker. It currently strips monotonic data and uses epoch subtraction, so a forward wall correction can look like suspension. Preserve the use case, not the implementation. |
| Signals and bounded cleanup | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/signals.go:15`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/signals.go:343` | Graceful cancel, forced cleanup, and subprocess flush use distinct budgets. Primitive should provide owned timer/deadline mechanics; Witness owns the phases and their values. |
| Viewer and operational cadence | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/view/view.go:215`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/view/view.go:1776`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/memory_guard.go:65` | Viewer and memory guard own tickers, ETA, and polling. Cadence is Witness policy; ticker lifecycle and deterministic scheduler control are generic. |
| Ledger ordering | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/ledger.go:249`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledger/replay.go:692`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:docs/ledger.md:120` | Ledger replay rejects timestamp regression, allows equality in defined cases, and keeps exact/text projections coherent. Terminal records may clamp to prior authority. Ordering policy remains Witness-owned; typed rollback/ordering inputs can be generic. |
| Human time projection | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/artifact/human.go:3592` | Formatting occurs downstream from sealed Core values. Primitive should own canonical machine projections, not Witness wording or display choices. |

#### Gems that survive the age qualification

The following are requirements to compare against the newer archive, not old
APIs to copy:

1. **Signed trusted-time commitments.** Bind an authoritative time fact to the
   subject identity, progression/generation, request nonce, signer, and exact
   signed bytes; persist a high-water floor and reject regression or
   conflicting equal-time commitments.
2. **Compiler-visible rollback policy.** Different domains need distinct
   choices: reject regression, retain high-water, clamp elapsed duration,
   allow equality, or clamp a terminal record. One implicit comparison rule is
   insufficient.
3. **Wall plus monotonic sealed measurement.** Produce an absolute start,
   monotonic elapsed duration, checked absolute finish, and explicit precision
   projection without reconstructing elapsed time from serialized endpoints.
4. **Strict clock scripts.** Tests need call-counted clocks that fail when
   exhausted. Witness's forgiving sequences repeat or invent values and can
   hide added production clock reads.
5. **Notary-grade RFC 3161 verification.** Verify imprint, nonce, CMS
   signature, chain, revocation freshness, policy, and `genTime`, with bounded
   parallel authority attempts and deterministic evidence order.
6. **Calendar periods distinct from fixed durations.** Month-end clamping,
   leap years, and year rollover are a different domain from nanosecond
   duration arithmetic.
7. **Timer and cancellation doctrine.** Witness statically rejects production
   sleeps, unowned `time.After`/`time.Tick`, unstopped timers/tickers, and lost
   cancellation. A future ratchet must resolve import identity rather than
   depend on the local spelling `time`.
8. **Host-suspend observation.** Preserve the bounded O(1) use case, but base
   it on an observation contract that can distinguish elapsed monotonic time
   from hostile wall movement.

#### Hostile proof and failure lessons

Strong historical proofs include:

- hostile numeric wire decoding, non-mutation after rejection, and checked
  instant arithmetic:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/_foundation_source/core/core_hostile_test.go:15`;
- invalid lease ordering, one-nanosecond edges, negative grace, and
  `MaxInt64` overflow:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/lease_test.go:226`;
- leap-year and month-end calendar behavior:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/offer_test.go:256`;
- signed-time substitution, regression, foreign identity, and durable
  high-water selection:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/time_commitment_test.go:25`;
- ledger regression/equality and state preservation:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledger/ledger_test.go:14785`,
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledger/ledger_test.go:17725`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledger/ledger_test.go:17891`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledger/ledger_test.go:18118`;
- precision normalization:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/run/run_test.go:13178`;
- benchmark timing overflow:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/exec_test.go:4044`; and
- exact injected backoff schedules without sleeping:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:3862`.

Weak or misleading proofs include:

- scripted clocks that repeat the last value or invent later values after
  exhaustion:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledger/ledger_test.go:13431`,
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/run/run_test.go:15606`,
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:4798`;
- a tautological duration test that assigns its expected subtraction before
  asserting it:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/core_test.go:21680`;
- ledger tests that accept a negative first timestamp despite
  `EpochNs.Validate` rejecting negative values:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledger/ledger_test.go:18443`;
- synchronization sleeps:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/cli/cli_test.go:710`,
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:3791`;
- real-ticker viewer lifecycle proof with no `testing/synctest`:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/view/view_test.go:3084`; and
- doctrine tests that acknowledge alias and closure bypasses for `Sleep`,
  `After`, and unstopped timers:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/doctrine_extended_test.go:953`,
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/doctrine_extended_test.go:1118`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/doctrine_extended_test.go:1881`.

Targeted temporal fuzzing is missing for checked arithmetic, lease-window
ordering, rollback state machines, calendar transitions, and clock-read
ordering.

#### Archaeological debt to retire

- `protocol/_foundation_source/core` is a dormant second Primitive source of
  truth and must not survive consumer cutover.
- Active `protocol/license` still contains byte-copied Primitive code and both
  Bug and Witness product schemas.
- Witness has three temporal type systems: local `EpochNs`/`DurationNs`, old
  Primitive Core wrappers, and raw standard-library values. Its sealed records
  further split duration facts among `DurationNs`, raw `int64`, raw
  `time.Duration`, and ambiguous `DeadlineNs` naming.
- The mature notary and weak custody timestamp path duplicate RFC 3161
  responsibility with different trust semantics.
- Generic retry/backoff exists in old Primitive, local notary, and extracted
  protocol clients.
- `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/ledger/replay.go:1641` silently disables a verifier budget when
  validation fails; execution configuration must fail closed at ingress.
- `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:606` uses `time.AfterFunc`, which the local timer
  doctrine does not reject.
- Protocol-relevant notary failures remain package-local prose identities
  rather than a shared typed `timeproof` error contract.

#### Witness-owned policy

Witness retains:

- canonical evidence field names and record schemas;
- deadline owner, overrun blame, and retry disposition;
- benchmark resampling and execution profile budgets;
- ledger acceptance/equality/terminal ordering decisions;
- receipt and custody binding to Witness artifacts;
- subscription plans, check-in cadence, retention, machine limits, grace
  durations, and product actions;
- TSA provider selection and operational budgets where they are deployment
  choices; and
- human formatting and product wording.

Primitive may own temporal values/observation, checked arithmetic, precision
projections, deadlines/waits/tickers, generic exchange retry, authoritative
timeproof verification, and product-neutral signed timeline evaluation.

#### Witness conclusion

The old Witness temporal integration itself is not a gem. Its direct
`temporal` use is shallow and its migration is incomplete. The standout
consumer requirement is the signed server-time commitment plus durable
anti-rollback floor. Additional worthwhile requirements are sealed
wall/monotonic measurement, strict rollback modes, notary-grade RFC 3161
verification, calendar-period arithmetic, bounded timer doctrine, and
host-suspend observation.

Each must be compared with the July 27 archived implementation before it enters
a new Primitive contract.

### Bug

#### Source boundary and age

- Committed Bug HEAD:
  `39ce96242240d7174d562c90bb255860946595dc`.
- The only dirty item is untracked `.ledger_pending.md`.
- Committed and live Primitive pin:
  `v2026.0.1-0.20260720145921-388e593231a2`, resolving to
  `388e593231a28434f6faae9f0ab9dffcf332dfc3`
  (`bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9`).
- The July 20 live pin predates Primitive `temporal`. Bug runtime code consumes
  old vendored `core`, `license`, and `release`.
- `protocol/` is a July 24 preservation extraction based on
  `8c6328da729bd17ee743c1be28f6c575655749af`. It imports newer packages absent
  from vendor and is not imported by live Bug code
  (`bug@39ce96242240d7174d562c90bb255860946595dc:protocol/FOUNDATION_EXTRACTION.md:3`, `bug@39ce96242240d7174d562c90bb255860946595dc:protocol/FOUNDATION_EXTRACTION.md:45`).
- Bug's preserved protocol packages do not build at the live pin. They are
  archaeological evidence only.

#### Distinct production requirements

| Concern | Bug evidence | Boundary lesson |
| --- | --- | --- |
| Certified operation time | `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/store.go:611`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:301` | Bug selects one rollback-resistant durable floor from observed, device, state, and signed-server facts, then verifies the writer-certificate interval at exactly that instant. Temporal supplies validated instants/comparison; provenance belongs `timeproof`/lease; Bug owns the operation-certification rule. |
| Bounded future-floor repair | `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/device.go:45`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:209` | Device high-water is capped at lease write-grace so a poisoned far-future local observation cannot brick later valid leases. Concurrent acceptance preserves usage and takes maximum trusted progression. This is a valuable higher-level rollback rule, not a primitive clock API. |
| Signed writer time | `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/writer_attestation.go:44`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/writer_proof.go:74` | Attestations bind occurrence time to repository, operation digest, writer, device, and certificate window. Signing domains and proof policy remain Bug-owned. |
| Server-time progression | `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/store.go:394` | Older commitments and different commitments at one server instant are rejected; exact replay is idempotent. The generic lesson is typed provenance plus durable progression. |
| Timestamp ingress/output | `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/time.go:11`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/core/date_contracts.go:3` | Bug preserves nanoseconds and emits UTC RFC3339Nano, but accepts several legacy grammars and loses the instant type through strings and integers. Legacy parsing is compatibility debt; Bug record field meaning remains downstream. |
| Release timing | `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/gate.go:42`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/facts.go:184` | Gate clocks are injected and commit provenance time comes from Git rather than the build host. Gate start/finish use independent wall samples, so rollback can invalidate the interval. Temporal needs one observation/elapsed mechanism. |
| Lock waiting | `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/lock.go:171`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/git.go:274` | Two local loops duplicate manual wall deadlines and owned timers. Exact cadence and reclamation policy remain Bug-owned; waiting/deadline mechanics are generic. |
| Stale Git lock | `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/git.go:312` | Reclamation requires minimum age, file-use proof, process inspection, and exact modtime recheck. This is stronger than age-only deletion. Temporal owns typed age; filestore/process own observations; Bug owns Git policy. |
| Detached bounded cleanup | `bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/behavioral_binding.go:362`, `bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:687` | Cleanup may detach from a canceled parent but receives a fresh finite timeout. The operation and values remain Bug policy. |

#### Bug-specific gems

- Cap durable high-water at a signed validity ceiling so a future clock poison
  can be repaired by a later valid grant.
- Select certified operation time from one durable trusted floor and verify the
  subject certificate at that same instant.
- Merge concurrent usage and clock progression without allowing either fact to
  be lost.
- Treat stale-lock reclamation as a multi-proof decision and recheck the exact
  object before deletion.
- Derive release creation time from source provenance rather than ambient build
  time.

The first three are adjacent to `timeproof`, `lease`, or consumer state. They
must not inflate primitive `temporal`.

#### Strong and weak proof

Strong Bug evidence includes:

- rollback-stretched expiry, missing authority, authority rotation, durable
  device high-water, and revocation cutoff edges:
  `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/gate_test.go:104`;
- concurrent usage/high-water merge and commitment fork rejection:
  `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/store_merge_test.go:70`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/store_merge_test.go:187`;
- poisoned future-clock repair:
  `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/usage_test.go:112`;
- exact retry and signed check-in boundaries at one nanosecond:
  `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/usage_test.go:124`;
- canonical signed-time bytes and cross-product domain rejection:
  `bug@39ce96242240d7174d562c90bb255860946595dc:protocol/license/checkin_time_commitment_test.go:12`;
- checked arithmetic and retry-policy extremes in the preserved source:
  `bug@39ce96242240d7174d562c90bb255860946595dc:protocol/_foundation_source/core/core_hostile_test.go:15`, `bug@39ce96242240d7174d562c90bb255860946595dc:protocol/_foundation_source/core/core_hostile_test.go:767`;
- exact UTC nanosecond mutation output:
  `bug@39ce96242240d7174d562c90bb255860946595dc:cli/certified_mutation_test.go:11`; and
- release gate rejection when the injected clock regresses by one nanosecond:
  `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/gate_test.go:140`.

Weak proof and missing coverage:

- release's scripted clock produces unlimited later values and cannot detect
  extra reads (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/gate_test.go:187`);
- Git staleness tests prove only obviously ancient files rather than
  one-nanosecond threshold edges (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/git_test.go:37`);
- several tests use real timers as scheduling guesses despite containing no
  literal `time.Sleep`;
- legacy timestamp parser proof omits hostile offsets, whitespace, invalid
  dates, precision, and range boundaries;
- negative nonzero timestamp integers can reach pre-epoch formatting without a
  hostile rejection proof; and
- there is no temporal fuzz target or `testing/synctest` use.

#### Debt and policy boundary

Bug must retain commercial lease windows, plan terms, check-in cadence,
command-usage windows, writer proof domains, record field meanings, human
rounding, lock policy, and operation budgets.

Debt to retire includes:

- unchecked `UnixNanoTime.Add` beside a checked helper;
- `CloseWindow` fabricating `start + 1ns` after equality or rollback;
- direct `time.Now` inside old `Retry-After` parsing;
- strings and raw integers between certified time, persistence, workflow, and
  output;
- duplicated timeout layers and manual contention loops; and
- copied combined Bug/Witness protocol packages.

Bug confirms Witness's trusted-time requirement and adds two distinct lessons:
cap future-floor poisoning at an authenticated ceiling, and carry the selected
certified instant through the complete signed operation boundary.

### Peachfuzz

#### Source boundary and age

- Committed Peachfuzz HEAD:
  `2b2d080c455edaadf88502c1c253845605a4336a`.
- The only dirty source is `.ledger_pending.md`; production and tests match
  HEAD.
- Primitive pin:
  `v2026.0.1-0.20260723215117-3f74d8fc35b4`
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5`).
- The local checksum and `go.sum` disagree, so the pin cannot currently be
  authenticated and `go list ./...` fails before compilation. This is a
  provenance blocker, not a temporal design fact.
- `protocol/` is a preserved, non-live extraction and explicitly not an
  approved API (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/FOUNDATION_EXTRACTION.md:9`).
- Live Peachfuzz has no direct `temporal` or `timeproof` integration.

#### Distinct production requirements

| Concern | Peachfuzz evidence | Boundary lesson |
| --- | --- | --- |
| Wall plus monotonic interval | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/timing/timing.go:24` | Monotonic elapsed is authoritative; persisted End is projected from Start plus elapsed; validation requires exact agreement. This belongs directly in `temporal`. |
| Causal deadline classification | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/context_contracts.go:5`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/runner/run.go:85`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/reproduce/reproduce.go:90` | A wall timeout is reported only when the child observed cancellation, its wall context is deadline-exceeded, and the parent was not canceled. Cause/state observation belongs `contextstate`/`temporal`; fuzz outcome policy remains Peachfuzz-owned. |
| Pure scheduling decision | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/scheduler/decide.go:62`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/scheduler/schedule.go:44`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/scheduler/update.go:31` | Scheduler accepts `Now` as input, clamps rollback age, saturates quarantine arithmetic, and clamps exponential doubling before overflow. Temporal owns safe arithmetic/rollback facts; target selection and cadence are Peachfuzz policy. |
| Process escalation | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/process/spec.go:1`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/process/run.go:127` | Cancellation sends SIGINT, waits a grace duration, then kills the owned process group; pipe teardown has a separate bound. Timer mechanics are temporal; process ownership is not. |
| Accepted but unresolved persistence | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/sched_persistence.go:21`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/sched_persistence.go:89` | Expired waits on durability-required mutations produce a typed stalled/unknown terminal outcome rather than an ordinary timeout. Generic deadline/result facts belong temporal/filestore; sequence-reuse policy remains Peachfuzz. |
| Finding reclamation | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/finding/finding.go:234`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/lifecycle.go:327`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/finding/store.go:410`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/store/store.go:292` | Reclamation requires remote proof, two UTC civil midnights, all shared owners, and a local quiet period. Calendar-day arithmetic is temporal; object/filestore proof belongs their owners; retention values remain Peachfuzz. |
| Token refresh | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/googleauth/identity.go:44`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/googleauth/identity.go:151`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/googleauth/identity.go:264` | Refresh is single-flight with leader/waiter/cached outcomes; a canceled leader does not permanently poison an unaffected waiter. Workload identity owns concurrency and cache policy; temporal supplies TTL/expiry facts. |
| Wide CPU effort | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/process/run.go:179`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/effort.go:14` | Real child CPU effort is distinct from wall time and can exceed `time.Duration`; an allocation-free unsigned-128 accumulator preserves it. This belongs measurement/accounting or aggregate duration, not ordinary bounded duration. |

#### Peachfuzz gems

- A validated interval whose persisted wall end is derived from monotonic
  elapsed rather than independently observed.
- Causal deadline classification that distinguishes parent cancellation,
  owned wall deadline, and whether the child actually observed cancellation.
- Separate bounded wall duration from wide cumulative CPU effort.
- Treat UTC calendar deadlines as calendar algebra, not fixed elapsed
  durations.
- Carry typed unresolved durability when a wait expires after a mutation may
  already have been accepted.

#### Hostile proof and gaps

Strong evidence includes:

- interval agreement at unset, zero, one nanosecond, maximum, backward, and
  plus-or-minus-one disagreement
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/timing/timing_test.go:14`);
- scheduler epoch/maximum inputs, exact quarantine release, backoff overflow,
  vruntime saturation, and threshold edges
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/scheduler/scheduler_test.go:57`);
- unsigned-128 carry, maximum, noncanonical input, overflow, and 100-million
  core-year effort (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/effort_test.go:11`);
- finding reclamation one nanosecond before and exactly at the calendar
  boundary (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/finding/finding_test.go:934`);
- causal context classification
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/context_contracts_test.go:11`); and
- strict duration ingress followed immediately by typed validation
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/config/schema.go:180`,
  `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/config/load_fuzz_test.go:57`).

Weaknesses:

- the claimed monotonic interval test uses two `time.Unix` values and therefore
  contains no monotonic reading;
- sequence clocks repeat their terminal value and hide extra observations;
- zero informally means never-run in scheduler state;
- failure-attempt increment can overflow;
- real timeouts and repeated `runtime.Gosched` guesses stand in for
  deterministic single-flight enrollment;
- no `testing/synctest`, temporal fuzz target, or leak harness exists; and
- timer/ticker lifecycle proof lacks exact stop/drain and boundary cases.

#### Debt and policy boundary

Peachfuzz retains fuzz/minimize/discovery budgets, scheduling weights,
quarantine thresholds, archive/report/publish cadence, shutdown values,
retention policy, cache TTLs, and human rendering.

Debt to retire includes raw nanosecond state, inconsistent rollback treatment,
age-only Git lock deletion, hidden mtime-as-custody signaling, repeated clock
sampling within one reclamation decision, zero report timestamps, and local GCS
retry logic that belongs `exchange`.

Peachfuzz adds direct evidence that primitive temporal must preserve a wall
instant and monotonic elapsed fact together. It also strengthens adjacent
contracts for `contextstate`, `filestore`, `objectstore`, `process`,
`workloadidentity`, and aggregate measurement.

## Strong mechanics and proof

### Archived Primitive `temporal`

#### Source boundary

- Immutable archive:
  `d046f7b675fcb797398d7cdc87b5504f43978056`.
- Package paths: `temporal/*.go` and `temporal/SPEC.md`.
- The archive's untracked hostile Core test is outside `temporal`; no archived
  temporal source differs from the immutable commit.
- Temporal first appeared after the Primitive revisions pinned by Witness and
  Bug. Kernel's dirty July 27 pin contains the same temporal production, tests,
  and specification as the archive.

#### Strong archived mechanics

The archived package is materially stronger than the older consumer wrappers:

- `Instant` has private representation, distinguishes unset from Unix epoch,
  validates all public operations, performs checked add/subtract, and uses
  exact canonical string JSON
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/instant.go:11`);
- non-negative `Duration` uses private representation and checked arithmetic
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/duration.go:12`);
- `AggregateDuration` implements bounded unsigned-128 arithmetic with
  `math/bits` and canonical decimal projection
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/aggregate.go:10`);
- `Observation` retains Go's wall and monotonic readings, and `Interval`
  derives its persisted End from Start plus monotonic elapsed rather than an
  independently shifted wall end
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/observation.go:9`,
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/public.go:70`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/interval.go:13`);
- context cancellation is classified through the hostile-safe
  `contextstate` boundary
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/context.go:19`);
- cancellation-aware wait and ticker operations define cancellation/stopped
  precedence and own cleanup
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/wait.go:20`,
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/ticker.go:55`);
- exact nanoseconds remain authoritative while microseconds are a validated
  query projection with canonical UTC and no monotonic payload
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/persistence.go:9`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/persistence.go:131`);
- humanization is bounded, deterministic, integer-only, and shared across
  ordinary and aggregate duration
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/humanize.go:35`);
- public exports, production imports, and every production struct are closed
  by ratchets
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/public_api_test.go:18`,
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/contract_inventory_test.go:15`); and
- stable temporal identity composes through `errors.Is`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/errors.go:10`,
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:171`).

Production is constant-space, starts no goroutines, satisfies `gocyclo <= 10`,
and passes package tests, race, vet, static analysis, security analysis, and
Linux/Windows/Darwin cross-builds. Cross-builds are not native runtime proof.

## Defects and blockers

### Archived defects and missing proof

1. **Operating-clock tests violate the written testing contract.**
   `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/wait_ticker_hostile_test.go:451` uses live observation, waits,
   tickers, deadlines, and wall assertions under parallel tests despite
   `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/SPEC.md:484` prohibiting live-clock proof.
2. **Test goroutine ownership is incomplete.**
   The live Wait proof uses `context.Background` without a cancellation path,
   and ticker proof does not wait for goroutine exit
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/wait_ticker_hostile_test.go:469`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/wait_ticker_hostile_test.go:511`).
3. **The monotonic guarantee is not behaviorally proved.**
   Injected observations use wall-only `time.Unix` values, while the structural
   ratchet proves routing but not that `Observation.Since` preserves
   `time.Time.Sub` semantics
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/observation_hostile_test.go:13`,
   `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/interval_hostile_test.go:104`).
4. **Wait cleanup proof is incomplete.**
   Exactly-once cleanup is not established for readiness, construction
   cancellation, and blocked cancellation
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/wait_ticker_hostile_test.go:31`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/wait_ticker_hostile_test.go:336`).
5. **Unexpected ticker-source closure has inconsistent state.**
   `Ticker.Next` reports `ErrTemporalTickerStopped` on a closed source but does
   not transition `Ticker.Validate` to stopped
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/ticker.go:73`). Existing proof checks only returned identity.
6. **The ticker stop-channel capacity is a false contract.**
   `core.TemporalTickerStopSignalCapacity` is used for a close-only channel;
   nothing is sent, so buffering expresses no semantic requirement
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/temporal_constants.go:43`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/ticker.go:20`).
7. **Aggregate comparison misses the reason for its width.**
   Tests construct only high-limb-zero aggregates and do not exercise
   cross-limb ordering
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/aggregate.go:103`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/value_test.go:160`).
8. **Observation close near maximum is under-proved.**
   No hostile case proves Start plus monotonic elapsed overflow through the
   observation path.
9. **Persistence lacks a direct monotonic-bearing rejection case.**
   Structural validation should reject it, but current tests cover location,
   precision, and negative cases only
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/persistence_test.go:60`).
10. **Temporal-local vocabulary is over-centralized in Core.**
    Stable shared error identities belong in Core. Temporal-only wire tokens,
    humanization abbreviations, unit divisors, decimal parameters, and ticker
    implementation details do not
    (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/temporal_constants.go:8`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_constants.go:220`).
11. **Unit magnitudes have parallel encodings.**
    Constructors use standard-library duration constants, humanization uses
    Core divisors, and tests repeat local literal divisors
    (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/public.go:121`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/humanize.go:283`,
    `archive@d046f7b675fcb797398d7cdc87b5504f43978056:temporal/humanize_test.go:13`).

### Consumer requirements absent from archived `temporal`

- strict, call-counted scripted observations that fail when exhausted;
- compiler-visible rollback disposition where a generic decision genuinely
  exists;
- calendar periods with month-end and leap-year behavior;
- a direct wall-versus-monotonic skew observation useful to host-suspend
  detection; and
- wider exact elapsed/effort aggregation proof across high limbs.

Signed trusted time, durable high-water, lease progression, RFC 3161
verification, process escalation, object reclamation, cache policy, and
consumer deadline blame do not belong in primitive temporal. They belong in
`timeproof`, `lease`, the owning effect package, or the consumer.

## Primitive 2026 ownership and DAG

The archive already contains the strongest coherent primitive implementation.
The new package should begin from its semantics, not from the earlier
`UnixNanoTime`/`NanosecondsDuration` shapes.

Evidence supports this ownership conclusion:

- `temporal`: exact instants, bounded durations, wide aggregate duration,
  wall-plus-monotonic observation, intervals, checked arithmetic, ordering,
  canonical machine projections, deadline/wait/ticker mechanisms, and narrowly
  generic rollback/calendar facts only if later architecture proves one owner;
- `core`: stable shared temporal error identities and genuinely cross-package
  protocol constants only;
- `contextstate`: hostile-safe context observation and causal state;
- `callbudget`: call and admission quota facts, durable consumption,
  reconciliation, and rollback or conflict decisions. It does not own elapsed
  transport budgets;
- `exchange`: attempt limits, retries, total operation timeout, retryability,
  `Retry-After`, backoff, jitter, and HTTP attempt execution;
- `timeproof`: authoritative time acquisition and verification;
- `lease` or consumer state: signed progression, durable floors, authenticated
  ceilings, commercial windows, and action policy; and
- consumer packages: exact TTLs, cadence, display, deadline blame, domain
  transitions, and operational choices.

The archived code is a strong reuse candidate after the new architecture is
reviewed, but it is not ready to copy unchanged. Ticker lifecycle, live-clock
proof, goroutine ownership, monotonic regression proof, cleanup proof,
cross-limb aggregate coverage, Core ownership, and duplicated units must be
resolved under the new specification and testing protocol.

## Decision rationale and conditions

The chosen decision preserves the coherent primitive capability while rejecting archive
ownership drift and incomplete proof. Admission requires Temporal-only
vocabulary to leave Core, duplicated unit magnitudes to collapse into one
owner, ticker and goroutine lifecycle to close, observation overflow and
monotonic regression to gain hostile proof, and every persistence boundary to
reject monotonic-bearing values directly.

The later design must preserve the resolved package split: Exchange owns
attempt limits, retries, and total operation timeout; Callbudget owns call and
admission quota facts. Consumer packages retain exact TTLs, cadence, display,
deadline blame, and domain transition policy. No package specification,
production code, or migration is authorized by this recon decision.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
